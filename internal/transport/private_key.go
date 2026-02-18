package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	envSSHKeyPath     = "HTMLCTL_SSH_KEY_PATH"
	envKnownHostsPath = "HTMLCTL_SSH_KNOWN_HOSTS_PATH"
)

func resolvePrivateKeyPath(explicit string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envSSHKeyPath)); v != "" {
		return v
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		path := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
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
	keyPath := strings.TrimSpace(path)
	if keyPath == "" {
		return nil, fmt.Errorf("private key path is empty")
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: %w", keyPath, err)
	}

	return ssh.PublicKeys(signer), nil
}
