//go:build !windows

package transport

import (
	"fmt"
	"os"
	"syscall"
)

func validateAgentSocket(path string) error {
	return validateAgentSocketOwnedBy(path, os.Geteuid())
}

func validateAgentSocketOwnedBy(path string, expectedUID int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: inspect SSH_AUTH_SOCK: %v", ErrSSHAgentUnavailable, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%w: SSH_AUTH_SOCK is not a unix socket", ErrSSHAgentUnavailable)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%w: inspect SSH_AUTH_SOCK ownership", ErrSSHAgentUnavailable)
	}
	// Root often operates with bind-mounted agent sockets owned by another uid in CI/containers.
	// In that mode we enforce socket type but skip uid ownership enforcement.
	if expectedUID == 0 {
		return nil
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("%w: SSH_AUTH_SOCK ownership mismatch", ErrSSHAgentUnavailable)
	}
	return nil
}
