package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestPromoteCommandTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != "POST" || req.Path != "/api/v1/websites/futurelab/promote" {
				t.Fatalf("unexpected request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"from":"staging"`) || !strings.Contains(body, `"to":"prod"`) {
				t.Fatalf("unexpected request body: %s", body)
			}
			return jsonHTTPResponse(200, `{
  "website":"futurelab",
  "fromEnvironment":"staging",
  "toEnvironment":"prod",
  "sourceReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAA",
  "releaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAB",
  "fileCount":3,
  "hash":"sha256:abc",
  "hashVerified":true,
  "strategy":"hardlink"
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"promote", "website/futurelab", "--from", "staging", "--to", "prod"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Promoted futurelab: staging -> prod") || !strings.Contains(out, "release 01ARZ3NDEKTSV4RRFFQ69G5FAB") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestPromoteCommandJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{
  "website":"futurelab",
  "fromEnvironment":"staging",
  "toEnvironment":"prod",
  "sourceReleaseId":"A",
  "releaseId":"B",
  "fileCount":2,
  "hash":"sha256:def",
  "hashVerified":true,
  "strategy":"copy"
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"promote", "website/futurelab", "--from", "staging", "--to", "prod", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"hashVerified": true`) || !strings.Contains(out, `"strategy": "copy"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestPromoteCommandRejectsInvalidFlags(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"promote", "website/futurelab", "--from", "staging", "--to", "staging"}, tr)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "--from and --to must be different") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromoteCommandRequiresBothFlags(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"promote", "website/futurelab", "--from", "staging"}, tr)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "both --from and --to are required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromoteCommandRejectsInvalidWebsiteRef(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"promote", "badref", "--from", "staging", "--to", "prod"}, tr)
	if err == nil {
		t.Fatalf("expected website ref validation error")
	}
	if !strings.Contains(err.Error(), "expected website/<name>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromoteCommandPropagatesAPIError(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(409, `{"error":"source environment has no active release to promote"}`), nil
		},
	}
	_, _, err := runCommandWithTransport(t, []string{"promote", "website/futurelab", "--from", "staging", "--to", "prod"}, tr)
	if err == nil {
		t.Fatalf("expected promote conflict error")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("unexpected error: %v", err)
	}
}
