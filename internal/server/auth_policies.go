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
	authpolicypkg "github.com/benedict2310/htmlctl/internal/authpolicy"
	backendpkg "github.com/benedict2310/htmlctl/internal/backend"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

type authPolicyRequest struct {
	PathPrefix   string `json:"pathPrefix"`
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}

type authPolicyResponse struct {
	ID          int64  `json:"id"`
	PathPrefix  string `json:"pathPrefix"`
	Username    string `json:"username"`
	Website     string `json:"website"`
	Environment string `json:"environment"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type authPoliciesResponse struct {
	Website      string               `json:"website"`
	Environment  string               `json:"environment"`
	AuthPolicies []authPolicyResponse `json:"authPolicies"`
}

func (s *Server) handleAuthPolicies(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseAuthPoliciesPath(pathValue)
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
		s.handleListAuthPolicies(w, r, website, env)
	case http.MethodPost:
		s.handleAddAuthPolicy(w, r, website, env)
	case http.MethodDelete:
		s.handleRemoveAuthPolicy(w, r, website, env)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost+", "+http.MethodDelete)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (s *Server) handleListAuthPolicies(w http.ResponseWriter, r *http.Request, website, env string) {
	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}
	rows, err := q.ListAuthPoliciesByEnvironment(r.Context(), envRow.ID)
	if err != nil {
		s.writeInternalAPIError(w, r, "list auth policies failed", err, "website", website, "environment", env)
		return
	}

	items := make([]authPolicyResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAuthPolicyRow(row, website, env))
	}
	writeJSON(w, http.StatusOK, authPoliciesResponse{
		Website:      website,
		Environment:  env,
		AuthPolicies: items,
	})
}

func (s *Server) handleAddAuthPolicy(w http.ResponseWriter, r *http.Request, website, env string) {
	var req authPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}

	pathPrefix, err := backendpkg.ValidatePathPrefix(req.PathPrefix)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy pathPrefix", []string{err.Error()})
		return
	}
	username, err := authpolicypkg.ValidateUsername(req.Username)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy username", []string{err.Error()})
		return
	}
	passwordHash, err := authpolicypkg.ValidatePasswordHash(req.PasswordHash)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy passwordHash", []string{err.Error()})
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

	existing, err := q.GetAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	created := false
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		created = true
	default:
		s.writeInternalAPIError(w, r, "get auth policy failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	if err := validateAuthPolicyPathOverlap(r.Context(), q, envRow.ID, pathPrefix); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy pathPrefix", []string{err.Error()})
		return
	}
	if s.cfg.Telemetry.Enabled && backendpkg.PathPrefixOverlapsPath(pathPrefix, "/collect/v1/events") {
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy pathPrefix", []string{fmt.Sprintf("path prefix %q overlaps reserved telemetry endpoint", pathPrefix)})
		return
	}

	if err := q.UpsertAuthPolicy(r.Context(), dbpkg.AuthPolicyRow{
		EnvironmentID: envRow.ID,
		PathPrefix:    pathPrefix,
		Username:      username,
		PasswordHash:  passwordHash,
	}); err != nil {
		s.writeInternalAPIError(w, r, "upsert auth policy failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	row, err := q.GetAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		s.writeInternalAPIError(w, r, "load auth policy failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	if s.caddyReloader != nil {
		reason := "authpolicy.add " + website + "/" + env + " " + pathPrefix
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after auth policy add", "website", website, "environment", env, "path_prefix", pathPrefix, "reason", reason, "error", err)
			if created {
				rolledBack, rollbackErr := q.DeleteAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix)
				if rollbackErr != nil {
					if reconcileErr := s.reconcileAuthPolicyConfig(r.Context(), "add rollback failure "+website+"/"+env+" "+pathPrefix); reconcileErr != nil {
						s.writeInternalAPIError(w, r, "auth policy created but caddy reload failed and rollback failed", err, "website", website, "environment", env, "path_prefix", pathPrefix, "rollback_error", rollbackErr, "reconcile_error", reconcileErr)
						return
					}
					s.logger.Warn("auth policy caddy reconcile succeeded after failed create rollback", "website", website, "environment", env, "path_prefix", pathPrefix)
				} else if rolledBack {
					s.writeInternalAPIError(w, r, "auth policy add was rolled back because caddy reload failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
					return
				}
			} else {
				if _, deleteErr := q.DeleteAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix); deleteErr != nil {
					if reconcileErr := s.reconcileAuthPolicyConfig(r.Context(), "update rollback delete failure "+website+"/"+env+" "+pathPrefix); reconcileErr != nil {
						s.writeInternalAPIError(w, r, "auth policy updated but caddy reload failed and rollback delete failed", err, "website", website, "environment", env, "path_prefix", pathPrefix, "rollback_error", deleteErr, "reconcile_error", reconcileErr)
						return
					}
					s.logger.Warn("auth policy caddy reconcile succeeded after failed update rollback delete", "website", website, "environment", env, "path_prefix", pathPrefix)
				} else if rollbackErr := q.RestoreAuthPolicy(r.Context(), existing); rollbackErr != nil {
					if reconcileErr := s.reconcileAuthPolicyConfig(r.Context(), "update rollback restore failure "+website+"/"+env+" "+pathPrefix); reconcileErr != nil {
						s.writeInternalAPIError(w, r, "auth policy updated but caddy reload failed and rollback restore failed", err, "website", website, "environment", env, "path_prefix", pathPrefix, "rollback_error", rollbackErr, "reconcile_error", reconcileErr)
						return
					}
					s.logger.Warn("auth policy caddy reconcile succeeded after failed update rollback restore", "website", website, "environment", env, "path_prefix", pathPrefix)
					s.writeInternalAPIError(w, r, "auth policy update failed and rollback restore failed", err, "website", website, "environment", env, "path_prefix", pathPrefix, "rollback_error", rollbackErr)
					return
				} else {
					s.writeInternalAPIError(w, r, "auth policy update was rolled back because caddy reload failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
					return
				}
			}
		}
	}

	s.logAuthPolicyAudit(r.Context(), actorFromRequest(r), audit.OperationAuthPolicyAdd, envRow.ID, website, env, row.PathPrefix, row.Username)
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapAuthPolicyRow(row, website, env))
}

func (s *Server) handleRemoveAuthPolicy(w http.ResponseWriter, r *http.Request, website, env string) {
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
		writeAPIError(w, http.StatusBadRequest, "invalid auth policy path", []string{err.Error()})
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

	row, err := q.GetAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("auth policy %q not found", pathPrefix), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get auth policy failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}

	deleted, err := q.DeleteAuthPolicyByPathPrefix(r.Context(), envRow.ID, pathPrefix)
	if err != nil {
		s.writeInternalAPIError(w, r, "delete auth policy failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("auth policy %q not found", pathPrefix), nil)
		return
	}

	if s.caddyReloader != nil {
		reason := "authpolicy.remove " + website + "/" + env + " " + pathPrefix
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after auth policy remove", "website", website, "environment", env, "path_prefix", pathPrefix, "reason", reason, "error", err)
			if rollbackErr := q.RestoreAuthPolicy(r.Context(), row); rollbackErr != nil {
				if reconcileErr := s.reconcileAuthPolicyConfig(r.Context(), "remove rollback failure "+website+"/"+env+" "+pathPrefix); reconcileErr != nil {
					s.writeInternalAPIError(w, r, "auth policy removed but caddy reload failed and rollback failed", err, "website", website, "environment", env, "path_prefix", pathPrefix, "rollback_error", rollbackErr, "reconcile_error", reconcileErr)
					return
				}
				s.logger.Warn("auth policy caddy reconcile succeeded after failed remove rollback", "website", website, "environment", env, "path_prefix", pathPrefix)
			} else {
				s.writeInternalAPIError(w, r, "auth policy removal was rolled back because caddy reload failed", err, "website", website, "environment", env, "path_prefix", pathPrefix)
				return
			}
		}
	}

	s.logAuthPolicyAudit(r.Context(), actorFromRequest(r), audit.OperationAuthPolicyRemove, envRow.ID, website, env, row.PathPrefix, row.Username)
	w.WriteHeader(http.StatusNoContent)
}

func validateAuthPolicyPathOverlap(ctx context.Context, q *dbpkg.Queries, environmentID int64, pathPrefix string) error {
	rows, err := q.ListAuthPoliciesByEnvironment(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("list auth policies: %w", err)
	}
	for _, row := range rows {
		if row.PathPrefix == pathPrefix {
			continue
		}
		if backendpkg.PathPrefixesOverlap(row.PathPrefix, pathPrefix) {
			return fmt.Errorf("path prefix %q overlaps existing auth policy %q", pathPrefix, row.PathPrefix)
		}
	}

	backends, err := q.ListBackendsByEnvironment(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("list backends: %w", err)
	}
	for _, backend := range backends {
		if backendpkg.PathPrefixesOverlap(backend.PathPrefix, pathPrefix) && backend.PathPrefix != pathPrefix {
			return fmt.Errorf("path prefix %q overlaps backend %q; only exact matches are allowed", pathPrefix, backend.PathPrefix)
		}
	}
	return nil
}

func (s *Server) logAuthPolicyAudit(ctx context.Context, actor, operation string, environmentID int64, website, environment, pathPrefix, username string) {
	if s.auditLogger == nil {
		return
	}
	if err := s.auditLogger.Log(ctx, audit.Entry{
		Actor:           actor,
		EnvironmentID:   &environmentID,
		Operation:       operation,
		ResourceSummary: fmt.Sprintf("%s %s for %s/%s as %s", operation, pathPrefix, website, environment, username),
		Metadata: map[string]any{
			"pathPrefix":  pathPrefix,
			"username":    username,
			"website":     website,
			"environment": environment,
		},
	}); err != nil {
		s.logger.Error("failed to write auth policy audit entry", "operation", operation, "path_prefix", pathPrefix, "error", err)
		return
	}
	if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
		waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		if err := flusher.WaitIdle(waitCtx); err != nil {
			s.logger.Warn("timed out waiting for async auth policy audit flush", "operation", operation, "path_prefix", pathPrefix, "error", err)
		}
		cancel()
	}
}

func mapAuthPolicyRow(row dbpkg.AuthPolicyRow, website, environment string) authPolicyResponse {
	return authPolicyResponse{
		ID:          row.ID,
		PathPrefix:  row.PathPrefix,
		Username:    row.Username,
		Website:     website,
		Environment: environment,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func parseAuthPoliciesPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "auth-policies" {
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

func (s *Server) reconcileAuthPolicyConfig(ctx context.Context, reason string) error {
	if s.caddyReloader == nil {
		return nil
	}
	reconcileReason := "authpolicy.reconcile " + strings.TrimSpace(reason)
	if err := s.caddyReloader.Reload(ctx, reconcileReason); err != nil {
		return fmt.Errorf("reconcile caddy config: %w", err)
	}
	s.logger.Warn("auth policy caddy reconciliation reload succeeded", "reason", reconcileReason)
	return nil
}
