package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
)

const (
	DefaultSSHPort = 22
	// Keep in sync with internal/server.DefaultPort.
	DefaultRemoteAddr  = "127.0.0.1:9400"
	DefaultDialTimeout = 10 * time.Second
)

// SSHConfig configures an SSH-backed transport.
type SSHConfig struct {
	ServerURL      string
	RemoteAddr     string
	Timeout        time.Duration
	KnownHostsPath string
	PrivateKeyPath string
	AuthMethods    []ssh.AuthMethod
}

// ServerEndpoint is the parsed ssh:// server target.
type ServerEndpoint struct {
	User string
	Host string
	Port int
}

func (e ServerEndpoint) Address() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

// ParseServerURL parses ssh://user@host[:port] server URLs.
func ParseServerURL(raw string) (ServerEndpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ServerEndpoint{}, fmt.Errorf("server URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ServerEndpoint{}, fmt.Errorf("parse server URL %q: %w", raw, err)
	}
	if u.Scheme != "ssh" {
		return ServerEndpoint{}, fmt.Errorf("invalid server URL scheme %q: expected ssh", u.Scheme)
	}
	if u.User == nil || strings.TrimSpace(u.User.Username()) == "" {
		return ServerEndpoint{}, fmt.Errorf("server URL %q must include user (ssh://user@host)", raw)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return ServerEndpoint{}, fmt.Errorf("server URL %q must include host", raw)
	}
	if path := strings.TrimSpace(u.EscapedPath()); path != "" && path != "/" {
		return ServerEndpoint{}, fmt.Errorf("server URL %q must not include path", raw)
	}

	port := DefaultSSHPort
	if p := strings.TrimSpace(u.Port()); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return ServerEndpoint{}, fmt.Errorf("server URL %q has invalid port %q", raw, p)
		}
		port = n
	}

	return ServerEndpoint{
		User: u.User.Username(),
		Host: host,
		Port: port,
	}, nil
}

// SSHTransport forwards HTTP requests through an SSH local port-forward.
type SSHTransport struct {
	endpoint   ServerEndpoint
	remoteAddr string

	sshClient *ssh.Client
	listener  net.Listener
	http      *http.Client

	closers   []io.Closer
	closeOnce sync.Once
}

// NewSSHTransport opens a per-command SSH tunnel transport.
func NewSSHTransport(ctx context.Context, cfg SSHConfig) (*SSHTransport, error) {
	endpoint, err := ParseServerURL(cfg.ServerURL)
	if err != nil {
		return nil, err
	}

	remoteAddr := strings.TrimSpace(cfg.RemoteAddr)
	if remoteAddr == "" {
		remoteAddr = DefaultRemoteAddr
	}
	remoteHost, remotePortStr, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid remote address %q: %v", ErrSSHTunnel, remoteAddr, err)
	}
	if strings.TrimSpace(remoteHost) == "" {
		return nil, fmt.Errorf("%w: invalid remote address %q: host is required", ErrSSHTunnel, remoteAddr)
	}
	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil || remotePort < 1 || remotePort > 65535 {
		return nil, fmt.Errorf("%w: invalid remote address %q: port must be in range 1..65535", ErrSSHTunnel, remoteAddr)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	dialCtx := ctx
	if _, ok := dialCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	hostKeyCB, err := knownHostsCallback(resolveKnownHostsPath(cfg.KnownHostsPath))
	if err != nil {
		return nil, err
	}

	authMethods := cfg.AuthMethods
	var closers []io.Closer
	if len(authMethods) == 0 {
		authMethod, closer, err := authMethodFromSSHAgent()
		if err != nil {
			keyPath, keyPathErr := resolvePrivateKeyPath(cfg.PrivateKeyPath)
			if keyPathErr != nil {
				return nil, fmt.Errorf("ssh agent unavailable (%v); private key fallback failed: %w", err, keyPathErr)
			}
			authMethod, keyErr := authMethodFromPrivateKey(keyPath)
			if keyErr != nil {
				if errors.Is(keyErr, ErrSSHKeyPath) {
					return nil, fmt.Errorf("ssh agent unavailable (%v); private key fallback failed: %w", err, keyErr)
				}
				return nil, fmt.Errorf("%w: %v; private key fallback failed: %v", ErrSSHAgentUnavailable, err, keyErr)
			}
			authMethods = []ssh.AuthMethod{authMethod}
		} else {
			authMethods = []ssh.AuthMethod{authMethod}
			if closer != nil {
				closers = append(closers, closer)
			}
		}
	}

	clientCfg := &ssh.ClientConfig{
		User:            endpoint.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCB,
	}

	sshClient, err := dialSSHClient(dialCtx, endpoint, clientCfg)
	if err != nil {
		closeAll(closers...)
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = sshClient.Close()
		closeAll(closers...)
		return nil, fmt.Errorf("%w: open local listener: %v", ErrSSHTunnel, err)
	}

	t := &SSHTransport{
		endpoint:   endpoint,
		remoteAddr: remoteAddr,
		sshClient:  sshClient,
		listener:   listener,
		http: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		closers: closers,
	}
	go t.acceptLoop()
	return t, nil
}

func (t *SSHTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.URL == nil {
		return nil, fmt.Errorf("request URL is required")
	}

	outReq := req.Clone(ctx)
	urlCopy := *outReq.URL
	urlCopy.Scheme = "http"
	urlCopy.Host = t.listener.Addr().String()
	outReq.URL = &urlCopy
	outReq.RequestURI = ""
	if outReq.Host == "" {
		outReq.Host = urlCopy.Host
	}

	resp, err := t.http.Do(outReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSSHTunnel, err)
	}
	return resp, nil
}

func (t *SSHTransport) Close() error {
	var outErr error
	t.closeOnce.Do(func() {
		if t.listener != nil {
			outErr = errors.Join(outErr, t.listener.Close())
		}
		if t.sshClient != nil {
			outErr = errors.Join(outErr, t.sshClient.Close())
		}
		for _, c := range t.closers {
			if c != nil {
				outErr = errors.Join(outErr, c.Close())
			}
		}
	})
	return outErr
}

func (t *SSHTransport) acceptLoop() {
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			return
		}
		go t.forwardConn(localConn)
	}
}

func (t *SSHTransport) forwardConn(localConn net.Conn) {
	remoteConn, err := t.sshClient.Dial("tcp", t.remoteAddr)
	if err != nil {
		_ = localConn.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remoteConn, localConn)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(localConn, remoteConn)
	}()
	wg.Wait()
	_ = localConn.Close()
	_ = remoteConn.Close()
}

func dialSSHClient(ctx context.Context, endpoint ServerEndpoint, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.Address())
	if err != nil {
		return nil, classifySSHConnectError(err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, endpoint.Address(), cfg)
	if err != nil {
		_ = conn.Close()
		return nil, classifySSHConnectError(err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}

func classifySSHConnectError(err error) error {
	if err == nil {
		return nil
	}

	var keyErr *xknownhosts.KeyError
	if errors.As(err, &keyErr) {
		return fmt.Errorf("%w: %v", ErrSSHHostKey, err)
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unable to authenticate"),
		strings.Contains(msg, "no supported methods remain"),
		strings.Contains(msg, "permission denied"):
		return fmt.Errorf("%w: %v", ErrSSHAuth, err)
	case strings.Contains(msg, "knownhosts"),
		strings.Contains(msg, "host key"):
		return fmt.Errorf("%w: %v", ErrSSHHostKey, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", ErrSSHUnreachable, err)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Errorf("%w: %v", ErrSSHUnreachable, err)
	}

	return fmt.Errorf("%w: %v", ErrSSHTunnel, err)
}

func closeAll(closers ...io.Closer) {
	for _, c := range closers {
		if c != nil {
			_ = c.Close()
		}
	}
}
