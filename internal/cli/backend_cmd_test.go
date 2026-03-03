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
	if !strings.Contains(out, "Routing changes apply immediately on sample/staging.") {
		t.Fatalf("expected routing guidance, got: %s", out)
	}
	if !strings.Contains(out, "htmlctl backend list website/sample --env staging --context staging") {
		t.Fatalf("expected backend list guidance, got: %s", out)
	}
	if !strings.Contains(out, "check the live URL under /api/*") {
		t.Fatalf("expected live URL guidance, got: %s", out)
	}
}

func TestBackendAddUsesContextDefaults(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodPost || req.Path != "/api/v1/websites/sample/environments/staging/backends" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(201, `{"pathPrefix":"/api/*","upstream":"https://api.example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "add", "--path", "/api/*", "--upstream", "https://api.example.com"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "sample/staging") {
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

func TestBackendAddWarnsOnSuspiciousPrefixInTableMode(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(201, `{"pathPrefix":"/styles/*","upstream":"https://cdn.example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "add", "--path", "/styles/*", "--upstream", "https://cdn.example.com"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "Warning: backend path /styles/* will hide generated static styles under that prefix") {
		t.Fatalf("expected warning in table output, got: %s", out)
	}
}

func TestBackendAddDoesNotPrintWarningsInStructuredOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(201, `{"pathPrefix":"/styles/*","upstream":"https://cdn.example.com","website":"sample","environment":"staging","createdAt":"2026-03-01T12:00:00Z","updatedAt":"2026-03-01T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "add", "--path", "/styles/*", "--upstream", "https://cdn.example.com", "--output", "json"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(out, "Warning:") {
		t.Fatalf("expected structured output to stay warning-free, got: %s", out)
	}
	if !strings.Contains(out, `"pathPrefix": "/styles/*"`) {
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

func TestBackendListUsesContextDefaults(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet || req.Path != "/api/v1/websites/sample/environments/staging/backends" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","backends":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "list"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "No backends configured.") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestBackendRemoveUsesContextDefaults(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodDelete || req.Path != "/api/v1/websites/sample/environments/staging/backends" || req.Query != "path=%2Fapi%2F%2A" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(204, ``), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "remove", "--path", "/api/*"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "backend /api/* removed from sample/staging") {
		t.Fatalf("unexpected output: %s", out)
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

func TestBackendExplicitOverridesContextDefaults(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Method != http.MethodGet || req.Path != "/api/v1/websites/other/environments/prod/backends" {
				t.Fatalf("unexpected request: %#v", req)
			}
			return jsonHTTPResponse(200, `{"website":"other","environment":"prod","backends":[]}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"backend", "list", "website/other", "--env", "prod"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "No backends configured.") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestBackendMissingContextEnvironmentProvidesGuidance(t *testing.T) {
	configPath := writeRemoteCommandConfig(t, `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: sample
    environment: ""
`)

	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransportAndConfigPath(t, []string{"backend", "list"}, tr, configPath)
	if err == nil {
		t.Fatalf("expected missing environment error")
	}
	if !strings.Contains(err.Error(), "no environment selected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "htmlctl context set <name> --environment <environment>") {
		t.Fatalf("expected context set guidance, got %v", err)
	}
}

func TestBackendHelpDocumentsContextDefaults(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"backend", "add", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Omit website/<name> to use the active context website.") {
		t.Fatalf("expected website default help text, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Omit --env to use the active context environment.") {
		t.Fatalf("expected environment default help text, got: %s", out.String())
	}
}

func TestBackendListInvalidWebsiteReferenceProvidesGuidance(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransport(t, []string{"backend", "list", "sample"}, tr)
	if err == nil {
		t.Fatalf("expected invalid website reference error")
	}
	if !strings.Contains(err.Error(), "expected website/<name>") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "omit it to use the active context website") {
		t.Fatalf("expected recovery guidance, got %v", err)
	}
}
