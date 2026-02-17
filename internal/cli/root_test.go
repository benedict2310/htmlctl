package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"github.com/benedict2310/htmlctl/internal/transport"
	"github.com/spf13/cobra"
)

func TestRootCommandNoArgsPrintsUsage(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Usage:") {
		t.Fatalf("expected usage output, got: %s", help)
	}
	for _, sub := range []string{"config", "render", "serve", "get", "status", "diff", "apply", "rollout", "promote", "logs", "version"} {
		if !strings.Contains(help, sub) {
			t.Fatalf("expected help output to include %q", sub)
		}
	}
}

func TestRootPersistentConfigLoadOnlyAppliesToConfigCommands(t *testing.T) {
	t.Setenv("HTMLCTL_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))

	cmd := NewRootCmd("test-version")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command should not require config: %v", err)
	}
	if strings.TrimSpace(out.String()) != "test-version" {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}

func TestRootPersistentTransportLifecycleForAnnotatedCommand(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	mock := &mockRuntimeTransport{}
	prevFactory := buildTransportForContext
	buildTransportForContext = func(ctx context.Context, info config.ContextInfo, cfg transport.SSHConfig) (transport.Transport, error) {
		if info.Name != "staging" {
			t.Fatalf("expected resolved staging context, got %#v", info)
		}
		return mock, nil
	}
	defer func() { buildTransportForContext = prevFactory }()

	cmd := NewRootCmd("test")
	remoteCmd := &cobra.Command{
		Use: "fake-remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			tr, err := runtimeTransportFromCommand(cmd)
			if err != nil {
				return err
			}
			if tr == nil {
				t.Fatalf("expected runtime transport")
			}
			_, _ = cmd.OutOrStdout().Write([]byte("ok\n"))
			return nil
		},
	}
	markRequiresTransport(remoteCmd)
	cmd.AddCommand(remoteCmd)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"fake-remote"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "ok" {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if !mock.closed {
		t.Fatalf("expected transport to be closed in PersistentPostRunE")
	}
}

type mockRuntimeTransport struct {
	closed bool
}

func (m *mockRuntimeTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func (m *mockRuntimeTransport) Close() error {
	m.closed = true
	return nil
}
