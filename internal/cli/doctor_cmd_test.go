package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
)

func TestDoctorReportsConfigFailure(t *testing.T) {
	t.Setenv(config.EnvConfigPath, "/nonexistent/config.yaml")

	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected doctor failure")
	}
	if got := ExitCode(err); got != 1 {
		t.Fatalf("expected exit code 1, got %d", got)
	}
	if !strings.Contains(out.String(), "config") || !strings.Contains(out.String(), "Next steps:") {
		t.Fatalf("unexpected doctor output: %s", out.String())
	}
}

func TestDoctorReportsTransportFailure(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		return nil, transport.ErrSSHHostKey
	}
	t.Cleanup(func() {
		buildTransportForContext = prevFactory
	})

	cmd := NewRootCmd("1.2.3")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected doctor failure")
	}
	if !strings.Contains(out.String(), "transport") || !strings.Contains(out.String(), "known_hosts") {
		t.Fatalf("unexpected doctor output: %s", out.String())
	}
}

func TestDoctorReportsHealthyServer(t *testing.T) {
	configPath := writeTestConfigFileWithToken(t, "staging", "super-secret-token")
	t.Setenv(config.EnvConfigPath, configPath)

	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				if req.Path != "/healthz" {
					t.Fatalf("unexpected request path %s", req.Path)
				}
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 2:
				if req.Path != "/readyz" {
					t.Fatalf("unexpected request path %s", req.Path)
				}
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 3:
				if req.Path != "/api/v1/websites" {
					t.Fatalf("unexpected request path %s", req.Path)
				}
				if got := req.Headers.Get("Authorization"); got != "Bearer super-secret-token" {
					t.Fatalf("unexpected authorization header %q", got)
				}
				return jsonHTTPResponse(200, `{"websites":[]}`), nil
			case 4:
				if req.Path != "/version" {
					t.Fatalf("unexpected request path %s", req.Path)
				}
				return jsonHTTPResponse(200, `{"version":"1.2.3"}`), nil
			default:
				t.Fatalf("unexpected call %d", call)
				return nil, nil
			}
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
	cmd.SetArgs([]string{"doctor", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"layer": "auth"`) {
		t.Fatalf("expected auth layer in output: %s", got)
	}
	if !strings.Contains(got, `"remoteVersion": "1.2.3"`) || !strings.Contains(got, `"ok": true`) {
		t.Fatalf("unexpected doctor output: %s", got)
	}
	if strings.Contains(got, "super-secret-token") {
		t.Fatalf("expected token to stay redacted, got: %s", got)
	}
}

func TestDoctorReportsHealthyServerYAMLOutput(t *testing.T) {
	configPath := writeTestConfigFileWithToken(t, "staging", "super-secret-token")
	t.Setenv(config.EnvConfigPath, configPath)

	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1, 2:
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 3:
				return jsonHTTPResponse(200, `{"websites":[]}`), nil
			case 4:
				return jsonHTTPResponse(200, `{"version":"1.2.3"}`), nil
			default:
				t.Fatalf("unexpected call %d", call)
				return nil, nil
			}
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
	cmd.SetArgs([]string{"doctor", "--output", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "localVersion: 1.2.3") || !strings.Contains(got, "remoteVersion: 1.2.3") || !strings.Contains(got, "layer: auth") {
		t.Fatalf("unexpected YAML output: %s", got)
	}
	if strings.Contains(got, "super-secret-token") {
		t.Fatalf("expected token to stay redacted, got: %s", got)
	}
}

func TestDoctorReportsVersionSkew(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1, 2:
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 3:
				return jsonHTTPResponse(200, `{"websites":[]}`), nil
			case 4:
				return jsonHTTPResponse(200, `{"version":"9.9.9"}`), nil
			default:
				return nil, errors.New("unexpected request")
			}
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
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected version skew failure")
	}
	if !strings.Contains(out.String(), "local=1.2.3 remote=9.9.9") || !strings.Contains(out.String(), "Update htmlctl or htmlservd") {
		t.Fatalf("unexpected doctor output: %s", out.String())
	}
}

func TestDoctorReportsAuthFailure(t *testing.T) {
	configPath := writeTestConfigFileWithToken(t, "staging", "super-secret-token")
	t.Setenv(config.EnvConfigPath, configPath)

	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1, 2:
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 3:
				return jsonHTTPResponse(http.StatusUnauthorized, `{"error":"unauthorized"}`), nil
			case 4:
				return jsonHTTPResponse(200, `{"version":"1.2.3"}`), nil
			default:
				return nil, errors.New("unexpected request")
			}
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
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected auth failure")
	}
	got := out.String()
	if !strings.Contains(got, "auth") || !strings.Contains(got, "Check the selected context token") {
		t.Fatalf("unexpected doctor output: %s", got)
	}
	if !strings.Contains(got, "remote_version") || !strings.Contains(got, "1.2.3") {
		t.Fatalf("expected version output alongside auth failure: %s", got)
	}
	if strings.Contains(got, "super-secret-token") {
		t.Fatalf("expected token to stay redacted, got: %s", got)
	}
}

func TestDoctorContinuesAfterReadinessFailure(t *testing.T) {
	configPath := writeTestConfigFileWithToken(t, "staging", "super-secret-token")
	t.Setenv(config.EnvConfigPath, configPath)

	call := 0
	tr := &scriptedTransport{
		handle: func(callIndex int, req recordedRequest) (*http.Response, error) {
			call++
			switch call {
			case 1:
				return jsonHTTPResponse(200, `{"status":"ok"}`), nil
			case 2:
				return jsonHTTPResponse(http.StatusServiceUnavailable, `{"error":"server is not ready"}`), nil
			case 3:
				return jsonHTTPResponse(200, `{"websites":[]}`), nil
			case 4:
				return jsonHTTPResponse(200, `{"version":"1.2.3"}`), nil
			default:
				return nil, errors.New("unexpected request")
			}
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
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected readiness failure")
	}
	got := out.String()
	if !strings.Contains(got, "readiness") || !strings.Contains(got, "auth") || !strings.Contains(got, "version") {
		t.Fatalf("expected layered diagnostics to continue, got: %s", got)
	}
	if !strings.Contains(got, "remote_version") || !strings.Contains(got, "1.2.3") {
		t.Fatalf("expected version details after readiness failure, got: %s", got)
	}
}
