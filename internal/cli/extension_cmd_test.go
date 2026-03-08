package cli

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
)

func TestExtensionValidateCommandLocalSuccess(t *testing.T) {
	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"extension", "validate", newsletterManifestPath(t)})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "newsletter") || !strings.Contains(got, "PASS") {
		t.Fatalf("expected successful extension validation output, got %q", got)
	}
}

func TestExtensionValidateCommandFailsMinimumHTMLCTL(t *testing.T) {
	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"extension", "validate", writeExtensionManifestFixture(t, "9.9.9", "0.1.0")})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected extension validation failure")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("expected exit code 1, got %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), "lower than required minimum") {
		t.Fatalf("expected minimum-version failure detail, got %q", out.String())
	}
}

func TestExtensionValidateCommandRemoteSuccess(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/version" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"version":"2.1.0"}`), nil
		},
	}

	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		return tr, nil
	}
	t.Cleanup(func() {
		buildTransportForContext = prevFactory
	})

	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"extension", "validate", writeExtensionManifestFixture(t, "1.2.0", "2.0.0"), "--remote"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "remote_version") || !strings.Contains(got, "2.1.0") {
		t.Fatalf("expected remote version in output, got %q", got)
	}
	if !strings.Contains(got, "htmlservd") || !strings.Contains(got, "PASS") {
		t.Fatalf("expected successful remote compatibility check, got %q", got)
	}
}

func newsletterManifestPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "extensions", "newsletter", "extension.yaml"))
}

func writeExtensionManifestFixture(t *testing.T, minHTMLCTL, minHTMLSERVD string) string {
	t.Helper()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "extension.yaml")
	content := "apiVersion: htmlctl.dev/extensions/v1\n" +
		"kind: Extension\n" +
		"metadata:\n" +
		"  name: newsletter\n" +
		"  version: 0.1.0\n" +
		"spec:\n" +
		"  summary: test extension\n" +
		"  compatibility:\n" +
		"    minHTMLCTL: " + minHTMLCTL + "\n" +
		"    minHTMLSERVD: " + minHTMLSERVD + "\n" +
		"  runtime:\n" +
		"    requires: [postgresql]\n" +
		"    healthEndpoints: [/healthz]\n" +
		"  integration:\n" +
		"    backendPaths: [/newsletter/*]\n" +
		"  env:\n" +
		"    - name: NEWSLETTER_ENV\n" +
		"      required: true\n" +
		"      secret: false\n" +
		"      description: env\n" +
		"  security:\n" +
		"    listenerPolicy: loopback-only\n" +
		"    requiresRateLimiting: true\n" +
		"    requiresSanitized5xx: true\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", manifestPath, err)
	}
	return manifestPath
}
