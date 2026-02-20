package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRunInvalidConfigPath(t *testing.T) {
	err := run([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	if err == nil {
		t.Fatalf("expected missing config error")
	}
}

func TestRunInvalidEnvPort(t *testing.T) {
	t.Setenv("HTMLSERVD_PORT", "bad")
	err := run(nil)
	if err == nil {
		t.Fatalf("expected invalid env port error")
	}
}

func TestRunConfigValidationFailure(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 70000\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := run([]string{"--config", configPath})
	if err == nil {
		t.Fatalf("expected config validation error")
	}
}

func TestRunPortInUseFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := []byte("bind: 127.0.0.1\nport: " + portStr + "\ndataDir: " + filepath.Join(t.TempDir(), "data") + "\nlogLevel: info\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_ = port // keep explicit parse coverage branch
	err = run([]string{"--config", cfgPath})
	if err == nil {
		t.Fatalf("expected run failure due to occupied port")
	}
}

func TestRunRequireAuthWithoutTokenFails(t *testing.T) {
	err := run([]string{"--require-auth"})
	if err == nil {
		t.Fatalf("expected require-auth failure without token")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "api authentication required") {
		t.Fatalf("unexpected require-auth error: %v", err)
	}
}
