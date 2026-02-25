package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetWebsitesTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected call index %d", call)
			}
			if req.Method != "GET" || req.Path != "/api/v1/websites" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"websites":[{"name":"sample","defaultStyleBundle":"default","baseTemplate":"default","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "websites"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "sample") {
		t.Fatalf("expected website in output, got: %s", out)
	}
	if !tr.closed {
		t.Fatalf("expected transport to be closed")
	}
}

func TestGetEnvironmentsJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environments":[{"name":"staging","activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"get", "environments", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"activeReleaseId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"`) {
		t.Fatalf("expected active release in output, got: %s", out)
	}
}
