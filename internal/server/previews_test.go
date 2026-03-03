package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestPreviewsCreateListRemove(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	baseURL := "http://" + srv.Addr()
	reloader := &fakeCaddyReloader{}
	srv.caddyReloader = reloader
	seedPreviewRelease(t, srv, "sample", "staging", "R1")

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/previews", "application/json", bytes.NewBufferString(`{"releaseId":"R1","ttl":"72h"}`))
	if err != nil {
		t.Fatalf("POST /previews error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(body))
	}
	var created previewResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()
	if created.ID == 0 || created.Hostname == "" || created.ReleaseID != "R1" {
		t.Fatalf("unexpected create response: %#v", created)
	}
	if reloader.count != 1 {
		t.Fatalf("expected one caddy reload after create, got %d", reloader.count)
	}

	listResp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/previews")
	if err != nil {
		t.Fatalf("GET /previews error = %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		listResp.Body.Close()
		t.Fatalf("expected 200 list, got %d body=%s", listResp.StatusCode, string(body))
	}
	var listed previewsResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		listResp.Body.Close()
		t.Fatalf("decode list response: %v", err)
	}
	listResp.Body.Close()
	if len(listed.Previews) != 1 || listed.Previews[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/previews/"+itoa64(created.ID), nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /previews error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 delete, got %d", deleteResp.StatusCode)
	}
	if reloader.count != 2 {
		t.Fatalf("expected second caddy reload after delete, got %d", reloader.count)
	}
}

func TestPreviewsValidationAndMissingRelease(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if err := q.InsertRelease(context.Background(), dbpkg.ReleaseRow{
		ID:            "R-no-artifact",
		EnvironmentID: envRow.ID,
		ManifestJSON:  `{}`,
		OutputHashes:  `{}`,
		BuildLog:      "ok",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(R-no-artifact) error = %v", err)
	}

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid ttl", body: `{"releaseId":"R1","ttl":"30m"}`, wantStatus: http.StatusBadRequest},
		{name: "missing release", body: `{"releaseId":"missing","ttl":"72h"}`, wantStatus: http.StatusNotFound},
		{name: "missing release artifact", body: `{"releaseId":"R-no-artifact","ttl":"72h"}`, wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/previews", "application/json", bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatalf("POST /previews error = %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tc.wantStatus, resp.StatusCode, string(body))
			}
		})
	}
}

func TestPreviewsRequireAuthenticationWhenConfigured(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	baseURL := "http://" + srv.Addr()

	for _, tc := range []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: baseURL + "/api/v1/websites/sample/environments/staging/previews"},
		{method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/previews", body: `{"releaseId":"R1"}`},
		{method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/previews/7"},
	} {
		req, err := http.NewRequest(tc.method, tc.target, bytes.NewBufferString(tc.body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s error = %v", tc.method, tc.target, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s, got %d", tc.method, resp.StatusCode)
		}
	}
}

func TestPreviewsSanitizeInternalErrors(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	baseURL := "http://" + srv.Addr()
	seedPreviewRelease(t, srv, "sample", "staging", "R1")

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/previews", "application/json", bytes.NewBufferString(`{"releaseId":"R1"}`))
	if err != nil {
		t.Fatalf("POST /previews error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}
	if bytes.Contains(body, []byte("sqlite")) || bytes.Contains(body, []byte("htmlctl")) {
		t.Fatalf("expected sanitized 500 body, got %s", string(body))
	}
}

func TestPreviewCreateRollsBackWhenReloadFails(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	srv.caddyReloader = &fakeCaddyReloader{err: errors.New("reload failed")}
	baseURL := "http://" + srv.Addr()
	envID := seedPreviewRelease(t, srv, "sample", "staging", "R1")

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/previews", "application/json", bytes.NewBufferString(`{"releaseId":"R1","ttl":"72h"}`))
	if err != nil {
		t.Fatalf("POST /previews error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}

	rows, err := dbpkg.NewQueries(srv.db).ListReleasePreviewsByEnvironment(context.Background(), envID, formatPreviewTimestamp(time.Now().UTC()))
	if err != nil {
		t.Fatalf("ListReleasePreviewsByEnvironment() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected preview create rollback to leave no rows, got %#v", rows)
	}
}

func TestPreviewRemoveRollsBackWhenReloadFails(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	envID := seedPreviewRelease(t, srv, "sample", "staging", "R1")
	q := dbpkg.NewQueries(srv.db)
	previewID, err := q.InsertReleasePreview(context.Background(), dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R1",
		Hostname:      "active--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}
	srv.caddyReloader = &fakeCaddyReloader{err: errors.New("reload failed")}
	baseURL := "http://" + srv.Addr()

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/previews/"+itoa64(previewID), nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /previews error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}

	row, err := q.GetReleasePreviewByID(context.Background(), envID, previewID)
	if err != nil {
		t.Fatalf("GetReleasePreviewByID() error = %v", err)
	}
	if row.Hostname != "active--staging--sample.preview.example.com" {
		t.Fatalf("unexpected restored preview row: %#v", row)
	}
}

func TestRunPreviewCleanupDeletesExpiredRowsAndReloads(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	reloader := &fakeCaddyReloader{}
	srv.caddyReloader = reloader

	envID := seedPreviewRelease(t, srv, "sample", "staging", "R1")
	seedPreviewRelease(t, srv, "sample", "staging", "R2")
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()
	if _, err := q.InsertReleasePreview(ctx, dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R1",
		Hostname:      "expired--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2000-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview(expired) error = %v", err)
	}
	if _, err := q.InsertReleasePreview(ctx, dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R2",
		Hostname:      "active--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2099-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview(active) error = %v", err)
	}

	srv.runPreviewCleanup()

	rows, err := q.ListReleasePreviewsByEnvironment(ctx, envID, formatPreviewTimestamp(time.Now().UTC()))
	if err != nil {
		t.Fatalf("ListReleasePreviewsByEnvironment() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Hostname != "active--staging--sample.preview.example.com" {
		t.Fatalf("unexpected preview rows after cleanup: %#v", rows)
	}
	if reloader.count != 1 {
		t.Fatalf("expected one caddy reload after cleanup, got %d", reloader.count)
	}
}

func TestRunPreviewCleanupRetriesReloadAfterFailure(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	reloader := &fakeCaddyReloader{err: errors.New("reload failed")}
	srv.caddyReloader = reloader

	envID := seedPreviewRelease(t, srv, "sample", "staging", "R1")
	q := dbpkg.NewQueries(srv.db)
	if _, err := q.InsertReleasePreview(context.Background(), dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R1",
		Hostname:      "expired--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2000-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}

	srv.runPreviewCleanup()
	if reloader.count != 1 {
		t.Fatalf("expected first cleanup to attempt one reload, got %d", reloader.count)
	}

	reloader.err = nil
	srv.runPreviewCleanup()
	if reloader.count != 2 {
		t.Fatalf("expected second cleanup to retry reload, got %d", reloader.count)
	}

	srv.runPreviewCleanup()
	if reloader.count != 2 {
		t.Fatalf("expected no extra reload after pending flag cleared, got %d", reloader.count)
	}
}

func TestResolveTelemetryEnvironmentIDUsesActivePreview(t *testing.T) {
	srv := startPreviewFeatureServer(t, Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		Preview: PreviewConfig{
			Enabled:         true,
			BaseDomain:      "preview.example.com",
			DefaultTTLHours: 72,
			MaxTTLHours:     168,
		},
	})
	envID := seedPreviewRelease(t, srv, "sample", "staging", "R1")
	q := dbpkg.NewQueries(srv.db)
	if _, err := q.InsertReleasePreview(context.Background(), dbpkg.ReleasePreviewRow{
		EnvironmentID: envID,
		ReleaseID:     "R1",
		Hostname:      "active--staging--sample.preview.example.com",
		CreatedBy:     "alice",
		ExpiresAt:     "2099-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("InsertReleasePreview() error = %v", err)
	}

	resolvedEnvID, err := srv.resolveTelemetryEnvironmentID(context.Background(), "active--staging--sample.preview.example.com")
	if err != nil {
		t.Fatalf("resolveTelemetryEnvironmentID() error = %v", err)
	}
	if resolvedEnvID != envID {
		t.Fatalf("unexpected environment id %d want %d", resolvedEnvID, envID)
	}
}

func TestPreviewRouteDispatchDoesNotHijackWebsiteNames(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "previews-site", "staging")

	resp, err := http.Get(baseURL + "/api/v1/websites/previews-site/environments/staging/status")
	if err != nil {
		t.Fatalf("GET /status error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 status response, got %d body=%s", resp.StatusCode, string(body))
	}
}

func startPreviewFeatureServer(t *testing.T, cfg Config) *Server {
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
		_ = srv.Shutdown(ctx)
	})
	return srv
}

func seedPreviewRelease(t *testing.T, srv *Server, website, environment, releaseID string) int64 {
	t.Helper()
	seedDomainWebsiteEnv(t, srv, website, environment)
	q := dbpkg.NewQueries(srv.db)
	ctx := context.Background()

	websiteRow, err := q.GetWebsiteByName(ctx, website)
	if err != nil {
		t.Fatalf("GetWebsiteByName(%q) error = %v", website, err)
	}
	envRow, err := q.GetEnvironmentByName(ctx, websiteRow.ID, environment)
	if err != nil {
		t.Fatalf("GetEnvironmentByName(%q) error = %v", environment, err)
	}
	if _, err := q.GetReleaseByID(ctx, releaseID); err == nil {
		releaseRoot := filepath.Join(srv.dataPaths.WebsitesRoot, website, "envs", environment, "releases", releaseID)
		if err := os.MkdirAll(releaseRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", releaseRoot, err)
		}
		return envRow.ID
	}
	if err := q.InsertRelease(ctx, dbpkg.ReleaseRow{
		ID:            releaseID,
		EnvironmentID: envRow.ID,
		ManifestJSON:  `{}`,
		OutputHashes:  `{}`,
		BuildLog:      "ok",
		Status:        "active",
	}); err != nil {
		t.Fatalf("InsertRelease(%q) error = %v", releaseID, err)
	}
	releaseRoot := filepath.Join(srv.dataPaths.WebsitesRoot, website, "envs", environment, "releases", releaseID)
	if err := os.MkdirAll(releaseRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", releaseRoot, err)
	}
	return envRow.ID
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}
