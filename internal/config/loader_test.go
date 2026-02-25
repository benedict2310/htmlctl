package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromPathValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `current-context: staging
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

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.APIVersion != DefaultAPIVersion {
		t.Fatalf("expected default apiVersion %q, got %q", DefaultAPIVersion, cfg.APIVersion)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current-context staging, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(cfg.Contexts))
	}
	if cfg.Contexts[0].Server != "ssh://root@staging.example.com" {
		t.Fatalf("expected ssh URL to be preserved, got %q", cfg.Contexts[0].Server)
	}
}

func TestLoadFromPathMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatalf("expected missing file error")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("expected helpful missing-file error, got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected path in missing-file error, got %v", err)
	}
}

func TestLoadFromPathMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("current-context: ["), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("expected parse error context, got %v", err)
	}
}

func TestLoadFromPathAllowsEmptyContexts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `current-context: staging
contexts: []
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("expected empty contexts to load, got %v", err)
	}
	if len(cfg.Contexts) != 0 {
		t.Fatalf("expected 0 contexts, got %d", len(cfg.Contexts))
	}
}

func TestLoadFromPathMissingRequiredField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: ""
    environment: staging
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatalf("expected missing required field error")
	}
	if !strings.Contains(err.Error(), "website is required") {
		t.Fatalf("expected website required error, got %v", err)
	}
}

func TestLoadFromPathInvalidContextPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: sample
    environment: staging
    port: 70000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatalf("expected invalid port validation error")
	}
	if !strings.Contains(err.Error(), "port must be in range") {
		t.Fatalf("expected port validation error, got %v", err)
	}
}

func TestLoadUsesHTMLCTLConfigEnvVar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.yaml")
	content := `current-context: staging
contexts:
  - name: staging
    server: ssh://root@staging.example.com
    website: sample
    environment: staging
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv(EnvConfigPath, path)

	cfg, resolvedPath, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resolvedPath != path {
		t.Fatalf("expected resolved path %q, got %q", path, resolvedPath)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("unexpected current-context: %q", cfg.CurrentContext)
	}
}

func TestLoadUsesDefaultPathUnderHome(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".htmlctl", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	content := `current-context: prod
contexts:
  - name: prod
    server: ssh://root@prod.example.com
    website: sample
    environment: prod
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv(EnvConfigPath, "")

	cfg, resolvedPath, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resolvedPath != configPath {
		t.Fatalf("expected default path %q, got %q", configPath, resolvedPath)
	}
	if cfg.CurrentContext != "prod" {
		t.Fatalf("unexpected current-context: %q", cfg.CurrentContext)
	}
}

func TestSaveWritesUpdatedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Config{
		CurrentContext: "staging",
		Contexts: []Context{
			{
				Name:        "staging",
				Server:      "ssh://root@staging.example.com",
				Website:     "sample",
				Environment: "staging",
			},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() after Save error = %v", err)
	}
	if loaded.CurrentContext != "staging" {
		t.Fatalf("expected current-context staging, got %q", loaded.CurrentContext)
	}
	if loaded.APIVersion != DefaultAPIVersion {
		t.Fatalf("expected default apiVersion %q, got %q", DefaultAPIVersion, loaded.APIVersion)
	}
}
