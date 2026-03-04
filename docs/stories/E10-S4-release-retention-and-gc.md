# E10-S4 — Release Retention and Storage GC

**Epic:** Epic 10 — Review, Automation, and Lifecycle
**Status:** Implemented (2026-03-04)
**Priority:** P1
**Estimated Effort:** 4-5 days
**Dependencies:** E4-S1 (release history), E4-S2 (rollback), E10-S1 (preview URLs)
**Target:** `internal/release/`, `internal/db/`, `internal/server/`, `internal/cli/`, `internal/blob/`
**Design Reference:** operational gap identified after Epic 9 rollout

---

## 1. Summary

Add safe, operator-invoked retention and garbage collection so long-running htmlservd instances can reclaim disk space without breaking active traffic, rollback, or preview URLs.

## 2. Architecture Context and Reuse Guidance

- Reuse existing release history and environment lock patterns.
- Treat retention as a server-side data operation exposed through authenticated API/CLI, not a shell script.
- Preserve the release safety invariants:
  - never delete the active release
  - never delete the newest previous release needed for `rollout undo`
  - never delete a release referenced by an active preview
- Blob GC should be conservative:
  - delete only 64-char hash-named files under the blob root
  - preserve hashes referenced by current desired-state rows
  - deleting unreferenced OG cache blobs is acceptable; it only causes future re-render work

## 3. Proposed Changes and Architecture Improvements

### 3.1 Retention command

Add authenticated API:

- `POST /api/v1/websites/{website}/environments/{env}/retention/run`

Request body:

```json
{ "keep": 20, "dryRun": true, "blobGC": true }
```

CLI:

- `htmlctl retention run website/<name> --env prod --keep 20 [--dry-run] [--blob-gc]`

### 3.2 Retention rules

Candidate releases are all releases older than the newest `keep` releases, minus pinned releases:

- active release
- newest previous release in history
- releases referenced by non-expired previews

Deletion order:

1. atomically rename each prunable release directory to a quarantine name inside the same environment release root
2. delete DB release rows only for successfully quarantined releases
3. best-effort remove quarantined directories after DB delete succeeds

If DB deletion fails after quarantine rename, move quarantined directories back before returning an error.

### 3.3 Blob GC

Mark phase:

- collect `content_hash` values referenced by:
  - websites
  - pages
  - components
  - style bundle file hashes from `style_bundles.files_json`
  - assets
  - website icons

Sweep phase:

- scan blob root
- delete only hash-named files not in the mark set

Do not recurse outside the blob root and do not delete non-hash filenames.

## 4. File Touch List

### Files to Create

- `internal/release/retention.go`
- `internal/release/retention_test.go`
- `internal/server/retention.go`
- `internal/server/retention_test.go`
- `internal/cli/retention_cmd.go`
- `internal/cli/retention_cmd_test.go`

### Files to Modify

- `internal/db/queries.go` — release lookup/delete helpers and hash-reference queries
- `internal/db/queries_test.go`
- `internal/server/routes.go`
- `internal/server/apply.go` or shared lock helpers if additional lock composition is needed
- `internal/client/client.go`
- `internal/client/types.go`
- `internal/cli/root.go`
- `docs/technical-spec.md`
- `docs/operations-manual-agent.md`

## 5. Implementation Steps

1. Add query helpers to list pinned previews and release history for one environment.
2. Implement retention planner that returns:
   - retained release IDs
   - prunable release IDs
   - blob hashes to preserve/delete
3. Implement retention executor under environment lock.
4. Quarantine release directories before DB deletion so history never points at already-deleted directories.
5. Implement optional blob sweep after release pruning.
6. Add server handler, response DTO, and audit log entry.
7. Add CLI command with dry-run output.

## 6. Tests and Validation

### Automated

- Retention planner tests:
  - active release preserved
  - previous rollback target preserved
  - preview-pinned release preserved
  - dry-run returns correct counts
- Execution tests:
  - quarantine rename rolls back when DB delete fails
  - old release directories deleted
  - active and pinned releases remain
  - blob GC deletes only orphaned hash files
  - non-hash files under blob root are untouched
- Server tests:
  - invalid keep values return `400`
  - unauthenticated requests return `401`
  - sanitized `500` behavior
- CLI tests:
  - dry-run table/json output
  - `--blob-gc` flag propagation

### Manual

- Create multiple releases, pin one with preview, run retention, verify only unpinned old releases are removed.
- Run `rollout undo` after retention and verify the newest previous release still works.
- Confirm disk usage decreases after pruning.

## 7. Acceptance Criteria

- [ ] AC-1: Operators can run retention for one environment with configurable `keep` count and optional dry-run.
- [ ] AC-2: Active releases, rollback targets, and preview-pinned releases are never deleted.
- [ ] AC-3: Old release directories and DB rows are removed deterministically when not pinned.
- [ ] AC-4: Optional blob GC removes only unreferenced hash-named files and never traverses outside the blob root.
- [ ] AC-5: Release rows are deleted only after their directories are quarantined successfully; failed DB deletion restores quarantined directories.
- [ ] AC-6: Retention operations are audit-logged and exposed through deterministic CLI output.
- [ ] AC-7: `rollout undo` still works after retention when at least one previous release exists.
- [ ] AC-8: All API 5xx responses remain sanitized.
- [ ] AC-9: `go test -race ./internal/release/... ./internal/server/... ./internal/cli/...` passes.

## 8. Risks and Open Questions

### Risks

- **Retention deletes a release still needed by another feature.**
  Mitigation: pin previews explicitly and always retain the immediate rollback target.
- **Blob GC over-deletes because a reference source was forgotten.**
  Mitigation: keep mark roots limited to current desired-state DB rows and restrict sweep to hash-named files only.
- **Large delete batches block deploy traffic too long.**
  Mitigation: dry-run first; perform deletion under one environment lock, not a global server lock.

### Open Questions

- None blocking. v1 is operator-invoked retention, not automatic background pruning.
