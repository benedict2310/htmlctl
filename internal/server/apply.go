package server

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	"github.com/benedict2310/htmlctl/internal/bundle"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/state"
)

const maxApplyBundleBytes = 50 * 1024 * 1024

type applyResponse struct {
	Website          string                   `json:"website"`
	Environment      string                   `json:"environment"`
	Mode             string                   `json:"mode"`
	DryRun           bool                     `json:"dryRun"`
	AcceptedResource []state.AcceptedResource `json:"acceptedResources"`
	Warnings         []string                 `json:"warnings,omitempty"`
	Changes          state.ChangeSummary      `json:"changes"`
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	website, env, ok := parseApplyPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil || s.blobStore == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	dryRun := false
	if raw := strings.TrimSpace(r.URL.Query().Get("dry_run")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid dry_run query parameter", []string{err.Error()})
			return
		}
		dryRun = parsed
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxApplyBundleBytes)
	defer r.Body.Close()

	b, err := bundle.ReadTar(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "bundle too large", []string{fmt.Sprintf("maximum allowed size is %d bytes", maxApplyBundleBytes)})
			return
		}
		var validationErr *bundle.ValidationError
		if errors.As(err, &validationErr) {
			details := append([]string{}, validationErr.MissingFiles...)
			details = append(details, validationErr.HashMismatches...)
			writeAPIError(w, http.StatusBadRequest, "bundle failed validation", details)
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid bundle", []string{err.Error()})
		return
	}
	if b.Manifest.Website != website {
		writeAPIError(w, http.StatusBadRequest, "manifest.website does not match URL website", []string{fmt.Sprintf("manifest=%s url=%s", b.Manifest.Website, website)})
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	applier, err := state.NewApplier(s.db, s.blobStore)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to initialize apply handler", []string{err.Error()})
		return
	}
	result, err := applier.Apply(r.Context(), website, env, b, dryRun)
	if err != nil {
		var badReqErr *state.BadRequestError
		if errors.As(err, &badReqErr) {
			writeAPIError(w, http.StatusBadRequest, badReqErr.Error(), nil)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "apply failed", []string{err.Error()})
		return
	}

	warnings := append([]string{}, result.Warnings...)
	if len(b.ExtraFiles) > 0 {
		warnings = append(warnings, fmt.Sprintf("ignored %d extra file(s)", len(b.ExtraFiles)))
	}
	if !dryRun && s.auditLogger != nil {
		q := dbpkg.NewQueries(s.db)
		websiteRow, err := q.GetWebsiteByName(r.Context(), website)
		if err == nil {
			envRow, envErr := q.GetEnvironmentByName(r.Context(), websiteRow.ID, env)
			if envErr == nil {
				metadata := map[string]any{
					"mode":          b.Manifest.Mode,
					"acceptedCount": len(result.Accepted),
					"changes":       result.Changes,
				}
				if err := s.auditLogger.Log(r.Context(), audit.Entry{
					Actor:           actorFromRequest(r),
					EnvironmentID:   &envRow.ID,
					Operation:       audit.OperationApply,
					ResourceSummary: summarizeAcceptedResources(result.Accepted),
					Metadata:        metadata,
				}); err != nil {
					s.logger.Error("failed to write apply audit entry", "website", website, "environment", env, "error", err)
				}
				if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
					waitCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
					if err := flusher.WaitIdle(waitCtx); err != nil {
						s.logger.Warn("timed out waiting for async apply audit flush", "website", website, "environment", env, "error", err)
					}
					cancel()
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, applyResponse{
		Website:          website,
		Environment:      env,
		Mode:             b.Manifest.Mode,
		DryRun:           dryRun,
		AcceptedResource: result.Accepted,
		Warnings:         warnings,
		Changes:          result.Changes,
	})
}

func parseApplyPath(pathValue string) (website string, env string, ok bool) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "apply" {
		return "", "", false
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false
	}
	return website, env, true
}

func writeAPIError(w http.ResponseWriter, status int, message string, details []string) {
	resp := map[string]any{"error": message}
	if len(details) > 0 {
		resp["details"] = details
	}
	writeJSON(w, status, resp)
}

func (s *Server) environmentLock(website, env string) *sync.Mutex {
	idx := s.environmentLockIndex(website, env)
	return &s.applyLockStripes[idx]
}

func (s *Server) domainLock(domain string) *sync.Mutex {
	idx := s.domainLockIndex(domain)
	return &s.domainLockStripes[idx]
}

func (s *Server) lockEnvironmentPair(website, envA, envB string) func() {
	idxA := s.environmentLockIndex(website, envA)
	idxB := s.environmentLockIndex(website, envB)

	if idxA == idxB {
		lock := &s.applyLockStripes[idxA]
		lock.Lock()
		return func() {
			lock.Unlock()
		}
	}

	firstIdx := idxA
	secondIdx := idxB
	if firstIdx > secondIdx {
		firstIdx, secondIdx = secondIdx, firstIdx
	}
	first := &s.applyLockStripes[firstIdx]
	second := &s.applyLockStripes[secondIdx]
	first.Lock()
	second.Lock()
	return func() {
		second.Unlock()
		first.Unlock()
	}
}

func (s *Server) environmentLockIndex(website, env string) uint32 {
	key := website + "/" + env
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % uint32(len(s.applyLockStripes))
}

func (s *Server) domainLockIndex(domain string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(domain))
	return h.Sum32() % uint32(len(s.domainLockStripes))
}

func summarizeAcceptedResources(accepted []state.AcceptedResource) string {
	if len(accepted) == 0 {
		return "no resources changed"
	}
	max := 5
	if len(accepted) < max {
		max = len(accepted)
	}
	items := make([]string, 0, max)
	for i := 0; i < max; i++ {
		items = append(items, fmt.Sprintf("%s %s", accepted[i].Kind, accepted[i].Name))
	}
	if len(accepted) > max {
		return fmt.Sprintf("%s (+%d more)", strings.Join(items, ", "), len(accepted)-max)
	}
	return strings.Join(items, ", ")
}
