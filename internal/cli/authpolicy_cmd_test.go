package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
)

func TestAuthPolicyAddHashesPasswordFromStdin(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			if req.Method != http.MethodPost || req.Path != "/api/v1/websites/sample/environments/staging/auth-policies" {
				t.Fatalf("unexpected add request: %#v", req)
			}
			body := string(req.Body)
			if !strings.Contains(body, `"pathPrefix":"\/docs\/*"`) && !strings.Contains(body, `"pathPrefix":"/docs/*"`) {
				t.Fatalf("expected pathPrefix in request body, got %s", body)
			}
			if !strings.Contains(body, `"username":"reviewer"`) {
				t.Fatalf("expected username in request body, got %s", body)
			}
			if strings.Contains(body, "super-secret") {
				t.Fatalf("did not expect plaintext password in request body: %s", body)
			}
			if !strings.Contains(body, `"passwordHash":"$2`) {
				t.Fatalf("expected bcrypt hash in request body, got %s", body)
			}
			return jsonHTTPResponse(201, `{"pathPrefix":"/docs/*","username":"reviewer","createdAt":"2026-03-03T12:00:00Z","updatedAt":"2026-03-03T12:00:00Z"}`), nil
		},
	}

	out, _, err := runCommandWithTransportAndStdin(t, []string{"authpolicy", "add", "--path", "/docs/*", "--username", "reviewer", "--password-stdin"}, tr, "super-secret\n")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "auth policy /docs/* added to sample/staging for reviewer") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAuthPolicyAddRequiresPasswordStdin(t *testing.T) {
	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransportAndStdin(t, []string{"authpolicy", "add", "--path", "/docs/*", "--username", "reviewer"}, tr, "")
	if err == nil {
		t.Fatalf("expected --password-stdin requirement error")
	}
	if !strings.Contains(err.Error(), "--password-stdin is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tr.requests) != 0 {
		t.Fatalf("expected no API requests, got %d", len(tr.requests))
	}
}

func TestAuthPolicyListOmitsHashMaterial(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","authPolicies":[{"pathPrefix":"/docs/*","username":"reviewer","createdAt":"2026-03-03T12:00:00Z","updatedAt":"2026-03-03T12:00:00Z"}]}`), nil
		},
	}

	out, _, err := runCommandWithTransportAndStdin(t, []string{"authpolicy", "list", "--output", "json"}, tr, "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(out, "$2") || strings.Contains(out, "passwordHash") {
		t.Fatalf("did not expect hash material in output: %s", out)
	}
	if !strings.Contains(out, `"username": "reviewer"`) {
		t.Fatalf("expected username in output: %s", out)
	}
}

func TestAuthPolicyRemoveBuildsExpectedRequest(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if call != 0 {
				t.Fatalf("unexpected transport call %d: %#v", call, req)
			}
			if req.Method != http.MethodDelete || req.Path != "/api/v1/websites/sample/environments/staging/auth-policies" || req.Query != "path=%2Fdocs%2F%2A" {
				t.Fatalf("unexpected remove request: %#v", req)
			}
			return jsonHTTPResponse(204, ``), nil
		},
	}

	out, _, err := runCommandWithTransportAndStdin(t, []string{"authpolicy", "remove", "--path", "/docs/*"}, tr, "")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "auth policy /docs/* removed from sample/staging") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func runCommandWithTransportAndStdin(t *testing.T, args []string, tr *scriptedTransport, stdin string) (string, string, error) {
	t.Helper()

	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		return tr, nil
	}
	t.Cleanup(func() {
		buildTransportForContext = prevFactory
	})

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), errOut.String(), err
}
