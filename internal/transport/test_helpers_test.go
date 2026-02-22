package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEmptyKnownHostsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}
