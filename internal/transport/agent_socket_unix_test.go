//go:build !windows

package transport

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAgentSocketAcceptsOwnedUnixSocket(t *testing.T) {
	sockPath := testUnixSocketPath(t)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	if err := validateAgentSocket(sockPath); err != nil {
		t.Fatalf("validateAgentSocket() error = %v", err)
	}
}

func TestValidateAgentSocketRejectsOwnershipMismatch(t *testing.T) {
	sockPath := testUnixSocketPath(t)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	err = validateAgentSocketOwnedBy(sockPath, os.Geteuid()+1)
	if err == nil {
		t.Fatalf("expected ownership mismatch error")
	}
	if !errors.Is(err, ErrSSHAgentUnavailable) {
		t.Fatalf("expected ErrSSHAgentUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "ownership mismatch") {
		t.Fatalf("expected ownership mismatch detail, got %v", err)
	}
}

func TestValidateAgentSocketSkipsOwnershipCheckForRootUID(t *testing.T) {
	sockPath := testUnixSocketPath(t)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	err = validateAgentSocketOwnedBy(sockPath, 0)
	if err != nil {
		t.Fatalf("expected root-uid ownership bypass, got %v", err)
	}
}

func TestValidateAgentSocketRejectsNonSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write regular file: %v", err)
	}

	err := validateAgentSocket(path)
	if err == nil {
		t.Fatalf("expected non-socket error")
	}
	if !errors.Is(err, ErrSSHAgentUnavailable) {
		t.Fatalf("expected ErrSSHAgentUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "not a unix socket") {
		t.Fatalf("expected socket-type detail, got %v", err)
	}
}

func TestValidateAgentSocketRejectsMissingPath(t *testing.T) {
	err := validateAgentSocket(filepath.Join(t.TempDir(), "missing.sock"))
	if err == nil {
		t.Fatalf("expected missing-path error")
	}
	if !errors.Is(err, ErrSSHAgentUnavailable) {
		t.Fatalf("expected ErrSSHAgentUnavailable, got %v", err)
	}
}

func TestValidateAgentSocketRejectsSymlinkPath(t *testing.T) {
	socketPath := testUnixSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()

	linkPath := filepath.Join(t.TempDir(), "agent.sock")
	if err := os.Symlink(socketPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	err = validateAgentSocket(linkPath)
	if err == nil {
		t.Fatalf("expected symlink path rejection")
	}
	if !errors.Is(err, ErrSSHAgentUnavailable) {
		t.Fatalf("expected ErrSSHAgentUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "not a unix socket") {
		t.Fatalf("expected socket-type detail for symlink path, got %v", err)
	}
}

func testUnixSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(os.TempDir(), "htmlctl-agent-")
	if err != nil {
		t.Fatalf("mkdir temp socket dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}
