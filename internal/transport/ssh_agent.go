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

	return ssh.PublicKeysCallback(agentClient.Signers), conn, nil
}
