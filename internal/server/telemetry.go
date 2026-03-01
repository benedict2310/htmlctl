package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	stdpath "path"
	"regexp"
	"strconv"
	"strings"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	domainpkg "github.com/benedict2310/htmlctl/internal/domain"
)

const (
	defaultTelemetryListLimit = 100
	maxTelemetryListLimit     = 1000

	maxTelemetryPathBytes = 1024
	maxTelemetryAttrs     = 16
	maxTelemetryAttrKey   = 64
	maxTelemetryAttrValue = 256

	maxTelemetryFutureSkew = 24 * time.Hour
	maxTelemetryPastSkew   = 30 * 24 * time.Hour

	telemetryRetentionCleanupInterval = time.Hour
	telemetryRetentionCleanupTimeout  = 30 * time.Second
)

var (
	eventNameRE  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	sessionIDRE  = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
	telemetryKey = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]*$`)

	errTelemetryHostNotBound = errors.New("telemetry host is not bound")
)

type telemetryIngestRequest struct {
	Events []telemetryIngestEvent `json:"events"`
}

type telemetryIngestEvent struct {
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	OccurredAt string            `json:"occurredAt,omitempty"`
	SessionID  string            `json:"sessionId,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
}

type telemetryEventsResponse struct {
	Website     string                   `json:"website"`
	Environment string                   `json:"environment"`
	Limit       int                      `json:"limit"`
	Offset      int                      `json:"offset"`
	Events      []telemetryEventResponse `json:"events"`
}

type telemetryEventResponse struct {
	ID         int64             `json:"id"`
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	OccurredAt *string           `json:"occurredAt,omitempty"`
	ReceivedAt string            `json:"receivedAt"`
	SessionID  *string           `json:"sessionId,omitempty"`
	Attrs      map[string]string `json:"attrs"`
}

func (s *Server) handleTelemetryIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusBadRequest, "cors preflight is not supported; telemetry ingest accepts authenticated same-origin POSTs only", nil)
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
	if err := validateTelemetryContentType(r.Header.Get("Content-Type")); err != nil {
		writeAPIError(w, http.StatusUnsupportedMediaType, err.Error(), nil)
		return
	}
	if err := validateTelemetrySameOrigin(r.Header.Get("Origin"), r); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(s.telemetryMaxBodyBytes()))
	defer r.Body.Close()

	var req telemetryIngestRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if isMaxBytesError(err) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large", nil)
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{err.Error()})
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if isMaxBytesError(err) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large", nil)
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid request body", []string{"request body must contain a single JSON object"})
		return
	}
	if len(req.Events) == 0 {
		writeAPIError(w, http.StatusBadRequest, "events must contain at least one event", nil)
		return
	}
	if len(req.Events) > s.telemetryMaxEvents() {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("events must contain at most %d items", s.telemetryMaxEvents()), nil)
		return
	}

	host, err := normalizeTelemetryHost(r.Host)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "host is not bound to any environment", nil)
		return
	}
	environmentID, err := s.resolveTelemetryEnvironmentID(r.Context(), host)
	if err != nil {
		if errors.Is(err, errTelemetryHostNotBound) {
			writeAPIError(w, http.StatusBadRequest, "host is not bound to any environment", nil)
			return
		}
		s.writeInternalAPIError(w, r, "telemetry ingest failed", err)
		return
	}

	now := time.Now().UTC()
	rows := make([]dbpkg.TelemetryEventRow, 0, len(req.Events))
	for i, event := range req.Events {
		row, err := buildTelemetryEventRow(environmentID, event, now)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("events[%d]: %v", i, err), nil)
			return
		}
		rows = append(rows, row)
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.writeInternalAPIError(w, r, "telemetry ingest failed", err)
		return
	}
	q := dbpkg.NewQueries(tx)
	for _, row := range rows {
		if _, err := q.InsertTelemetryEvent(r.Context(), row); err != nil {
			_ = tx.Rollback()
			s.writeInternalAPIError(w, r, "telemetry ingest failed", err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		s.writeInternalAPIError(w, r, "telemetry ingest failed", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": len(rows)})
}

func (s *Server) handleTelemetryEvents(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, ok, err := parseTelemetryEventsPath(pathValue)
	if !ok {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if s.db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	limit, offset, err := parseListTelemetryPagination(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{err.Error()})
		return
	}

	eventFilter := strings.TrimSpace(r.URL.Query().Get("event"))
	if eventFilter != "" {
		if err := validateEventName(eventFilter); err != nil {
			writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("invalid event filter: %v", err), nil)
			return
		}
	}

	var since *string
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		ts, err := parseRFC3339Timestamp(raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{fmt.Sprintf("invalid since: %v", err)})
			return
		}
		formatted := ts.UTC().Format(time.RFC3339)
		since = &formatted
	}
	var until *string
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		ts, err := parseRFC3339Timestamp(raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{fmt.Sprintf("invalid until: %v", err)})
			return
		}
		formatted := ts.UTC().Format(time.RFC3339)
		until = &formatted
	}
	if since != nil && until != nil && *since > *until {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{"since must be <= until"})
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

	rows, err := q.ListTelemetryEvents(r.Context(), dbpkg.ListTelemetryEventsParams{
		EnvironmentID: envRow.ID,
		EventName:     eventFilter,
		Since:         since,
		Until:         until,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		s.writeInternalAPIError(w, r, "list telemetry events failed", err, "website", website, "environment", env, "limit", limit, "offset", offset)
		return
	}

	items := make([]telemetryEventResponse, 0, len(rows))
	for _, row := range rows {
		attrs := map[string]string{}
		if strings.TrimSpace(row.AttrsJSON) != "" {
			if err := json.Unmarshal([]byte(row.AttrsJSON), &attrs); err != nil {
				s.writeInternalAPIError(w, r, "list telemetry events failed", err, "website", website, "environment", env)
				return
			}
		}
		items = append(items, telemetryEventResponse{
			ID:         row.ID,
			Name:       row.EventName,
			Path:       row.Path,
			OccurredAt: row.OccurredAt,
			ReceivedAt: row.ReceivedAt,
			SessionID:  row.SessionID,
			Attrs:      attrs,
		})
	}

	writeJSON(w, http.StatusOK, telemetryEventsResponse{
		Website:     websiteRow.Name,
		Environment: envRow.Name,
		Limit:       limit,
		Offset:      offset,
		Events:      items,
	})
}

func parseTelemetryEventsPath(pathValue string) (website, env string, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) != 8 {
		return "", "", false, nil
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "telemetry" || parts[7] != "events" {
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

func parseListTelemetryPagination(r *http.Request) (int, int, error) {
	limit := defaultTelemetryListLimit
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
	if limit > maxTelemetryListLimit {
		limit = maxTelemetryListLimit
	}
	return limit, offset, nil
}

func buildTelemetryEventRow(environmentID int64, in telemetryIngestEvent, now time.Time) (dbpkg.TelemetryEventRow, error) {
	var out dbpkg.TelemetryEventRow

	name := strings.TrimSpace(in.Name)
	if err := validateEventName(name); err != nil {
		return out, err
	}
	cleanPath, err := normalizeTelemetryPath(in.Path)
	if err != nil {
		return out, err
	}
	occurredAt, err := normalizeOccurredAt(in.OccurredAt, now)
	if err != nil {
		return out, err
	}
	sessionID, err := normalizeSessionID(in.SessionID)
	if err != nil {
		return out, err
	}
	attrsJSON, err := normalizeTelemetryAttrs(in.Attrs)
	if err != nil {
		return out, err
	}

	out = dbpkg.TelemetryEventRow{
		EnvironmentID: environmentID,
		EventName:     name,
		Path:          cleanPath,
		OccurredAt:    occurredAt,
		SessionID:     sessionID,
		AttrsJSON:     attrsJSON,
	}
	return out, nil
}

func validateEventName(name string) error {
	if !eventNameRE.MatchString(name) {
		return fmt.Errorf("event name must match [a-zA-Z0-9][a-zA-Z0-9_-]* and be at most 64 characters")
	}
	return nil
}

func normalizeTelemetryPath(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("path must not contain null bytes")
	}
	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("path must be a valid URL path")
	}
	if strings.ContainsRune(unescaped, 0) {
		return "", fmt.Errorf("path must not contain null bytes")
	}
	clean := stdpath.Clean(unescaped)
	if clean == "." {
		clean = "/"
	}
	if !strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("path must start with /")
	}
	if len([]byte(clean)) > maxTelemetryPathBytes {
		return "", fmt.Errorf("path must be at most %d bytes", maxTelemetryPathBytes)
	}
	return clean, nil
}

func normalizeOccurredAt(value string, now time.Time) (*string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	ts, err := parseRFC3339Timestamp(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid occurredAt: must be ISO 8601 / RFC 3339")
	}
	ts = ts.UTC()
	if ts.After(now.Add(maxTelemetryFutureSkew)) {
		return nil, fmt.Errorf("occurredAt is too far in the future")
	}
	if ts.Before(now.Add(-maxTelemetryPastSkew)) {
		return nil, fmt.Errorf("occurredAt is too far in the past")
	}
	formatted := ts.Format(time.RFC3339)
	return &formatted, nil
}

func normalizeSessionID(value string) (*string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	if !sessionIDRE.MatchString(raw) {
		return nil, fmt.Errorf("sessionId must match ^[a-zA-Z0-9_-]{1,128}$")
	}
	return &raw, nil
}

func normalizeTelemetryAttrs(attrs map[string]string) (string, error) {
	if len(attrs) == 0 {
		return "{}", nil
	}
	if len(attrs) > maxTelemetryAttrs {
		return "", fmt.Errorf("attrs must contain at most %d keys", maxTelemetryAttrs)
	}
	for key, value := range attrs {
		if len([]byte(key)) > maxTelemetryAttrKey {
			return "", fmt.Errorf("attrs key %q exceeds %d bytes", key, maxTelemetryAttrKey)
		}
		if !telemetryKey.MatchString(key) {
			return "", fmt.Errorf("attrs key %q must match ^[a-zA-Z0-9_][a-zA-Z0-9_-]*$", key)
		}
		if len([]byte(value)) > maxTelemetryAttrValue {
			return "", fmt.Errorf("attrs value for %q exceeds %d bytes", key, maxTelemetryAttrValue)
		}
		if strings.ContainsRune(value, 0) {
			return "", fmt.Errorf("attrs value for %q must not contain null bytes", key)
		}
	}
	encoded, err := json.Marshal(attrs)
	if err != nil {
		return "", fmt.Errorf("marshal attrs: %w", err)
	}
	return string(encoded), nil
}

func normalizeTelemetryHost(rawHost string) (string, error) {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	return domainpkg.Normalize(host)
}

func validateTelemetryContentType(rawContentType string) error {
	contentType := strings.TrimSpace(rawContentType)
	if contentType == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid Content-Type header")
	}
	switch mediaType {
	case "application/json", "text/plain":
		return nil
	default:
		return fmt.Errorf("unsupported Content-Type %q; expected application/json or text/plain", mediaType)
	}
}

func validateTelemetrySameOrigin(rawOrigin string, r *http.Request) error {
	origin := strings.TrimSpace(rawOrigin)
	if origin == "" {
		return nil
	}
	originURL, err := url.Parse(origin)
	if err != nil || strings.TrimSpace(originURL.Host) == "" {
		return fmt.Errorf("invalid Origin header")
	}
	originScheme, err := normalizeTelemetryScheme(originURL.Scheme)
	if err != nil {
		return fmt.Errorf("invalid Origin header")
	}
	expectedScheme, err := telemetryRequestScheme(r)
	if err != nil {
		return fmt.Errorf("invalid Origin header")
	}
	originHost, err := normalizeTelemetryHost(originURL.Host)
	if err != nil {
		return fmt.Errorf("invalid Origin header")
	}
	host, err := normalizeTelemetryHost(r.Host)
	if err != nil {
		return fmt.Errorf("host is not bound to any environment")
	}
	originPort := normalizedTelemetryPort(originURL.Host, originScheme)
	hostPort := normalizedTelemetryPort(r.Host, expectedScheme)
	if originScheme != expectedScheme || originHost != host || originPort != hostPort {
		return fmt.Errorf("cross-origin telemetry ingest is not supported")
	}
	return nil
}

func normalizeTelemetryScheme(raw string) (string, error) {
	scheme := strings.ToLower(strings.TrimSpace(raw))
	switch scheme {
	case "http", "https":
		return scheme, nil
	default:
		return "", fmt.Errorf("unsupported scheme %q", raw)
	}
}

func telemetryRequestScheme(r *http.Request) (string, error) {
	if r == nil {
		return "", fmt.Errorf("request is required")
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return normalizeTelemetryScheme(first)
	}
	if r.TLS != nil {
		return "https", nil
	}
	return "http", nil
}

func normalizedTelemetryPort(rawHost, scheme string) string {
	port := ""
	if strings.Contains(rawHost, ":") {
		if parsed, err := url.Parse("http://" + rawHost); err == nil {
			port = strings.TrimSpace(parsed.Port())
		}
	}
	if port != "" {
		return port
	}
	switch scheme {
	case "https":
		return "443"
	default:
		return "80"
	}
}

func (s *Server) resolveTelemetryEnvironmentID(ctx context.Context, host string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	q := dbpkg.NewQueries(s.db)
	row, err := q.GetDomainBindingByDomain(ctx, host)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errTelemetryHostNotBound
		}
		return 0, err
	}
	return row.EnvironmentID, nil
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func (s *Server) telemetryMaxBodyBytes() int {
	// 0 means "use server default", not "unlimited".
	if s.cfg.Telemetry.MaxBodyBytes <= 0 {
		return DefaultTelemetryMaxBodyBytes
	}
	return s.cfg.Telemetry.MaxBodyBytes
}

func (s *Server) telemetryMaxEvents() int {
	// 0 means "use server default", not "unlimited".
	if s.cfg.Telemetry.MaxEvents <= 0 {
		return DefaultTelemetryMaxEvents
	}
	return s.cfg.Telemetry.MaxEvents
}

func (s *Server) telemetryRetentionDays() int {
	if s.cfg.Telemetry.RetentionDays < 0 {
		return 0
	}
	return s.cfg.Telemetry.RetentionDays
}

func (s *Server) startTelemetryRetentionCleanupLoop() {
	if !s.cfg.Telemetry.Enabled || s.db == nil {
		return
	}
	retentionDays := s.telemetryRetentionDays()
	if retentionDays == 0 {
		return
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	s.telemetryCleanupStop = stopCh
	s.telemetryCleanupDone = doneCh

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(telemetryRetentionCleanupInterval)
		defer ticker.Stop()

		s.runTelemetryRetentionCleanup(retentionDays)
		for {
			select {
			case <-ticker.C:
				s.runTelemetryRetentionCleanup(retentionDays)
			case <-stopCh:
				return
			}
		}
	}()
}

func (s *Server) runTelemetryRetentionCleanup(retentionDays int) {
	if retentionDays <= 0 || s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), telemetryRetentionCleanupTimeout)
	defer cancel()

	deleted, err := dbpkg.NewQueries(s.db).DeleteTelemetryEventsOlderThanDays(ctx, retentionDays)
	if err != nil {
		s.logger.Warn("telemetry retention cleanup failed", "retention_days", retentionDays, "error", err)
		return
	}
	if deleted > 0 {
		s.logger.Info("telemetry retention cleanup complete", "retention_days", retentionDays, "deleted", deleted)
	}
}

func (s *Server) stopTelemetryRetentionCleanupLoop(ctx context.Context) error {
	stopCh := s.telemetryCleanupStop
	doneCh := s.telemetryCleanupDone
	if stopCh == nil || doneCh == nil {
		return nil
	}
	s.telemetryCleanupStop = nil
	s.telemetryCleanupDone = nil

	close(stopCh)
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
