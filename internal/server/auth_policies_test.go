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

	"github.com/benedict2310/htmlctl/internal/audit"
	dbpkg "github.com/benedict2310/htmlctl/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthPoliciesAddListRemove(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	reloader := &fakeCaddyReloader{}
	srv.caddyReloader = reloader

	hash := mustBcryptHash(t, "secret-one")
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+hash+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(body))
	}
	var created authPolicyResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()
	if created.PathPrefix != "/docs/*" || created.Username != "reviewer" {
		t.Fatalf("unexpected create response: %#v", created)
	}

	listResp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/auth-policies")
	if err != nil {
		t.Fatalf("GET /auth-policies error = %v", err)
	}
	body, _ := io.ReadAll(listResp.Body)
	if listResp.StatusCode != http.StatusOK {
		listResp.Body.Close()
		t.Fatalf("expected 200 list, got %d body=%s", listResp.StatusCode, string(body))
	}
	if bytes.Contains(body, []byte("$2")) {
		listResp.Body.Close()
		t.Fatalf("expected list response to omit hash material, got %s", string(body))
	}
	var listed authPoliciesResponse
	if err := json.Unmarshal(body, &listed); err != nil {
		listResp.Body.Close()
		t.Fatalf("decode list response: %v", err)
	}
	listResp.Body.Close()
	if len(listed.AuthPolicies) != 1 || listed.AuthPolicies[0].Username != "reviewer" {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/auth-policies?path=%2Fdocs%2F%2A", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /auth-policies error = %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 delete, got %d", deleteResp.StatusCode)
	}
	if reloader.count != 2 {
		t.Fatalf("expected 2 caddy reload calls, got %d", reloader.count)
	}
}

func TestAuthPoliciesValidationAndMissingPath(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "invalid path prefix", method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies", body: `{"pathPrefix":"/docs/","username":"reviewer","passwordHash":"` + mustBcryptHash(t, "secret-two") + `"}`},
		{name: "invalid username", method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies", body: `{"pathPrefix":"/docs/*","username":"review er","passwordHash":"` + mustBcryptHash(t, "secret-three") + `"}`},
		{name: "invalid hash", method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies", body: `{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"not-bcrypt"}`},
		{name: "missing delete path", method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies"},
		{name: "empty delete path", method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies?path="},
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

func TestAuthPoliciesOverlapValidation(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	hash := mustBcryptHash(t, "secret-four")
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+hash+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies create error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/private/*","username":"reviewer2","passwordHash":"`+mustBcryptHash(t, "secret-five")+`"}`))
	if err != nil {
		t.Fatalf("POST overlapping /auth-policies error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for overlapping auth policy, got %d", resp.StatusCode)
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
	if err := q.UpsertBackend(context.Background(), dbpkg.BackendRow{
		EnvironmentID: envRow.ID,
		PathPrefix:    "/api/internal/*",
		Upstream:      "https://api.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend() error = %v", err)
	}

	resp, err = http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/api/*","username":"reviewer3","passwordHash":"`+mustBcryptHash(t, "secret-six")+`"}`))
	if err != nil {
		t.Fatalf("POST backend-overlap /auth-policies error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for backend-overlap auth policy, got %d", resp.StatusCode)
	}
}

func TestAuthPoliciesRejectTelemetryOverlap(t *testing.T) {
	srv := startTestServer(t)
	srv.cfg.Telemetry.Enabled = true
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/collect/*","username":"reviewer","passwordHash":"`+mustBcryptHash(t, "secret-telemetry")+`"}`))
	if err != nil {
		t.Fatalf("POST telemetry-overlap /auth-policies error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for telemetry-overlap auth policy, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestAuthPoliciesExactBackendPathAllowed(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{}

	q := dbpkg.NewQueries(srv.db)
	websiteRow, err := q.GetWebsiteByName(context.Background(), "sample")
	if err != nil {
		t.Fatalf("GetWebsiteByName() error = %v", err)
	}
	envRow, err := q.GetEnvironmentByName(context.Background(), websiteRow.ID, "staging")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if err := q.UpsertBackend(context.Background(), dbpkg.BackendRow{
		EnvironmentID: envRow.ID,
		PathPrefix:    "/docs/*",
		Upstream:      "https://api.example.com",
	}); err != nil {
		t.Fatalf("UpsertBackend() error = %v", err)
	}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+mustBcryptHash(t, "secret-seven")+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for exact backend path auth policy, got %d", resp.StatusCode)
	}
}

func TestAuthPoliciesRollbackWhenReloadFails(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	srv.caddyReloader = &fakeCaddyReloader{err: context.DeadlineExceeded}

	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+mustBcryptHash(t, "secret-eight")+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
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
	rows, err := q.ListAuthPoliciesByEnvironment(context.Background(), envRow.ID)
	if err != nil {
		t.Fatalf("ListAuthPoliciesByEnvironment() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected auth policy create rollback to leave no rows, got %#v", rows)
	}
}

func TestAuthPoliciesRemoveRollbackWhenReloadFails(t *testing.T) {
	srv := startTestServer(t)
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
	if err := q.UpsertAuthPolicy(context.Background(), dbpkg.AuthPolicyRow{
		EnvironmentID: envRow.ID,
		PathPrefix:    "/docs/*",
		Username:      "reviewer",
		PasswordHash:  mustBcryptHash(t, "secret-nine"),
	}); err != nil {
		t.Fatalf("UpsertAuthPolicy() error = %v", err)
	}
	srv.caddyReloader = &fakeCaddyReloader{err: context.DeadlineExceeded}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/auth-policies?path=%2Fdocs%2F%2A", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /auth-policies error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", resp.StatusCode, string(body))
	}

	row, err := q.GetAuthPolicyByPathPrefix(context.Background(), envRow.ID, "/docs/*")
	if err != nil {
		t.Fatalf("GetAuthPolicyByPathPrefix() error = %v", err)
	}
	if row.Username != "reviewer" {
		t.Fatalf("unexpected restored auth policy row: %#v", row)
	}
}

func TestAuthPoliciesAuditOmitsPasswordHash(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")
	auditLogger := &captureApplyAuditLogger{}
	srv.auditLogger = auditLogger
	srv.caddyReloader = &fakeCaddyReloader{}

	hash := mustBcryptHash(t, "secret-ten")
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+hash+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if auditLogger.entry.Operation != audit.OperationAuthPolicyAdd {
		t.Fatalf("unexpected audit operation %#v", auditLogger.entry)
	}
	if auditLogger.entry.Metadata["username"] != "reviewer" || auditLogger.entry.Metadata["pathPrefix"] != "/docs/*" {
		t.Fatalf("unexpected audit metadata %#v", auditLogger.entry.Metadata)
	}
	if _, ok := auditLogger.entry.Metadata["passwordHash"]; ok {
		t.Fatalf("did not expect password hash in audit metadata: %#v", auditLogger.entry.Metadata)
	}
	if bytes.Contains([]byte(auditLogger.entry.ResourceSummary), []byte("$2")) {
		t.Fatalf("did not expect password hash in audit summary: %q", auditLogger.entry.ResourceSummary)
	}
}

func TestAuthPoliciesAuthRequired(t *testing.T) {
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
		{method: http.MethodGet, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies"},
		{method: http.MethodPost, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies", body: `{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"` + mustBcryptHash(t, "secret-eleven") + `"}`},
		{method: http.MethodDelete, target: baseURL + "/api/v1/websites/sample/environments/staging/auth-policies?path=%2Fdocs%2F%2A"},
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

func TestAuthPoliciesCreateSanitizeInternalErrors(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}
	resp, err := http.Post(baseURL+"/api/v1/websites/sample/environments/staging/auth-policies", "application/json", bytes.NewBufferString(`{"pathPrefix":"/docs/*","username":"reviewer","passwordHash":"`+mustBcryptHash(t, "secret-twelve")+`"}`))
	if err != nil {
		t.Fatalf("POST /auth-policies error = %v", err)
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

func TestAuthPoliciesListSanitizeInternalErrors(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}
	resp, err := http.Get(baseURL + "/api/v1/websites/sample/environments/staging/auth-policies")
	if err != nil {
		t.Fatalf("GET /auth-policies error = %v", err)
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

func TestAuthPoliciesDeleteSanitizeInternalErrors(t *testing.T) {
	srv := startTestServer(t)
	baseURL := "http://" + srv.Addr()
	seedDomainWebsiteEnv(t, srv, "sample", "staging")

	if err := srv.db.Close(); err != nil {
		t.Fatalf("Close() db error = %v", err)
	}
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/websites/sample/environments/staging/auth-policies?path=%2Fdocs%2F%2A", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /auth-policies error = %v", err)
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

func mustBcryptHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}
