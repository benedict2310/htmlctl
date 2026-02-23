package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	"github.com/benedict2310/htmlctl/internal/release"
)

type promoteRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type promoteResponse struct {
	Website         string `json:"website"`
	FromEnvironment string `json:"fromEnvironment"`
	ToEnvironment   string `json:"toEnvironment"`
	SourceReleaseID string `json:"sourceReleaseId"`
	ReleaseID       string `json:"releaseId"`
	FileCount       int    `json:"fileCount"`
	Hash            string `json:"hash"`
	HashVerified    bool   `json:"hashVerified"`
	Strategy        string `json:"strategy"`
}

func (s *Server) handlePromote(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, ok, err := parsePromotePath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	reqBody := promoteRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}
	sourceEnv := strings.TrimSpace(reqBody.From)
	targetEnv := strings.TrimSpace(reqBody.To)
	if sourceEnv == "" || targetEnv == "" {
		writeAPIError(w, http.StatusBadRequest, "both from and to environments are required", nil)
		return
	}
	if err := validateResourceName(sourceEnv); err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("invalid source environment name %q: %v", sourceEnv, err), nil)
		return
	}
	if err := validateResourceName(targetEnv); err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("invalid target environment name %q: %v", targetEnv, err), nil)
		return
	}
	if sourceEnv == targetEnv {
		writeAPIError(w, http.StatusBadRequest, "from and to environments must be different", nil)
		return
	}

	unlock := s.lockEnvironmentPair(website, sourceEnv, targetEnv)
	defer unlock()

	result, err := release.Promote(r.Context(), s.db, s.dataPaths.WebsitesRoot, website, sourceEnv, targetEnv)
	if err != nil {
		var notFoundErr *release.NotFoundError
		var hashMismatchErr *release.HashMismatchError
		switch {
		case errors.As(err, &notFoundErr):
			writeAPIError(w, http.StatusNotFound, notFoundErr.Error(), nil)
		case errors.Is(err, release.ErrPromotionSourceNoActive):
			writeAPIError(w, http.StatusConflict, "source environment has no active release to promote", nil)
		case errors.Is(err, release.ErrPromotionSourceTargetMatch):
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
		case errors.As(err, &hashMismatchErr):
			s.writeInternalAPIError(w, r, "promotion hash verification failed", hashMismatchErr, "website", website, "from", sourceEnv, "to", targetEnv)
		default:
			s.writeInternalAPIError(w, r, "promotion failed", err, "website", website, "from", sourceEnv, "to", targetEnv)
		}
		return
	}

	if s.auditLogger != nil {
		targetReleaseID := result.ReleaseID
		if err := s.auditLogger.Log(r.Context(), audit.Entry{
			Actor:           actorFromRequest(r),
			EnvironmentID:   &result.TargetEnvironmentID,
			Operation:       audit.OperationPromote,
			ResourceSummary: fmt.Sprintf("promoted release %s from %s to %s as %s", result.SourceReleaseID, sourceEnv, targetEnv, result.ReleaseID),
			ReleaseID:       &targetReleaseID,
			Metadata: map[string]any{
				"fromEnvironment": sourceEnv,
				"toEnvironment":   targetEnv,
				"sourceReleaseId": result.SourceReleaseID,
				"releaseId":       result.ReleaseID,
				"fileCount":       result.FileCount,
				"hash":            result.Hash,
				"strategy":        result.Strategy,
			},
		}); err != nil {
			s.logger.Error("failed to write promote audit entry", "website", website, "from", sourceEnv, "to", targetEnv, "error", err)
		}
		if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
			waitCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
			if err := flusher.WaitIdle(waitCtx); err != nil {
				s.logger.Warn("timed out waiting for async promote audit flush", "website", website, "from", sourceEnv, "to", targetEnv, "error", err)
			}
			cancel()
		}
	}

	writeJSON(w, http.StatusOK, promoteResponse{
		Website:         website,
		FromEnvironment: sourceEnv,
		ToEnvironment:   targetEnv,
		SourceReleaseID: result.SourceReleaseID,
		ReleaseID:       result.ReleaseID,
		FileCount:       result.FileCount,
		Hash:            result.Hash,
		HashVerified:    true,
		Strategy:        result.Strategy,
	})
}

func parsePromotePath(pathValue string) (website string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 5 {
		return "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "promote" {
		return "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	if website == "" {
		return "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	return website, true, nil
}
