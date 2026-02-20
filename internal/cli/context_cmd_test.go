package cli

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
)

func TestContextSetTokenUpdatesConfig(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "set", "staging", "--token", "super-secret"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Updated context "staging"`) {
		t.Fatalf("expected update confirmation, got %q", out.String())
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	for _, ctx := range cfg.Contexts {
		if ctx.Name == "staging" {
			if ctx.Token != "super-secret" {
				t.Fatalf("expected staging token to be updated, got %q", ctx.Token)
			}
			return
		}
	}
	t.Fatalf("staging context not found after update")
}

func TestContextTokenGeneratePrintsHex(t *testing.T) {
	t.Setenv(config.EnvConfigPath, filepath.Join(t.TempDir(), "missing.yaml"))

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "token", "generate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	token := strings.TrimSpace(out.String())
	if len(token) != 64 {
		t.Fatalf("expected 64-char token, got %q", token)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(token) {
		t.Fatalf("expected hex token, got %q", token)
	}
}
