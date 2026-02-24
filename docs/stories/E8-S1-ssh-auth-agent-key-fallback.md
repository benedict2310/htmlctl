# E8-S1 - SSH Auth: Fall Back to Key File When Agent Key Is Rejected

**Epic:** Epic 8 — DX & Reliability
**Status:** Not Started
**Priority:** P1 (High — silent failure affects real users)
**Estimated Effort:** 0.5 days
**Dependencies:** E3-S2 (SSH tunnel transport)
**Target:** htmlctl CLI
**Design Reference:** `internal/transport/ssh.go`, `internal/transport/ssh_agent.go`

---

## 1. Objective

When the SSH agent is reachable but holds a key that is rejected by the server, `htmlctl` currently fails with a cryptic auth error and no recovery path. The key file at `HTMLCTL_SSH_KEY_PATH` (or `~/.ssh/id_ed25519`) is never tried. Users with multiple keys in their agent — a normal state on developer machines — hit this silently.

Fix: when both an SSH agent and a key file are available, pass both as auth methods. The SSH library tries them in order; if the agent key is rejected, the key file is tried automatically.

## 2. User Story

As an `htmlctl` user with multiple SSH keys loaded in my agent, I want auth to automatically fall back to my configured key file when the agent's key is rejected, so I don't need to manually clear `SSH_AUTH_SOCK` to run commands.

## 3. Scope

### In Scope

- Modify `NewSSHTransport` in `internal/transport/ssh.go` to append the key-file auth method when a private key path is resolvable, regardless of whether the agent was successfully contacted.
- Unit tests covering the multi-method auth path.
- Integration test: agent available with wrong key + key file with correct key → auth succeeds.

### Out of Scope

- Changing the agent availability check logic (`authMethodFromSSHAgent`).
- Prompting for key passphrase interactively.
- Any changes to `ssh_agent.go`.

## 4. Architecture Context

- All changes are confined to `internal/transport/ssh.go:NewSSHTransport`.
- The existing `resolvePrivateKeyPath` and `authMethodFromPrivateKey` helpers are already available.
- No new dependencies; the `golang.org/x/crypto/ssh` library already tries `[]ssh.AuthMethod` in order during the handshake — no custom retry loop needed.
- Security invariants unchanged: host-key verification is unaffected; key file is only loaded if the path resolves without error (same guard as the current fallback path).

## 5. Implementation Plan

### 5.1 Files to Modify

- `internal/transport/ssh.go` — extend the `len(authMethods) == 0` branch so that when the agent is successfully contacted, the key file (if resolvable) is appended as a second auth method:

  ```go
  // after: authMethods = []ssh.AuthMethod{agentMethod}
  if keyPath, err := resolvePrivateKeyPath(cfg.PrivateKeyPath); err == nil {
      if keyMethod, err := authMethodFromPrivateKey(keyPath); err == nil {
          authMethods = append(authMethods, keyMethod)
      }
  }
  ```

### 5.2 Tests to Add

- `internal/transport/ssh_test.go` — table-driven unit test for the auth method assembly logic (mock agent available, key path resolvable → two methods in slice; agent available, no key path → one method; agent unavailable, key path resolvable → one method; agent unavailable, no key path → error).

## 6. Acceptance Criteria

- [ ] AC-1: When `SSH_AUTH_SOCK` is set and the agent contains a key not in the server's `authorized_keys`, but `HTMLCTL_SSH_KEY_PATH` points to the correct key, `htmlctl apply` succeeds without any manual env-var workaround.
- [ ] AC-2: When `SSH_AUTH_SOCK` is unset and `HTMLCTL_SSH_KEY_PATH` is set, behaviour is unchanged (key-file-only auth).
- [ ] AC-3: When `SSH_AUTH_SOCK` is set and `HTMLCTL_SSH_KEY_PATH` is unset/unresolvable, behaviour is unchanged (agent-only auth).
- [ ] AC-4: The resulting `authMethods` slice is agent-first, key-file-second (agent key takes precedence when it works).
- [ ] AC-5: Unit tests pass under `go test -race ./internal/transport/...`.

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: `TestNewSSHTransportAuthMethodAssembly` covering all four cases in AC-1–4.
- [ ] Existing transport tests continue to pass with race detector.

### Manual Tests

- [ ] Reproduce the bug locally: start Docker container, load a different ed25519 key into the agent, set `HTMLCTL_SSH_KEY_PATH` to the correct key, run `htmlctl apply` — verify it succeeds.
- [ ] Verify the workaround (`SSH_AUTH_SOCK=""`) still works as before.

## 8. Performance / Reliability Considerations

- No performance impact: key file is loaded once per command invocation, same as the existing fallback path.
- Failure to load the key file is silently ignored (only the agent method is used) — no regression in environments where `HTMLCTL_SSH_KEY_PATH` is not set.

## 9. Risks & Mitigations

- **Risk:** Appending the key file method could cause unexpected auth with the wrong identity in unusual setups. **Mitigation:** agent method is first in the slice; the correct server key wins. Operators who want strict agent-only auth can unset `HTMLCTL_SSH_KEY_PATH`.

## 10. Open Questions

- None.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
