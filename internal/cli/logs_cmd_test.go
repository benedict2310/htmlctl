package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestLogsCommandYAMLOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/futurelab/environments/staging/logs" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			if req.Query != "limit=1" {
				t.Fatalf("expected limit query, got %q", req.Query)
			}
			return jsonHTTPResponse(200, `{
  "entries":[
    {
      "id":1,
      "actor":"bene",
      "timestamp":"2026-01-02T00:00:00Z",
      "operation":"apply",
      "resourceSummary":"Component header",
      "releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV"
    }
  ],
  "total":1,
  "limit":1,
  "offset":0
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"logs", "website/futurelab", "--limit", "1", "--output", "yaml"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "entries:") || !strings.Contains(out, "operation: apply") {
		t.Fatalf("unexpected yaml output: %s", out)
	}
}
