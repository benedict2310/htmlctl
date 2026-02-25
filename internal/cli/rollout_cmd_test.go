package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestRolloutHistoryTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != "GET" {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.Path != "/api/v1/websites/sample/environments/staging/releases" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			if req.Query != "limit=2&offset=1" {
				t.Fatalf("unexpected request query %q", req.Query)
			}
			return jsonHTTPResponse(200, `{
  "website":"sample",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAC",
  "limit":2,
  "offset":1,
  "releases":[
    {"releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAB","actor":"bene","status":"previous","createdAt":"2026-02-01T00:00:00Z","active":false},
    {"releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAA","actor":"ci","status":"previous","createdAt":"2026-01-31T00:00:00Z","active":false}
  ]
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"rollout", "history", "website/sample", "--limit", "2", "--offset", "1"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, fragment := range []string{"ID", "TIMESTAMP", "ACTOR", "STATUS", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "bene"} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected output to contain %q, got: %s", fragment, out)
		}
	}
}

func TestRolloutHistoryJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{
  "website":"sample",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAC",
  "limit":20,
  "offset":0,
  "releases":[
    {"releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAC","actor":"bene","status":"active","createdAt":"2026-02-02T00:00:00Z","active":true}
  ]
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"rollout", "history", "website/sample", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"actor": "bene"`) || !strings.Contains(out, `"status": "active"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestRolloutHistoryShowsEmptyMessage(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","limit":20,"offset":0,"releases":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"rollout", "history", "website/sample"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "No releases found.") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRolloutHistoryRejectsNegativePagination(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"rollout", "history", "website/sample", "--limit", "-1"}, tr)
	if err == nil {
		t.Fatalf("expected error for negative limit")
	}
	if !strings.Contains(err.Error(), "limit must be >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRolloutHistoryRejectsNegativeOffset(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"rollout", "history", "website/sample", "--offset", "-1"}, tr)
	if err == nil {
		t.Fatalf("expected error for negative offset")
	}
	if !strings.Contains(err.Error(), "offset must be >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRolloutUndoTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != "POST" {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.Path != "/api/v1/websites/sample/environments/staging/rollback" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{
  "website":"sample",
  "environment":"staging",
  "fromReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAC",
  "toReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAB"
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"rollout", "undo", "website/sample"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Rolled back sample/staging") || !strings.Contains(out, "01ARZ3NDEKTSV4RRFFQ69G5FAB") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRolloutUndoJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{
  "website":"sample",
  "environment":"staging",
  "fromReleaseId":"A",
  "toReleaseId":"B"
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"rollout", "undo", "website/sample", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"fromReleaseId": "A"`) || !strings.Contains(out, `"toReleaseId": "B"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestRolloutUndoRejectsInvalidWebsiteRef(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"rollout", "undo", "badref"}, tr)
	if err == nil {
		t.Fatalf("expected website ref validation error")
	}
	if !strings.Contains(err.Error(), "expected website/<name>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRolloutUndoPropagatesAPIError(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(409, `{"error":"rollback is not possible because no previous release exists"}`), nil
		},
	}
	_, _, err := runCommandWithTransport(t, []string{"rollout", "undo", "website/sample"}, tr)
	if err == nil {
		t.Fatalf("expected rollback conflict error")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRolloutParentCommandShowsHelp(t *testing.T) {
	tr := &scriptedTransport{}
	out, _, err := runCommandWithTransport(t, []string{"rollout"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Inspect and manage release rollout state") {
		t.Fatalf("expected rollout help output, got: %s", out)
	}
}
