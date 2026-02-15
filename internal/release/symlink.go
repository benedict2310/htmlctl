package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func SwitchCurrentSymlink(envDir, releaseID string) error {
	if releaseID == "" {
		return fmt.Errorf("release id is required")
	}
	return SetCurrentSymlinkTarget(envDir, filepath.ToSlash(filepath.Join("releases", releaseID)))
}

func SetCurrentSymlinkTarget(envDir, target string) error {
	currentPath := filepath.Join(envDir, "current")
	if target == "" {
		if err := os.Remove(currentPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove current symlink %s: %w", currentPath, err)
		}
		return nil
	}
	tmpLinkPath := filepath.Join(envDir, ".current.tmp")

	if err := os.Remove(tmpLinkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temp symlink %s: %w", tmpLinkPath, err)
	}
	if err := os.Symlink(target, tmpLinkPath); err != nil {
		return fmt.Errorf("create temp symlink %s -> %s: %w", tmpLinkPath, target, err)
	}
	if err := os.Rename(tmpLinkPath, currentPath); err != nil {
		_ = os.Remove(tmpLinkPath)
		return fmt.Errorf("activate current symlink %s -> %s: %w", currentPath, target, err)
	}
	return nil
}

func ReadCurrentSymlinkTarget(envDir string) (string, bool, error) {
	currentPath := filepath.Join(envDir, "current")
	target, err := os.Readlink(currentPath)
	if err == nil {
		return target, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("read current symlink %s: %w", currentPath, err)
}
