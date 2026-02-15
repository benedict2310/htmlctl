package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDataDirCreatesExpectedLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "htmlservd")

	paths, err := InitDataDir(root)
	if err != nil {
		t.Fatalf("InitDataDir() error = %v", err)
	}

	for _, p := range []string{paths.BlobsSHA256, paths.WebsitesRoot, paths.DBPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected path %s to exist: %v", p, err)
		}
	}
}

func TestInitDataDirIsIdempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "htmlservd")
	if _, err := InitDataDir(root); err != nil {
		t.Fatalf("first InitDataDir() error = %v", err)
	}
	if _, err := InitDataDir(root); err != nil {
		t.Fatalf("second InitDataDir() error = %v", err)
	}
}

func TestInitDataDirPermissionError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(root, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	_, err := InitDataDir(root)
	if err == nil {
		t.Fatalf("expected initialization error")
	}
}
