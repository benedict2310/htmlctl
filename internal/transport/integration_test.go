package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHTransportForwardsHTTPAndCloses(t *testing.T) {
	backendAddr, shutdownBackend := startHTTPBackend(t)
	defer shutdownBackend()

	allowedSigner := newSigner(t)
	srv := startSSHServer(t, allowedSigner.PublicKey())
	defer srv.Close()

	knownHostsPath := writeKnownHostsFile(t, srv.addr, srv.hostSigner.PublicKey())
	serverURL := fmt.Sprintf("ssh://tester@%s", srv.addr)

	tr, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:      serverURL,
		RemoteAddr:     backendAddr,
		KnownHostsPath: knownHostsPath,
		AuthMethods:    []ssh.AuthMethod{ssh.PublicKeys(allowedSigner)},
		Timeout:        3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSSHTransport() error = %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://placeholder/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := tr.Do(t.Context(), req)
	if err != nil {
		_ = tr.Close()
		t.Fatalf("Do() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = tr.Close()
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "ok:/healthz" {
		_ = tr.Close()
		t.Fatalf("unexpected response body: %q", string(body))
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	req2, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://placeholder/after-close", nil)
	if err != nil {
		t.Fatalf("new request after close: %v", err)
	}
	if _, err := tr.Do(t.Context(), req2); err == nil {
		t.Fatalf("expected Do() to fail after Close()")
	}
}

func TestSSHTransportRejectsUnknownHostKey(t *testing.T) {
	backendAddr, shutdownBackend := startHTTPBackend(t)
	defer shutdownBackend()

	allowedSigner := newSigner(t)
	srv := startSSHServer(t, allowedSigner.PublicKey())
	defer srv.Close()

	emptyKnownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(emptyKnownHostsPath, []byte(""), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	_, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:      fmt.Sprintf("ssh://tester@%s", srv.addr),
		RemoteAddr:     backendAddr,
		KnownHostsPath: emptyKnownHostsPath,
		AuthMethods:    []ssh.AuthMethod{ssh.PublicKeys(allowedSigner)},
		Timeout:        3 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected host key verification error")
	}
	if !errors.Is(err, ErrSSHHostKey) {
		t.Fatalf("expected ErrSSHHostKey, got %v", err)
	}
}

func TestSSHTransportAuthFailureClassification(t *testing.T) {
	backendAddr, shutdownBackend := startHTTPBackend(t)
	defer shutdownBackend()

	allowedSigner := newSigner(t)
	srv := startSSHServer(t, allowedSigner.PublicKey())
	defer srv.Close()

	knownHostsPath := writeKnownHostsFile(t, srv.addr, srv.hostSigner.PublicKey())
	wrongSigner := newSigner(t)

	_, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:      fmt.Sprintf("ssh://tester@%s", srv.addr),
		RemoteAddr:     backendAddr,
		KnownHostsPath: knownHostsPath,
		AuthMethods:    []ssh.AuthMethod{ssh.PublicKeys(wrongSigner)},
		Timeout:        3 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected auth error")
	}
	if !errors.Is(err, ErrSSHAuth) {
		t.Fatalf("expected ErrSSHAuth, got %v", err)
	}
}

func TestSSHTransportUsesPrivateKeyFallbackWhenAgentUnavailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	backendAddr, shutdownBackend := startHTTPBackend(t)
	defer shutdownBackend()

	allowedSigner, privateKeyPath := newRSASignerWithPrivateKeyFile(t)
	srv := startSSHServer(t, allowedSigner.PublicKey())
	defer srv.Close()

	knownHostsPath := writeKnownHostsFile(t, srv.addr, srv.hostSigner.PublicKey())
	tr, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:      fmt.Sprintf("ssh://tester@%s", srv.addr),
		RemoteAddr:     backendAddr,
		KnownHostsPath: knownHostsPath,
		PrivateKeyPath: privateKeyPath,
		Timeout:        3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSSHTransport() error = %v", err)
	}
	defer tr.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://placeholder/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := tr.Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "ok:/healthz" {
		t.Fatalf("unexpected fallback response status=%d body=%q", resp.StatusCode, string(body))
	}
}

func startHTTPBackend(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen backend: %v", err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok:"+r.URL.Path)
		}),
	}
	go func() {
		_ = server.Serve(listener)
	}()

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = listener.Close()
	}
	return listener.Addr().String(), shutdown
}

type sshTestServer struct {
	addr       string
	hostSigner ssh.Signer

	listener net.Listener
	wg       sync.WaitGroup
}

func startSSHServer(t *testing.T, authorized ssh.PublicKey) *sshTestServer {
	t.Helper()

	hostSigner := newSigner(t)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() != "tester" {
				return nil, fmt.Errorf("unknown user %q", conn.User())
			}
			if !publicKeysEqual(key, authorized) {
				return nil, fmt.Errorf("unauthorized key")
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ssh server: %v", err)
	}

	s := &sshTestServer{
		addr:       listener.Addr().String(),
		hostSigner: hostSigner,
		listener:   listener,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handleConn(conn, cfg)
			}()
		}
	}()

	return s
}

func (s *sshTestServer) Close() {
	_ = s.listener.Close()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (s *sshTestServer) handleConn(conn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		var channelData struct {
			DestAddr   string
			DestPort   uint32
			OriginAddr string
			OriginPort uint32
		}
		if err := ssh.Unmarshal(newCh.ExtraData(), &channelData); err != nil {
			_ = newCh.Reject(ssh.Prohibited, "invalid direct-tcpip payload")
			continue
		}

		downstream, err := net.Dial("tcp", net.JoinHostPort(channelData.DestAddr, strconv.Itoa(int(channelData.DestPort))))
		if err != nil {
			_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}

		upstream, reqs, err := newCh.Accept()
		if err != nil {
			_ = downstream.Close()
			continue
		}
		go ssh.DiscardRequests(reqs)

		go func() {
			defer upstream.Close()
			defer downstream.Close()

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, _ = io.Copy(downstream, upstream)
			}()
			go func() {
				defer wg.Done()
				_, _ = io.Copy(upstream, downstream)
			}()
			wg.Wait()
		}()
	}
	_ = sshConn.Close()
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer
}

func newRSASignerWithPrivateKeyFile(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer from rsa key: %v", err)
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve user home: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("create .ssh dir: %v", err)
	}
	path := filepath.Join(home, ".ssh", "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	return signer, path
}

func writeKnownHostsFile(t *testing.T, addr string, hostKey ssh.PublicKey) string {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host:port: %v", err)
	}
	line := knownhosts.Line([]string{fmt.Sprintf("[%s]:%s", host, port)}, hostKey) + "\n"

	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

func publicKeysEqual(a, b ssh.PublicKey) bool {
	if a == nil || b == nil {
		return false
	}
	return string(a.Marshal()) == string(b.Marshal())
}

// TestSSHTransportFallsBackToKeyFileWhenAgentHoldsWrongKey verifies that when
// an SSH agent is reachable but contains only a key that the server rejects,
// NewSSHTransport falls back to the configured private key file and succeeds.
func TestSSHTransportFallsBackToKeyFileWhenAgentHoldsWrongKey(t *testing.T) {
	backendAddr, shutdownBackend := startHTTPBackend(t)
	defer shutdownBackend()

	// Server only accepts the "correct" signer.
	correctSigner, privateKeyPath := newRSASignerWithPrivateKeyFile(t)
	srv := startSSHServer(t, correctSigner.PublicKey())
	defer srv.Close()

	knownHostsPath := writeKnownHostsFile(t, srv.addr, srv.hostSigner.PublicKey())

	// Start an in-memory SSH agent that holds only a wrong key.
	wrongSigner, wrongKey := newEd25519SignerWithKey(t)
	_ = wrongSigner
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: wrongKey}); err != nil {
		t.Fatalf("add wrong key to agent: %v", err)
	}
	agentSockPath := serveAgentOnSocket(t, keyring)
	t.Setenv("SSH_AUTH_SOCK", agentSockPath)

	tr, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:      fmt.Sprintf("ssh://tester@%s", srv.addr),
		RemoteAddr:     backendAddr,
		KnownHostsPath: knownHostsPath,
		PrivateKeyPath: privateKeyPath,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSSHTransport() should fall back to key file when agent holds wrong key, got: %v", err)
	}
	defer tr.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://placeholder/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := tr.Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "ok:/healthz" {
		t.Fatalf("unexpected response status=%d body=%q", resp.StatusCode, string(body))
	}
}

// newEd25519SignerWithKey returns an ssh.Signer and the underlying private key.
func newEd25519SignerWithKey(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatalf("new signer from ed25519 key: %v", err)
	}
	return signer, private
}

// serveAgentOnSocket starts an in-memory SSH agent listening on a temp unix
// socket and returns the socket path. Uses os.TempDir() directly to keep the
// path short enough to satisfy the macOS 104-character unix socket path limit.
// The listener and temp dir are cleaned up via t.Cleanup.
func serveAgentOnSocket(t *testing.T, ag agent.Agent) string {
	t.Helper()
	dir, err := os.MkdirTemp(os.TempDir(), "htmlctl-agent-")
	if err != nil {
		t.Fatalf("mkdir temp agent dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen agent socket: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = agent.ServeAgent(ag, conn)
			}()
		}
	}()
	return sockPath
}
