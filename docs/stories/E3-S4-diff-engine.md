# E3-S4 - Diff Engine

**Epic:** Epic 3 — Remote transport + kubectl UX
**Status:** Not Started
**Priority:** P2 (High)
**Estimated Effort:** 3 days
**Dependencies:** E3-S1 (context config), E3-S2 (SSH tunnel transport), E2-S3 (server state API)
**Target:** htmlctl v1
**Design Reference:** Technical Spec Section 9.2 (Core commands — diff), PRD Section 5 (Core user journey 2: diff -> apply -> verify)

---

## 1. Objective

Implement a diff engine that compares the local desired state (site directory on disk) against the server's current desired state for a given environment. This enables operators and AI agents to preview exactly what an `apply` would change before executing it — a critical safety feature for production workflows. The diff shows component-level changes (added, modified, removed) with file names and hash comparisons, following the kubectl diff pattern.

## 2. User Story

As an operator or AI agent, I want to run `htmlctl diff -f ./site --context staging` before applying so that I can see exactly which components, pages, styles, and assets have changed, been added, or been removed — preventing unintended deployments and giving me confidence that only the expected changes will be applied.

## 3. Scope

### In Scope

- `htmlctl diff -f ./site --context <ctx>` — compare local site directory to server environment's current desired state
- Fetch the server's current desired state manifest (file paths + SHA256 hashes) via API
- Compute local file hashes (SHA256) for the site directory
- Compare hashes to classify each file as: added, modified, removed, or unchanged
- Display component-level diff summary: file name, change type, old hash (truncated), new hash (truncated)
- Color-coded terminal output: green for added, yellow for modified, red for removed
- `--dry-run` flag on `htmlctl apply` that runs the diff and shows what would happen without executing
- Exit code convention: 0 = no changes, 1 = changes detected, 2 = error (matches `diff` and `kubectl diff` conventions)
- `--output json` and `--output yaml` for machine-parseable diff output
- Group changes by resource type (pages, components, styles, scripts, assets) for readability

### Out of Scope

- Content-level diff (line-by-line file comparison) — only hash-based change detection in v1
- Rendered output diff (comparing final HTML output) — only source file diff
- Three-way merge or conflict resolution
- Diff between two remote environments (e.g., staging vs prod)
- Diff against a specific historical release (only compares to current active state)
- Interactive diff approval workflow

## 4. Architecture Alignment

- **Technical Spec Section 9.2**: `htmlctl diff -f ./site --context staging` is a defined core command. This story implements it.
- **PRD Section 5, Journey 2**: "Deploy to staging: diff -> apply -> verify" — diff is the first step in the safe deployment workflow.
- **PRD Section 6**: "diff shows small component-level changes for typical edits" — the diff engine must present changes at the component/file level, not as a monolithic blob.
- **Technical Spec Section 2.6 (Asset)**: Assets use content-addressed storage by SHA256. The diff engine reuses this hashing strategy for all file types, making comparison uniform.
- **Technical Spec Section 6.4 (Bundle validation)**: Bundle manifests include file hashes. The diff engine uses the same hash computation, ensuring consistency between diff and apply.
- **Component boundary**: Diff logic lives in `internal/diff/` as a standalone package. It depends on `internal/bundle` (for local hash computation) and `internal/client` (for fetching server state). It does not depend on specific commands — the `diff` and `apply --dry-run` commands both use the same diff engine.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/diff/diff.go` — Core diff engine: `ComputeDiff(local Manifest, remote Manifest) DiffResult`
- `internal/diff/types.go` — `DiffResult`, `FileChange` (path, change type, old hash, new hash), `ChangeType` enum (Added, Modified, Removed, Unchanged)
- `internal/diff/display.go` — Terminal display: grouped by resource type, color-coded, summary counts
- `internal/diff/display_json.go` — JSON/YAML output for machine consumption
- `cmd/htmlctl/diff_cmd.go` — `htmlctl diff` command wiring: resolve context, fetch remote manifest, compute local manifest, run diff, display

### 5.2 Files to Modify

- `cmd/htmlctl/root.go` — Register `diff` subcommand
- `cmd/htmlctl/apply_cmd.go` — Add `--dry-run` flag that delegates to diff engine instead of uploading
- `internal/client/client.go` — Add `GetDesiredStateManifest()` method to fetch server's current file manifest (paths + hashes)
- `internal/client/types.go` — Add `DesiredStateManifest` response type
- `internal/bundle/manifest.go` — Ensure manifest computation is reusable by both bundle creation and diff

### 5.3 Tests to Add

- `internal/diff/diff_test.go` — Unit tests for diff computation: added files, modified files, removed files, unchanged files, empty states, mixed changes
- `internal/diff/display_test.go` — Test display output formatting (table, JSON, YAML)
- `cmd/htmlctl/diff_cmd_test.go` — Integration test with mock server: verify end-to-end diff flow
- `cmd/htmlctl/apply_cmd_test.go` — Test `--dry-run` flag triggers diff instead of apply

### 5.4 Dependencies/Config

- No new external dependencies; uses `crypto/sha256` from standard library (already used by bundle package)
- Terminal color output: use `os.Getenv("NO_COLOR")` / `os.IsTerminal()` to respect color preferences; consider lightweight color library or ANSI escape codes directly

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl diff -f ./site --context staging` compares local site files to the server's current desired state and displays changes
- [ ] AC-2: Added files (present locally, absent on server) are shown with change type "added" and new hash
- [ ] AC-3: Modified files (present both locally and on server with different hashes) are shown with change type "modified", old hash, and new hash
- [ ] AC-4: Removed files (absent locally, present on server) are shown with change type "removed" and old hash
- [ ] AC-5: Unchanged files (matching hashes) are not displayed by default (only a summary count)
- [ ] AC-6: Output is grouped by resource type: pages, components, styles, scripts, assets
- [ ] AC-7: Terminal output uses color coding: green for added, yellow for modified, red for removed
- [ ] AC-8: Color output is suppressed when `NO_COLOR` env var is set or stdout is not a terminal
- [ ] AC-9: `--output json` produces a machine-parseable JSON diff result
- [ ] AC-10: `--output yaml` produces a YAML diff result
- [ ] AC-11: Exit code is 0 when no changes are detected, 1 when changes exist, 2 on error
- [ ] AC-12: `htmlctl apply -f ./site --context staging --dry-run` shows the diff and prints "Dry run: no changes applied" without uploading or creating a release
- [ ] AC-13: A summary line shows total counts: "N added, N modified, N removed, N unchanged"
- [ ] AC-14: Hash values in display output are truncated to first 8 characters for readability (full hash available in JSON/YAML output)

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for `ComputeDiff`: all combinations of added/modified/removed/unchanged across resource types
- [ ] Unit tests for edge cases: empty local dir vs populated server, populated local dir vs empty server, both empty
- [ ] Unit tests for display formatting: verify table output alignment, color escape codes, summary line
- [ ] Unit tests for JSON and YAML output: verify parseable and correct structure
- [ ] Unit tests for exit code logic: 0/1/2 mapping
- [ ] Integration test: mock server returns a known manifest, local files have known hashes, verify diff output matches expected

### Manual Tests

- [ ] Deploy a site to staging, modify one component locally, run `htmlctl diff` and verify only that component shows as modified
- [ ] Add a new component locally, run diff, verify it shows as added
- [ ] Delete a component locally, run diff, verify it shows as removed
- [ ] Run `htmlctl apply --dry-run` and verify no release is created on the server
- [ ] Run `htmlctl diff --output json` and pipe to `jq` to verify machine parseability
- [ ] Run diff with `NO_COLOR=1` and verify no ANSI escape codes in output

## 8. Performance / Reliability Considerations

- Local hash computation is I/O-bound. For a typical site (< 100 files, < 50MB total), this completes in under 1 second. For larger sites, hashing should stream files rather than loading entirely into memory.
- Server manifest fetch is a single HTTP GET returning a JSON list of paths and hashes — lightweight and fast.
- The diff computation itself is O(n) where n is the union of local and remote file sets — negligible even for large sites.
- Color detection (`os.IsTerminal`) must check stdout specifically, not stderr, to correctly handle piped output.

## 9. Risks & Mitigations

- **Risk:** Server does not yet expose a desired-state manifest endpoint (E2-S3 may not include this). **Mitigation:** Define the expected API contract here; coordinate with E2-S3 implementation. At minimum, the server needs a `GET /api/v1/websites/{name}/environments/{env}/manifest` endpoint returning `{files: [{path, hash}]}`.
- **Risk:** Hash mismatch due to line-ending normalization differences between local OS and server. **Mitigation:** Both local and server hash computation must normalize line endings (LF) before hashing, consistent with Technical Spec Section 3.2 (determinism requirements).
- **Risk:** Large binary assets (images, fonts) slow down local hashing. **Mitigation:** Hash computation streams files in chunks; does not load entire file into memory. Acceptable performance for v1 site sizes.
- **Risk:** `--dry-run` on apply may give users false confidence if the diff misses server-side validation errors. **Mitigation:** Document that `--dry-run` shows file-level changes only; server-side validation (component validation, page validation) runs only during actual apply.

## 10. Open Questions

- Should diff support `--verbose` to also list unchanged files? **Tentative answer:** Yes, add `--verbose` flag that includes unchanged files in the output.
- Should the server manifest endpoint return the full desired state or just the active release state? **Tentative answer:** Return the current desired state (what the next apply would merge into), not the rendered release output. This aligns with "desired state vs actual state" semantics.
- Should diff compare `website.yaml` and page YAML files, or only content files (components, styles, scripts, assets)? **Tentative answer:** Compare all files in the site directory including manifests. Any file change is a meaningful diff.
- Should `--dry-run` also show what the server would validate (e.g., "component X would fail validation")? **Tentative answer:** Not in v1. Dry-run is diff-only; full server-side validation requires actual apply.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
