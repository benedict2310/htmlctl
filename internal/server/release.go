package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	"github.com/benedict2310/htmlctl/internal/release"
)

type releaseResponse struct {
	Website           string  `json:"website"`
	Environment       string  `json:"environment"`
	ReleaseID         string  `json:"releaseId"`
	PreviousReleaseID *string `json:"previousReleaseId,omitempty"`
	Status            string  `json:"status"`
}

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	website, env, ok := parseReleasePath(r.URL.Path)
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

	lock := s.applyLock(website)
	lock.Lock()
	defer lock.Unlock()

	builder, err := release.NewBuilder(s.db, s.blobStore, s.dataPaths.WebsitesRoot, s.logger)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to initialize release builder", []string{err.Error()})
		return
	}
	result, err := builder.Build(r.Context(), website, env)
	if err != nil {
		var notFoundErr *release.NotFoundError
		if errors.As(err, &notFoundErr) {
			writeAPIError(w, http.StatusNotFound, notFoundErr.Error(), nil)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "release build failed", []string{err.Error()})
		return
	}

	if s.auditLogger != nil {
		releaseID := result.ReleaseID
		if err := s.auditLogger.Log(r.Context(), audit.Entry{
			Actor:           actorFromRequest(r),
			EnvironmentID:   &result.EnvironmentID,
			Operation:       audit.OperationReleaseBuild,
			ResourceSummary: fmt.Sprintf("built release %s", releaseID),
			ReleaseID:       &releaseID,
			Metadata: map[string]any{
				"status": "active",
			},
		}); err != nil {
			s.logger.Error("failed to write release.build audit entry", "release_id", releaseID, "error", err)
		}

		metadata := map[string]any{}
		if result.PreviousReleaseID != nil {
			metadata["previousReleaseId"] = *result.PreviousReleaseID
		}
		if err := s.auditLogger.Log(r.Context(), audit.Entry{
			Actor:           actorFromRequest(r),
			EnvironmentID:   &result.EnvironmentID,
			Operation:       audit.OperationReleaseActivate,
			ResourceSummary: fmt.Sprintf("activated release %s", releaseID),
			ReleaseID:       &releaseID,
			Metadata:        metadata,
		}); err != nil {
			s.logger.Error("failed to write release.activate audit entry", "release_id", releaseID, "error", err)
		}
		if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
			waitCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
			if err := flusher.WaitIdle(waitCtx); err != nil {
				s.logger.Warn("timed out waiting for async release audit flush", "release_id", releaseID, "error", err)
			}
			cancel()
		}
	}

	writeJSON(w, http.StatusCreated, releaseResponse{
		Website:           website,
		Environment:       env,
		ReleaseID:         result.ReleaseID,
		PreviousReleaseID: result.PreviousReleaseID,
		Status:            "active",
	})
}

func parseReleasePath(pathValue string) (website, env string, ok bool) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "releases" {
		return "", "", false
	}
	website, err := url.PathUnescape(parts[3])
	if err != nil {
		return "", "", false
	}
	env, err = url.PathUnescape(parts[5])
	if err != nil {
		return "", "", false
	}
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
		return "", "", false
	}
	return website, env, true
}
