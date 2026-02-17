package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.BindAddr != DefaultBindAddr || cfg.Port != DefaultPort || cfg.DataDir != DefaultDataDir || cfg.LogLevel != DefaultLogLevel {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if !cfg.DBWAL {
		t.Fatalf("expected DBWAL default true")
	}
	if cfg.CaddyfilePath != DefaultCaddyfilePath || cfg.CaddyBinaryPath != DefaultCaddyBinary {
		t.Fatalf("unexpected caddy defaults: %#v", cfg)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("bind: 127.0.0.2\nport: 9500\ndataDir: /tmp/htmlservd-data\nlogLevel: debug\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.2" || cfg.Port != 9500 || cfg.DataDir != "/tmp/htmlservd-data" || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected file config: %#v", cfg)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("bind: 127.0.0.2\nport: 9500\ndataDir: /tmp/htmlservd-data\nlogLevel: info\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("HTMLSERVD_BIND", "127.0.0.3")
	t.Setenv("HTMLSERVD_PORT", "9700")
	t.Setenv("HTMLSERVD_DATA_DIR", "/tmp/override")
	t.Setenv("HTMLSERVD_LOG_LEVEL", "warn")
	t.Setenv("HTMLSERVD_DB_PATH", "/tmp/override/db.sqlite")
	t.Setenv("HTMLSERVD_DB_WAL", "false")
	t.Setenv("HTMLSERVD_CADDYFILE_PATH", "/tmp/caddy/Caddyfile")
	t.Setenv("HTMLSERVD_CADDY_BINARY", "/usr/local/bin/caddy")
	t.Setenv("HTMLSERVD_CADDY_CONFIG_BACKUP", "/tmp/caddy/Caddyfile.bak")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.3" || cfg.Port != 9700 || cfg.DataDir != "/tmp/override" || cfg.LogLevel != "warn" || cfg.DBPath != "/tmp/override/db.sqlite" || cfg.DBWAL || cfg.CaddyfilePath != "/tmp/caddy/Caddyfile" || cfg.CaddyBinaryPath != "/usr/local/bin/caddy" || cfg.CaddyConfigBackupPath != "/tmp/caddy/Caddyfile.bak" {
		t.Fatalf("unexpected overridden config: %#v", cfg)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("bind 127.0.0.1\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("expected parse config error, got %v", err)
	}
}

func TestLoadConfigInvalidEnvPort(t *testing.T) {
	t.Setenv("HTMLSERVD_PORT", "not-a-number")
	_, err := LoadConfig("")
	if err == nil {
		t.Fatalf("expected port parse error")
	}
	if !strings.Contains(err.Error(), "HTMLSERVD_PORT") {
		t.Fatalf("expected env var mention in error, got %v", err)
	}
}

func TestLoadConfigInvalidEnvDBWAL(t *testing.T) {
	t.Setenv("HTMLSERVD_DB_WAL", "not-a-bool")
	_, err := LoadConfig("")
	if err == nil {
		t.Fatalf("expected db wal parse error")
	}
	if !strings.Contains(err.Error(), "HTMLSERVD_DB_WAL") {
		t.Fatalf("expected env var mention in error, got %v", err)
	}
}

func TestConfigValidateErrors(t *testing.T) {
	tests := []Config{
		{BindAddr: "", Port: 9400, DataDir: "/tmp/x", LogLevel: "info"},
		{BindAddr: "127.0.0.1", Port: -1, DataDir: "/tmp/x", LogLevel: "info"},
		{BindAddr: "127.0.0.1", Port: 70000, DataDir: "/tmp/x", LogLevel: "info"},
		{BindAddr: "127.0.0.1", Port: 9400, DataDir: "", LogLevel: "info"},
		{BindAddr: "127.0.0.1", Port: 9400, DataDir: "/tmp/x", LogLevel: "bad"},
	}
	for i, cfg := range tests {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}
