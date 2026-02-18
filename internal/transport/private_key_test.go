package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrivateKeyPathExplicitWins(t *testing.T) {
	t.Setenv(envSSHKeyPath, "/env/key")
	got := resolvePrivateKeyPath("/explicit/key")
	if got != "/explicit/key" {
		t.Fatalf("expected explicit key path, got %q", got)
	}
}

func TestResolvePrivateKeyPathEnvFallback(t *testing.T) {
	t.Setenv(envSSHKeyPath, "/env/key")
	got := resolvePrivateKeyPath("")
	if got != "/env/key" {
		t.Fatalf("expected env key path, got %q", got)
	}
}

func TestResolveKnownHostsPathEnvFallback(t *testing.T) {
	t.Setenv(envKnownHostsPath, "/env/known_hosts")
	got := resolveKnownHostsPath("")
	if got != "/env/known_hosts" {
		t.Fatalf("expected env known_hosts path, got %q", got)
	}
}

func TestAuthMethodFromPrivateKeyParsesRSAKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	method, err := authMethodFromPrivateKey(path)
	if err != nil {
		t.Fatalf("authMethodFromPrivateKey() error = %v", err)
	}
	if method == nil {
		t.Fatalf("expected auth method")
	}
}
