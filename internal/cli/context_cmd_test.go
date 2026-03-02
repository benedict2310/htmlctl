package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
)

func TestContextListPrintsAllContextsAndCurrentMarker(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `* "staging"`) {
		t.Fatalf("expected current context marker, got %q", got)
	}
	if !strings.Contains(got, "prod") {
		t.Fatalf("expected prod context in list, got %q", got)
	}
}

func TestContextListRedactsServerPassword(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root:secret@staging.example.com
    website: sample
    environment: staging
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "secret") {
		t.Fatalf("expected server password to be redacted, got %q", got)
	}
	if !strings.Contains(got, config.RedactServerURL("ssh://root:secret@staging.example.com")) {
		t.Fatalf("expected redaction marker, got %q", got)
	}
}

func TestContextListRedactsSecretsEmbeddedInServerURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com?token=super-secret-token
    website: sample
    environment: staging
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "super-secret-token") {
		t.Fatalf("expected server URL secret to be redacted, got %q", got)
	}
	if !strings.Contains(got, "redacted") {
		t.Fatalf("expected redacted marker, got %q", got)
	}
}

func TestContextUseUpdatesCurrentContext(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "use", "prod"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Switched to context "prod"`) {
		t.Fatalf("expected switch confirmation, got %q", out.String())
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.CurrentContext != "prod" {
		t.Fatalf("expected current-context prod, got %q", cfg.CurrentContext)
	}
}

func TestContextCreateCreatesConfigFromScratch(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "ssh://root@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
		"--token", "super-secret",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Created context "staging"`) {
		t.Fatalf("expected create confirmation, got %q", out.String())
	}
	if !strings.Contains(out.String(), `Current context is now "staging"`) {
		t.Fatalf("expected current-context confirmation, got %q", out.String())
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current-context staging, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(cfg.Contexts))
	}
	if cfg.Contexts[0].Token != "super-secret" {
		t.Fatalf("expected token to be saved, got %q", cfg.Contexts[0].Token)
	}
}

func TestContextCreateAllowsCompatibleNonResourceStyleName(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "prod.us-east-1",
		"--server", "ssh://root@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.Contexts[0].Name != "prod.us-east-1" {
		t.Fatalf("expected context name to be preserved, got %q", cfg.Contexts[0].Name)
	}
}

func TestContextCreateRejectsDuplicateNameWithGuidance(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "ssh://root@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected duplicate context error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if !strings.Contains(msg, "htmlctl context use <name>") {
		t.Fatalf("expected use guidance, got %v", err)
	}
	if !strings.Contains(msg, "htmlctl context set <name>") {
		t.Fatalf("expected set guidance, got %v", err)
	}
}

func TestContextCreateRejectsInvalidServerURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "http://staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid server error")
	}
	if !strings.Contains(err.Error(), "expected ssh") {
		t.Fatalf("expected ssh guidance, got %v", err)
	}
}

func TestContextCreateRejectsServerURLWithPassword(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "ssh://root:secret@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid server error")
	}
	if !strings.Contains(err.Error(), "must not include password") {
		t.Fatalf("expected password guidance, got %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected password to be redacted, got %v", err)
	}
}

func TestContextCreateRejectsInvalidWebsiteName(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "ssh://root@staging.example.com",
		"--website", "bad/name",
		"--environment", "staging",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid website error")
	}
	if !strings.Contains(err.Error(), "invalid website name") {
		t.Fatalf("expected invalid website guidance, got %v", err)
	}
}

func TestContextCreateRejectsUnsafeContextName(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "bad;name",
		"--server", "ssh://root@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid context name error")
	}
	if !strings.Contains(err.Error(), "must match") {
		t.Fatalf("expected invalid context name guidance, got %v", err)
	}
}

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

func TestContextSetUpdatesValidatedFields(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "set", "staging",
		"--server", "ssh://deploy@staging.example.com:2222",
		"--website", "sample2",
		"--environment", "preview",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	for _, ctx := range cfg.Contexts {
		if ctx.Name == "staging" {
			if ctx.Server != "ssh://deploy@staging.example.com:2222" {
				t.Fatalf("expected updated server, got %q", ctx.Server)
			}
			if ctx.Website != "sample2" {
				t.Fatalf("expected updated website, got %q", ctx.Website)
			}
			if ctx.Environment != "preview" {
				t.Fatalf("expected updated environment, got %q", ctx.Environment)
			}
			return
		}
	}
	t.Fatalf("staging context not found after update")
}

func TestContextSetRejectsInvalidServerURL(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "set", "staging", "--server", "http://bad.example.com"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid server error")
	}
	if !strings.Contains(err.Error(), "expected ssh") {
		t.Fatalf("expected ssh guidance, got %v", err)
	}
}

func TestContextSetRejectsInvalidEnvironmentName(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "set", "staging", "--environment", "bad/name"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid environment error")
	}
	if !strings.Contains(err.Error(), "invalid environment name") {
		t.Fatalf("expected invalid environment guidance, got %v", err)
	}
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

func TestContextListMissingConfigGuidesCreate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"context", "list"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected missing config error")
	}
	if !strings.Contains(err.Error(), "htmlctl context create") {
		t.Fatalf("expected create guidance, got %v", err)
	}
}

func TestContextCreatePreservesExistingCurrentContext(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "prod-admin",
		"--server", "ssh://root@prod-admin.example.com",
		"--website", "sample",
		"--environment", "prod",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current-context to stay staging, got %q", cfg.CurrentContext)
	}
}

func TestContextCreateRepairsStaleCurrentContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `apiVersion: htmlctl.dev/v1
current-context: missing
contexts: []
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv(config.EnvConfigPath, configPath)

	cmd := NewRootCmd("test")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{
		"context", "create", "staging",
		"--server", "ssh://root@staging.example.com",
		"--website", "sample",
		"--environment", "staging",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current-context to be repaired to staging, got %q", cfg.CurrentContext)
	}
}

func TestShouldSelectCreatedContext(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want bool
	}{
		{
			name: "blank current-context selects created context",
			cfg:  config.Config{},
			want: true,
		},
		{
			name: "blank current-context selects created context",
			cfg: config.Config{
				Contexts: []config.Context{{Name: "staging"}},
			},
			want: true,
		},
		{
			name: "valid current-context is preserved",
			cfg: config.Config{
				CurrentContext: "staging",
				Contexts:       []config.Context{{Name: "staging"}},
			},
			want: false,
		},
		{
			name: "stale current-context is repaired",
			cfg: config.Config{
				CurrentContext: "missing",
				Contexts:       []config.Context{{Name: "staging"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSelectCreatedContext(tt.cfg, "created"); got != tt.want {
				t.Fatalf("shouldSelectCreatedContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfigForContextCreateReturnsEmptyConfigForMissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	t.Setenv(config.EnvConfigPath, configPath)

	cfg, path, err := loadConfigForContextCreate()
	if err != nil {
		t.Fatalf("loadConfigForContextCreate() error = %v", err)
	}
	if path != configPath {
		t.Fatalf("expected resolved path %q, got %q", configPath, path)
	}
	if len(cfg.Contexts) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
}

func TestLoadConfigForContextCreateLoadsExistingConfig(t *testing.T) {
	configPath := writeTestConfigFile(t, "staging")
	t.Setenv(config.EnvConfigPath, configPath)

	cfg, path, err := loadConfigForContextCreate()
	if err != nil {
		t.Fatalf("loadConfigForContextCreate() error = %v", err)
	}
	if path != configPath {
		t.Fatalf("expected resolved path %q, got %q", configPath, path)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current-context staging, got %q", cfg.CurrentContext)
	}
}
