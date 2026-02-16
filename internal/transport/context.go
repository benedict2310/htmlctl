package transport

import (
	"context"
	"fmt"

	"github.com/benedict2310/htmlctl/internal/config"
)

// NewSSHTransportFromContext creates a transport using context server details.
func NewSSHTransportFromContext(ctx context.Context, info config.ContextInfo, cfg SSHConfig) (*SSHTransport, error) {
	if cfg.ServerURL == "" {
		cfg.ServerURL = info.Server
	}
	if cfg.RemoteAddr == "" && info.RemotePort > 0 {
		cfg.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", info.RemotePort)
	}
	return NewSSHTransport(ctx, cfg)
}
