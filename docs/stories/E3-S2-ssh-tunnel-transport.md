# E3-S2 - SSH Tunnel Transport

**Epic:** Epic 3 — Remote transport + kubectl UX
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E3-S1 (context config provides server URL)
**Target:** htmlctl v1
**Design Reference:** Technical Spec Section 7 (Control plane security model), Section 9.1 (Config / contexts)

---

## 1. Objective

Implement the SSH tunnel transport layer that enables htmlctl to securely communicate with a remote htmlservd instance. Since htmlservd binds to `127.0.0.1` only (by design — no public admin plane), the CLI must open an SSH tunnel (local port forward) to reach the server's localhost API. This transport layer is the secure bridge between the CLI and daemon, and all remote commands depend on it.

## 2. User Story

As an operator or AI agent, I want htmlctl to automatically establish an SSH tunnel to the remote server so that I can run remote commands (apply, diff, status, logs) securely without manually setting up port forwarding or exposing the control plane to the internet.

## 3. Scope

### In Scope

- Parse `ssh://user@host[:port]` server URLs from context config
- Open an SSH connection using Go's `golang.org/x/crypto/ssh` library
- Set up a local port forward (dynamic local port -> remote `127.0.0.1:htmlservd_port`)
- Multiplex per command: open tunnel, make HTTP request(s) to local forwarded port, close tunnel
- SSH key-based authentication via system SSH agent (`SSH_AUTH_SOCK`)
- Support custom SSH port in server URL (e.g., `ssh://root@server:2222`)
- Default remote target port for htmlservd (e.g., `127.0.0.1:8420` on the server)
- Expose a `Transport` interface that command layer uses to make HTTP requests to the server
- Connection timeout and error handling (unreachable host, auth failure, tunnel failure)
- Known hosts verification using `~/.ssh/known_hosts`

### Out of Scope

- Password-based SSH authentication (key-based only in v1)
- SSH certificate authentication
- Persistent / long-lived tunnel connections (each command opens and closes its own)
- Connection pooling or multiplexing across multiple commands
- SSH config file parsing (`~/.ssh/config` — host aliases, proxy jumps)
- Custom SSH key file paths (agent-only in v1; key file support can be added later)
- Non-SSH transport (e.g., direct HTTPS, mTLS)

## 4. Architecture Alignment

- **Technical Spec Section 7**: htmlservd binds to `127.0.0.1` only. Remote access is via SSH tunnel. This story implements the client side of that security model.
- **Technical Spec Section 9.1**: Server URLs in config use `ssh://` scheme. The transport layer parses these URLs to extract user, host, and port.
- **Component boundary**: Transport is a standalone package (`internal/transport/`) that exports a `Transport` interface and an `SSHTransport` implementation. It depends on `internal/config` for context resolution but has no dependency on specific commands.
- **Security model**: Authentication relies on the SSH agent — no secrets stored in config files. Known hosts checking prevents MITM attacks.
- **PRD Section 0**: "Private control plane by default (SSH tunnel / localhost)" — this story directly implements this design decision.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/transport/transport.go` — `Transport` interface definition: `Do(ctx context.Context, req *http.Request) (*http.Response, error)` and `Close() error`
- `internal/transport/ssh.go` — `SSHTransport` struct: parses ssh:// URL, connects via SSH agent, opens local port forward, proxies HTTP requests through the tunnel
- `internal/transport/ssh_agent.go` — SSH agent integration: connect to `SSH_AUTH_SOCK`, obtain signers
- `internal/transport/known_hosts.go` — Known hosts callback using `golang.org/x/crypto/ssh/knownhosts`
- `internal/transport/errors.go` — Typed errors: `ErrSSHAuth`, `ErrSSHTunnel`, `ErrSSHHostKey`, `ErrSSHUnreachable`

### 5.2 Files to Modify

- `go.mod` — Add `golang.org/x/crypto` dependency
- `cmd/htmlctl/root.go` — Wire transport creation into command execution flow (create transport from resolved context, pass to command handlers)

### 5.3 Tests to Add

- `internal/transport/ssh_test.go` — Unit tests for SSH URL parsing (user, host, port extraction), default port handling
- `internal/transport/transport_test.go` — Test `Transport` interface contract with a mock implementation
- `internal/transport/ssh_agent_test.go` — Test SSH agent connection (skip if no agent available in CI)
- `internal/transport/integration_test.go` — Integration test using an in-process SSH server (testable with `golang.org/x/crypto/ssh` server-side) to verify tunnel setup and HTTP request forwarding

### 5.4 Dependencies/Config

- `go.mod` — `golang.org/x/crypto` for SSH client, agent, and known hosts
- Runtime dependency: `SSH_AUTH_SOCK` environment variable must be set (SSH agent running)
- Runtime dependency: `~/.ssh/known_hosts` must contain the server's host key

## 6. Acceptance Criteria

- [ ] AC-1: `ssh://user@host` URLs from context config are correctly parsed into user, host, and default port (22)
- [ ] AC-2: `ssh://user@host:2222` URLs correctly parse custom port
- [ ] AC-3: SSH connection is established using keys from the system SSH agent (`SSH_AUTH_SOCK`)
- [ ] AC-4: A local port forward is opened to `127.0.0.1:<htmlservd_port>` on the remote server
- [ ] AC-5: HTTP requests made through the transport reach htmlservd on the remote server's localhost
- [ ] AC-6: The tunnel is opened at command start and closed after the command completes (per-command lifecycle)
- [ ] AC-7: Known hosts verification is performed using `~/.ssh/known_hosts`; unknown hosts produce a clear error
- [ ] AC-8: When SSH agent is not available (`SSH_AUTH_SOCK` unset), a clear error message is returned
- [ ] AC-9: When the remote host is unreachable, a timeout error is returned within 10 seconds
- [ ] AC-10: When SSH authentication fails, a clear error distinguishes auth failure from connectivity failure
- [ ] AC-11: The `Transport` interface is generic enough that a mock/local implementation can be used for testing commands without SSH

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for SSH URL parsing (valid URLs, missing port, invalid scheme, missing user)
- [ ] Unit tests for error type classification (auth failure, tunnel failure, host key mismatch)
- [ ] Integration test with in-process SSH server: connect, open tunnel, forward HTTP request, verify response
- [ ] Test Transport interface with mock implementation to verify command layer integration

### Manual Tests

- [ ] Set up htmlservd on a remote server (or local VM), configure a context, run `htmlctl status --context <ctx>` and verify it connects via SSH tunnel
- [ ] Test with SSH agent containing multiple keys
- [ ] Test with wrong host key in known_hosts (should reject)
- [ ] Test with unreachable host (should timeout with clear message)
- [ ] Test with `SSH_AUTH_SOCK` unset (should produce helpful error)

## 8. Performance / Reliability Considerations

- SSH connection establishment typically takes 200-500ms depending on network latency. This is acceptable for CLI commands.
- Each command opens a fresh tunnel. This avoids stale connection issues but adds connection overhead per command. For v1 this is acceptable; connection pooling is a post-v1 optimization.
- The local forwarded port is bound to `127.0.0.1` on the client side to avoid exposing it to the local network.
- A dynamic (ephemeral) local port is used to avoid port conflicts.
- Connection timeout should be configurable but defaults to 10 seconds.

## 9. Risks & Mitigations

- **Risk:** SSH agent not available in some deployment environments (containers, CI). **Mitigation:** Provide clear error message; document SSH agent setup. Post-v1: add `--ssh-key` flag for explicit key file path.
- **Risk:** Known hosts file missing or incomplete causes connection failures. **Mitigation:** Provide actionable error message with `ssh-keyscan` command suggestion. Consider a `--insecure-skip-host-key` flag (with warning) for initial setup.
- **Risk:** Port forwarding fails if htmlservd is not running on expected port. **Mitigation:** Return clear error; document expected server configuration. Allow overriding the remote port in context config.
- **Risk:** Go's `x/crypto/ssh` library may have different behavior than OpenSSH for edge cases (key types, ciphers). **Mitigation:** Test with common key types (ed25519, rsa); document supported algorithms.

## 10. Open Questions

- Should the remote htmlservd port be configurable per context, or always use a fixed default (e.g., 8420)? **Tentative answer:** Default to 8420, allow override via `port` field in context config.
- Should we support `~/.ssh/config` parsing for host aliases and proxy jumps in v1? **Tentative answer:** No, keep it simple. Users can use the full hostname in the ssh:// URL.
- Should we implement a `--insecure-skip-host-key` flag for convenience? **Tentative answer:** Yes, with a visible warning, to ease initial setup and testing.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
