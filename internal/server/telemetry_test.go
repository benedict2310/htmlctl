package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestTelemetryIngestAcceptedAndPersisted(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	envID := seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	occurred := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	payload := `{"events":[{"name":"page_view","path":"/pricing/../pricing","occurredAt":"` + occurred + `","sessionId":"session_123","attrs":{"source":"landing"}}]}`
	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Host = "futurelab.studio"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, string(b))
	}
	var out map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if out["accepted"] != 1 {
		t.Fatalf("expected accepted=1, got %#v", out)
	}

	rows, err := dbpkg.NewQueries(srv.db).ListTelemetryEvents(context.Background(), dbpkg.ListTelemetryEventsParams{
		EnvironmentID: envID,
		Limit:         10,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("ListTelemetryEvents() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one telemetry row, got %d", len(rows))
	}
	if rows[0].EventName != "page_view" {
		t.Fatalf("unexpected event name: %q", rows[0].EventName)
	}
	if rows[0].Path != "/pricing" {
		t.Fatalf("expected normalized path /pricing, got %q", rows[0].Path)
	}
}

func TestTelemetryIngestNoAuthRequired(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = "futurelab.studio"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected telemetry ingest to bypass API auth middleware, got 401 body=%s", string(b))
	}
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryReadRequiresAuth(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/api/v1/websites/futurelab/environments/staging/telemetry/events")
	if err != nil {
		t.Fatalf("GET telemetry events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401 for unauthenticated telemetry read, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryIngestValidationAndSanitizedErrors(t *testing.T) {
	t.Run("too many events", func(t *testing.T) {
		srv := startTelemetryServer(t, Config{
			BindAddr: "127.0.0.1",
			Port:     0,
			DataDir:  t.TempDir(),
			LogLevel: "info",
			DBWAL:    true,
			Telemetry: TelemetryConfig{
				Enabled:       true,
				MaxBodyBytes:  64 * 1024,
				MaxEvents:     1,
				RetentionDays: 0,
			},
		})
		baseURL := "http://" + srv.Addr()
		seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

		req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"},{"name":"page_view","path":"/x"}]}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Host = "futurelab.studio"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /collect/v1/events error = %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
		}
	})

	t.Run("request too large", func(t *testing.T) {
		srv := startTelemetryServer(t, Config{
			BindAddr: "127.0.0.1",
			Port:     0,
			DataDir:  t.TempDir(),
			LogLevel: "info",
			DBWAL:    true,
			Telemetry: TelemetryConfig{
				Enabled:       true,
				MaxBodyBytes:  64,
				MaxEvents:     50,
				RetentionDays: 0,
			},
		})
		baseURL := "http://" + srv.Addr()
		seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

		largeAttrs := strings.Repeat("a", 1024)
		payload := `{"events":[{"name":"page_view","path":"/","attrs":{"x":"` + largeAttrs + `"}}]}`
		req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Host = "futurelab.studio"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /collect/v1/events error = %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 413, got %d body=%s", resp.StatusCode, string(b))
		}
	})

	t.Run("unresolved host", func(t *testing.T) {
		srv := startTelemetryServer(t, Config{
			BindAddr: "127.0.0.1",
			Port:     0,
			DataDir:  t.TempDir(),
			LogLevel: "info",
			DBWAL:    true,
			Telemetry: TelemetryConfig{
				Enabled:       true,
				MaxBodyBytes:  64 * 1024,
				MaxEvents:     50,
				RetentionDays: 0,
			},
		})
		baseURL := "http://" + srv.Addr()

		req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Host = "missing.example.com"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /collect/v1/events error = %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
		}
	})

	t.Run("db error is sanitized", func(t *testing.T) {
		srv := startTelemetryServer(t, Config{
			BindAddr: "127.0.0.1",
			Port:     0,
			DataDir:  t.TempDir(),
			LogLevel: "info",
			DBWAL:    true,
			Telemetry: TelemetryConfig{
				Enabled:       true,
				MaxBodyBytes:  64 * 1024,
				MaxEvents:     50,
				RetentionDays: 0,
			},
		})
		baseURL := "http://" + srv.Addr()
		seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")
		if err := srv.db.Close(); err != nil {
			t.Fatalf("Close() db error = %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Host = "futurelab.studio"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /collect/v1/events error = %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(b))
		}
		rawBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		if strings.Contains(string(rawBody), "database is closed") {
			t.Fatalf("response leaked internal database detail: %s", string(rawBody))
		}
		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["error"] != "telemetry ingest failed" {
			t.Fatalf("expected sanitized error message, got %#v", payload)
		}
	})
}

func TestTelemetryReadListFilters(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	envID := seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()
	idA, err := q.InsertTelemetryEvent(ctx, dbpkg.TelemetryEventRow{EnvironmentID: envID, EventName: "page_view", Path: "/", AttrsJSON: `{"source":"home"}`})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(a) error = %v", err)
	}
	idB, err := q.InsertTelemetryEvent(ctx, dbpkg.TelemetryEventRow{EnvironmentID: envID, EventName: "cta_click", Path: "/pricing", AttrsJSON: `{"button":"buy"}`})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(b) error = %v", err)
	}
	if _, err := srv.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2026-02-20T12:00:00Z", idA); err != nil {
		t.Fatalf("update telemetry received_at(a): %v", err)
	}
	if _, err := srv.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2026-02-22T12:00:00Z", idB); err != nil {
		t.Fatalf("update telemetry received_at(b): %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/websites/futurelab/environments/staging/telemetry/events?event=cta_click&since=2026-02-21T00:00:00Z&until=2026-02-23T00:00:00Z&limit=10&offset=0", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET telemetry events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}

	var out telemetryEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode telemetry list response: %v", err)
	}
	if out.Website != "futurelab" || out.Environment != "staging" {
		t.Fatalf("unexpected response identity: %#v", out)
	}
	if len(out.Events) != 1 {
		t.Fatalf("expected 1 filtered event, got %d", len(out.Events))
	}
	if out.Events[0].ID != idB || out.Events[0].Name != "cta_click" {
		t.Fatalf("unexpected filtered event: %#v", out.Events[0])
	}
	if out.Events[0].Attrs["button"] != "buy" {
		t.Fatalf("unexpected attrs payload: %#v", out.Events[0].Attrs)
	}
}

func TestParseTelemetryHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites/futurelab/environments/staging/telemetry/events", nil)
	limit, offset, err := parseListTelemetryPagination(req)
	if err != nil {
		t.Fatalf("parseListTelemetryPagination(default) error = %v", err)
	}
	if limit != defaultTelemetryListLimit || offset != 0 {
		t.Fatalf("unexpected default pagination: limit=%d offset=%d", limit, offset)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/websites/futurelab/environments/staging/telemetry/events?limit=99999&offset=7", nil)
	limit, offset, err = parseListTelemetryPagination(req)
	if err != nil {
		t.Fatalf("parseListTelemetryPagination(clamped) error = %v", err)
	}
	if limit != maxTelemetryListLimit || offset != 7 {
		t.Fatalf("unexpected clamped pagination: limit=%d offset=%d", limit, offset)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/websites/futurelab/environments/staging/telemetry/events?limit=-1", nil)
	if _, _, err := parseListTelemetryPagination(req); err == nil {
		t.Fatalf("expected negative limit error")
	}

	website, env, ok, err := parseTelemetryEventsPath("/api/v1/websites/futurelab/environments/staging/telemetry/events")
	if err != nil || !ok {
		t.Fatalf("expected telemetry path parse success, got ok=%v err=%v", ok, err)
	}
	if website != "futurelab" || env != "staging" {
		t.Fatalf("unexpected telemetry parse result website=%q env=%q", website, env)
	}
	if _, _, ok, _ := parseTelemetryEventsPath("/api/v1/websites/futurelab/environments/staging/telemetry/event"); ok {
		t.Fatalf("expected invalid telemetry path to fail parsing")
	}
}

func TestRunTelemetryRetentionCleanup(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 30,
		},
	})
	envID := seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()
	oldID, err := q.InsertTelemetryEvent(ctx, dbpkg.TelemetryEventRow{EnvironmentID: envID, EventName: "page_view", Path: "/old", AttrsJSON: `{}`})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(old) error = %v", err)
	}
	newID, err := q.InsertTelemetryEvent(ctx, dbpkg.TelemetryEventRow{EnvironmentID: envID, EventName: "page_view", Path: "/new", AttrsJSON: `{}`})
	if err != nil {
		t.Fatalf("InsertTelemetryEvent(new) error = %v", err)
	}
	if _, err := srv.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", oldID); err != nil {
		t.Fatalf("update old telemetry timestamp: %v", err)
	}
	if _, err := srv.db.ExecContext(ctx, `UPDATE telemetry_events SET received_at = ? WHERE id = ?`, "2099-01-01T00:00:00Z", newID); err != nil {
		t.Fatalf("update new telemetry timestamp: %v", err)
	}

	srv.runTelemetryRetentionCleanup(30)

	rows, err := q.ListTelemetryEvents(ctx, dbpkg.ListTelemetryEventsParams{EnvironmentID: envID, Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("ListTelemetryEvents() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != newID {
		t.Fatalf("expected only new telemetry row %d to remain, got %#v", newID, rows)
	}
}

func startTelemetryServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if srv.db != nil {
			_ = srv.db.PingContext(context.Background())
		}
		_ = srv.Shutdown(ctx)
	})
	return srv
}

func seedTelemetryEnvironment(t *testing.T, srv *Server, website, environment, domain string) int64 {
	t.Helper()
	seedDomainWebsiteEnv(t, srv, website, environment)
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, environment)
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if domain != "" {
		if _, err := q.InsertDomainBinding(ctx, dbpkg.DomainBindingRow{Domain: domain, EnvironmentID: envRow.ID}); err != nil {
			t.Fatalf("InsertDomainBinding(%q) error = %v", domain, err)
		}
	}
	return envRow.ID
}

func TestNormalizeTelemetryHostRejectsIP(t *testing.T) {
	cases := []string{
		"127.0.0.1:8080",
		"[::1]",
		"[::1]:8080",
	}
	for _, raw := range cases {
		if _, err := normalizeTelemetryHost(raw); err == nil {
			t.Fatalf("expected IP host normalization to fail for %q", raw)
		}
	}
}

func TestBuildTelemetryEventRowValidation(t *testing.T) {
	now := time.Now().UTC()

	if _, err := buildTelemetryEventRow(1, telemetryIngestEvent{Name: "bad name", Path: "/"}, now); err == nil {
		t.Fatalf("expected invalid event name to fail")
	}
	if _, err := buildTelemetryEventRow(1, telemetryIngestEvent{Name: "page_view", Path: "not/absolute"}, now); err == nil {
		t.Fatalf("expected invalid path to fail")
	}
	if _, err := buildTelemetryEventRow(1, telemetryIngestEvent{Name: "page_view", Path: "/", SessionID: "bad session id!"}, now); err == nil {
		t.Fatalf("expected invalid session ID to fail")
	}
	if _, err := buildTelemetryEventRow(1, telemetryIngestEvent{Name: "page_view", Path: "/", OccurredAt: now.Add(48 * time.Hour).Format(time.RFC3339)}, now); err == nil {
		t.Fatalf("expected too-far-future occurredAt to fail")
	}
	if _, err := buildTelemetryEventRow(1, telemetryIngestEvent{Name: "page_view", Path: "/", Attrs: map[string]string{"bad key": "x"}}, now); err == nil {
		t.Fatalf("expected invalid attrs key to fail")
	}
}

func TestTelemetryReadSanitizesAttrsUnmarshalFailure(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	envID := seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")
	if _, err := srv.db.ExecContext(context.Background(), `
INSERT INTO telemetry_events(environment_id, event_name, path, attrs_json)
VALUES(?, 'page_view', '/', '{bad')`, envID); err != nil {
		t.Fatalf("insert malformed telemetry attrs row: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/websites/futurelab/environments/staging/telemetry/events", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET telemetry events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(b))
	}
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if strings.Contains(string(rawBody), "invalid character") {
		t.Fatalf("response leaked internal JSON decode details: %s", string(rawBody))
	}
}

func TestResolveTelemetryEnvironmentIDNotFound(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})

	_, err := srv.resolveTelemetryEnvironmentID(context.Background(), "futurelab.studio")
	if !errors.Is(err, errTelemetryHostNotBound) {
		t.Fatalf("expected errTelemetryHostNotBound, got %v", err)
	}
}

func TestTelemetryRetentionLoopStopsOnShutdown(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 1,
		},
	})

	if srv.telemetryCleanupStop == nil || srv.telemetryCleanupDone == nil {
		t.Fatalf("expected telemetry cleanup loop channels to be initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.stopTelemetryRetentionCleanupLoop(ctx); err != nil {
		t.Fatalf("stopTelemetryRetentionCleanupLoop() error = %v", err)
	}
	if srv.telemetryCleanupStop != nil || srv.telemetryCleanupDone != nil {
		t.Fatalf("expected telemetry cleanup loop channels to be cleared")
	}
}

func TestTelemetryIngestHostWithPort(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = "futurelab.studio:443"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryIngestAcceptsSendBeaconContentType(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = "futurelab.studio"
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryIngestRejectsUnsupportedContentType(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = "futurelab.studio"
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 415, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryIngestRejectsCrossOrigin(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedTelemetryEnvironment(t, srv, "futurelab", "staging", "futurelab.studio")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/collect/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = "futurelab.studio"
	req.Header.Set("Origin", "https://www.futurelab.studio")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryIngestMethodNotAllowed(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()

	resp, err := http.Get(baseURL + "/collect/v1/events")
	if err != nil {
		t.Fatalf("GET /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 405, got %d body=%s", resp.StatusCode, string(b))
	}
}

func TestTelemetryZeroLimitSettingsUseDefaults(t *testing.T) {
	s := &Server{
		cfg: Config{
			Telemetry: TelemetryConfig{
				MaxBodyBytes: 0,
				MaxEvents:    0,
			},
		},
	}
	if got := s.telemetryMaxBodyBytes(); got != DefaultTelemetryMaxBodyBytes {
		t.Fatalf("telemetryMaxBodyBytes() = %d, want %d", got, DefaultTelemetryMaxBodyBytes)
	}
	if got := s.telemetryMaxEvents(); got != DefaultTelemetryMaxEvents {
		t.Fatalf("telemetryMaxEvents() = %d, want %d", got, DefaultTelemetryMaxEvents)
	}
}

func TestTelemetryIngestOptionsReturnsSameOriginGuidance(t *testing.T) {
	srv := startTelemetryServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Telemetry: TelemetryConfig{
			Enabled:       true,
			MaxBodyBytes:  64 * 1024,
			MaxEvents:     50,
			RetentionDays: 0,
		},
	})
	baseURL := "http://" + srv.Addr()

	req, err := http.NewRequest(http.MethodOptions, baseURL+"/collect/v1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /collect/v1/events error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode, string(b))
	}
}
