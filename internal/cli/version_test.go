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

func TestVersionCommandPrintsVersion(t *testing.T) {
	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "1.2.3" {
		t.Fatalf("version output = %q, want %q", got, "1.2.3")
	}
}

func TestVersionCommandRemotePrintsLocalAndRemoteVersions(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			if req.Path != "/version" {
				t.Fatalf("unexpected request path %s", req.Path)
			}
			return jsonHTTPResponse(200, `{"version":"2.0.0"}`), nil
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
	cmd.SetArgs([]string{"version", "--remote"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "local_version") || !strings.Contains(got, "1.2.3") {
		t.Fatalf("expected local version in output, got %q", got)
	}
	if !strings.Contains(got, "remote_version") || !strings.Contains(got, "2.0.0") {
		t.Fatalf("expected remote version in output, got %q", got)
	}
}

func TestVersionCommandRemoteStructuredOutput(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"version":"2.0.0"}`), nil
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
	cmd.SetArgs([]string{"version", "--remote", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"localVersion": "1.2.3"`) || !strings.Contains(out.String(), `"remoteVersion": "2.0.0"`) {
		t.Fatalf("unexpected JSON output: %s", out.String())
	}
}

func TestVersionCommandRemoteYAMLOutput(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	tr := &scriptedTransport{
		handle: func(call int, req recordedRequest) (*http.Response, error) {
			return jsonHTTPResponse(200, `{"version":"2.0.0"}`), nil
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
	cmd.SetArgs([]string{"version", "--remote", "--output", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "localVersion: 1.2.3") || !strings.Contains(out.String(), "remoteVersion: 2.0.0") {
		t.Fatalf("unexpected YAML output: %s", out.String())
	}
}
