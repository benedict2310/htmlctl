//go:build windows

package transport

// Windows ssh-agent typically uses named pipes rather than unix-domain sockets.
// v1 transport socket validation is a unix-only hardening check.
func validateAgentSocket(path string) error {
	return nil
}
