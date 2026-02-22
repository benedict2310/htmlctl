package transport

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnownHostsCallbackMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-known-hosts")
	_, err := knownHostsCallback(path)
	if err == nil {
		t.Fatalf("expected missing known_hosts error")
	}
	if !errors.Is(err, ErrSSHHostKey) {
		t.Fatalf("expected ErrSSHHostKey, got %v", err)
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Fatalf("expected setup hint in error, got %v", err)
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("expected known_hosts path to be omitted from error, got %v", err)
	}
}
