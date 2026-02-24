package transport

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func authMethodFromSSHAgent() (ssh.AuthMethod, io.Closer, error) {
	signersFn, closer, err := agentSignersFn()
	if err != nil {
		return nil, nil, err
	}
	return ssh.PublicKeysCallback(signersFn), closer, nil
}

// agentSignersFn returns the agent's Signers callback and a closer for the
// underlying connection. Callers may wrap the callback to combine it with
// additional signers (e.g., a private key file) in a single PublicKeysCallback.
func agentSignersFn() (func() ([]ssh.Signer, error), io.Closer, error) {
	sockPath := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if sockPath == "" {
		return nil, nil, fmt.Errorf("%w: SSH_AUTH_SOCK is not set", ErrSSHAgentUnavailable)
	}
	if err := validateAgentSocket(sockPath); err != nil {
		return nil, nil, err
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: connect SSH_AUTH_SOCK at %s: %v", ErrSSHAgentUnavailable, sockPath, err)
	}

	agentClient := agent.NewClient(conn)
	if _, err := agentClient.Signers(); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("%w: list agent signers: %v", ErrSSHAgentUnavailable, err)
	}

	return agentClient.Signers, conn, nil
}
