package caddy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ConfigGenerator func(context.Context) (string, error)

type Reloader struct {
	BinaryPath string
	ConfigPath string
	BackupPath string

	Generator ConfigGenerator
	Runner    CommandRunner

	mu sync.Mutex
}

func NewReloader(binaryPath, configPath, backupPath string, generator ConfigGenerator, runner CommandRunner) (*Reloader, error) {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		binaryPath = "caddy"
	}
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, fmt.Errorf("caddy config path is required")
	}
	backupPath = strings.TrimSpace(backupPath)
	if backupPath == "" {
		backupPath = configPath + ".bak"
	}
	if generator == nil {
		return nil, fmt.Errorf("config generator is required")
	}
	if runner == nil {
		runner = NewExecRunner()
	}
	return &Reloader{
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		BackupPath: backupPath,
		Generator:  generator,
		Runner:     runner,
	}, nil
}

func (r *Reloader) Reload(ctx context.Context, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := r.Generator(ctx)
	if err != nil {
		return fmt.Errorf("generate caddy config: %w", err)
	}

	tmpPath := r.ConfigPath + ".tmp"
	if err := WriteConfig(tmpPath, content); err != nil {
		return fmt.Errorf("write temporary caddy config: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, stderr, err := r.Runner.Run(ctx, r.BinaryPath, "validate", "--config", tmpPath, "--adapter", "caddyfile"); err != nil {
		return fmt.Errorf("caddy validate failed: %w: %s", err, strings.TrimSpace(stderr))
	}

	hadPreviousConfig := false
	if _, err := os.Stat(r.ConfigPath); err == nil {
		hadPreviousConfig = true
		if err := copyFile(r.ConfigPath, r.BackupPath); err != nil {
			return fmt.Errorf("backup caddy config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat current caddy config: %w", err)
	}

	if err := os.Rename(tmpPath, r.ConfigPath); err != nil {
		return fmt.Errorf("activate new caddy config: %w", err)
	}

	if _, stderr, err := r.Runner.Run(ctx, r.BinaryPath, "reload", "--config", r.ConfigPath, "--adapter", "caddyfile"); err != nil {
		if restoreErr := r.restoreBackup(hadPreviousConfig); restoreErr != nil {
			return fmt.Errorf("caddy reload failed: %w: %s (restore failed: %v)", err, strings.TrimSpace(stderr), restoreErr)
		}
		return fmt.Errorf("caddy reload failed: %w: %s", err, strings.TrimSpace(stderr))
	}

	return nil
}

func (r *Reloader) restoreBackup(hadPreviousConfig bool) error {
	if hadPreviousConfig {
		return copyFile(r.BackupPath, r.ConfigPath)
	}
	if err := os.Remove(r.ConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, content, info.Mode().Perm())
}
