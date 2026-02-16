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
			return nil, fmt.Errorf("resolve user home for known_hosts: %w", err)
		}
		khPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	cb, err := knownhosts.New(khPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: known_hosts file not found at %s (create it via 'ssh <user>@<host>' or ssh-keyscan)", ErrSSHHostKey, khPath)
		}
		return nil, fmt.Errorf("%w: load known_hosts %s: %v", ErrSSHHostKey, khPath, err)
	}
	return cb, nil
}
