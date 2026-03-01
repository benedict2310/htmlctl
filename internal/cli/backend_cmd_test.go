package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestBackendAddCommand(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodPost || req.Path != "/api/v1/websites/sample/environments/staging/backends" {
				t.Fatalf("unexpected request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"pathPrefix":"/api/*"`) || !strings.Contains(body, `"upstream":"https://api.example.com"`) {
				t.Fatalf("unexpected request body: %s", body)
			}
			return jsonHTTPResponse(201, `{"pathPrefix":"/api/*","upstream":"https://api.example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "add", "website/sample", "--env", "staging", "--path", "/api/*", "--upstream", "https://api.example.com"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "backend /api/* -> https://api.example.com added to sample/staging") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestBackendAddJSONOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(201, `{"pathPrefix":"/api/*","upstream":"https://api.example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}`), nil
		},
	}
	out, _, err := runCommandWithTransport(t, []string{"backend", "add", "website/sample", "--env", "staging", "--path", "/api/*", "--upstream", "https://api.example.com", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, `"pathPrefix": "/api/*"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
}

func TestBackendListAndRemoveCommands(t *testing.T) {
	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				if req.Method != http.MethodGet || req.Path != "/api/v1/websites/sample/environments/staging/backends" {
					t.Fatalf("unexpected list request: %#v", req)
				}
				return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[{"pathPrefix":"/api/*","upstream":"https://api.example.com","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}]}`), nil
			case 2:
				if req.Method != http.MethodDelete || req.Path != "/api/v1/websites/sample/environments/staging/backends" || req.Query != "path=%2Fapi%2F%2A" {
					t.Fatalf("unexpected remove request: %#v", req)
				}
				return jsonHTTPResponse(204, ``), nil
			default:
				t.Fatalf("unexpected request count %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "list", "website/sample", "--env", "staging"}, tr)
	if err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out, "/api/*") || !strings.Contains(out, "https://api.example.com") {
		t.Fatalf("unexpected list output: %s", out)
	}

	out, _, err = runCommandWithTransport(t, []string{"backend", "remove", "website/sample", "--env", "staging", "--path", "/api/*"}, tr)
	if err != nil {
		t.Fatalf("remove Execute() error = %v", err)
	}
	if !strings.Contains(out, "backend /api/* removed from sample/staging") {
		t.Fatalf("unexpected remove output: %s", out)
	}
}

func TestBackendListEmptyAndRemoveJSON(t *testing.T) {
	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[]}`), nil
			case 2:
				return jsonHTTPResponse(204, ``), nil
			default:
				t.Fatalf("unexpected request count %d", call)
				return nil, nil
			}
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "list", "website/sample", "--env", "staging"}, tr)
	if err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out, "No backends configured.") {
		t.Fatalf("unexpected empty list output: %s", out)
	}

	out, _, err = runCommandWithTransport(t, []string{"backend", "remove", "website/sample", "--env", "staging", "--path", "/api/*", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("remove Execute() error = %v", err)
	}
	if !strings.Contains(out, `"removed": true`) {
		t.Fatalf("unexpected remove JSON output: %s", out)
	}
}
