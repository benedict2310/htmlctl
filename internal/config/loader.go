package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigRelativePath = ".htmlctl/config.yaml"

// DefaultPath returns the default config path under the user's home directory.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", fmt.Errorf("resolve user home directory: empty path")
	}
	return filepath.Join(home, defaultConfigRelativePath), nil
}

// ResolvePath resolves the config path from explicit input, env var, or default.
func ResolvePath(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv(EnvConfigPath)); path != "" {
		return path, nil
	}
	return DefaultPath()
}

// Load loads config from the resolved path and returns the config and path used.
func Load(explicitPath string) (Config, string, error) {
	path, err := ResolvePath(explicitPath)
	if err != nil {
		return Config{}, "", err
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

// LoadFromPath loads and validates config from the provided path.
func LoadFromPath(path string) (Config, error) {
	cfg := Config{}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("config file not found at %s (create it or set %s)", path, EnvConfigPath)
		}
		return cfg, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file %s: %w", path, err)
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("validate config file %s: %w", path, err)
	}

	return cfg, nil
}

// Save writes config back to path, replacing the file atomically.
func Save(path string, cfg Config) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is required")
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config file %s: %w", path, err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config file %s: %w", path, err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	perm := os.FileMode(0o600)
	info, err := os.Stat(path)
	if err == nil {
		perm = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config file %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp config %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config file %s: %w", path, err)
	}

	cleanup = false
	return nil
}
