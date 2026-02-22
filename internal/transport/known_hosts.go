package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func knownHostsCallback(path string) (ssh.HostKeyCallback, error) {
	khPath := strings.TrimSpace(path)
	if khPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("%w: resolve user home for known_hosts: %w", ErrSSHHostKey, err)
		}
		khPath = filepath.Join(home, ".ssh", "known_hosts")
	}
	khPath = filepath.Clean(khPath)
	if !filepath.IsAbs(khPath) {
		abs, err := filepath.Abs(khPath)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve known_hosts path: %w", ErrSSHHostKey, err)
		}
		khPath = filepath.Clean(abs)
	}

	cb, err := knownhosts.New(khPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: known_hosts file not found (create it via 'ssh <user>@<host>' or ssh-keyscan)", ErrSSHHostKey)
		}
		return nil, fmt.Errorf("%w: load known_hosts: %w", ErrSSHHostKey, redactPathError(err))
	}
	return cb, nil
}
