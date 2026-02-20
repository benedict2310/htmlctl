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

func TestNewWithAuthAddsAuthorizationAndActorHeaders(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("expected authorization header, got %q", got)
			}
			if got := req.Header.Get("X-Actor"); got != "staging" {
				t.Fatalf("expected actor header from context, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"websites":[]}`), nil
		},
	}
	api := NewWithAuth(mock, "staging", "test-token")
	if _, err := api.ListWebsites(context.Background()); err != nil {
		t.Fatalf("ListWebsites() error = %v", err)
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

func TestListReleasesPageBuildsPaginationQuery(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/releases" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.URL.RawQuery != "limit=5&offset=10" {
				t.Fatalf("unexpected query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"website":"futurelab","environment":"staging","limit":5,"offset":10,"releases":[]}`), nil
		},
	}
	api := New(mock)
	out, err := api.ListReleasesPage(context.Background(), "futurelab", "staging", 5, 10)
	if err != nil {
		t.Fatalf("ListReleasesPage() error = %v", err)
	}
	if out.Limit != 5 || out.Offset != 10 {
		t.Fatalf("unexpected pagination fields: %#v", out)
	}
}

func TestListReleasesFetchesAllPages(t *testing.T) {
	call := 0
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			call++
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/releases" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			switch call {
			case 1:
				if req.URL.RawQuery != "limit=200" {
					t.Fatalf("unexpected first query %q", req.URL.RawQuery)
				}
				return jsonResponse(http.StatusOK, `{
  "website":"futurelab",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "limit":200,
  "offset":0,
  "releases":[{"releaseId":"A","status":"active","createdAt":"2026-01-01T00:00:00Z","active":true}]
}`), nil
			case 2:
				if req.URL.RawQuery != "limit=200&offset=1" {
					t.Fatalf("unexpected second query %q", req.URL.RawQuery)
				}
				return jsonResponse(http.StatusOK, `{
  "website":"futurelab",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "limit":200,
  "offset":1,
  "releases":[]
}`), nil
			default:
				t.Fatalf("unexpected extra call %d", call)
			}
			return nil, nil
		},
	}

	api := New(mock)
	out, err := api.ListReleases(context.Background(), "futurelab", "staging")
	if err != nil {
		t.Fatalf("ListReleases() error = %v", err)
	}
	if len(out.Releases) != 1 || out.Releases[0].ReleaseID != "A" {
		t.Fatalf("unexpected ListReleases payload: %#v", out)
	}
}

func TestRollbackBuildsExpectedRequest(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/rollback" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{"website":"futurelab","environment":"staging","fromReleaseId":"A","toReleaseId":"B"}`), nil
		},
	}
	api := New(mock)
	out, err := api.Rollback(context.Background(), "futurelab", "staging")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if out.FromReleaseID != "A" || out.ToReleaseID != "B" {
		t.Fatalf("unexpected rollback response: %#v", out)
	}
}

func TestPromoteBuildsExpectedRequest(t *testing.T) {
	var gotBody string
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/promote" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("expected JSON content type, got %q", req.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			gotBody = string(body)
			return jsonResponse(http.StatusOK, `{"website":"futurelab","fromEnvironment":"staging","toEnvironment":"prod","sourceReleaseId":"A","releaseId":"B","fileCount":3,"hash":"sha256:abc","hashVerified":true,"strategy":"hardlink"}`), nil
		},
	}
	api := New(mock)
	out, err := api.Promote(context.Background(), "futurelab", "staging", "prod")
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if !strings.Contains(gotBody, `"from":"staging"`) || !strings.Contains(gotBody, `"to":"prod"`) {
		t.Fatalf("unexpected promote payload body: %s", gotBody)
	}
	if out.ReleaseID != "B" || !out.HashVerified {
		t.Fatalf("unexpected promote response: %#v", out)
	}
}

func TestCreateReleaseBuildsExpectedRequest(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/releases" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			return jsonResponse(http.StatusCreated, `{"website":"futurelab","environment":"staging","releaseId":"R1","status":"active"}`), nil
		},
	}
	api := New(mock)
	out, err := api.CreateRelease(context.Background(), "futurelab", "staging")
	if err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	if out.ReleaseID != "R1" {
		t.Fatalf("unexpected CreateRelease response: %#v", out)
	}
}

func TestGetLogsBuildsExpectedRequest(t *testing.T) {
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/api/v1/websites/futurelab/environments/staging/logs" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.URL.RawQuery != "limit=50" {
				t.Fatalf("unexpected query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"entries":[],"total":0,"limit":50,"offset":0}`), nil
		},
	}
	api := New(mock)
	out, err := api.GetLogs(context.Background(), "futurelab", "staging", 50)
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}
	if out.Limit != 50 {
		t.Fatalf("unexpected GetLogs response: %#v", out)
	}
}

func TestMapTransportErrorBranches(t *testing.T) {
	tests := []struct {
		err      error
		wantText string
	}{
		{transport.ErrSSHHostKey, "ssh host key verification failed"},
		{transport.ErrSSHAgentUnavailable, "ssh agent unavailable"},
		{transport.ErrSSHUnreachable, "ssh host unreachable"},
		{transport.ErrSSHTunnel, "ssh tunnel failed"},
	}
	for _, tc := range tests {
		got := mapTransportError(tc.err)
		if !strings.Contains(got.Error(), tc.wantText) {
			t.Fatalf("expected %q in error %v", tc.wantText, got)
		}
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

func TestDomainBindingClientMethods(t *testing.T) {
	var gotCreateBody string
	mock := &mockTransport{
		doFn: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			switch req.Method {
			case http.MethodPost:
				if req.URL.Path != "/api/v1/domains" {
					t.Fatalf("unexpected create path %s", req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read create body: %v", err)
				}
				gotCreateBody = string(body)
				return jsonResponse(http.StatusCreated, `{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`), nil
			case http.MethodGet:
				if req.URL.Path == "/api/v1/domains" {
					if req.URL.RawQuery != "website=futurelab" {
						t.Fatalf("unexpected list query %q", req.URL.RawQuery)
					}
					return jsonResponse(http.StatusOK, `{"domains":[{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`), nil
				}
				if req.URL.Path == "/api/v1/domains/futurelab.studio" {
					return jsonResponse(http.StatusOK, `{"id":1,"domain":"futurelab.studio","website":"futurelab","environment":"staging","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`), nil
				}
				t.Fatalf("unexpected GET path %s", req.URL.Path)
			case http.MethodDelete:
				if req.URL.Path != "/api/v1/domains/futurelab.studio" {
					t.Fatalf("unexpected delete path %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		},
	}

	api := New(mock)
	created, err := api.CreateDomainBinding(context.Background(), "futurelab.studio", "futurelab", "staging")
	if err != nil {
		t.Fatalf("CreateDomainBinding() error = %v", err)
	}
	if !strings.Contains(gotCreateBody, `"domain":"futurelab.studio"`) || !strings.Contains(gotCreateBody, `"website":"futurelab"`) || !strings.Contains(gotCreateBody, `"environment":"staging"`) {
		t.Fatalf("unexpected create payload %s", gotCreateBody)
	}
	if created.Domain != "futurelab.studio" {
		t.Fatalf("unexpected created domain: %#v", created)
	}

	listed, err := api.ListDomainBindings(context.Background(), "futurelab", "")
	if err != nil {
		t.Fatalf("ListDomainBindings() error = %v", err)
	}
	if len(listed.Domains) != 1 {
		t.Fatalf("expected one listed domain, got %#v", listed.Domains)
	}

	got, err := api.GetDomainBinding(context.Background(), "futurelab.studio")
	if err != nil {
		t.Fatalf("GetDomainBinding() error = %v", err)
	}
	if got.Domain != "futurelab.studio" {
		t.Fatalf("unexpected get domain response: %#v", got)
	}

	if err := api.DeleteDomainBinding(context.Background(), "futurelab.studio"); err != nil {
		t.Fatalf("DeleteDomainBinding() error = %v", err)
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
