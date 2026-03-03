package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestPreviewCreateCommand(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodPost || req.Path != "/api/v1/websites/sample/environments/staging/previews" {
				t.Fatalf("unexpected request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"releaseId":"R1"`) || !strings.Contains(body, `"ttl":"72h"`) {
				t.Fatalf("unexpected request body: %s", body)
			}
			return jsonHTTPResponse(201, `{"id":7,"releaseId":"R1","hostname":"abc123--staging--sample.preview.example.com","website":"sample","environment":"staging","createdBy":"alice","expiresAt":"2026-03-06T12:00:00Z","createdAt":"2026-03-03T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"preview", "create", "website/sample", "--env", "staging", "--release", "R1", "--ttl", "72h"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "preview 7 created for release R1 on sample/staging") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Hostname: abc123--staging--sample.preview.example.com") {
		t.Fatalf("expected hostname in output, got: %s", out)
	}
}

func TestPreviewCreateJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(201, `{"id":7,"releaseId":"R1","hostname":"abc123--staging--sample.preview.example.com","website":"sample","environment":"staging","createdBy":"alice","expiresAt":"2026-03-06T12:00:00Z","createdAt":"2026-03-03T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"preview", "create", "--release", "R1", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"hostname": "abc123--staging--sample.preview.example.com"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestPreviewListAndRemoveCommands(t *testing.T) {
	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				if req.Method != http.MethodGet || req.Path != "/api/v1/websites/sample/environments/staging/previews" {
					t.Fatalf("unexpected list request: %#v", req)
				}
				return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","previews":[{"id":7,"releaseId":"R1","hostname":"abc123--staging--sample.preview.example.com","expiresAt":"2026-03-06T12:00:00Z","createdAt":"2026-03-03T12:00:00Z"}]}`), nil
			case 2:
				if req.Method != http.MethodDelete || req.Path != "/api/v1/websites/sample/environments/staging/previews/7" {
					t.Fatalf("unexpected remove request: %#v", req)
				}
				return jsonHTTPResponse(204, ``), nil
			default:
				t.Fatalf("unexpected request count %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"preview", "list", "website/sample", "--env", "staging"}, tr)
	if err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out, "abc123--staging--sample.preview.example.com") || !strings.Contains(out, "R1") {
		t.Fatalf("unexpected list output: %s", out)
	}

	out, _, err = runCommandWithTransport(t, []string{"preview", "remove", "website/sample", "--env", "staging", "--id", "7"}, tr)
	if err != nil {
		t.Fatalf("remove Execute() error = %v", err)
	}
	if !strings.Contains(out, "preview 7 removed from sample/staging") {
		t.Fatalf("unexpected remove output: %s", out)
	}
}

func TestPreviewListUsesContextDefaults(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet || req.Path != "/api/v1/websites/sample/environments/staging/previews" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","previews":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"preview", "list"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "No previews configured.") {
		t.Fatalf("unexpected output: %s", out)
	}
}
