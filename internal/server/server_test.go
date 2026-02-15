package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServerHealthAndVersion(t *testing.T) {
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "debug"}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	base := "http://" + srv.Addr()

	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", resp.StatusCode)
	}
	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode /healthz response: %v", err)
	}
	if health["status"] != "ok" {
		t.Fatalf("unexpected /healthz payload: %#v", health)
	}

	versionResp, err := http.Get(base + "/version")
	if err != nil {
		t.Fatalf("GET /version error = %v", err)
	}
	defer versionResp.Body.Close()
	if versionResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /version, got %d", versionResp.StatusCode)
	}
	var version map[string]string
	if err := json.NewDecoder(versionResp.Body).Decode(&version); err != nil {
		t.Fatalf("decode /version response: %v", err)
	}
	if version["version"] != "v-test" {
		t.Fatalf("unexpected /version payload: %#v", version)
	}
}

func TestServerRunGracefulShutdownOnContextCancel(t *testing.T) {
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "info"}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		cancel()
		t.Fatalf("server never started")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error after cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run() did not exit after cancel")
	}
}

func TestServerPortInUseError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pre-listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	cfg := Config{BindAddr: "127.0.0.1", Port: port, DataDir: t.TempDir(), LogLevel: "info"}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = srv.Start()
	if err == nil {
		t.Fatalf("expected listen error on occupied port")
	}
	if !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("expected listen error message, got %v", err)
	}
}

func TestParseLogLevelAndLogger(t *testing.T) {
	cases := []string{"debug", "info", "", "warn", "warning", "error"}
	for _, c := range cases {
		if _, err := parseLogLevel(c); err != nil {
			t.Fatalf("parseLogLevel(%q) unexpected error: %v", c, err)
		}
		if _, err := NewLogger(c); err != nil {
			t.Fatalf("NewLogger(%q) unexpected error: %v", c, err)
		}
	}
	if _, err := parseLogLevel("bogus"); err == nil {
		t.Fatalf("expected invalid level error")
	}
	if _, err := NewLogger("bogus"); err == nil {
		t.Fatalf("expected invalid logger level error")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	if !isLoopbackHost("localhost") {
		t.Fatalf("expected localhost to be loopback")
	}
	if !isLoopbackHost("127.0.0.1") {
		t.Fatalf("expected 127.0.0.1 to be loopback")
	}
	if isLoopbackHost("192.168.1.10") {
		t.Fatalf("did not expect private LAN address to be loopback")
	}
	if isLoopbackHost("not-an-ip") {
		t.Fatalf("did not expect hostname to be loopback")
	}
}

func TestNewDefaultsVersionAndLogger(t *testing.T) {
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "info"}
	srv, err := New(cfg, nil, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if srv.version != "dev" {
		t.Fatalf("expected default version dev, got %q", srv.version)
	}
	if srv.logger == nil {
		t.Fatalf("expected default logger to be set")
	}
}

func TestRunStartFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: blocker, LogLevel: "info"}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := srv.Run(context.Background()); err == nil {
		t.Fatalf("expected Run() to fail when Start() fails")
	}
}

func TestShutdownBeforeStartIsNoop(t *testing.T) {
	cfg := Config{BindAddr: "127.0.0.1", Port: 0, DataDir: t.TempDir(), LogLevel: "info"}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "v-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() before Start() should be no-op, got %v", err)
	}
}
