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

type retentionRunRequest struct {
	Keep   *int `json:"keep"`
	DryRun bool `json:"dryRun"`
	BlobGC bool `json:"blobGC"`
}

type retentionResponse struct {
	Website                 string   `json:"website"`
	Environment             string   `json:"environment"`
	Keep                    int      `json:"keep"`
	DryRun                  bool     `json:"dryRun"`
	BlobGC                  bool     `json:"blobGC"`
	ActiveReleaseID         *string  `json:"activeReleaseId,omitempty"`
	RollbackReleaseID       *string  `json:"rollbackReleaseId,omitempty"`
	PreviewPinnedReleaseIDs []string `json:"previewPinnedReleaseIds"`
	RetainedReleaseIDs      []string `json:"retainedReleaseIds"`
	PrunableReleaseIDs      []string `json:"prunableReleaseIds"`
	PrunedReleaseIDs        []string `json:"prunedReleaseIds"`
	MarkedBlobCount         int      `json:"markedBlobCount"`
	BlobDeleteCandidates    []string `json:"blobDeleteCandidates"`
	DeletedBlobHashes       []string `json:"deletedBlobHashes"`
	Warnings                []string `json:"warnings,omitempty"`
}

func (s *Server) handleRetention(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseRetentionRunPath(pathValue)
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

	var req retentionRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}
	if req.Keep == nil {
		writeAPIError(w, http.StatusBadRequest, "keep is required", nil)
		return
	}
	if *req.Keep < 0 {
		writeAPIError(w, http.StatusBadRequest, "keep must be >= 0", nil)
		return
	}
	if req.BlobGC && s.blobStore == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	result, err := release.RunRetention(r.Context(), s.db, s.blobStore, s.dataPaths.WebsitesRoot, website, env, release.RetentionOptions{
		Keep:   *req.Keep,
		DryRun: req.DryRun,
		BlobGC: req.BlobGC,
	})
	if err != nil {
		var notFoundErr *release.NotFoundError
		if errors.As(err, &notFoundErr) {
			writeAPIError(w, http.StatusNotFound, notFoundErr.Error(), nil)
			return
		}
		s.writeInternalAPIError(w, r, "retention run failed", err, "website", website, "environment", env, "keep", *req.Keep, "dry_run", req.DryRun, "blob_gc", req.BlobGC)
		return
	}

	s.logRetentionAudit(r.Context(), actorFromRequest(r), result)
	writeJSON(w, http.StatusOK, mapRetentionResult(result))
}

func parseRetentionRunPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 8 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "retention" || parts[7] != "run" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if website == "" || env == "" {
		return "", "", false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	return website, env, true, nil
}

func mapRetentionResult(result release.RetentionResult) retentionResponse {
	return retentionResponse{
		Website:                 result.Website,
		Environment:             result.Environment,
		Keep:                    result.Keep,
		DryRun:                  result.DryRun,
		BlobGC:                  result.BlobGC,
		ActiveReleaseID:         result.ActiveReleaseID,
		RollbackReleaseID:       result.RollbackReleaseID,
		PreviewPinnedReleaseIDs: append([]string(nil), result.PreviewPinnedReleaseIDs...),
		RetainedReleaseIDs:      append([]string(nil), result.RetainedReleaseIDs...),
		PrunableReleaseIDs:      append([]string(nil), result.PrunableReleaseIDs...),
		PrunedReleaseIDs:        append([]string(nil), result.PrunedReleaseIDs...),
		MarkedBlobCount:         result.MarkedBlobCount,
		BlobDeleteCandidates:    append([]string(nil), result.BlobDeleteCandidates...),
		DeletedBlobHashes:       append([]string(nil), result.DeletedBlobHashes...),
		Warnings:                append([]string(nil), result.Warnings...),
	}
}

func (s *Server) logRetentionAudit(ctx context.Context, actor string, result release.RetentionResult) {
	if s.auditLogger == nil {
		return
	}
	metadata := map[string]any{
		"website":                 result.Website,
		"environment":             result.Environment,
		"keep":                    result.Keep,
		"dryRun":                  result.DryRun,
		"blobGC":                  result.BlobGC,
		"retainedReleaseIds":      append([]string(nil), result.RetainedReleaseIDs...),
		"prunableReleaseIds":      append([]string(nil), result.PrunableReleaseIDs...),
		"prunedReleaseIds":        append([]string(nil), result.PrunedReleaseIDs...),
		"markedBlobCount":         result.MarkedBlobCount,
		"blobDeleteCandidates":    append([]string(nil), result.BlobDeleteCandidates...),
		"deletedBlobHashes":       append([]string(nil), result.DeletedBlobHashes...),
		"previewPinnedReleaseIds": append([]string(nil), result.PreviewPinnedReleaseIDs...),
	}
	if result.ActiveReleaseID != nil {
		metadata["activeReleaseId"] = *result.ActiveReleaseID
	}
	if result.RollbackReleaseID != nil {
		metadata["rollbackReleaseId"] = *result.RollbackReleaseID
	}
	if len(result.Warnings) > 0 {
		metadata["warnings"] = append([]string(nil), result.Warnings...)
	}

	summary := fmt.Sprintf("retention run for %s/%s pruned=%d dryRun=%t blobGC=%t", result.Website, result.Environment, len(result.PrunedReleaseIDs), result.DryRun, result.BlobGC)
	if err := s.auditLogger.Log(ctx, audit.Entry{
		Actor:           actor,
		EnvironmentID:   &result.EnvironmentID,
		Operation:       audit.OperationRetentionRun,
		ResourceSummary: summary,
		Metadata:        metadata,
	}); err != nil {
		s.logger.Error("failed to write retention audit entry", "website", result.Website, "environment", result.Environment, "error", err)
		return
	}
	if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
		waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		if err := flusher.WaitIdle(waitCtx); err != nil {
			s.logger.Warn("timed out waiting for async retention audit flush", "website", result.Website, "environment", result.Environment, "error", err)
		}
		cancel()
	}
}
