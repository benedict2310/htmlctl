package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	domainpkg "github.com/benedict2310/htmlctl/internal/domain"
	sqlite3 "modernc.org/sqlite"
)

type domainBindingRequest struct {
	Domain      string `json:"domain"`
	Website     string `json:"website"`
	Environment string `json:"environment"`
}

type domainBindingResponse struct {
	ID          int64  `json:"id"`
	Domain      string `json:"domain"`
	Website     string `json:"website"`
	Environment string `json:"environment"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type domainBindingsResponse struct {
	Domains []domainBindingResponse `json:"domains"`
}

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	if r.URL.Path == "/api/v1/domains" {
		switch r.Method {
		case http.MethodGet:
			s.handleListDomains(w, r)
		case http.MethodPost:
			s.handleCreateDomain(w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		}
		return
	}

	domainValue, ok := parseDomainItemPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetDomain(w, r, domainValue)
	case http.MethodDelete:
		s.handleDeleteDomain(w, r, domainValue)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodDelete)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	websiteFilter := strings.TrimSpace(r.URL.Query().Get("website"))
	environmentFilter := strings.TrimSpace(r.URL.Query().Get("environment"))

	q := dbpkg.NewQueries(s.db)
	rows, err := q.ListDomainBindings(r.Context(), websiteFilter, environmentFilter)
	if err != nil {
		s.writeInternalAPIError(w, r, "list domain bindings failed", err, "website_filter", websiteFilter, "environment_filter", environmentFilter)
		return
	}

	items := make([]domainBindingResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDomainBindingRow(row))
	}
	writeJSON(w, http.StatusOK, domainBindingsResponse{Domains: items})
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request, domainValue string) {
	normalized, err := domainpkg.Normalize(domainValue)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	row, err := q.GetDomainBindingByDomain(r.Context(), normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("domain %q not found", normalized), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get domain binding failed", err, "domain", normalized)
		return
	}

	writeJSON(w, http.StatusOK, mapDomainBindingRow(row))
}

func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var req domainBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}

	normalized, err := domainpkg.Normalize(req.Domain)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	website := strings.TrimSpace(req.Website)
	environment := strings.TrimSpace(req.Environment)
	if website == "" || environment == "" {
		writeAPIError(w, http.StatusBadRequest, "website and environment are required", nil)
		return
	}
	if err := validateResourceName(website); err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("invalid website name %q: %v", website, err), nil)
		return
	}
	if err := validateResourceName(environment); err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("invalid environment name %q: %v", environment, err), nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup website failed", err, "website", website, "domain", normalized)
		return
	}
	envRow, err := q.GetEnvironmentByName(r.Context(), websiteRow.ID, environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", environment), nil)
			return
		}
		s.writeInternalAPIError(w, r, "lookup environment failed", err, "website", website, "environment", environment, "domain", normalized)
		return
	}

	lock := s.domainLock(normalized)
	lock.Lock()
	defer lock.Unlock()

	_, err = q.InsertDomainBinding(r.Context(), dbpkg.DomainBindingRow{
		Domain:        normalized,
		EnvironmentID: envRow.ID,
	})
	if err != nil {
		if isDomainUniqueConstraintError(err) {
			writeAPIError(w, http.StatusConflict, fmt.Sprintf("domain %q is already bound", normalized), nil)
			return
		}
		s.writeInternalAPIError(w, r, "create domain binding failed", err, "domain", normalized, "website", website, "environment", environment)
		return
	}

	row, err := q.GetDomainBindingByDomain(r.Context(), normalized)
	if err != nil {
		s.writeInternalAPIError(w, r, "load domain binding failed", err, "domain", normalized)
		return
	}

	if s.caddyReloader != nil {
		reason := "domain.add " + normalized
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after domain add", "domain", normalized, "reason", reason, "error", err)
			rolledBack, rollbackErr := q.DeleteDomainBindingByDomain(r.Context(), normalized)
			rollbackInconclusive := !rolledBack
			if rollbackErr != nil {
				if reconcileErr := s.reconcileDomainConfig(r.Context(), "add rollback failure "+normalized); reconcileErr != nil {
					s.writeInternalAPIError(
						w,
						r,
						"domain binding created but caddy reload failed and rollback failed",
						err,
						"domain", normalized,
						"rollback_error", rollbackErr,
						"reconcile_error", reconcileErr,
					)
					return
				}
				// Reconcile succeeded while DB rollback failed; keep the response based on the
				// row already loaded before reload failure instead of forcing another DB read.
				s.logger.Warn("caddy reconcile succeeded after failed domain add rollback", "domain", normalized)
				rollbackInconclusive = false
			}
			if rollbackInconclusive {
				reconciledRow, rowErr := q.GetDomainBindingByDomain(r.Context(), normalized)
				if rowErr != nil {
					s.writeInternalAPIError(w, r, "domain binding created but caddy reload failed and rollback was inconclusive", err, "domain", normalized, "lookup_error", rowErr)
					return
				}
				row = reconciledRow
			}
			if rolledBack {
				s.writeInternalAPIError(w, r, "domain binding was rolled back because caddy reload failed", err, "domain", normalized)
				return
			}
		}
		s.logger.Info("caddy reload succeeded after domain add", "domain", normalized, "reason", reason)
	}
	s.logDomainAudit(r.Context(), actorFromRequest(r), audit.OperationDomainAdd, row.EnvironmentID, normalized, row.WebsiteName, row.EnvironmentName)

	writeJSON(w, http.StatusCreated, mapDomainBindingRow(row))
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request, domainValue string) {
	normalized, err := domainpkg.Normalize(domainValue)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	lock := s.domainLock(normalized)
	lock.Lock()
	defer lock.Unlock()

	q := dbpkg.NewQueries(s.db)
	row, err := q.GetDomainBindingByDomain(r.Context(), normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("domain %q not found", normalized), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get domain binding failed", err, "domain", normalized)
		return
	}

	deleted, err := q.DeleteDomainBindingByDomain(r.Context(), normalized)
	if err != nil {
		s.writeInternalAPIError(w, r, "delete domain binding failed", err, "domain", normalized)
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("domain %q not found", normalized), nil)
		return
	}

	if s.caddyReloader != nil {
		reason := "domain.remove " + normalized
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after domain remove", "domain", normalized, "reason", reason, "error", err)
			rollbackErr := q.RestoreDomainBinding(r.Context(), dbpkg.DomainBindingRow{
				ID:            row.ID,
				Domain:        row.Domain,
				EnvironmentID: row.EnvironmentID,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
			})
			if rollbackErr != nil {
				if reconcileErr := s.reconcileDomainConfig(r.Context(), "remove rollback failure "+normalized); reconcileErr != nil {
					s.writeInternalAPIError(
						w,
						r,
						"domain binding removed but caddy reload failed and rollback failed",
						err,
						"domain", normalized,
						"rollback_error", rollbackErr,
						"reconcile_error", reconcileErr,
					)
					return
				}
				s.logger.Warn("caddy reconcile succeeded after failed domain remove rollback", "domain", normalized)
			} else {
				s.writeInternalAPIError(w, r, "domain binding removal was rolled back because caddy reload failed", err, "domain", normalized)
				return
			}
		}
		s.logger.Info("caddy reload succeeded after domain remove", "domain", normalized, "reason", reason)
	}
	s.logDomainAudit(r.Context(), actorFromRequest(r), audit.OperationDomainRemove, row.EnvironmentID, normalized, row.WebsiteName, row.EnvironmentName)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) logDomainAudit(ctx context.Context, actor, operation string, environmentID int64, domainValue, website, environment string) {
	if s.auditLogger == nil {
		return
	}
	if err := s.auditLogger.Log(ctx, audit.Entry{
		Actor:           actor,
		EnvironmentID:   &environmentID,
		Operation:       operation,
		ResourceSummary: fmt.Sprintf("%s %s for %s/%s", operation, domainValue, website, environment),
		Metadata: map[string]any{
			"domain":      domainValue,
			"website":     website,
			"environment": environment,
		},
	}); err != nil {
		s.logger.Error("failed to write domain audit entry", "operation", operation, "domain", domainValue, "error", err)
		return
	}
	if flusher, ok := s.auditLogger.(interface{ WaitIdle(context.Context) error }); ok {
		waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		if err := flusher.WaitIdle(waitCtx); err != nil {
			s.logger.Warn("timed out waiting for async domain audit flush", "operation", operation, "domain", domainValue, "error", err)
		}
		cancel()
	}
}

func mapDomainBindingRow(row dbpkg.DomainBindingResolvedRow) domainBindingResponse {
	return domainBindingResponse{
		ID:          row.ID,
		Domain:      row.Domain,
		Website:     row.WebsiteName,
		Environment: row.EnvironmentName,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func parseDomainItemPath(pathValue string) (domain string, ok bool) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "domains" {
		return "", false
	}
	domain = strings.TrimSpace(parts[3])
	if domain == "" {
		return "", false
	}
	return domain, true
}

func isDomainUniqueConstraintError(err error) bool {
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case 2067, 1555:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") && strings.Contains(msg, "domain_bindings.domain")
}

func (s *Server) reconcileDomainConfig(ctx context.Context, reason string) error {
	if s.caddyReloader == nil {
		return nil
	}
	reconcileReason := "domain.reconcile " + strings.TrimSpace(reason)
	if err := s.caddyReloader.Reload(ctx, reconcileReason); err != nil {
		return fmt.Errorf("reconcile caddy config: %w", err)
	}
	s.logger.Warn("caddy reconciliation reload succeeded", "reason", reconcileReason)
	return nil
}
