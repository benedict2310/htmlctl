package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/transport"
)

func TestListWebsitesBuildsExpectedRequest(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.Header.Get("Accept") != "application/json" {
				t.Fatalf("unexpected Accept header %q", req.Header.Get("Accept"))
			}
			return jsonResponse(http.StatusOK, `{"websites":[{"name":"futurelab","defaultStyleBundle":"default","baseTemplate":"default","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`), nil
		},
	}

	api := New(mock)
	out, err := api.ListWebsites(context.Background())
	if err != nil {
		t.Fatalf("ListWebsites() error = %v", err)
	}
	if len(out.Websites) != 1 || out.Websites[0].Name != "futurelab" {
		t.Fatalf("unexpected websites response: %#v", out)
	}
}

func TestApplyBundleSetsContentTypeAndDryRunQuery(t *testing.T) {
	var body []byte
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/apply" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.URL.Query().Get("dry_run") != "true" {
				t.Fatalf("expected dry_run=true query param")
			}
			if req.Header.Get("Content-Type") != "application/x-tar" {
				t.Fatalf("unexpected Content-Type header %q", req.Header.Get("Content-Type"))
			}
			var err error
			body, err = io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			return jsonResponse(http.StatusOK, `{"website":"futurelab","environment":"staging","mode":"full","dryRun":true,"acceptedResources":[],"changes":{"created":0,"updated":0,"deleted":0}}`), nil
		},
	}

	api := New(mock)
	_, err := api.ApplyBundle(context.Background(), "futurelab", "staging", bytes.NewReader([]byte("bundle-bytes")), true)
	if err != nil {
		t.Fatalf("ApplyBundle() error = %v", err)
	}
	if string(body) != "bundle-bytes" {
		t.Fatalf("unexpected request body %q", string(body))
	}
}

func TestListEnvironmentsMapsNotFoundError(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, `{"error":"website \"futurelab\" not found"}`), nil
		},
	}
	api := New(mock)
	_, err := api.ListEnvironments(context.Background(), "futurelab")
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if !strings.Contains(err.Error(), "resource not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListReleasesMapsConflictAndServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "conflict",
			statusCode: http.StatusConflict,
			body:       `{"error":"release build already running"}`,
			wantErr:    "conflict",
		},
		{
			name:       "server-error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"release build failed"}`,
			wantErr:    "server error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockTransport{
				doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
					return jsonResponse(tc.statusCode, tc.body), nil
				},
			}
			api := New(mock)
			_, err := api.ListReleases(context.Background(), "futurelab", "staging")
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetStatusMapsTransportErrors(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			return nil, transport.ErrSSHAuth
		},
	}
	api := New(mock)
	_, err := api.GetStatus(context.Background(), "futurelab", "staging")
	if err == nil {
		t.Fatalf("expected transport error")
	}
	if !strings.Contains(err.Error(), "ssh authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetDesiredStateManifestBuildsExpectedRequest(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/manifest" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{"website":"futurelab","environment":"staging","files":[{"path":"components/header.html","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`), nil
		},
	}

	api := New(mock)
	out, err := api.GetDesiredStateManifest(context.Background(), "futurelab", "staging")
	if err != nil {
		t.Fatalf("GetDesiredStateManifest() error = %v", err)
	}
	if len(out.Files) != 1 || out.Files[0].Path != "components/header.html" {
		t.Fatalf("unexpected manifest response: %#v", out)
	}
}

type mockTransport struct {
	doFn func(ctx context.Context, req *http.Request) (*http.Response, error)
}

func (m *mockTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if m.doFn == nil {
		return nil, errors.New("unexpected call")
	}
	return m.doFn(ctx, req)
}

func (m *mockTransport) Close() error { return nil }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
