package transport

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	envSSHKeyPath     = "HTMLCTL_SSH_KEY_PATH"
	envKnownHostsPath = "HTMLCTL_SSH_KNOWN_HOSTS_PATH"
)

func resolvePrivateKeyPath(explicit string) (string, error) {
	if v := strings.TrimSpace(explicit); v != "" {
		return sanitizePrivateKeyPath(v)
	}
	if v := strings.TrimSpace(os.Getenv(envSSHKeyPath)); v != "" {
		return sanitizePrivateKeyPath(v)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: resolve user home for private key path: %w", ErrSSHKeyPath, err)
	}
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		path, err := sanitizePrivateKeyPath(filepath.Join(home, ".ssh", name))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

func resolveKnownHostsPath(explicit string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envKnownHostsPath)); v != "" {
		return v
	}
	return ""
}

func authMethodFromPrivateKey(path string) (ssh.AuthMethod, error) {
	signer, err := signerFromPrivateKey(path)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func signerFromPrivateKey(path string) (ssh.Signer, error) {
	keyPath, err := sanitizePrivateKeyPath(path)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf("read private key: %w", os.ErrNotExist)
		case errors.Is(err, os.ErrPermission):
			return nil, fmt.Errorf("read private key: %w", os.ErrPermission)
		default:
			return nil, fmt.Errorf("read private key: %w", redactPathError(err))
		}
	}

	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}

func sanitizePrivateKeyPath(input string) (string, error) {
	path := strings.TrimSpace(input)
	if path == "" {
		return "", fmt.Errorf("%w: private key path is empty", ErrSSHKeyPath)
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("%w: resolve private key path: %w", ErrSSHKeyPath, err)
		}
		path = filepath.Clean(abs)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: resolve user home for private key path: %w", ErrSSHKeyPath, err)
	}
	homeResolved := filepath.Clean(home)
	if evalHome, err := filepath.EvalSymlinks(homeResolved); err == nil {
		homeResolved = filepath.Clean(evalHome)
	}

	pathResolved := path
	if evalPath, err := filepath.EvalSymlinks(path); err == nil {
		pathResolved = filepath.Clean(evalPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: resolve private key symlinks: %w", ErrSSHKeyPath, redactPathError(err))
	} else if evalDir, dirErr := filepath.EvalSymlinks(filepath.Dir(path)); dirErr == nil {
		pathResolved = filepath.Join(filepath.Clean(evalDir), filepath.Base(path))
	} else if !errors.Is(dirErr, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: resolve private key directory symlinks: %w", ErrSSHKeyPath, redactPathError(dirErr))
	}

	rel, err := filepath.Rel(homeResolved, pathResolved)
	if err != nil {
		return "", fmt.Errorf("%w: validate private key path: %w", ErrSSHKeyPath, err)
	}
	if rel == "." {
		return "", fmt.Errorf("%w: private key path must point to a file under user home directory", ErrSSHKeyPath)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: private key path must be within user home directory", ErrSSHKeyPath)
	}

	return pathResolved, nil
}

func redactPathError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) && linkErr.Err != nil {
		return linkErr.Err
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) && syscallErr.Err != nil {
		return syscallErr.Err
	}
	return err
}
