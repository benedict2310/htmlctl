# E4-S3 - Promote (Artifact Promotion)

**Epic:** Epic 4 — Promotion and rollback
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E4-S1 (release history), E2-S4 (release management), E2-S5 (audit log)
**Target:** v1
**Design Reference:** PRD sections 2, 5.3, 6, 8, 9; Technical spec sections 4, 5, 10

---

## 1. Objective

The core deployment safety guarantee of htmlctl is that what you verify in staging is exactly what runs in production — byte for byte, no rebuild. This story implements the `htmlctl promote` command and server-side promotion logic that copies the exact release artifact from one environment to another, verifies hash equality, activates the promoted release, and records the action in the audit log.

## 2. User Story

As an operator or AI agent, I want to promote a verified staging release to production without rebuilding so that production receives the exact same bytes I tested, eliminating build-time drift and guaranteeing deterministic deployment.

## 3. Scope

### In Scope

- CLI command: `htmlctl promote website/<name> --from <source-env> --to <target-env>`
- Server API endpoint: `POST /api/v1/websites/{name}/promote` with JSON body `{"from": "<env>", "to": "<env>"}`
- Server promotion logic:
  - Resolve the active release in the source environment
  - Copy/link release artifact content files from source env release store to target env release store (hard-link when same filesystem; byte-copy fallback otherwise)
  - Regenerate target release metadata (new release ID, target env) and persist lineage fields (`source_release_id`, `source_env`) without hard-linking metadata files
  - Verify output hash equality between source and target copies (SHA-256 of all output files)
  - Atomically activate the promoted release in the target environment (symlink switch)
- Hash verification: compare against stored source manifest from E2-S4 when available; otherwise compute a deterministic source manifest on demand and compare
- Audit log entry for promotion (actor, timestamp, source env, target env, release ID, hash)
- CLI output: confirmation showing release ID, source env, target env, and hash verification result
- Error handling: clear errors for missing source release, hash mismatch, target env not found

### Out of Scope

- Promoting a specific release by ID (v1 promotes the currently active release from the source env)
- Cross-server promotion (both environments must be on the same htmlservd instance)
- Automatic promotion triggers (e.g., promote on passing health check)
- Rollback of a failed promotion (use `rollout undo` from E4-S2 instead)
- Promotion approval workflow or gating

## 4. Architecture Alignment

- **Server component (htmlservd):** New HTTP handler for the promote endpoint. Orchestrates the full promotion pipeline: resolve source release, copy/link artifacts, verify hashes, activate in target, write audit log.
- **CLI component (htmlctl):** New `promote` command using the remote command framework from E3-S3. Sends HTTP POST and displays results.
- **Storage:** Reads from source env's `releases/<id>/` directory. Writes to target env's `releases/<id>/` directory. Modifies target env's `current` symlink. On the same filesystem, `os.Link` is preferred for content files and falls back to byte copy on failure. Metadata files for the target release are always regenerated or copied+rewritten (never hard-linked) to avoid cross-environment mutation.
- **Concurrency:** Must acquire per-environment locks on both source (read lock conceptually, but shared mutex suffices) and target (write lock) to prevent races. Lock ordering: always acquire source lock before target lock to prevent deadlocks.
- **Hash verification:** After copy/link, walk the target release directory and compute SHA-256 for every file. Compare against the stored source release manifest when available; otherwise recompute source manifest from source files and compare. This is the critical safety guarantee.
- **Audit logging:** Required per PRD section 9. Entry includes: actor, timestamp, action (`promote`), source env, target env, release ID, output hash.

### References
- PRD section 2: "Deploy reliably to staging then promote to production without drift"
- PRD section 5.3: "Promote exact release from staging to prod"
- PRD section 6: "`promote` produces identical artifact in prod as staging (hash match)"
- PRD section 8: "Promotion should be artifact promotion (no rebuild): staging release bytes copied/linked into prod"
- PRD section 9: "Promote staging->prod activates identical artifact (hash match)"
- Technical spec section 4: "Promotion (staging -> prod): copy/link the exact release artifact bytes from staging into prod release store, then activate it in prod"
- Technical spec section 5: Storage layout showing parallel env release stores
- Technical spec section 10: "Use robust file ops: `os.Rename` for atomic finalize, symlink switch for activation"

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/server/handler_promote.go` — HTTP handler for `POST /api/v1/websites/{name}/promote`
- `internal/server/handler_promote_test.go` — Unit tests for the promote handler
- `internal/cli/cmd/promote.go` — CLI command implementation for `htmlctl promote`
- `internal/cli/cmd/promote_test.go` — Unit tests for the CLI command
- `internal/release/promote.go` — Core promotion logic: copy/link artifacts, verify hashes, activate
- `internal/release/promote_test.go` — Unit tests for promotion logic
- `internal/release/hash.go` — Hash manifest computation (walk directory, SHA-256 each file, produce sorted manifest)
- `internal/release/hash_test.go` — Tests for hash manifest computation

### 5.2 Files to Modify

- `internal/server/routes.go` — Register the promote endpoint
- `internal/cli/cmd/root.go` — Register `promote` as a top-level command
- `internal/release/symlink.go` — Reuse atomic symlink switch helper (created in E4-S2, or shared from E2-S4)
- `internal/store/release_store.go` — Add `GetActiveRelease(websiteName, envName string) (*Release, error)` if not already present; add `CreatePromotedRelease(websiteName, targetEnv string, release Release) error` to record the promoted release in the target env

### 5.3 Tests to Add

- `internal/release/promote_test.go` — Test full promotion pipeline: copy, verify hash, activate. Test hard-link path. Test byte-copy fallback. Test hash mismatch detection (simulated corruption). Test promotion with missing source release.
- `internal/release/hash_test.go` — Test hash manifest for known directory produces expected output. Test empty directory. Test manifest comparison detects added/removed/changed files.
- `internal/server/handler_promote_test.go` — Test 200 on successful promotion with correct response body. Test 404 for unknown website/env. Test 409 when source env has no active release. Test 500 on hash mismatch (internal error).
- `internal/cli/cmd/promote_test.go` — Test output formatting, flag parsing, error display.

### 5.4 Dependencies/Config

- Depends on release store and `releases` table from E2-S2 / E2-S4
- Depends on audit log module from E2-S5
- Depends on per-environment lock from E2-S4
- Depends on atomic symlink helper from E4-S2 / E2-S4
- Depends on release history (E4-S1) for resolving active release
- Depends on remote command framework from E3-S3
- Uses `crypto/sha256` from Go standard library (no external dependency)

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl promote website/<name> --from staging --to prod` copies the active staging release to prod and activates it
- [ ] AC-2: No rebuild occurs — the promoted release contains the exact same bytes as the source release
- [ ] AC-3: After promotion, SHA-256 hash manifest of the target release matches the source release exactly
- [ ] AC-4: If hash verification fails (e.g., disk corruption during copy), the promotion is aborted and the target env `current` symlink is not modified
- [ ] AC-5: The promoted release appears in the target environment's release history (via `rollout history`)
- [ ] AC-6: An audit log entry is created with action `promote`, actor, timestamp, source env, target env, release ID, and output hash
- [ ] AC-7: If the source environment has no active release, the command returns a clear error and exits with non-zero code
- [ ] AC-8: If the target environment does not exist, the command returns a clear error
- [ ] AC-9: Hard links are used when source and target are on the same filesystem; byte copy is used as fallback
- [ ] AC-10: CLI output confirms promotion: displays release ID, source env, target env, file count, and hash verification status
- [ ] AC-11: Concurrent promote and apply operations on the same target environment are serialized via per-environment locking
- [ ] AC-12: Target release metadata is environment-specific (new target release ID + target env) and includes lineage fields `source_release_id` and `source_env`
- [ ] AC-13: Promotion works when no precomputed source manifest is present by computing source/target manifests on demand before verification
- [ ] AC-14: Target metadata files are not hard links to source metadata files

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: Promotion copies all files from source release directory to target release directory
- [ ] Unit test: Hard-link path is used when `os.Link` succeeds; byte-copy path is used when `os.Link` returns `EXDEV`
- [ ] Unit test: Hash manifest computation produces deterministic sorted output
- [ ] Unit test: Hash verification detects a single byte difference in any file
- [ ] Unit test: Hash verification detects added or removed files
- [ ] Unit test: Promotion aborts and returns error on hash mismatch (target dir is cleaned up)
- [ ] Unit test: Promotion activates the release in the target env via atomic symlink switch
- [ ] Unit test: Audit log entry is written with all required fields
- [ ] Unit test: HTTP handler returns 200 with release details on success
- [ ] Unit test: HTTP handler returns 404 for unknown website or environment
- [ ] Unit test: HTTP handler returns 409 when source has no active release
- [ ] Integration test: Full round-trip — apply to staging, promote to prod, verify prod serves identical content
- [ ] Unit test: target metadata includes `source_release_id`/`source_env` and target-specific release identifiers
- [ ] Unit test: modifying target metadata does not modify source metadata (proves metadata is not hard-linked)
- [ ] Unit test: promotion succeeds when stored source manifest is absent by recomputing manifests

### Manual Tests

- [ ] Apply a release to staging, run `htmlctl promote website/futurelab --from staging --to prod`, verify prod serves the same content
- [ ] Run `htmlctl rollout history website/futurelab --context prod` and verify the promoted release appears
- [ ] Compare file hashes manually between staging and prod release directories (`sha256sum`)
- [ ] Attempt promotion when staging has no releases, verify clear error message
- [ ] Verify audit log entry via `htmlctl logs`

## 8. Performance / Reliability Considerations

- **Hard links vs copy:** Hard links are O(1) per file and use zero additional disk space. On the same filesystem (typical deployment), promotion is nearly instant regardless of release size. Byte copy is O(n) but only used as cross-device fallback.
- **Hash verification cost:** Walking and hashing the release directory is O(n) in total file size. For typical websites (< 100 MB), this completes in under 1 second. This is the price of the safety guarantee and is non-negotiable.
- **Atomicity:** The target release directory is built in a temporary location (`releases/<id>.tmp`), hash-verified, then renamed to final location. Only after successful verification is the `current` symlink switched. A failure at any step leaves the target environment unchanged.
- **Lock duration:** The target environment lock is held during the entire promotion (copy + verify + activate). For typical releases this is < 2 seconds. During this time, concurrent applies to the target environment are blocked.

## 9. Risks & Mitigations

- **Risk:** Hard link fails silently or produces unexpected behavior on certain filesystems (NFS, FUSE). **Mitigation:** Attempt `os.Link`, check for error; on any failure (not just `EXDEV`), fall back to byte copy. Log which strategy was used.
- **Risk:** Hash mismatch due to disk corruption during copy. **Mitigation:** This is the exact scenario hash verification is designed to catch. Abort promotion, clean up partial target directory, return error with details of which files differ.
- **Risk:** Source release is garbage-collected or deleted between resolve and copy. **Mitigation:** Acquire source environment lock (read) before starting copy. Future release pruning (not in v1) must respect active + previous releases.
- **Risk:** Large releases (many assets) make promotion slow. **Mitigation:** Hard links eliminate this for same-filesystem deployments. For byte copy, stream files sequentially with progress reporting. v1 targets < 100 MB releases.
- **Risk:** Promoting to the same environment (`--from staging --to staging`). **Mitigation:** Validate that source and target environments are different. Return clear error.

## 10. Open Questions

- Should promotion preserve the original release ID (ULID from staging) or mint a new ULID for the prod copy? (Recommendation: mint a new ULID for the target env to maintain unique, chronologically sorted IDs per environment. Store the source release ID as metadata for traceability.)
- Should `--dry-run` be supported for promote? (Recommendation: yes — it would resolve the source release, compute hashes, and report what would be promoted without actually copying or activating.)
- Should the command support `--from` and `--to` as context names or environment names? (Recommendation: environment names (`staging`, `prod`) since promotion is intra-server. The `--context` flag or config determines which server to talk to.)

## 11. Research Notes

- **Artifact promotion vs rebuild:** The industry best practice for deployment safety is to promote the exact artifact that was tested, rather than rebuilding from source. Docker image promotion (tag the same digest in a new repo) and Debian package promotion (copy `.deb` between repos) follow this pattern. htmlctl applies the same principle to static website releases.
- **Hard links vs copy:** Hard links (`os.Link`) create additional directory entries pointing to the same inode. They use no extra disk space, are instant, and the data is shared. Limitation: only works within the same filesystem. `os.Link` returns `*os.LinkError` with `syscall.EXDEV` for cross-device attempts. Go code pattern:
  ```go
  err := os.Link(src, dst)
  if err != nil {
      // fallback to io.Copy
  }
  ```
- **Hash verification pattern:** Walk the release directory in sorted order (deterministic), compute `sha256.Sum256` for each file, produce a manifest of `<hash> <relative-path>` lines. Compare manifest strings. This catches any difference: content changes, added files, removed files, permission changes (if hashing metadata).
- **Atomic directory placement:** Build the target release in `releases/<id>.tmp/`, verify hashes, then `os.Rename("releases/<id>.tmp", "releases/<id>")`. This ensures the target release directory is either fully present or not present at all.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
