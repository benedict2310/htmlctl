# E4-S2 - Rollback

**Epic:** Epic 4 — Promotion and rollback
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E4-S1 (release history), E2-S4 (release/symlink management), E2-S5 (audit log)
**Target:** v1
**Design Reference:** PRD sections 2, 5.4, 6, 9; Technical spec sections 4, 5, 7, 9.2

---

## 1. Objective

Production incidents require instant recovery. This story implements the `htmlctl rollout undo` command and server-side rollback logic that atomically switches the `current` symlink back to the previous release. Rollback must complete in under 1 second since it is a symlink switch with no rebuild, re-render, or file copying required.

## 2. User Story

As an operator or AI agent, I want to instantly roll back an environment to its previous release so that I can recover from a bad deployment in under 1 second without rebuilding or redeploying.

## 3. Scope

### In Scope

- CLI command: `htmlctl rollout undo website/<name> --context <ctx>`
- Server API endpoint: `POST /api/v1/websites/{name}/envs/{env}/rollback`
- Server logic: resolve the previous release from history, atomically switch the `current` symlink to point to it, and persist the new active release in `environments.active_release_id`
- Safety checks before rollback:
  - Verify a previous release exists (at least 2 releases in history)
  - Verify the previous release directory still exists on disk
  - Reject rollback if environment has only one release (clear error message)
- Atomic symlink switch using rename pattern (`symlink.tmp` -> rename to `current`)
- Audit log entry for every rollback action (actor, timestamp, environment, from-release, to-release)
- CLI output: confirmation message showing old release ID and new active release ID
- Non-zero exit code and descriptive error when rollback is not possible

### Out of Scope

- Rollback to an arbitrary release by ID (future: `rollout undo --to-release <id>`)
- Multi-step rollback (rolling back multiple times consecutively is supported naturally by repeating the command, but no `--steps N` flag in v1)
- Automatic rollback triggered by health checks (post-MVP)
- Rollback confirmation prompt (agents need non-interactive operation; rely on audit log for accountability)

## 4. Architecture Alignment

- **Server component (htmlservd):** New HTTP handler for the rollback endpoint. Uses the release store to look up the previous release from the currently active release, performs an atomic symlink switch on the filesystem, updates `environments.active_release_id`, and writes an audit log entry via E2-S5.
- **CLI component (htmlctl):** New `rollout undo` subcommand using the remote command framework from E3-S3. Sends HTTP POST to the server and displays the result.
- **Storage:** Reads from `releases` to find the release immediately before the currently active one and updates `environments.active_release_id` after a successful switch. Modifies the `current` symlink in `/var/lib/htmlservd/websites/{name}/envs/{env}/current`. No files are copied, moved, or rebuilt.
- **Concurrency:** Rollback must acquire the same per-environment lock used by the release builder (E2-S4) to prevent races between concurrent apply and rollback operations. The lock is held only for the duration of the symlink switch (microseconds).
- **Atomic symlink switch:** Create a temporary symlink (`current.tmp` -> target release), then `os.Rename("current.tmp", "current")`. On Linux/macOS, rename of a symlink is atomic.
- **Audit logging:** Required per PRD section 9. Entry includes: actor, timestamp, environment, action (`rollback`), from-release ID, to-release ID.

### References
- PRD section 2: "Production-safe deployments: immutable releases, atomic switch, instant rollback"
- PRD section 6: "Deployment is atomic; rollback < 1 second"
- PRD section 9: "Audit log records all applies/promotions/rollbacks"
- Technical spec section 4: "Rollback: switch `current` back to previous release"
- Technical spec section 5: Storage layout with `current -> releases/<releaseId>/` symlink
- Technical spec section 10: "Use robust file ops: `os.Rename` for atomic finalize, symlink switch for activation"

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/server/handler_rollback.go` — HTTP handler for `POST /api/v1/websites/{name}/envs/{env}/rollback`
- `internal/server/handler_rollback_test.go` — Unit tests for the rollback handler
- `internal/cli/cmd/rollout_undo.go` — CLI command implementation for `htmlctl rollout undo`
- `internal/cli/cmd/rollout_undo_test.go` — Unit tests for the CLI command
- `internal/release/rollback.go` — Core rollback logic: resolve previous release, atomic symlink switch, audit log
- `internal/release/rollback_test.go` — Unit tests for rollback logic

### 5.2 Files to Modify

- `internal/server/routes.go` — Register the rollback endpoint
- `internal/cli/cmd/rollout.go` — Register `undo` as a subcommand of `rollout`
- `internal/store/release_store.go` — Add `GetPreviousRelease(websiteName, envName string) (*Release, error)` method (must resolve predecessor of the currently active release) and `SetActiveRelease(websiteName, envName, releaseID string) error`
- `internal/release/symlink.go` — Reuse existing atomic symlink helper from E2-S4 (extend only if required)

### 5.3 Tests to Add

- `internal/release/rollback_test.go` — Test successful rollback switches symlink, test rollback with no previous release returns error, test rollback with missing release directory returns error, and repeated rollback (`V3 -> V2 -> V1`) with DB/symlink active-state consistency checks
- `internal/server/handler_rollback_test.go` — Test 200 response on successful rollback, test 409 when no previous release, test 404 for unknown website/env
- `internal/cli/cmd/rollout_undo_test.go` — Test output formatting, error handling

### 5.4 Dependencies/Config

- Depends on release store and `releases` table from E2-S2 / E2-S4
- Depends on audit log module from E2-S5
- Depends on per-environment lock from E2-S4 (reused for rollback safety)
- Depends on remote command framework from E3-S3
- Depends on release history (E4-S1) for resolving previous release

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl rollout undo website/<name> --context <ctx>` switches the active release to the previous release
- [ ] AC-2: Rollback completes in under 1 second (symlink switch only, no rebuild)
- [ ] AC-3: After rollback, `htmlctl rollout history` shows the previous release as `active`
- [ ] AC-4: An audit log entry is created with action `rollback`, actor, timestamp, from-release ID, and to-release ID
- [ ] AC-5: If no previous release exists (only one release in history), the command returns a clear error and exits with non-zero code
- [ ] AC-6: If the previous release directory is missing from disk, the command returns a clear error and does not modify the `current` symlink
- [ ] AC-7: The symlink switch is atomic — concurrent readers never see a broken or intermediate state
- [ ] AC-8: Rollback acquires the per-environment lock, preventing races with concurrent apply operations
- [ ] AC-9: CLI output confirms the rollback: displays the old active release ID and the new active release ID
- [ ] AC-10: After rollback, `environments.active_release_id` is updated to the same release targeted by the `current` symlink
- [ ] AC-11: Repeated rollback commands follow active state (`V3 -> V2 -> V1`) rather than oscillating on stale history reads

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: Rollback correctly resolves the previous release from the store
- [ ] Unit test: Atomic symlink switch creates `current.tmp` then renames to `current`
- [ ] Unit test: Rollback returns error when only one release exists
- [ ] Unit test: Rollback returns error when previous release directory is missing
- [ ] Unit test: Audit log entry is written with correct fields on successful rollback
- [ ] Unit test: HTTP handler returns 200 with release IDs on success
- [ ] Unit test: HTTP handler returns 409 Conflict when no previous release exists
- [ ] Integration test: Full round-trip — apply two releases, rollback, verify `current` symlink target
- [ ] Integration test: apply three releases, run rollback twice, verify state transitions `V3 -> V2 -> V1`
- [ ] Integration test: after each rollback, `environments.active_release_id` equals the release currently targeted by `current`

### Manual Tests

- [ ] Deploy 2+ releases to staging, run `htmlctl rollout undo website/futurelab --context staging`, verify the site serves the previous release content
- [ ] Verify rollback completes in under 1 second (time the command)
- [ ] Verify `htmlctl rollout history` reflects the rollback
- [ ] Attempt rollback on environment with only 1 release, verify clear error message
- [ ] Verify audit log entry via `htmlctl logs`

## 8. Performance / Reliability Considerations

- Rollback is O(1): single symlink switch regardless of release size. Target: < 1 second end-to-end including SSH tunnel overhead.
- Atomic rename guarantees no partial state visible to the front proxy (Caddy) serving the `current` directory.
- Per-environment lock prevents apply-during-rollback races. Lock scope is minimal (microseconds for symlink switch).
- Immutable release directories mean rollback target is always intact (no mutation after creation).

## 9. Risks & Mitigations

- **Risk:** Race between rollback and concurrent apply — both try to switch `current` symlink. **Mitigation:** Reuse the per-environment mutex from the release builder (E2-S4). Both apply and rollback acquire the same lock.
- **Risk:** Previous release directory was manually deleted from disk. **Mitigation:** Check `os.Stat` on the target release directory before switching. Return a clear error if missing.
- **Risk:** On some filesystems, `os.Rename` of a symlink may not be atomic. **Mitigation:** Use the well-established `symlink(tmp) + rename(tmp, target)` pattern which is atomic on Linux (ext4, XFS, btrfs) and macOS (APFS, HFS+). Document supported filesystems.
- **Risk:** Repeated rollbacks oscillate between two releases. **Mitigation:** This is expected behavior (undo/redo pattern). Each rollback is audited. Future enhancement could add `--to-release <id>` for targeted rollback.

## 10. Open Questions

- Should rollback support a `--dry-run` flag that shows what would change without actually switching? (Recommendation: yes, aligns with kubectl patterns, low implementation cost.)
- Should consecutive rollbacks be limited or rate-limited to prevent accidental oscillation? (Recommendation: no limit in v1; audit log provides accountability.)

## 11. Research Notes

- **Atomic symlink switch pattern:** The standard Go pattern is: `os.Symlink(target, path+".tmp")` followed by `os.Rename(path+".tmp", path)`. `os.Rename` on POSIX is atomic for the directory entry update. This is the same pattern used by Capistrano, Kubernetes kubelet, and other deployment tools.
- **Rollback safety checks:** Kubernetes checks that the target revision exists before rollback. htmlctl should verify both the database record and the filesystem directory exist.
- **kubectl rollout undo:** `kubectl rollout undo deployment/nginx` rolls back to the previous revision. It also supports `--to-revision=N` for targeted rollback. htmlctl mirrors the default behavior in v1.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
