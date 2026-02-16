package transport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestParseServerURLDefaultPort(t *testing.T) {
	endpoint, err := ParseServerURL("ssh://root@example.com")
	if err != nil {
		t.Fatalf("ParseServerURL() error = %v", err)
	}
	if endpoint.User != "root" || endpoint.Host != "example.com" || endpoint.Port != DefaultSSHPort {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
}

func TestParseServerURLCustomPort(t *testing.T) {
	endpoint, err := ParseServerURL("ssh://deploy@example.com:2222")
	if err != nil {
		t.Fatalf("ParseServerURL() error = %v", err)
	}
	if endpoint.User != "deploy" || endpoint.Host != "example.com" || endpoint.Port != 2222 {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
}

func TestParseServerURLRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"",
		"http://root@example.com",
		"ssh://example.com",
		"ssh://root@",
		"ssh://root@example.com/bad-path",
		"ssh://root@example.com:0",
		"ssh://root@example.com:99999",
	}
	for _, input := range tests {
		_, err := ParseServerURL(input)
		if err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}
}

func TestNewSSHTransportUnreachableHostClassifiesAsUnreachable(t *testing.T) {
	_, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:   "ssh://root@127.0.0.1:1",
		HostKeyCB:   ssh.InsecureIgnoreHostKey(),
		AuthMethods: []ssh.AuthMethod{ssh.Password("unused")},
		Timeout:     300 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected unreachable host error")
	}
	if !errors.Is(err, ErrSSHUnreachable) && !strings.Contains(err.Error(), ErrSSHUnreachable.Error()) {
		t.Fatalf("expected unreachable classification, got %v", err)
	}
}

func TestNewSSHTransportRejectsInvalidRemoteAddr(t *testing.T) {
	_, err := NewSSHTransport(t.Context(), SSHConfig{
		ServerURL:  "ssh://root@example.com",
		RemoteAddr: "bad-addr",
	})
	if err == nil {
		t.Fatalf("expected invalid remote address error")
	}
	if !errors.Is(err, ErrSSHTunnel) {
		t.Fatalf("expected ErrSSHTunnel, got %v", err)
	}
}
