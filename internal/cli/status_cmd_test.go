package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestStatusCommandTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/futurelab/environments/staging/status" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{
  "website":"futurelab",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "activeReleaseTimestamp":"2026-01-02T00:00:00Z",
  "resourceCounts":{"pages":1,"components":1,"styles":1,"assets":1,"scripts":0}
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"status", "website/futurelab"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "active_release") || !strings.Contains(out, "01ARZ3NDEKTSV4RRFFQ69G5FAV") {
		t.Fatalf("expected active release in output, got: %s", out)
	}
	if !strings.Contains(out, "components") || !strings.Contains(out, "1") {
		t.Fatalf("expected resource counts in output, got: %s", out)
	}
}
