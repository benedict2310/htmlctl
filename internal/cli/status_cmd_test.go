package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestStatusCommandTableOutput(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments/staging/status" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{
  "website":"sample",
  "environment":"staging",
  "activeReleaseId":"01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "activeReleaseTimestamp":"2026-01-02T00:00:00Z",
  "resourceCounts":{"pages":1,"components":1,"styles":1,"assets":1,"scripts":0}
}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"status", "website/sample"}, tr)
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

func TestStatusCommandUsesContextWebsiteByDefault(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/sample/environments/staging/status" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"website":"sample","environment":"staging","resourceCounts":{"pages":0,"components":0,"styles":0,"assets":0,"scripts":0}}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"status"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "website") || !strings.Contains(out, "sample") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestStatusCommandExplicitWebsiteOverridesContextDefault(t *testing.T) {
	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/api/v1/websites/other/environments/staging/status" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"website":"other","environment":"staging","resourceCounts":{"pages":0,"components":0,"styles":0,"assets":0,"scripts":0}}`), nil
		},
	}

	out, _, err := runCommandWithTransport(t, []string{"status", "website/other"}, tr)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "other") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestStatusCommandMissingContextWebsiteProvidesGuidance(t *testing.T) {
	configPath := writeRemoteCommandConfig(t, `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: ""
    environment: staging
`)

	tr := &scriptedTransport{}
	_, _, err := runCommandWithTransportAndConfigPath(t, []string{"status"}, tr, configPath)
	if err == nil {
		t.Fatalf("expected missing website error")
	}
	if !strings.Contains(err.Error(), "no website selected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "htmlctl context set <name> --website <website>") {
		t.Fatalf("expected context set guidance, got %v", err)
	}
}

func TestStatusHelpDocumentsContextDefault(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Omit website/<name> to use the active context website.") {
		t.Fatalf("expected context default help text, got: %s", out.String())
	}
}
