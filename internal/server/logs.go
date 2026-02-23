package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/benedict2310/htmlctl/internal/audit"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

type logsResponse struct {
	Entries []auditEntryResponse `json:"entries"`
	Total   int                  `json:"total"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
}

type auditEntryResponse struct {
	ID              int64          `json:"id"`
	Actor           string         `json:"actor"`
	Timestamp       string         `json:"timestamp"`
	EnvironmentID   *int64         `json:"environmentId,omitempty"`
	Operation       string         `json:"operation"`
	ResourceSummary string         `json:"resourceSummary"`
	ReleaseID       *string        `json:"releaseId,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	pathValue := r.URL.EscapedPath()
	if pathValue == "" {
		pathValue = r.URL.Path
	}
	website, env, envScoped, ok, err := parseLogsPath(pathValue)
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
	if s.db == nil || s.auditLogger == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "server is not ready", nil)
		return
	}

	q := dbpkg.NewQueries(s.db)
	websiteRow, err := q.GetWebsiteByName(r.Context(), website)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("website %q not found", website), nil)
		return
	}

	var envID *int64
	if envScoped {
		envRow, err := q.GetEnvironmentByName(r.Context(), websiteRow.ID, env)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("environment %q not found", env), nil)
			return
		}
		envID = &envRow.ID
	}

	filter, err := parseAuditFilter(r.URL.Query())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters", []string{err.Error()})
		return
	}
	filter.WebsiteID = websiteRow.ID
	filter.EnvironmentID = envID

	res, err := s.auditLogger.Query(r.Context(), filter)
	if err != nil {
		s.writeInternalAPIError(w, r, "query audit logs failed", err, "website", website, "environment", env)
		return
	}

	entries := make([]auditEntryResponse, 0, len(res.Entries))
	for _, entry := range res.Entries {
		entries = append(entries, auditEntryResponse{
			ID:              entry.ID,
			Actor:           entry.Actor,
			Timestamp:       entry.Timestamp.UTC().Format(time.RFC3339Nano),
			EnvironmentID:   entry.EnvironmentID,
			Operation:       entry.Operation,
			ResourceSummary: entry.ResourceSummary,
			ReleaseID:       entry.ReleaseID,
			Metadata:        entry.Metadata,
		})
	}

	writeJSON(w, http.StatusOK, logsResponse{
		Entries: entries,
		Total:   res.Total,
		Limit:   res.Limit,
		Offset:  res.Offset,
	})
}

func parseLogsPath(pathValue string) (website string, env string, envScoped bool, ok bool, err error) {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) == 7 {
		if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "environments" || parts[6] != "logs" {
			return "", "", false, false, nil
		}
		website = strings.TrimSpace(parts[3])
		env = strings.TrimSpace(parts[5])
		if strings.TrimSpace(website) == "" || strings.TrimSpace(env) == "" {
			return "", "", false, false, nil
		}
		if err := validateResourceName(website); err != nil {
			return website, env, false, false, fmt.Errorf("invalid website name %q: %w", website, err)
		}
		if err := validateResourceName(env); err != nil {
			return website, env, false, false, fmt.Errorf("invalid environment name %q: %w", env, err)
		}
		return website, env, true, true, nil
	}
	if len(parts) == 5 {
		if parts[0] != "api" || parts[1] != "v1" || parts[2] != "websites" || parts[4] != "logs" {
			return "", "", false, false, nil
		}
		website = strings.TrimSpace(parts[3])
		if strings.TrimSpace(website) == "" {
			return "", "", false, false, nil
		}
		if err := validateResourceName(website); err != nil {
			return website, "", false, false, fmt.Errorf("invalid website name %q: %w", website, err)
		}
		return website, "", false, true, nil
	}
	return "", "", false, false, nil
}

func parseAuditFilter(values url.Values) (audit.Filter, error) {
	filter := audit.Filter{}
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return filter, fmt.Errorf("invalid limit: %w", err)
		}
		if v < 0 {
			return filter, fmt.Errorf("limit must be >= 0")
		}
		filter.Limit = v
	}
	if raw := strings.TrimSpace(values.Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return filter, fmt.Errorf("invalid offset: %w", err)
		}
		if v < 0 {
			return filter, fmt.Errorf("offset must be >= 0")
		}
		filter.Offset = v
	}
	filter.Operation = strings.TrimSpace(values.Get("operation"))
	if raw := strings.TrimSpace(values.Get("since")); raw != "" {
		t, err := parseRFC3339Timestamp(raw)
		if err != nil {
			return filter, fmt.Errorf("invalid since: %w", err)
		}
		filter.Since = &t
	}
	if raw := strings.TrimSpace(values.Get("until")); raw != "" {
		t, err := parseRFC3339Timestamp(raw)
		if err != nil {
			return filter, fmt.Errorf("invalid until: %w", err)
		}
		filter.Until = &t
	}
	return filter, nil
}

func parseRFC3339Timestamp(v string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, v)
}
