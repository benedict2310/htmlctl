# E6-S5 - Container & Entrypoint Security Hardening

**Epic:** Epic 6 — Security Hardening
**Status:** Pending
**Priority:** P0 (Critical — command injection in production entrypoint)
**Estimated Effort:** 1 day
**Dependencies:** None (standalone shell/Dockerfile changes)
**Target:** Docker image (htmlservd + htmlservd-ssh variants)
**Design Reference:** Security Audit 2026-02-20, Vulns 4, 6, 15 & 16

---

## 1. Objective

Four security issues exist in the Docker entrypoint scripts and Dockerfile:

1. **Command injection (HIGH):** `htmlservd-ssh-entrypoint.sh` passes `${CADDYFILE_PATH}` unquoted inside a `su -c "..."` string, allowing shell metacharacters in the env var to inject arbitrary commands.
2. **Caddyfile config injection (HIGH):** Both entrypoints write a bootstrap Caddyfile using a heredoc that interpolates `CADDY_BOOTSTRAP_LISTEN` and `PREVIEW_ROOT` verbatim — newlines or Caddyfile metacharacters in these vars inject arbitrary directives, enabling SSRF or traffic hijacking.
3. **`authorized_keys` injection (MEDIUM):** `SSH_PUBLIC_KEY` is written to `authorized_keys` without stripping option prefixes; an attacker can embed `command="..."` to execute a forced command on every SSH login.
4. **`PermitTunnel yes` (MEDIUM):** TUN/TAP tunnel device creation is enabled, allowing an authenticated SSH user to establish a layer-3 VPN through the container and pivot to internal networks.

This story fixes all four.

## 2. User Story

As an infrastructure operator, I want the htmlservd container entrypoints to be immune to environment variable injection and to apply least-privilege SSH configuration, so that the container cannot be used as a pivot point even if a deployment environment variable is compromised.

## 3. Scope

### In Scope

- Fix the `su -c` command injection by switching to an exec-array pattern (same as the non-SSH entrypoint) or by using `su --` with arguments instead of a string.
- Add input validation for `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` and `HTMLSERVD_PREVIEW_ROOT` (and derived vars) before the heredoc write: reject values containing `\n`, `{`, or `}` (Caddyfile metacharacters).
- Validate `SSH_PUBLIC_KEY` format before writing to `authorized_keys`: parse it with `ssh-keygen -l -f -` (or equivalent inline `awk` check) to confirm it is a bare public key without option prefixes; prepend `restrict,` to the written entry to disable all SSH options by default, then re-enable only what is required (`port-forwarding`).
- Change `PermitTunnel yes` to `PermitTunnel no` in the Dockerfile sshd config block.
- Document the security constraints for each env var in a comment block at the top of each entrypoint script.
- Smoke test the container build and SSH login after changes.

### Out of Scope

- Replacing the shared-secret SSH approach with certificate-based auth (future hardening).
- Hardening the sshd configuration beyond the specific issues identified (cipher suites, MACs, etc.).
- Changes to the non-SSH entrypoint beyond the Caddyfile heredoc injection fix (it is already safe for the `su -c` issue but still vulnerable to heredoc injection).

## 4. Architecture Alignment

- **`su -c` fix:** The non-SSH entrypoint already uses `exec /usr/local/bin/caddy run --config "${CADDYFILE_PATH}" --adapter caddyfile` (exec-array style), which is the correct pattern. The SSH entrypoint should be updated to match using `su -s /bin/sh -c 'exec "$@"' -- htmlservd /usr/local/bin/caddy run --config "$CADDYFILE_PATH" --adapter caddyfile` — passing the command via `--` argument list prevents shell interpolation of `$CADDYFILE_PATH` at the `su -c` level.
- **Heredoc injection fix:** Validate env vars with a shell check before the heredoc (e.g., `case "$VAR" in *$'\n'*|*'{'*|*'}'*) echo "ERROR: ..."; exit 1 ;; esac`). For the listen address, also validate it matches a known pattern (`:PORT` or `HOST:PORT`) using a simple regex.
- **`authorized_keys` hardening:** Prepend `restrict,port-forwarding ` to every written key entry. This disables all default permissions (pty, agent forwarding, X11, TCP forwarding beyond what's explicitly re-enabled) and re-enables only port forwarding. If forced-command detection is desired, parse with `ssh-keygen -l -f /dev/stdin` and fail if the key line contains spaces before the key type prefix (naive option prefix detection).
- **`PermitTunnel no`:** Single-character change in the Dockerfile sshd config; no entrypoint script changes needed.
- **PRD references:** PRD Section 9 (deployment infrastructure), Technical Spec Section 10 (container hardening).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- None — all changes are in existing files.

### 5.2 Files to Modify

- `docker/htmlservd-ssh-entrypoint.sh`
  - **Fix command injection (line 60):** Replace `su -m -s /bin/sh -c "/usr/local/bin/caddy run --config ${CADDYFILE_PATH} --adapter caddyfile" htmlservd &` with a safe equivalent that passes `$CADDYFILE_PATH` as an argument rather than interpolated into the shell command string.
  - **Fix heredoc injection (lines 35–49):** Add validation of `CADDY_BOOTSTRAP_LISTEN` and `PREVIEW_ROOT` before the heredoc write. Reject any value containing newlines, `{`, or `}`.
  - **Fix `authorized_keys` injection (lines 4–11):** Prepend `restrict,port-forwarding ` to the written key entry. Optionally add a basic sanity check that the value starts with a known key-type prefix (`ssh-ed25519`, `ssh-rsa`, `ecdsa-sha2-nistp256`, etc.).
- `docker/htmlservd-entrypoint.sh`
  - **Fix heredoc injection (lines 15–29):** Add the same env var validation for `CADDY_BOOTSTRAP_LISTEN` and `PREVIEW_ROOT` before the heredoc write.
- `Dockerfile`
  - **Fix `PermitTunnel` (line 48):** Change `'PermitTunnel yes'` to `'PermitTunnel no'`.
  - Add an inline comment explaining why tunnel forwarding is disabled.

### 5.3 Tests to Add

- `docker/htmlservd-ssh-entrypoint.sh` — add a self-test section (or separate test script) that:
  - Calls the validation functions with injected values and confirms they exit non-zero.
  - Calls with valid values and confirms they succeed.
- `.github/workflows/docker-e2e.yml` — the existing E2E test validates the SSH connection; confirm it still passes after the `authorized_keys` format change.

### 5.4 Dependencies / Config

- No new dependencies.
- `restrict,port-forwarding` option prefix requires OpenSSH 7.2+ (available in all modern Linux distributions used as base images).

## 6. Acceptance Criteria

- [ ] AC-1: `htmlservd-ssh-entrypoint.sh` passes `$CADDYFILE_PATH` as a shell argument (not interpolated in a `su -c "..."` string); a value like `/etc/caddy/Caddyfile; id` does not execute the `id` command.
- [ ] AC-2: Both entrypoints validate `CADDY_BOOTSTRAP_LISTEN` and `PREVIEW_ROOT` before writing the Caddyfile; values containing `\n`, `{`, or `}` cause the script to exit with a non-zero status and a descriptive error message.
- [ ] AC-3: The `authorized_keys` entry written from `SSH_PUBLIC_KEY` is prefixed with `restrict,port-forwarding `, disabling all other SSH options by default.
- [ ] AC-4: `PermitTunnel no` is set in the Dockerfile sshd configuration.
- [ ] AC-5: The Docker E2E test passes end-to-end (SSH login, apply, verify) after all changes.
- [ ] AC-6: A container started with a valid `SSH_PUBLIC_KEY` and env vars allows `htmlctl apply` to succeed normally.
- [ ] AC-7: A container started with `HTMLSERVD_PREVIEW_ROOT` containing a newline fails to start with a clear error log message.

## 7. Verification Plan

### Automated Tests

- [ ] Docker E2E workflow (`.github/workflows/docker-e2e.yml`) passes with all changes applied.
- [ ] Shell validation unit tests: injected values are rejected; valid values are accepted.

### Manual Tests

- [ ] Build the image; start container with `HTMLSERVD_CADDYFILE_PATH='/etc/caddy/Caddyfile; id'`; confirm `id` output does not appear in container logs.
- [ ] Start container with `HTMLSERVD_PREVIEW_ROOT='/srv }reverse_proxy http://evil'`; confirm container exits with error before writing the Caddyfile.
- [ ] Attempt `ssh -w 0:0 htmlservd@host` after `PermitTunnel no`; confirm connection is rejected with `Tunnel forwarding disabled`.
- [ ] Provide a `SSH_PUBLIC_KEY` prefixed with `command="id",`; confirm the `authorized_keys` entry contains `restrict,port-forwarding` and does not contain the original `command=` option.

## 8. Performance / Reliability Considerations

- Shell-level input validation (string matching) adds sub-millisecond overhead to container startup; negligible.
- The `restrict,port-forwarding` prefix does not change the authentication flow; it only restricts what an authenticated session can do.

## 9. Risks & Mitigations

- **Risk:** Legitimate deployments rely on `PermitTunnel yes` for operator access patterns not anticipated here. **Mitigation:** Document the change in the release notes; operators who genuinely need tunnel forwarding can build a custom image variant.
- **Risk:** The `restrict,port-forwarding` prefix breaks existing SSH sessions that relied on pty or agent forwarding through the container. **Mitigation:** The `htmlctl` use case only requires port forwarding (for the API tunnel); pty and agent forwarding are not needed. Operators using the container for interactive terminal access should use `--bind-host` or a separate access method.
- **Risk:** The `su -c` fix using argument-passing style may not work with all `su` implementations. **Mitigation:** Test against the base image's `su` (from `shadow-utils` or `util-linux`); fall back to `runuser -u htmlservd --` if needed.

## 10. Open Questions

- Should the entrypoints fail-fast on any invalid env var, or should they warn and substitute a safe default? Leaning toward fail-fast for security-critical vars like `PREVIEW_ROOT` and `CADDYFILE_PATH`.
- Should a minimal set of allowed key types be enforced for `SSH_PUBLIC_KEY` (e.g., reject RSA < 3072 bits)? Deferred to a future key-policy story.
