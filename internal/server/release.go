package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"github.com/benedict2310/htmlctl/internal/release"
)

type releaseResponse struct {
	Website           string  `json:"website"`
	Environment       string  `json:"environment"`
	ReleaseID         string  `json:"releaseId"`
	PreviousReleaseID *string `json:"previousReleaseId,omitempty"`
	Status            string  `json:"status"`
}

type releasesResponse struct {
	Website         string            `json:"website"`
	Environment     string            `json:"environment"`
	ActiveReleaseID *string           `json:"activeReleaseId,omitempty"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
	Releases        []releaseListItem `json:"releases"`
}

type releaseListItem struct {
	ReleaseID string `json:"releaseId"`
	Actor     string `json:"actor"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	Active    bool   `json:"active"`
}

const (
	defaultReleaseHistoryLimit = 20
	maxReleaseHistoryLimit     = 200
)

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseReleasePath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		s.handleListReleases(w, r, website, env)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil || s.blobStore == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	builder, err := release.NewBuilder(s.db, s.blobStore, s.dataPaths.WebsitesRoot, s.logger)
	if err != nil {
		s.writeInternalAPIError(w, r, "failed to initialize release builder", err, "website", website, "environment", env)
		return
	}
	result, err := builder.Build(r.Context(), website, env)
	if err != nil {
		var notFoundErr *release.NotFoundError
		if errors.As(err, &notFoundErr) {
			writeAPIError(w, http.StatusNotFound, notFoundErr.Error(), nil)
			return
		}
		s.writeInternalAPIError(w, r, "release build failed", err, "website", website, "environment", env)
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

func (s *Server) handleListReleases(w http.ResponseWriter, r *http.Request, website, env string) {
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}
	limit, offset, err := parseListReleasesPagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{err.Error()})
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup website failed", err, "website", website, "environment", env)
		return
	}
	envRow, err := q.GetEnvironmentByName(r.Context(), websiteRow.ID, env)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", env), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup environment failed", err, "website", website, "environment", env)
		return
	}
	releases, err := q.ListReleasesByEnvironmentPage(r.Context(), envRow.ID, limit, offset)
	if err != nil {
		s.writeInternalAPIError(w, r, "list releases failed", err, "website", website, "environment", env, "limit", limit, "offset", offset)
		return
	}

	releaseIDs := make([]string, 0, len(releases))
	for _, rel := range releases {
		releaseIDs = append(releaseIDs, rel.ID)
	}
	actorByReleaseID := map[string]string{}
	actors, err := q.ListLatestReleaseActors(r.Context(), envRow.ID, releaseIDs)
	if err != nil {
		s.logger.Warn("release history actor lookup failed; using unknown actors", "website", websiteRow.Name, "environment", envRow.Name, "error", err)
	} else {
		actorByReleaseID = actors
	}

	items := make([]releaseListItem, 0, len(releases))
	for _, rel := range releases {
		active := envRow.ActiveReleaseID != nil && *envRow.ActiveReleaseID == rel.ID
		status := strings.TrimSpace(rel.Status)
		switch {
		case active:
			status = "active"
		case status == "" || status == "active":
			status = "previous"
		}
		actor := "unknown"
		auditActor := strings.TrimSpace(actorByReleaseID[rel.ID])
		if auditActor != "" {
			actor = auditActor
		}
		items = append(items, releaseListItem{
			ReleaseID: rel.ID,
			Actor:     actor,
			Status:    status,
			CreatedAt: rel.CreatedAt,
			Active:    active,
		})
	}
	writeJSON(w, http.StatusOK, releasesResponse{
		Website:         websiteRow.Name,
		Environment:     envRow.Name,
		ActiveReleaseID: envRow.ActiveReleaseID,
		Limit:           limit,
		Offset:          offset,
		Releases:        items,
	})
}

func parseListReleasesPagination(r *http.Request) (int, int, error) {
	limit := defaultReleaseHistoryLimit
	offset := 0

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit: %w", err)
		}
		if v < 0 {
			return 0, 0, fmt.Errorf("limit must be >= 0")
		}
		limit = v
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid offset: %w", err)
		}
		if v < 0 {
			return 0, 0, fmt.Errorf("offset must be >= 0")
		}
		offset = v
	}
	if limit > maxReleaseHistoryLimit {
		limit = maxReleaseHistoryLimit
	}
	return limit, offset, nil
}

func parseReleasePath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "releases" {
		return "", "", false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
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
