package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	"github.com/benedict2310/htmlctl/internal/release"
)

type rollbackResponse struct {
	Website       string `json:"website"`
	Environment   string `json:"environment"`
	FromReleaseID string `json:"fromReleaseId"`
	ToReleaseID   string `json:"toReleaseId"`
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseRollbackPath(pathValue)
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

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	result, err := release.Rollback(r.Context(), s.db, s.dataPaths.WebsitesRoot, website, env)
	if err != nil {
		var notFoundErr *release.NotFoundError
		var missingDirErr *release.MissingReleaseDirError
		switch {
		case errors.As(err, &notFoundErr):
			writeAPIError(w, http.StatusNotFound, notFoundErr.Error(), nil)
		case errors.Is(err, release.ErrNoPreviousRelease):
			writeAPIError(w, http.StatusConflict, "rollback is not possible because no previous release exists", nil)
		case errors.As(err, &missingDirErr):
			s.logger.ErrorContext(r.Context(), "rollback target release directory is missing", "error", missingDirErr, "website", website, "environment", env)
			writeAPIError(w, http.StatusConflict, "rollback target release directory is missing", nil)
		default:
			s.writeInternalAPIError(w, r, "rollback failed", err, "website", website, "environment", env)
		}
		return
	}

	if s.auditLogger != nil {
		targetReleaseID := result.ToReleaseID
		if err := s.auditLogger.Log(r.Context(), audit.Entry{
			Actor:           actorFromRequest(r),
			EnvironmentID:   &result.EnvironmentID,
			Operation:       audit.OperationRollback,
			ResourceSummary: fmt.Sprintf("rolled back release %s -> %s", result.FromReleaseID, result.ToReleaseID),
			ReleaseID:       &targetReleaseID,
			Metadata: map[string]any{
				"fromReleaseId": result.FromReleaseID,
				"toReleaseId":   result.ToReleaseID,
			},
		}); err != nil {
			s.logger.Error("failed to write rollback audit entry", "website", website, "environment", env, "error", err)
		}

		if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
			waitCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
			if err := flusher.WaitIdle(waitCtx); err != nil {
				s.logger.Warn("timed out waiting for async rollback audit flush", "website", website, "environment", env, "error", err)
			}
			cancel()
		}
	}

	writeJSON(w, http.StatusOK, rollbackResponse{
		Website:       website,
		Environment:   env,
		FromReleaseID: result.FromReleaseID,
		ToReleaseID:   result.ToReleaseID,
	})
}

func parseRollbackPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "rollback" {
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
