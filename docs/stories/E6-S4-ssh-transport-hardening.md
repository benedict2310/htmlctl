# E6-S4 - SSH Transport Hardening

**Epic:** Epic 6 — Security Hardening
**Status:** Done
**Priority:** P1 (High — MITM and credential exposure vectors)
**Estimated Effort:** 1 day
**Dependencies:** E3-S2 (SSH tunnel transport)
**Target:** htmlctl (SSH transport layer)
**Design Reference:** Security Audit 2026-02-20, Vulns 5, 13 & 14

---

## 1. Objective

Three weaknesses exist in the SSH transport layer:

1. `SSHConfig` exposes a public `HostKeyCB ssh.HostKeyCallback` field. If any caller passes `ssh.InsecureIgnoreHostKey()`, host key verification is silently skipped for that transport — the safe `known_hosts` path is only the fallback when the field is `nil`. Test files already normalise `InsecureIgnoreHostKey()` as a standard pattern, one step away from production misuse.
2. The SSH agent socket path is taken verbatim from `SSH_AUTH_SOCK` with no ownership or type checks. An attacker who controls that environment variable can redirect all signing requests to a fake agent.
3. The private key file path is taken verbatim from `cfg.PrivateKeyPath` or `HTMLCTL_SSH_KEY_PATH` with no sanitization, allowing an attacker with config-file or env-var control to redirect key loading to arbitrary filesystem paths.

This story hardens all three vectors.

## 2. User Story

As an operator, I want the htmlctl SSH transport to always verify the remote host's key against `known_hosts` and to reject agent socket or key file paths that are not owned by the current user, so that a compromised environment cannot silently downgrade the security of my deployments.

## 3. Scope

### In Scope

- Remove the public `HostKeyCB` field from `SSHConfig`. Tests that currently pass `ssh.InsecureIgnoreHostKey()` must be updated to use a test-only mechanism (e.g., a `WithTestHostKeyCallback` option that is only available in test builds via a build tag, or by injecting a known test key into a temporary `known_hosts` file).
- Add ownership and socket-type validation to `ssh_agent.go` before `net.Dial`: verify the socket path is owned by the current effective UID (`os.Lstat` + `syscall.Stat_t.Uid`) and is a Unix socket (`os.ModeSocket`).
- Canonicalize the private key path with `filepath.Clean` and restrict it to within the current user's home directory (reject paths that resolve outside `os.UserHomeDir()` or that traverse with `..`).
- Ensure error messages for key-file failures do not include the full resolved path in messages returned to the user (log the path internally; return a generic error message to the caller).
- All existing transport tests continue to pass; test infrastructure uses a safe alternative to `InsecureIgnoreHostKey`.

### Out of Scope

- mTLS or certificate pinning (future hardening).
- Rotating or caching SSH agent connections.
- Private key decryption / passphrase support changes.
- Windows path handling (Linux/macOS only for v1).

## 4. Architecture Alignment

- **HostKeyCB removal:** The correct production path (`knownhosts.New`) is already implemented and tested. The public field exists as a convenience escape hatch for tests; removing it forces tests to be explicit about their trust model.
- **Test-only callback:** A `testing`-gated helper (e.g., `transport.NewSSHTransportForTest(cfg, cb)`) or a temporary `known_hosts` file approach allows tests to control host key verification without exposing the escape hatch in production code.
- **Agent socket validation:** `os.Lstat` (not `os.Stat`) is used to inspect the socket itself rather than following symlinks, which prevents TOCTOU via symlink substitution. Ownership check uses `syscall.Stat_t`; cross-platform build tags may be needed for non-Unix targets.
- **Key path restriction:** `filepath.Clean` followed by `strings.HasPrefix(cleanPath, homeDir)` provides a simple, portable home-directory guard. Absolute paths outside home are rejected; relative paths are resolved against the caller's working directory then re-evaluated.
- **PRD references:** Technical Spec Section 7 (SSH transport security model).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/transport/agent_unix.go` — platform-specific socket ownership + type check using `syscall.Stat_t`; build tag `//go:build !windows`.
- `internal/transport/agent_other.go` — stub for non-Unix platforms that skips the ownership check with a log warning.

### 5.2 Files to Modify

- `internal/transport/ssh.go`
  - Remove `HostKeyCB ssh.HostKeyCallback` field from `SSHConfig`.
  - Add an internal-only `testHostKeyCB` field (unexported) accessible via a package-level `WithTestHostKeyCallback` function that is only compiled in `_test.go` files via a test helper, or inject a test known_hosts file.
  - Update `NewSSHTransport` to always derive the callback from `known_hosts`.
- `internal/transport/ssh_agent.go`
  - After resolving `sockPath`, call the platform-specific `validateAgentSocket(sockPath)` function before `net.Dial`.
  - Return a descriptive error if the socket is not owned by the current user or is not a socket type.
- `internal/transport/private_key.go`
  - Apply `filepath.Clean` to both the explicit path and the env-var path.
  - After cleaning, verify the path has a `strings.HasPrefix` match against `os.UserHomeDir()`; if not, return an error (do not attempt to read).
  - Change error messages from `fmt.Errorf("read private key %s: %w", keyPath, err)` to `fmt.Errorf("read private key: %w", err)` (omit path from user-visible message; the path is already captured in the wrapped error for log inspection).
- `internal/transport/ssh_test.go`, `context_test.go`, `integration_test.go`
  - Replace `HostKeyCB: ssh.InsecureIgnoreHostKey()` with a test helper that either injects a temporary `known_hosts` file containing the test server's host key, or uses a package-private test accessor.

### 5.3 Tests to Add

- `internal/transport/agent_unix_test.go`
  - Valid socket owned by current user: passes validation.
  - Socket owned by different UID: returns error.
  - Path pointing to a regular file (not a socket): returns error.
  - Missing path: returns error.
- `internal/transport/private_key_test.go`
  - Path within home directory: accepted.
  - Path with `..` components that escape home: rejected.
  - Absolute path outside home (`/etc/passwd`): rejected.
  - `filepath.Clean` is applied before the home-dir check.
  - Error messages do not contain the resolved key path.

### 5.4 Dependencies / Config

- No new Go dependencies; uses `syscall`, `os`, `path/filepath` from stdlib.
- Build tag `//go:build !windows` for Unix-specific socket validation.

## 6. Acceptance Criteria

- [x] AC-1: `SSHConfig` no longer has a public `HostKeyCB` field; all production callers derive the host key callback from `known_hosts`.
- [x] AC-2: `NewSSHTransport` always calls `knownHostsCallback`; there is no code path in non-test builds that can supply an arbitrary `ssh.HostKeyCallback`.
- [x] AC-3: All transport tests pass using a test mechanism that does not expose `InsecureIgnoreHostKey` in the production API surface.
- [x] AC-4: `ssh_agent.go` verifies the socket path is owned by the current effective UID before dialing; mis-owned paths return a clear error.
- [x] AC-5: `ssh_agent.go` verifies the path resolves to a Unix socket (`os.ModeSocket`); regular files and directories return a clear error.
- [x] AC-6: `private_key.go` applies `filepath.Clean` to all key paths and rejects any path that does not resolve inside `os.UserHomeDir()`.
- [x] AC-7: Error messages for key-load failures do not include the resolved key file path in the string returned to the user.

## 7. Verification Plan

### Automated Tests

- [x] Agent socket validation tests: UID mismatch and non-socket type return errors.
- [x] Private key path tests: paths outside home directory are rejected.
- [x] All existing transport integration tests pass with the updated test infrastructure (no `InsecureIgnoreHostKey` in production API).

### Manual Tests

- [ ] Point `SSH_AUTH_SOCK` at a regular file; confirm `htmlctl` reports a clear socket-type error rather than a confusing `dial` error.
- [ ] Set `HTMLCTL_SSH_KEY_PATH=/etc/passwd`; confirm `htmlctl` reports a path-restriction error without leaking the attempted path in user-visible output.
- [ ] Normal operation: `htmlctl apply` with a valid SSH key and correct `known_hosts` succeeds.

## 8. Performance / Reliability Considerations

- `os.Lstat` adds one syscall per SSH agent connection attempt, which is negligible compared to the TCP dial and SSH handshake.
- `filepath.Clean` and `strings.HasPrefix` are in-memory string operations with no I/O cost.

## 9. Risks & Mitigations

- **Risk:** Removing the public `HostKeyCB` field breaks any external code that imported the transport package and used the field. **Mitigation:** The field is in `internal/` so it cannot be imported by external packages; the change is safe within the module.
- **Risk:** Agent socket ownership check fails in Docker or CI environments where the socket is bind-mounted with a different UID. **Mitigation:** The check is skipped (or reduced to a warning) if `os.Getuid()` returns 0 (running as root); document this exception. Alternatively, make the check opt-in via a config flag.
- **Risk:** Stripping key paths from error messages makes debugging harder. **Mitigation:** The full path is available in structured log output at Debug level; only the user-facing error string is sanitized.

## 10. Open Questions

- Should the agent socket ownership check be a hard error or a warning with a `--insecure-agent` override? Leaning toward hard error for security, with clear documentation on how to work around it in unusual environments.
- Should the home-directory restriction be relaxed to allow any user-owned file (not just files under `$HOME`)? Some operators store SSH keys in `/etc/htmlctl/` or similar. Consider making the restriction configurable via an allowlist of key prefixes.

---

## 11. Implementation Summary

- Removed `HostKeyCB` from `internal/transport/ssh.go` `SSHConfig` and made `NewSSHTransport` always construct host verification from `known_hosts` via `knownHostsCallback`.
- Updated tunnel tests to use temporary `known_hosts` fixtures instead of `ssh.InsecureIgnoreHostKey`:
  - `internal/transport/ssh_test.go`
  - `internal/transport/context_test.go`
- Added SSH agent socket hardening:
  - `internal/transport/ssh_agent.go` now validates `SSH_AUTH_SOCK` before dialing.
  - Added Unix validator in `internal/transport/agent_socket_unix.go` (`os.Lstat`, socket-type check, effective-UID ownership check).
  - Added Windows stub in `internal/transport/agent_socket_windows.go`.
  - Added validator tests in `internal/transport/agent_socket_unix_test.go`.
- Hardened private key path handling in `internal/transport/private_key.go`:
  - `resolvePrivateKeyPath` now applies `filepath.Clean`, normalizes to absolute paths, and rejects paths outside `os.UserHomeDir()`.
  - `authMethodFromPrivateKey` now returns sanitized read/parse errors without resolved key paths.
- Expanded private key and integration coverage:
  - `internal/transport/private_key_test.go`
  - `internal/transport/integration_test.go` (private-key fallback uses a temp `$HOME/.ssh/id_rsa`)
- Updated architecture docs:
  - `docs/technical-spec.md`
  - `docs/epics.md`

## 12. Completion Status

- Implemented and verified.
