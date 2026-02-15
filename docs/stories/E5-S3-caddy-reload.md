# E5-S3 - Reload Caddy Safely

**Epic:** Epic 5 — Domains + TLS via Caddy
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E5-S2 (Caddy config generation)
**Target:** Linux server (htmlservd)
**Design Reference:** PRD Section 8 (Domains + TLS), Technical Spec Section 8

---

## 1. Objective

Provide a safe, atomic mechanism for htmlservd to apply generated Caddy configuration changes without downtime or risk of leaving Caddy in a broken state. This is the operational bridge between config generation (E5-S2) and user-facing domain commands (E5-S4): the reload layer ensures that config changes are validated before applying and that failures are handled gracefully with rollback.

## 2. User Story

As an operator, I want htmlservd to validate and reload Caddy configuration safely so that domain binding changes take effect without downtime, and if the new config is invalid, Caddy continues serving with the previous working configuration.

## 3. Scope

### In Scope

- `caddy.Reloader` component that orchestrates: generate config -> validate -> write -> reload
- Config validation using `caddy validate --config <path>` command before applying
- Config reload using `caddy reload --config <path>` command (or Caddy admin API at `localhost:2019`)
- Atomic config update sequence:
  1. Generate new Caddyfile to a temporary path
  2. Validate the temporary Caddyfile via `caddy validate`
  3. Back up the current Caddyfile
  4. Atomically replace the Caddyfile (rename)
  5. Trigger `caddy reload`
  6. On reload failure: restore backup, report error
- Error handling and structured error reporting for each failure mode
- Configurable Caddy binary path and admin API endpoint
- Reload locking (prevent concurrent reloads)
- Unit tests with mock command executor (no real Caddy required)
- Integration test helpers for environments with Caddy installed

### Out of Scope

- Caddy installation or setup
- Caddy process management (start/stop/restart) — assumes Caddy is already running (e.g., via systemd)
- Monitoring Caddy health after reload (beyond checking the reload command exit code)
- TLS certificate status checking (E5-S4 handles verify)
- CLI commands (E5-S4)
- Caddy JSON API config format (Caddyfile only for v1)

## 4. Architecture Alignment

- **Component boundaries:** The reloader lives in the `internal/caddy` package alongside the config generator (E5-S2). It depends on the config generator for producing the Caddyfile and on `os/exec` for running Caddy commands.
- **Concurrency:** A mutex protects the reload sequence to prevent concurrent config writes and reloads. Only one reload can be in progress at a time.
- **Command execution:** Uses an interface (`CommandRunner`) for executing `caddy validate` and `caddy reload` so that tests can use a mock implementation without requiring Caddy to be installed.
- **Error recovery:** On validation failure, the new config is discarded and the error is returned. On reload failure after the config file has been replaced, the backup is restored and the error is returned with context.
- **Audit logging:** Reload attempts (success and failure) should be logged. Successful reloads record the trigger (which domain binding change caused it). Failed reloads record the error.
- **PRD references:** PRD Section 8 ("htmlservd writes Caddy snippets and triggers reload safely"), Technical Spec Section 8.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/caddy/reloader.go` — Reloader struct with Reload() method orchestrating the validate-write-reload sequence
- `internal/caddy/reloader_test.go` — Unit tests with mock command runner
- `internal/caddy/command.go` — CommandRunner interface and real implementation wrapping os/exec

### 5.2 Files to Modify

- `internal/caddy/config.go` — Add method to generate config to a specific temporary path (if not already supported)
- `internal/config/config.go` — Add Caddy reload-related config fields (`CaddyBinaryPath`, `CaddyAdminAPI`)
- `cmd/htmlservd/main.go` — Initialize the reloader and wire it into the domain binding API flow
- `internal/api/domain_handler.go` — After domain binding create/delete, trigger config regeneration and reload

### 5.3 Tests to Add

- `internal/caddy/reloader_test.go`
  - Successful reload: validate passes, config written, reload succeeds
  - Validation failure: invalid config detected, old config preserved, error returned
  - Reload failure: config written but reload fails, backup restored, error returned
  - Concurrent reload attempts: second attempt blocked until first completes (mutex)
  - Missing Caddy binary: graceful error with actionable message
  - Backup creation and restoration verified
  - Empty domain bindings: generates minimal config, validates, reloads successfully
- `internal/caddy/command_test.go`
  - Mock command runner returns configurable exit codes and stderr
  - Real command runner integration test (skipped if Caddy not installed)

### 5.4 Dependencies/Config

- No new Go dependencies; uses `os/exec`, `sync` from standard library
- New config fields in htmlservd config:
  - `caddy.binary_path` (default: `caddy` — found via PATH)
  - `caddy.admin_api` (default: `http://localhost:2019` — for potential future use)
  - `caddy.config_backup_path` (default: `<caddyfile_path>.bak`)

## 6. Acceptance Criteria

- [ ] AC-1: `caddy.Reloader.Reload()` generates a new Caddyfile from current domain bindings, validates it via `caddy validate`, writes it atomically, and triggers `caddy reload`.
- [ ] AC-2: If `caddy validate` fails, the existing Caddyfile is not modified, and the error (including Caddy's stderr output) is returned to the caller.
- [ ] AC-3: If `caddy reload` fails after the Caddyfile has been replaced, the backup Caddyfile is restored, and the error is returned to the caller.
- [ ] AC-4: Before replacing the active Caddyfile, a backup copy is created at the configured backup path.
- [ ] AC-5: Concurrent calls to `Reload()` are serialized via a mutex — only one reload runs at a time.
- [ ] AC-6: The Caddy binary path is configurable; if the binary is not found, a clear error message is returned (not a panic).
- [ ] AC-7: Domain binding API handlers (create and delete) trigger a config regeneration and Caddy reload after the database operation succeeds.
- [ ] AC-8: Reload success and failure events are logged with structured context (trigger reason, error details).
- [ ] AC-9: All unit tests pass using a mock command runner (no Caddy binary required for tests).

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for the full reload sequence (generate -> validate -> write -> reload) using mock command runner
- [ ] Unit tests for each failure mode: validation failure, reload failure, missing binary
- [ ] Unit test for backup creation and restoration
- [ ] Unit test for concurrent reload serialization
- [ ] Integration test (skipped without Caddy): real validate + reload with a simple Caddyfile

### Manual Tests

- [ ] With Caddy running: create a domain binding via API, verify Caddyfile updated and Caddy reloaded (check `caddy list-modules` or curl the domain)
- [ ] Introduce a deliberately invalid domain binding, verify validation catches it and old config is preserved
- [ ] Stop Caddy, attempt a reload, verify graceful error handling
- [ ] Check backup file is created at expected path after a successful reload

## 8. Performance / Reliability Considerations

- The reload sequence involves two subprocess calls (`caddy validate` + `caddy reload`), each expected to complete in under 1 second for typical configurations.
- The mutex prevents reload storms if multiple domain binding changes happen in rapid succession. Consider debouncing (coalesce multiple changes into a single reload) as a future optimization.
- Backup file ensures recoverability even if htmlservd crashes mid-reload (the backup can be manually restored).
- File operations (write temp, rename, backup) use atomic patterns to minimize the window for corruption.

## 9. Risks & Mitigations

- **Risk:** Caddy binary not installed or not in PATH on the server. **Mitigation:** Check for Caddy at startup, log a warning if not found. Reload attempts return a clear error. Document Caddy as a required dependency.
- **Risk:** Caddy admin API not enabled or running on a different port. **Mitigation:** For v1, use `caddy reload` CLI command which does not require the admin API. Document the assumption that Caddy's admin API is at the default address.
- **Risk:** File permission issues writing to `/etc/caddy/Caddyfile`. **Mitigation:** Document that htmlservd must run as a user with write access to the Caddyfile path (or configure an alternative path). Check permissions at startup.
- **Risk:** Race between Caddy reading the config and htmlservd writing it. **Mitigation:** Atomic rename ensures Caddy either reads the old or new file, never a partial write.
- **Risk:** Backup restoration fails (e.g., disk full). **Mitigation:** Log the failure prominently. The operator can manually restore from the backup or fix the Caddyfile.

## 10. Open Questions

- Should the reloader debounce rapid successive domain changes (e.g., batch multiple adds into one reload)? For v1, each change triggers an immediate reload. Debouncing can be added later if reload frequency becomes a problem.
- Should the reloader use the Caddy admin API (`POST /load` at `localhost:2019`) instead of the CLI `caddy reload` command? The admin API avoids forking a subprocess and may be more reliable. Decision: CLI for v1 simplicity, consider admin API post-MVP.
- Should htmlservd verify that Caddy is actually serving the new config after reload (e.g., health check request)? Default: No for v1 — trust the reload exit code.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
