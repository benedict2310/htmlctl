package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	ts := httptest.NewServer(New(Options{PublicBaseURL: "https://staging.example.com"}))
	t.Cleanup(ts.Close)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIngestAcceptedAndForwarded(t *testing.T) {
	var gotHost string
	var gotScheme string
	var gotContentType string
	var calls int32
	ts := httptest.NewServer(New(Options{
		PublicBaseURL: "https://staging.example.com",
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			atomic.AddInt32(&calls, 1)
			gotHost = req.Host
			gotScheme = req.Scheme
			gotContentType = req.ContentType
			return &ForwardResponse{StatusCode: http.StatusAccepted, ContentType: "application/json; charset=utf-8", Body: []byte(`{"accepted":1}`)}, nil
		}),
		Now: func() time.Time { return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC) },
	}))
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/htmlctl","occurredAt":"2026-03-09T11:59:00Z","sessionId":"sess_abc","attrs":{"source":"browser"}}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 forward call, got %d", calls)
	}
	if gotHost != "staging.example.com" {
		t.Fatalf("expected forwarded host staging.example.com, got %q", gotHost)
	}
	if gotScheme != "https" {
		t.Fatalf("expected forwarded scheme https, got %q", gotScheme)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected forwarded content type application/json, got %q", gotContentType)
	}
}

func TestIngestAcceptsTextPlainBeaconContentType(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		PublicBaseURL: "https://staging.example.com",
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			if req.ContentType != "text/plain;charset=UTF-8" {
				t.Fatalf("unexpected content type: %q", req.ContentType)
			}
			return &ForwardResponse{StatusCode: http.StatusAccepted, ContentType: "application/json; charset=utf-8", Body: []byte(`{"accepted":1}`)}, nil
		}),
	}))
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestIngestRejectsOriginMismatch(t *testing.T) {
	ts := httptest.NewServer(New(Options{PublicBaseURL: "https://staging.example.com"}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIngestRejectsHostMismatch(t *testing.T) {
	ts := httptest.NewServer(New(Options{PublicBaseURL: "https://staging.example.com"}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "other.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIngestRejectsDisallowedEvent(t *testing.T) {
	ts := httptest.NewServer(New(Options{PublicBaseURL: "https://staging.example.com"}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"purchase","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIngestRejectsInvalidBody(t *testing.T) {
	ts := httptest.NewServer(New(Options{PublicBaseURL: "https://staging.example.com"}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIngestRateLimit(t *testing.T) {
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	ts := httptest.NewServer(New(Options{
		PublicBaseURL:  "https://staging.example.com",
		RequestsPerMin: 2,
		Now:            func() time.Time { return now },
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			return &ForwardResponse{StatusCode: http.StatusAccepted, ContentType: "application/json; charset=utf-8", Body: []byte(`{"accepted":1}`)}, nil
		}),
	}))
	t.Cleanup(ts.Close)

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://staging.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		req.Host = "staging.example.com"
		req.RemoteAddr = "127.0.0.1:12345"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST ingest: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", resp.StatusCode)
		}
	}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}

func TestIngestPassesThroughUpstream4xx(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		PublicBaseURL: "https://staging.example.com",
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			return &ForwardResponse{StatusCode: http.StatusBadRequest, ContentType: "application/json; charset=utf-8", Body: []byte(`{"error":"host is not bound to any environment"}`)}, nil
		}),
	}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected passthrough error body")
	}
}

func TestIngestSanitizesUpstream5xx(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		PublicBaseURL: "https://staging.example.com",
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			return &ForwardResponse{StatusCode: http.StatusInternalServerError, ContentType: "application/json; charset=utf-8", Body: []byte(`{"error":"/var/lib/htmlservd/db.sqlite blew up"}`)}, nil
		}),
	}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "telemetry ingest unavailable" {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func TestIngestSanitizesForwardTransportFailure(t *testing.T) {
	ts := httptest.NewServer(New(Options{
		PublicBaseURL: "https://staging.example.com",
		Forwarder: forwarderFunc(func(_ context.Context, req ForwardRequest) (*ForwardResponse, error) {
			return nil, errors.New("dial tcp 127.0.0.1:9400: connection refused")
		}),
	}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/site-telemetry/v1/events", strings.NewReader(`{"events":[{"name":"page_view","path":"/"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://staging.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "staging.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
}

func TestHTTPForwarderPreservesHostAndOmitsOrigin(t *testing.T) {
	var gotAuth string
	var gotHost string
	var gotOrigin string
	var gotProto string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotOrigin = r.Header.Get("Origin")
		gotProto = r.Header.Get("X-Forwarded-Proto")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":1}`))
	}))
	t.Cleanup(upstream.Close)

	resp, err := HTTPForwarder{BaseURL: upstream.URL, Token: "secret"}.Forward(context.Background(), ForwardRequest{
		Host:        "staging.example.com",
		Scheme:      "https",
		ContentType: "application/json",
		Body:        []byte(`{"events":[{"name":"page_view","path":"/"}]}`),
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotHost != "staging.example.com" {
		t.Fatalf("unexpected host: %q", gotHost)
	}
	if gotOrigin != "" {
		t.Fatalf("expected origin to be omitted, got %q", gotOrigin)
	}
	if gotProto != "https" {
		t.Fatalf("unexpected forwarded proto: %q", gotProto)
	}
}

type forwarderFunc func(context.Context, ForwardRequest) (*ForwardResponse, error)

func (fn forwarderFunc) Forward(ctx context.Context, req ForwardRequest) (*ForwardResponse, error) {
	return fn(ctx, req)
}
