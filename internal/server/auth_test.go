package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name            string
		configuredToken string
		authHeader      string
		wantStatus      int
		wantNextCall    bool
	}{
		{name: "valid bearer", configuredToken: "secret-token", authHeader: "Bearer secret-token", wantStatus: http.StatusNoContent, wantNextCall: true},
		{name: "missing header", configuredToken: "secret-token", authHeader: "", wantStatus: http.StatusUnauthorized, wantNextCall: false},
		{name: "wrong token", configuredToken: "secret-token", authHeader: "Bearer wrong", wantStatus: http.StatusUnauthorized, wantNextCall: false},
		{name: "trailing garbage", configuredToken: "secret-token", authHeader: "Bearer secret-token extra", wantStatus: http.StatusUnauthorized, wantNextCall: false},
		{name: "empty configured token: all requests pass", configuredToken: "", authHeader: "", wantStatus: http.StatusNoContent, wantNextCall: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := authMiddleware(tc.configuredToken, slog.New(slog.NewTextHandler(io.Discard, nil)))(next)
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
			if nextCalled != tc.wantNextCall {
				t.Fatalf("next called=%v want=%v", nextCalled, tc.wantNextCall)
			}
			if tc.wantStatus == http.StatusUnauthorized {
				var payload map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode unauthorized payload: %v", err)
				}
				if payload["error"] != "unauthorized" {
					t.Fatalf("unexpected payload: %#v", payload)
				}
			}
		})
	}
}

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantToken  string
		wantParsed bool
	}{
		{
			name:       "empty token",
			header:     "Bearer ",
			wantToken:  "",
			wantParsed: false,
		},
		{
			name:       "lowercase scheme",
			header:     "bearer secret",
			wantToken:  "secret",
			wantParsed: true,
		},
		{
			name:       "double space",
			header:     "Bearer  secret",
			wantToken:  "secret",
			wantParsed: true,
		},
		{
			name:       "non-ascii token",
			header:     "Bearer s3crèt",
			wantToken:  "s3crèt",
			wantParsed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotToken, gotParsed := parseBearerToken(tc.header)
			if gotParsed != tc.wantParsed {
				t.Fatalf("parsed=%v want=%v", gotParsed, tc.wantParsed)
			}
			if gotToken != tc.wantToken {
				t.Fatalf("token=%q want=%q", gotToken, tc.wantToken)
			}
		})
	}
}

func TestActorFromRequestTrustedOnlyAfterMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
	req.Header.Set("X-Actor", "forged")
	if got := actorFromRequest(req); got != "local" {
		t.Fatalf("expected local actor without auth middleware, got %q", got)
	}
}

func TestAuthIntegrationHealthBypassAndAPIGuard(t *testing.T) {
	cfg := Config{
		BindAddr: "127.0.0.1",
		Port:     0,
		DataDir:  t.TempDir(),
		LogLevel: "info",
		DBWAL:    true,
		APIToken: "secret-token",
	}
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

	healthResp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", healthResp.StatusCode)
	}

	readyResp, err := http.Get(baseURL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz error = %v", err)
	}
	readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected /readyz 200, got %d", readyResp.StatusCode)
	}

	apiResp, err := http.Get(baseURL + "/api/v1/websites")
	if err != nil {
		t.Fatalf("GET /api/v1/websites error = %v", err)
	}
	apiResp.Body.Close()
	if apiResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated API call to return 401, got %d", apiResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/websites", nil)
	if err != nil {
		t.Fatalf("new authorized request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")
	okResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authorized request failed: %v", err)
	}
	okResp.Body.Close()
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("expected authorized API request to pass, got %d", okResp.StatusCode)
	}
}
