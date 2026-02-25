package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
)

func TestConfigViewCommandPrintsYAML(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"config", "view"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "current-context: staging") {
		t.Fatalf("expected current-context in output, got: %s", got)
	}
	if !strings.Contains(got, "name: prod") {
		t.Fatalf("expected contexts in output, got: %s", got)
	}
}

func TestConfigCurrentContextCommandPrintsActiveName(t *testing.T) {
	configPath := writeTestConfigFile(t, "prod")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"config", "current-context"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.TrimSpace(out.String()) != "prod" {
		t.Fatalf("expected prod current context, got: %q", out.String())
	}
}

func TestConfigCurrentContextCommandRespectsContextOverrideFlag(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"--context", "prod", "config", "current-context"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.TrimSpace(out.String()) != "prod" {
		t.Fatalf("expected context override to resolve prod, got: %q", out.String())
	}
}

func TestConfigUseContextUpdatesCurrentContext(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"config", "use-context", "prod"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Switched to context "prod"`) {
		t.Fatalf("expected switch confirmation, got: %s", out.String())
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.CurrentContext != "prod" {
		t.Fatalf("expected current-context to be updated to prod, got %q", cfg.CurrentContext)
	}
}

func TestConfigUseContextUnknownContextFailsWithAvailableList(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"config", "use-context", "qa"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected missing context error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "available contexts") {
		t.Fatalf("expected available contexts in error, got %v", err)
	}
	if !strings.Contains(msg, "staging") || !strings.Contains(msg, "prod") {
		t.Fatalf("expected known contexts in error, got %v", err)
	}
}

func TestConfigCurrentContextMissingConfigFailsWithHelpfulError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-config.yaml")
	t.Setenv(config.EnvConfigPath, missingPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"config", "current-context"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected missing config error")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("expected helpful missing config message, got %v", err)
	}
	if !strings.Contains(err.Error(), config.EnvConfigPath) {
		t.Fatalf("expected env var hint in error, got %v", err)
	}
}

func writeTestConfigFile(t *testing.T, currentContext string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `apiVersion: htmlctl.dev/v1
current-context: ` + currentContext + `
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: sample
    environment: staging
  - name: prod
    server: ssh://root@prod.example.com
    website: sample
    environment: prod
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
