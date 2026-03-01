package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	dbpkg "github.com/benedict2310/htmlctl/internal/db"
)

func TestBackendsAddListRemove(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	reloader := &fakeCaddyReloader{}
	srv.caddyReloader = reloader

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/backends", "application/json", bytes.NewBufferString(`{"pathPrefix":"/api/*","upstream":"https://api.example.com"}`))
	if err != nil {
		t.Fatalf("POST /backends error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(body))
	}
	var created backendResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()
	if created.PathPrefix != "/api/*" || created.Upstream != "https://api.example.com" {
		t.Fatalf("unexpected create response: %#v", created)
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/environments/staging/backends", "application/json", bytes.NewBufferString(`{"pathPrefix":"/api/*","upstream":"https://api-v2.example.com"}`))
	if err != nil {
		t.Fatalf("POST /backends upsert error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200 on upsert, got %d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	listResp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/backends")
	if err != nil {
		t.Fatalf("GET /backends error = %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		listResp.Body.Close()
		t.Fatalf("expected 200 list, got %d body=%s", listResp.StatusCode, string(body))
	}
	var listed backendsResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		listResp.Body.Close()
		t.Fatalf("decode list response: %v", err)
	}
	listResp.Body.Close()
	if len(listed.Backends) != 1 || listed.Backends[0].Upstream != "https://api-v2.example.com" {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/backends?path=%2Fapi%2F%2A", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /backends error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 delete, got %d", deleteResp.StatusCode)
	}
	if reloader.count != 3 {
		t.Fatalf("expected 3 caddy reload calls, got %d", reloader.count)
	}
}

func TestBackendsListEmpty(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/backends")
	if err != nil {
		t.Fatalf("GET /backends error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}
	var listed backendsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		resp.Body.Close()
		t.Fatalf("decode response: %v", err)
	}
	resp.Body.Close()
	if len(listed.Backends) != 0 {
		t.Fatalf("expected empty backends, got %#v", listed.Backends)
	}
}

func TestBackendsValidationAndMissingPath(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "invalid path prefix", method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/backends", body: `{"pathPrefix":"/api/","upstream":"https://api.example.com"}`},
		{name: "invalid upstream", method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/backends", body: `{"pathPrefix":"/api/*","upstream":"ftp://api.example.com"}`},
		{name: "missing delete path", method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/backends"},
		{name: "empty delete path", method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/backends?path="},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			}
			req, err := http.NewRequest(tc.method, tc.target, body)
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
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestBackendsDeleteMalformedQueryAndNotFound(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/backends", nil)
	if err != nil {
		t.Fatalf("new malformed delete request: %v", err)
	}
	req.URL.RawQuery = "path=%ZZ"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE malformed query error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed query, got %d", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/backends?path=%2Fapi%2F%2A", nil)
	if err != nil {
		t.Fatalf("new missing delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE missing backend error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing backend, got %d", resp.StatusCode)
	}
}

func TestBackendsReloadFailureDoesNotRollback(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{err: context.DeadlineExceeded}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/backends", "application/json", bytes.NewBufferString(`{"pathPrefix":"/api/*","upstream":"https://api.example.com"}`))
	if err != nil {
		t.Fatalf("POST /backends error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 even when reload fails, got %d", resp.StatusCode)
	}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	row, err := q.GetBackendByPathPrefix(context.Background(), envRow.ID, "/api/*")
	if err != nil {
		t.Fatalf("GetBackendByPathPrefix() error = %v", err)
	}
	if row.Upstream != "https://api.example.com" {
		t.Fatalf("unexpected persisted backend: %#v", row)
	}
}

func TestBackendsAuthRequired(t *testing.T) {
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "info", DBWAL: true, APIToken: "secret-token"}
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

	baseURL := "http://" + srv.Addr()
	for _, tc := range []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: baseURL + "/api/v1/websites/sample/environments/staging/backends"},
		{method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/backends", body: `{"pathPrefix":"/api/*","upstream":"https://api.example.com"}`},
		{method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/backends?path=%2Fapi%2F%2A"},
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

func TestBackendsSanitizeInternalErrors(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/backends", "application/json", bytes.NewBufferString(`{"pathPrefix":"/api/*","upstream":"https://api.example.com"}`))
	if err != nil {
		t.Fatalf("POST /backends error = %v", err)
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
