package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrivateKeyPathExplicitWins(t *testing.T) {
	home := userHomeDirForTest(t)
	t.Setenv(envSSHKeyPath, filepath.Join(home, ".ssh", "env_key"))

	explicit := filepath.Join(home, ".ssh", "nested", "..", "id_rsa")
	got, err := resolvePrivateKeyPath(explicit)
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error = %v", err)
	}
	want := filepath.Clean(explicit)
	if got != want {
		t.Fatalf("expected explicit key path %q, got %q", want, got)
	}
}

func TestResolvePrivateKeyPathEnvFallback(t *testing.T) {
	home := userHomeDirForTest(t)
	envKey := filepath.Join(home, ".ssh", "env_key")
	t.Setenv(envSSHKeyPath, envKey)

	got, err := resolvePrivateKeyPath("")
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error = %v", err)
	}
	if got != envKey {
		t.Fatalf("expected env key path, got %q", got)
	}
}

func TestResolvePrivateKeyPathRejectsOutsideHome(t *testing.T) {
	_, err := resolvePrivateKeyPath("/etc/passwd")
	if err == nil {
		t.Fatalf("expected outside-home error")
	}
	if !errors.Is(err, ErrSSHKeyPath) {
		t.Fatalf("expected ErrSSHKeyPath, got %v", err)
	}
	if !strings.Contains(err.Error(), "within user home directory") {
		t.Fatalf("expected home-directory restriction error, got %v", err)
	}
}

func TestResolvePrivateKeyPathRejectsTraversalOutsideHome(t *testing.T) {
	home := userHomeDirForTest(t)

	_, err := resolvePrivateKeyPath(filepath.Join(home, "..", "escape", "id_rsa"))
	if err == nil {
		t.Fatalf("expected traversal outside-home error")
	}
	if !errors.Is(err, ErrSSHKeyPath) {
		t.Fatalf("expected ErrSSHKeyPath, got %v", err)
	}
	if !strings.Contains(err.Error(), "within user home directory") {
		t.Fatalf("expected home-directory restriction error, got %v", err)
	}
}

func TestSanitizePrivateKeyPathResolvesRelativeInsideHome(t *testing.T) {
	home := userHomeDirForTest(t)
	inside := tempDirUnderHome(t, "htmlctl-key-rel-inside-")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(inside); err != nil {
		t.Fatalf("chdir inside home: %v", err)
	}

	got, err := sanitizePrivateKeyPath("id_rsa")
	if err != nil {
		t.Fatalf("sanitizePrivateKeyPath() error = %v", err)
	}
	want, err := filepath.Abs("id_rsa")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	want = filepath.Clean(want)
	if got != want {
		t.Fatalf("expected relative path to resolve to %q, got %q", want, got)
	}
	if !strings.HasPrefix(filepath.Clean(got), filepath.Clean(home)) &&
		!strings.HasPrefix(filepath.Clean(got), filepath.Clean(mustEvalSymlinks(t, home))) {
		t.Fatalf("expected resolved key path to stay under home, got %q", got)
	}
}

func TestSanitizePrivateKeyPathRejectsRelativeOutsideHome(t *testing.T) {
	_ = userHomeDirForTest(t)
	outside := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(outside); err != nil {
		t.Fatalf("chdir outside home: %v", err)
	}

	_, err = sanitizePrivateKeyPath("id_rsa")
	if err == nil {
		t.Fatalf("expected relative outside-home rejection")
	}
	if !errors.Is(err, ErrSSHKeyPath) {
		t.Fatalf("expected ErrSSHKeyPath, got %v", err)
	}
	if !strings.Contains(err.Error(), "within user home directory") {
		t.Fatalf("expected home-directory restriction error, got %v", err)
	}
}

func TestSanitizePrivateKeyPathRejectsHomeDirectoryPath(t *testing.T) {
	home := userHomeDirForTest(t)

	_, err := sanitizePrivateKeyPath(home)
	if err == nil {
		t.Fatalf("expected home-directory-as-key rejection")
	}
	if !errors.Is(err, ErrSSHKeyPath) {
		t.Fatalf("expected ErrSSHKeyPath, got %v", err)
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

	path := filepath.Join(tempDirUnderHome(t, "htmlctl-key-parse-"), "id_rsa")
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

func TestAuthMethodFromPrivateKeyReadErrorDoesNotLeakPath(t *testing.T) {
	missingPath := filepath.Join(tempDirUnderHome(t, "htmlctl-key-missing-"), "missing")

	_, err := authMethodFromPrivateKey(missingPath)
	if err == nil {
		t.Fatalf("expected read error")
	}
	if strings.Contains(err.Error(), missingPath) {
		t.Fatalf("expected sanitized read error without path, got %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wrapped os.ErrNotExist, got %v", err)
	}
}

func TestAuthMethodFromPrivateKeyParseErrorDoesNotLeakPath(t *testing.T) {
	path := filepath.Join(tempDirUnderHome(t, "htmlctl-key-bad-"), "bad_key")
	if err := os.WriteFile(path, []byte("not-a-private-key"), 0o600); err != nil {
		t.Fatalf("write bad key: %v", err)
	}

	_, err := authMethodFromPrivateKey(path)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("expected sanitized parse error without path, got %v", err)
	}
}

func userHomeDirForTest(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve user home: %v", err)
	}
	return filepath.Clean(home)
}

func tempDirUnderHome(t *testing.T, pattern string) string {
	t.Helper()
	home := userHomeDirForTest(t)
	dir, err := os.MkdirTemp(home, pattern)
	if err != nil {
		t.Fatalf("mkdir temp under home %q: %v", home, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return filepath.Clean(resolved)
}
