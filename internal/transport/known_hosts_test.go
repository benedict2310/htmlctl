package transport

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnownHostsCallbackMissingFile(t *testing.T) {
	_, err := knownHostsCallback(filepath.Join(t.TempDir(), "missing-known-hosts"))
	if err == nil {
		t.Fatalf("expected missing known_hosts error")
	}
	if !errors.Is(err, ErrSSHHostKey) {
		t.Fatalf("expected ErrSSHHostKey, got %v", err)
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Fatalf("expected setup hint in error, got %v", err)
	}
}
