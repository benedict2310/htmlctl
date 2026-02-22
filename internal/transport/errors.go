package transport

import "errors"

var (
	// ErrSSHAuth indicates SSH authentication failure.
	ErrSSHAuth = errors.New("ssh authentication failed")
	// ErrSSHTunnel indicates local/remote tunnel forwarding failure.
	ErrSSHTunnel = errors.New("ssh tunnel failed")
	// ErrSSHHostKey indicates known-hosts verification failure.
	ErrSSHHostKey = errors.New("ssh host key verification failed")
	// ErrSSHUnreachable indicates host connectivity or timeout failure.
	ErrSSHUnreachable = errors.New("ssh host unreachable")
	// ErrSSHAgentUnavailable indicates SSH agent is unavailable/misconfigured.
	ErrSSHAgentUnavailable = errors.New("ssh agent unavailable")
	// ErrSSHKeyPath indicates private-key path sanitization/validation failure.
	ErrSSHKeyPath = errors.New("ssh private key path invalid")
)
