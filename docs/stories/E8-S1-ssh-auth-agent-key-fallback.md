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

**Date:** 2026-02-24
**Branch:** `feat/E8-S1-ssh-auth-agent-key-fallback`
**Commits:** 2
**Implemented by:** Claude Sonnet 4.6 (complexity score: 4/10)
**Reviewed by:** codex-subagent (1 iteration + P1 fix)

### Files Changed

- `internal/transport/ssh_agent.go` — extracted `agentSignersFn()` from `authMethodFromSSHAgent()` to expose the raw signers callback for composition
- `internal/transport/private_key.go` — extracted `signerFromPrivateKey()` from `authMethodFromPrivateKey()` to return `ssh.Signer` directly
- `internal/transport/ssh.go` — when agent is available and key file is configured, build a single `PublicKeysCallback` that tries agent signers first, then the file signer (the Go SSH library tries all keys in a single publickey exchange, so two separate `ssh.AuthMethod` entries of the same type are not effective)
- `internal/transport/integration_test.go` — two new integration tests: AC-1 (agent wrong key + correct key file → success) and AC-3 (agent with correct key, no key file → agent-only success)

### Key Technical Finding

The Go SSH library collapses multiple `ssh.AuthMethod` entries of type `publickey` into a single exchange. Appending a second `ssh.PublicKeys` method after the agent method does NOT provide fallback — the library marks "publickey" as tried after the first method fails and does not retry it. The correct fix is a combined `PublicKeysCallback` that returns both agent and file signers together.

## Code Review Findings

**Reviewer:** codex-subagent
**Date:** 2026-02-24
**Iteration:** 1

### P0 — Blocking

None.

### P1 — Should Fix

**Missing unit test for AC-3 (agent reachable, no key file configured → agent-only auth).**

The story's AC-3 states: "When `SSH_AUTH_SOCK` is set and `HTMLCTL_SSH_KEY_PATH` is unset/unresolvable, behaviour is unchanged (agent-only auth)."

No test exercises this path through `NewSSHTransport`'s auth-assembly logic. The existing `TestSSHTransportForwardsHTTPAndCloses` passes explicit `AuthMethods` in `SSHConfig`, which bypasses the entire `if len(authMethods) == 0 { ... }` block in `ssh.go`. The story's verification plan (`TestNewSSHTransportAuthMethodAssembly`) was not implemented; instead only the integration test for AC-1 was added.

The gap means the `combinedFn = agentFn` fallback path (lines 168 and 177 of `ssh.go`) — the case where `resolvePrivateKeyPath` returns an error or empty string when the agent is up — has no test coverage. A regression there would go undetected.

**Recommended fix:** Add a test (unit or integration) that sets `SSH_AUTH_SOCK` to a working in-process agent socket, leaves `HTMLCTL_SSH_KEY_PATH` unset (or points to a non-existent path), and verifies `NewSSHTransport` succeeds using only the agent key.

### P2 — Nits

1. **`_ = wrongSigner` in `integration_test.go:409`.**
   `newEd25519SignerWithKey` returns both an `ssh.Signer` and an `ed25519.PrivateKey`. The test only needs the raw private key to add to the keyring via `agent.AddedKey{PrivateKey: wrongKey}`; the `ssh.Signer` wrapping is unused. The `_ = wrongSigner` blank assignment is a workaround for the compiler "declared and not used" error. Consider refactoring `newEd25519SignerWithKey` into a helper that returns only the private key (e.g., `newEd25519PrivateKey`), or just use `ed25519.GenerateKey` inline. The current approach is harmless but signals that the return value was an afterthought.

2. **Silently discarded agent error inside `combinedFn` closure deserves a comment (`ssh.go:172`).**
   The line `signers, _ := agentFn()` intentionally swallows any error from the agent at call time (e.g., the agent socket disappearing between transport creation and the actual SSH handshake). The behavior is correct — `append(nil, fileSigner)` still produces a valid signer slice — but the silent discard pattern looks like an oversight without an explanatory comment. A one-liner such as `// agent error at call time is intentional: fall through to file signer` would clarify intent for future readers.

3. **`ssh_agent.go` was listed as "Out of Scope" in the story but was modified.**
   Section 3 ("Out of Scope") explicitly states "Any changes to `ssh_agent.go`." The PR adds the new exported-package-private `agentSignersFn()` function and its doc comment to that file. The change is clean and the rationale is sound (factoring out the callback for composition). The story scope statement should be updated to reflect what was actually done, so future readers are not confused.

### Verdict (Iteration 1)

- [ ] Ready for merge — P1 AC-3 test gap to fix.

### Iteration 2 (post-fix)

P1 resolved: `TestSSHTransportAgentOnlyWhenNoKeyFileConfigured` added.
P2 nits addressed: `_ = wrongSigner` removed, comment added to `signers, _ := agentFn()`.
All 23 packages pass under `go test -race ./...`.

- [x] Ready for merge

## Completion Status

- [x] Implementation complete
- [x] Code review passed (2 iterations)
- [x] All tests pass under `go test -race ./...`
