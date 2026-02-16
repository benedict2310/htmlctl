package transport

import (
	"errors"
	"testing"

	"github.com/benedict2310/htmlctl/internal/config"
	"golang.org/x/crypto/ssh"
)

func TestNewSSHTransportFromContextUsesContextServer(t *testing.T) {
	_, err := NewSSHTransportFromContext(t.Context(), config.ContextInfo{
		Server: "ssh://root@127.0.0.1:1",
	}, SSHConfig{
		HostKeyCB:   ssh.InsecureIgnoreHostKey(),
		AuthMethods: []ssh.AuthMethod{ssh.Password("unused")},
	})
	if err == nil {
		t.Fatalf("expected connection failure")
	}
	if !errors.Is(err, ErrSSHUnreachable) {
		t.Fatalf("expected ErrSSHUnreachable, got %v", err)
	}
}

func TestNewSSHTransportFromContextUsesConfiguredRemotePort(t *testing.T) {
	_, err := NewSSHTransportFromContext(t.Context(), config.ContextInfo{
		Server:     "ssh://root@127.0.0.1:1",
		RemotePort: 70000,
	}, SSHConfig{
		HostKeyCB:   ssh.InsecureIgnoreHostKey(),
		AuthMethods: []ssh.AuthMethod{ssh.Password("unused")},
	})
	if err == nil {
		t.Fatalf("expected remote address validation failure")
	}
	if !errors.Is(err, ErrSSHTunnel) {
		t.Fatalf("expected ErrSSHTunnel from invalid remote port, got %v", err)
	}
}
