package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	domainpkg "github.com/benedict2310/htmlctl/internal/domain"
	sqlite3 "modernc.org/sqlite"
)

const (
	previewCleanupInterval   = time.Hour
	previewCleanupTimeout    = 30 * time.Second
	previewHostnameAttempts  = 8
	previewHostnameTokenLen  = 10
	previewHostnameLabelPart = 20
	previewTimestampLayout   = "2006-01-02T15:04:05.000000000Z"
)

const previewRobotsTag = "noindex, nofollow, noarchive"

var previewHostnameAlphabet = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

type previewCreateRequest struct {
	ReleaseID string `json:"releaseId"`
	TTL       string `json:"ttl,omitempty"`
}

type previewResponse struct {
	ID          int64  `json:"id"`
	ReleaseID   string `json:"releaseId"`
	Hostname    string `json:"hostname"`
	Website     string `json:"website"`
	Environment string `json:"environment"`
	CreatedBy   string `json:"createdBy"`
	ExpiresAt   string `json:"expiresAt"`
	CreatedAt   string `json:"createdAt"`
}

type previewsResponse struct {
	Website     string            `json:"website"`
	Environment string            `json:"environment"`
	Previews    []previewResponse `json:"previews"`
}

func (s *Server) handlePreviews(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parsePreviewsCollectionPath(pathValue)
	itemID := int64(0)
	itemPath := false
	if !ok {
		website, env, itemID, ok, err = parsePreviewItemPath(pathValue)
		itemPath = ok && err == nil
	}
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
		if itemPath {
			writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		s.handleListPreviews(w, r, website, env)
	case http.MethodPost:
		if itemPath {
			writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		s.handleCreatePreview(w, r, website, env)
	case http.MethodDelete:
		if !itemPath {
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		s.handleRemovePreview(w, r, website, env, itemID)
	default:
		if itemPath {
			w.Header().Set("Allow", http.MethodDelete)
		} else {
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		}
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (s *Server) handleListPreviews(w http.ResponseWriter, r *http.Request, website, env string) {
	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}
	rows, err := q.ListReleasePreviewsByEnvironment(r.Context(), envRow.ID, formatPreviewTimestamp(time.Now().UTC()))
	if err != nil {
		s.writeInternalAPIError(w, r, "list previews failed", err, "website", website, "environment", env)
		return
	}
	items := make([]previewResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapPreviewRow(row, website, env))
	}
	writeJSON(w, http.StatusOK, previewsResponse{
		Website:     website,
		Environment: env,
		Previews:    items,
	})
}

func (s *Server) handleCreatePreview(w http.ResponseWriter, r *http.Request, website, env string) {
	if !s.cfg.Preview.Enabled {
		writeAPIError(w, http.StatusServiceUnavailable, "preview URLs are disabled", nil)
		return
	}

	var req previewCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}
	releaseID := strings.TrimSpace(req.ReleaseID)
	if releaseID == "" {
		writeAPIError(w, http.StatusBadRequest, "releaseId is required", nil)
		return
	}
	ttl, err := normalizePreviewTTL(req.TTL, s.previewDefaultTTL(), s.previewMaxTTL())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid ttl", []string{err.Error()})
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

	releaseRow, err := q.GetReleaseByID(r.Context(), releaseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("release %q not found", releaseID), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get release failed", err, "website", website, "environment", env, "release_id", releaseID)
		return
	}
	if releaseRow.EnvironmentID != envRow.ID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("release %q not found", releaseID), nil)
		return
	}
	releaseRoot := s.previewReleaseRoot(website, env, releaseID)
	info, err := os.Stat(releaseRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("release %q has no built artifact", releaseID), nil)
			return
		}
		s.writeInternalAPIError(w, r, "stat release preview root failed", err, "website", website, "environment", env, "release_id", releaseID)
		return
	}
	if !info.IsDir() {
		s.writeInternalAPIError(w, r, "release preview root invalid", fmt.Errorf("release root is not a directory"), "website", website, "environment", env, "release_id", releaseID)
		return
	}

	now := time.Now().UTC()
	expiresAt := formatPreviewTimestamp(now.Add(ttl))
	createdBy := actorFromRequest(r)

	var (
		insertedID int64
		hostname   string
	)
	for attempt := 0; attempt < previewHostnameAttempts; attempt++ {
		hostname, err = s.generatePreviewHostname(website, env)
		if err != nil {
			s.writeInternalAPIError(w, r, "generate preview hostname failed", err, "website", website, "environment", env)
			return
		}
		insertedID, err = q.InsertReleasePreview(r.Context(), dbpkg.ReleasePreviewRow{
			EnvironmentID: envRow.ID,
			ReleaseID:     releaseID,
			Hostname:      hostname,
			CreatedBy:     createdBy,
			ExpiresAt:     expiresAt,
		})
		if err == nil {
			break
		}
		if !isPreviewHostnameUniqueConstraintError(err) {
			s.writeInternalAPIError(w, r, "create preview failed", err, "website", website, "environment", env, "release_id", releaseID)
			return
		}
	}
	if insertedID == 0 {
		s.writeInternalAPIError(w, r, "create preview failed", fmt.Errorf("failed to generate unique preview hostname after %d attempts", previewHostnameAttempts), "website", website, "environment", env, "release_id", releaseID)
		return
	}

	row, err := q.GetReleasePreviewByID(r.Context(), envRow.ID, insertedID)
	if err != nil {
		s.writeInternalAPIError(w, r, "load preview failed", err, "website", website, "environment", env, "preview_id", insertedID)
		return
	}

	if s.caddyReloader != nil {
		reason := "preview.create " + website + "/" + env + " " + row.Hostname
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after preview create", "website", website, "environment", env, "preview_id", row.ID, "hostname", row.Hostname, "reason", reason, "error", err)
			rolledBack, rollbackErr := q.DeleteReleasePreviewByID(r.Context(), envRow.ID, row.ID)
			if rollbackErr != nil {
				if reconcileErr := s.reconcilePreviewConfig(r.Context(), fmt.Sprintf("create rollback failure %s/%s %d", website, env, row.ID)); reconcileErr != nil {
					s.writeInternalAPIError(
						w,
						r,
						"preview created but caddy reload failed and rollback failed",
						err,
						"website", website,
						"environment", env,
						"preview_id", row.ID,
						"rollback_error", rollbackErr,
						"reconcile_error", reconcileErr,
					)
					return
				}
				s.logger.Warn("caddy reconciliation succeeded after failed preview create rollback", "website", website, "environment", env, "preview_id", row.ID)
			} else if rolledBack {
				s.writeInternalAPIError(w, r, "preview was rolled back because caddy reload failed", err, "website", website, "environment", env, "preview_id", row.ID)
				return
			}
			s.writeInternalAPIError(w, r, "preview create state was left unchanged because caddy reload failed", err, "website", website, "environment", env, "preview_id", row.ID)
			return
		}
	}

	writeJSON(w, http.StatusCreated, mapPreviewRow(row, website, env))
}

func (s *Server) handleRemovePreview(w http.ResponseWriter, r *http.Request, website, env string, previewID int64) {
	q := dbpkg.NewQueries(s.db)
	envRow, err := lookupEnvironmentRow(r.Context(), q, website, env)
	if err != nil {
		s.handleEnvironmentLookupError(w, r, website, env, err)
		return
	}

	lock := s.environmentLock(website, env)
	lock.Lock()
	defer lock.Unlock()

	row, err := q.GetReleasePreviewByID(r.Context(), envRow.ID, previewID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("preview %d not found", previewID), nil)
			return
		}
		s.writeInternalAPIError(w, r, "get preview failed", err, "website", website, "environment", env, "preview_id", previewID)
		return
	}

	deleted, err := q.DeleteReleasePreviewByID(r.Context(), envRow.ID, previewID)
	if err != nil {
		s.writeInternalAPIError(w, r, "delete preview failed", err, "website", website, "environment", env, "preview_id", previewID)
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("preview %d not found", previewID), nil)
		return
	}

	if s.cfg.Preview.Enabled && s.caddyReloader != nil {
		reason := "preview.remove " + website + "/" + env + " " + row.Hostname
		if err := s.caddyReloader.Reload(r.Context(), reason); err != nil {
			s.logger.Error("caddy reload failed after preview remove", "website", website, "environment", env, "preview_id", previewID, "hostname", row.Hostname, "reason", reason, "error", err)
			if rollbackErr := q.RestoreReleasePreview(r.Context(), row); rollbackErr != nil {
				if reconcileErr := s.reconcilePreviewConfig(r.Context(), fmt.Sprintf("remove rollback failure %s/%s %d", website, env, previewID)); reconcileErr != nil {
					s.writeInternalAPIError(
						w,
						r,
						"preview removed but caddy reload failed and rollback failed",
						err,
						"website", website,
						"environment", env,
						"preview_id", previewID,
						"rollback_error", rollbackErr,
						"reconcile_error", reconcileErr,
					)
					return
				}
				s.logger.Warn("caddy reconciliation succeeded after failed preview remove rollback", "website", website, "environment", env, "preview_id", previewID)
			} else {
				s.writeInternalAPIError(w, r, "preview removal was rolled back because caddy reload failed", err, "website", website, "environment", env, "preview_id", previewID)
				return
			}
			s.writeInternalAPIError(w, r, "preview removal state was left unchanged because caddy reload failed", err, "website", website, "environment", env, "preview_id", previewID)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePreviewsCollectionPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "previews" {
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

func isPreviewPath(pathValue string) bool {
	if pathValue == "" {
		return false
	}
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 7 && len(parts) != 8 {
		return false
	}
	return parts[0] == "api" && parts[1] == "v1" && parts[2] == "websites" && parts[4] == "environments" && parts[6] == "previews"
}

func parsePreviewItemPath(pathValue string) (website, env string, id int64, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 8 {
		return "", "", 0, false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "previews" {
		return "", "", 0, false, nil
	}
	website = strings.TrimSpace(parts[3])
	env = strings.TrimSpace(parts[5])
	rawID := strings.TrimSpace(parts[7])
	if website == "" || env == "" || rawID == "" {
		return "", "", 0, false, nil
	}
	if err := validateResourceName(website); err != nil {
		return website, env, 0, false, fmt.Errorf("invalid website name %q: %w", website, err)
	}
	if err := validateResourceName(env); err != nil {
		return website, env, 0, false, fmt.Errorf("invalid environment name %q: %w", env, err)
	}
	id, err = strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return website, env, 0, false, fmt.Errorf("invalid preview id %q", rawID)
	}
	return website, env, id, true, nil
}

func mapPreviewRow(row dbpkg.ReleasePreviewRow, website, env string) previewResponse {
	return previewResponse{
		ID:          row.ID,
		ReleaseID:   row.ReleaseID,
		Hostname:    row.Hostname,
		Website:     website,
		Environment: env,
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
	}
}

func normalizePreviewTTL(raw string, defaultTTL, maxTTL time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultTTL, nil
	}
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("ttl must be a valid duration like 72h")
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("ttl must be > 0")
	}
	if ttl%time.Hour != 0 {
		return 0, fmt.Errorf("ttl must be a whole number of hours")
	}
	if ttl > maxTTL {
		return 0, fmt.Errorf("ttl must be <= %s", maxTTL)
	}
	return ttl, nil
}

func (s *Server) previewDefaultTTL() time.Duration {
	hours := s.cfg.Preview.DefaultTTLHours
	if hours <= 0 {
		hours = DefaultPreviewTTLHours
	}
	return time.Duration(hours) * time.Hour
}

func (s *Server) previewMaxTTL() time.Duration {
	hours := s.cfg.Preview.MaxTTLHours
	if hours <= 0 {
		hours = DefaultPreviewMaxTTLHours
	}
	return time.Duration(hours) * time.Hour
}

func (s *Server) previewBaseDomain() string {
	return strings.ToLower(strings.TrimSpace(s.cfg.Preview.BaseDomain))
}

func (s *Server) generatePreviewHostname(website, env string) (string, error) {
	token, err := randomPreviewToken(previewHostnameTokenLen)
	if err != nil {
		return "", err
	}
	envLabel := sanitizePreviewHostnameLabel(env, "env")
	websiteLabel := sanitizePreviewHostnameLabel(website, "site")
	hostname := fmt.Sprintf("%s--%s--%s.%s", token, envLabel, websiteLabel, s.previewBaseDomain())
	normalized, err := domainpkg.Normalize(hostname)
	if err != nil {
		return "", fmt.Errorf("normalize preview hostname: %w", err)
	}
	return normalized, nil
}

func randomPreviewToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("preview hostname token length must be > 0")
	}
	buf := make([]byte, length)
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read preview hostname randomness: %w", err)
	}
	for i := range raw {
		buf[i] = previewHostnameAlphabet[int(raw[i])%len(previewHostnameAlphabet)]
	}
	return string(buf), nil
}

func sanitizePreviewHostnameLabel(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
		if b.Len() >= previewHostnameLabelPart {
			break
		}
	}
	label := strings.Trim(b.String(), "-")
	if label == "" {
		return fallback
	}
	return label
}

func isPreviewHostnameUniqueConstraintError(err error) bool {
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case 2067, 1555:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") && strings.Contains(msg, "release_previews.hostname")
}

func (s *Server) startPreviewCleanupLoop() {
	if !s.cfg.Preview.Enabled || s.db == nil {
		return
	}
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	s.previewCleanupStop = stopCh
	s.previewCleanupDone = doneCh

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(previewCleanupInterval)
		defer ticker.Stop()

		s.runPreviewCleanup()
		for {
			select {
			case <-ticker.C:
				s.runPreviewCleanup()
			case <-stopCh:
				return
			}
		}
	}()
}

func (s *Server) runPreviewCleanup() {
	if !s.cfg.Preview.Enabled || s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), previewCleanupTimeout)
	defer cancel()

	pendingReload := s.previewCleanupNeedsReload()
	deleted, err := dbpkg.NewQueries(s.db).DeleteExpiredReleasePreviews(ctx, formatPreviewTimestamp(time.Now().UTC()))
	if err != nil {
		s.logger.Warn("preview cleanup failed", "error", err)
		return
	}
	if deleted == 0 && !pendingReload {
		return
	}
	if s.caddyReloader != nil {
		if err := s.caddyReloader.Reload(ctx, "preview.cleanup"); err != nil {
			s.setPreviewCleanupNeedsReload(true)
			s.logger.Error("caddy reload failed after preview cleanup", "deleted", deleted, "error", err)
			return
		}
	}
	s.setPreviewCleanupNeedsReload(false)
	s.logger.Info("preview cleanup complete", "deleted", deleted)
}

func (s *Server) stopPreviewCleanupLoop(ctx context.Context) error {
	stopCh := s.previewCleanupStop
	doneCh := s.previewCleanupDone
	if stopCh == nil || doneCh == nil {
		return nil
	}
	s.previewCleanupStop = nil
	s.previewCleanupDone = nil

	close(stopCh)
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) previewCleanupNeedsReload() bool {
	s.previewReloadMu.Lock()
	defer s.previewReloadMu.Unlock()
	return s.previewReloadPending
}

func (s *Server) setPreviewCleanupNeedsReload(v bool) {
	s.previewReloadMu.Lock()
	s.previewReloadPending = v
	s.previewReloadMu.Unlock()
}

func formatPreviewTimestamp(ts time.Time) string {
	return ts.UTC().Format(previewTimestampLayout)
}

func (s *Server) previewReleaseRoot(website, env, releaseID string) string {
	return filepath.Join(s.dataPaths.WebsitesRoot, website, "envs", env, "releases", releaseID)
}

func (s *Server) reconcilePreviewConfig(ctx context.Context, reason string) error {
	if s.caddyReloader == nil {
		return nil
	}
	reconcileReason := "preview.reconcile " + strings.TrimSpace(reason)
	if err := s.caddyReloader.Reload(ctx, reconcileReason); err != nil {
		return fmt.Errorf("reconcile caddy config: %w", err)
	}
	s.logger.Warn("preview caddy reconciliation reload succeeded", "reason", reconcileReason)
	return nil
}
