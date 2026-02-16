package transport

import (
	"errors"
	"testing"
)

func TestAuthMethodFromSSHAgentMissingSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	_, _, err := authMethodFromSSHAgent()
	if err == nil {
		t.Fatalf("expected missing SSH_AUTH_SOCK error")
	}
	if !errors.Is(err, ErrSSHAgentUnavailable) {
		t.Fatalf("expected ErrSSHAgentUnavailable, got %v", err)
	}
}
