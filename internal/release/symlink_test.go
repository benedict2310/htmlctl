package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwitchCurrentSymlink(t *testing.T) {
	envDir := filepath.Join(t.TempDir(), "websites", "sample", "envs", "staging")
	if err := os.MkdirAll(filepath.Join(envDir, "releases", "01A"), 0o755); err != nil {
		t.Fatalf("mkdir release 01A: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(envDir, "releases", "01B"), 0o755); err != nil {
		t.Fatalf("mkdir release 01B: %v", err)
	}

	if err := SwitchCurrentSymlink(envDir, "01A"); err != nil {
		t.Fatalf("SwitchCurrentSymlink(01A) error = %v", err)
	}
	target, err := os.Readlink(filepath.Join(envDir, "current"))
	if err != nil {
		t.Fatalf("readlink current after first switch: %v", err)
	}
	if target != "releases/01A" {
		t.Fatalf("unexpected first symlink target: %q", target)
	}

	if err := SwitchCurrentSymlink(envDir, "01B"); err != nil {
		t.Fatalf("SwitchCurrentSymlink(01B) error = %v", err)
	}
	target, err = os.Readlink(filepath.Join(envDir, "current"))
	if err != nil {
		t.Fatalf("readlink current after second switch: %v", err)
	}
	if target != "releases/01B" {
		t.Fatalf("unexpected second symlink target: %q", target)
	}
}
