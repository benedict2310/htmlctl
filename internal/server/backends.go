package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	backendpkg "github.com/benedict2310/htmlctl/internal/backend"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

type backendRequest struct {
	PathPrefix string `json:"pathPrefix"`
	Upstream   string `json:"upstream"`
}

type backendResponse struct {
	ID          int64  `json:"id"`
	PathPrefix  string `json:"pathPrefix"`
	Upstream    string `json:"upstream"`
	Website     string `json:"website"`
	Environment string `json:"environment"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type backendsResponse struct {
	Website     string            `json:"website"`
	Environment string            `json:"environment"`
	Backends    []backendResponse `json:"backends"`
}

func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseBackendsPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleListBackends(w, r, website, env)
	case http.MethodPost:
		s.handleAddBackend(w, r, website, env)
	case http.MethodDelete:
		s.handleRemoveBackend(w, r, website, env)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost+", "+http.MethodDelete)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (s *Server) handleListBackends(w http.ResponseWriter, r *http.Request, website, env string) {
	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}
	rows, err := q.ListBackendsByEnvironment(r.Context(), envRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list backends failed", err, "website", website, "environment", env)
		return
	}

	items := make([]backendResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapBackendRow(row, website, env))
	}
	writeJSON(w, http.StatusOK, backendsResponse{
		Website:     website,
		Environment: env,
		Backends:    items,
	})
}

func (s *Server) handleAddBackend(w http.ResponseWriter, r *http.Request, website, env string) {
	var req backendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}

	pathPrefix, err := backendpkg.ValidatePathPrefix(req.PathPrefix)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid backend pathPrefix", []string{err.Error()})
		return
	}
	upstream, err := backendpkg.ValidateUpstreamURL(req.Upstream)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid backend upstream", []string{err.Error()})
		return
	}

	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	existing, err := q.GetBackendByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	created := false
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		created = true
	default:
		s.writeInternalAPIError(w, r, "get backend failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	if err := q.UpsertBackend(r.Context(), dbpkg.BackendRow{
		EnvironmentID: envRow.ID,
		PathPrefix:    pathPrefix,
		Upstream:      upstream,
	}); err != nil {
		s.writeInternalAPIError(w, r, "upsert backend failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	row, err := q.GetBackendByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		s.writeInternalAPIError(w, r, "load backend failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}
	if !created && existing.Upstream == row.Upstream && existing.UpdatedAt == row.UpdatedAt {
		created = false
	}

	if s.caddyReloader != nil {
		reason := "backend.add " + website + "/" + env + " " + pathPrefix
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after backend add", "website", website, "environment", env, "path_prefix", pathPrefix, "reason", reason, "error", err)
		}
	}

	s.logBackendAudit(r.Context(), actorFromRequest(r), audit.OperationBackendAdd, envRow.ID, website, env, row.PathPrefix, row.Upstream)

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapBackendRow(row, website, env))
}

func (s *Server) handleRemoveBackend(w http.ResponseWriter, r *http.Request, website, env string) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{err.Error()})
		return
	}
	rawPathPrefix := strings.TrimSpace(values.Get("path"))
	if rawPathPrefix == "" {
		writeAPIError(w, http.StatusBadRequest, "path query parameter is required", nil)
		return
	}
	pathPrefix, err := backendpkg.ValidatePathPrefix(rawPathPrefix)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid backend path", []string{err.Error()})
		return
	}

	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	row, err := q.GetBackendByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("backend %q not found", pathPrefix), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get backend failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	deleted, err := q.DeleteBackendByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		s.writeInternalAPIError(w, r, "delete backend failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("backend %q not found", pathPrefix), nil)
		return
	}

	if s.caddyReloader != nil {
		reason := "backend.remove " + website + "/" + env + " " + pathPrefix
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after backend remove", "website", website, "environment", env, "path_prefix", pathPrefix, "reason", reason, "error", err)
		}
	}

	s.logBackendAudit(r.Context(), actorFromRequest(r), audit.OperationBackendRemove, envRow.ID, website, env, row.PathPrefix, "")
	w.WriteHeader(http.StatusNoContent)
}

func lookupEnvironmentRow(ctx context.Context, q *dbpkg.Queries, website, env string) (dbpkg.EnvironmentRow, error) {
	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		return dbpkg.EnvironmentRow{}, err
	}
	return q.GetEnvironmentByName(ctx, websiteRow.ID, env)
}

func (s *Server) handleEnvironmentLookupError(w http.ResponseWriter, r *http.Request, website, env string, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, websiteErr := dbpkg.NewQueries(s.db).GetWebsiteByName(r.Context(), website); errors.Is(websiteErr, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", env), nil)
	default:
		s.writeInternalAPIError(w, r, "lookup environment failed", err, "website", website, "environment", env)
	}
}

func (s *Server) logBackendAudit(ctx context.Context, actor, operation string, environmentID int64, website, environment, pathPrefix, upstream string) {
	if s.auditLogger == nil {
		return
	}
	metadata := map[string]any{
		"pathPrefix":  pathPrefix,
		"website":     website,
		"environment": environment,
	}
	if upstream != "" {
		metadata["upstream"] = upstream
	}
	summary := fmt.Sprintf("%s %s for %s/%s", operation, pathPrefix, website, environment)
	if upstream != "" {
		summary = fmt.Sprintf("%s -> %s", summary, upstream)
	}
	if err := s.auditLogger.Log(ctx, audit.Entry{
		Actor:           actor,
		EnvironmentID:   &environmentID,
		Operation:       operation,
		ResourceSummary: summary,
		Metadata:        metadata,
	}); err != nil {
		s.logger.Error("failed to write backend audit entry", "operation", operation, "path_prefix", pathPrefix, "error", err)
		return
	}
	if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
		waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		if err := flusher.WaitIdle(waitCtx); err != nil {
			s.logger.Warn("timed out waiting for async backend audit flush", "operation", operation, "path_prefix", pathPrefix, "error", err)
		}
		cancel()
	}
}

func mapBackendRow(row dbpkg.BackendRow, website, environment string) backendResponse {
	return backendResponse{
		ID:          row.ID,
		PathPrefix:  row.PathPrefix,
		Upstream:    row.Upstream,
		Website:     website,
		Environment: environment,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func parseBackendsPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "backends" {
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
