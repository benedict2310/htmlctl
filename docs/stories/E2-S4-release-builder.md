# E2-S4 - Release Builder

**Epic:** Epic 2 — Server daemon: state, releases, and API
**Status:** Not Started
**Priority:** P0 (Critical Path)
**Estimated Effort:** 4 days
**Dependencies:** E1-S2 (deterministic renderer), E2-S2 (SQLite schema for release records), E2-S3 (ingested desired state and blob store)
**Target:** v1
**Design Reference:** [Technical Spec - Sections 2.7, 3, 4, 5](../technical-spec.md)

---

## 1. Objective

Implement the release builder that takes an environment's current desired state (resource metadata in the database + blobs on disk), renders all pages using the deterministic renderer (E1-S2), produces an immutable release directory, and atomically activates it via symlink switch. This is the core of the deployment pipeline -- turning declared resources into a serveable static website.

## 2. User Story

As an operator, after applying changes to an environment, I want the server to build an immutable release containing the fully rendered static site, activate it atomically so visitors see the new version instantly with zero downtime, and record the release for future rollback or promotion.

## 3. Scope

### In Scope

- Release build API endpoint: `POST /api/v1/websites/{website}/environments/{env}/releases`
- Per-environment build lock (mutex) to prevent concurrent builds for the same environment
- Release ID generation using ULID (lexicographically sortable, time-ordered)
- Build pipeline:
  1. Read current desired state from database (pages, components, style bundles, assets)
  2. Resolve blob content from content-addressed store
  3. Render all pages using the deterministic renderer (E1-S2)
  4. Write rendered output to `releases/<ulid>/tmp/` (not served during build)
  5. Copy/link assets and styles into the release output directory
  6. Compute sha256 hashes of all output files
  7. Finalize: atomic `os.Rename` from `tmp/` to final directory
  8. Record release in database (manifest snapshot, output hashes, build log, status)
  9. Activate: atomically switch `current` symlink to point to new release directory
  10. Update environment's `active_release_id` in database
- Release directory structure:
  ```
  websites/<website>/envs/<env>/releases/<ulid>/
    index.html
    product/index.html
    assets/hero.jpg
    styles/tokens.css
    styles/default.css
    scripts/site.js
    .manifest.json       (snapshot of input manifests)
    .build-log.txt       (build log)
    .output-hashes.json  (sha256 of every output file)
  ```
- Symlink management: `websites/<website>/envs/<env>/current -> releases/<ulid>/`
- Build log capture: record what was rendered, any warnings, timing
- Release status tracking: `building` -> `active` (or `failed`)
- Error handling: if build fails, clean up temp directory, mark release as `failed`

### Out of Scope

- Promotion between environments (Epic 4, E4-S3)
- Rollback logic (Epic 4, E4-S2)
- Release history listing API (Epic 4, E4-S1)
- Release pruning / garbage collection (post-v1)
- Audit log entries for release events (E2-S5)
- Automatic build trigger after apply (manual API call in v1)
- Incremental/partial builds (full rebuild every time in v1)

## 4. Architecture Alignment

- **Release pipeline (Tech Spec Section 4):** This story implements steps 2-5 of the pipeline: "Validate -> Render to `releases/<id>/tmp` -> Finalize: rename tmp -> final directory -> Atomically switch active pointer."
- **Immutable releases (Tech Spec Section 2.7):** Each release is an immutable snapshot. The release row in the database is written once and never updated. The release directory on disk is never modified after finalization.
- **Storage layout (Tech Spec Section 5):** Release directories live under `websites/<website>/envs/<env>/releases/<ulid>/`. The `current` symlink points to the active release.
- **Determinism (Tech Spec Section 3.2):** The renderer (E1-S2) produces byte-stable output. The release builder hashes all outputs to verify this property.
- **Concurrency:** Per-environment mutex prevents concurrent builds. Different environments can build simultaneously. The mutex is held for the entire build-render-finalize-activate cycle.
- **Atomic operations (Tech Spec Section 10):** `os.Rename` for directory finalization. Symlink switch for activation (remove old symlink, create new one -- or use rename of a temporary symlink for atomicity).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/release/builder.go` — Release builder: orchestrates the build pipeline (read state, render, write, finalize, activate)
- `internal/release/ulid.go` — ULID generation for release IDs
- `internal/release/symlink.go` — Symlink creation and atomic switch logic
- `internal/release/buildlog.go` — Build log buffer that captures events during the build
- `internal/api/release.go` — HTTP handler for `POST .../releases` endpoint

### 5.2 Files to Modify

- `internal/server/server.go` — Register release API routes, inject renderer and blob store dependencies
- `internal/api/routes.go` — Add release endpoint route
- `internal/db/queries.go` — Add queries: insert release, update environment active_release_id, get desired state snapshot
- `go.mod` — Add `github.com/oklog/ulid/v2` dependency

### 5.3 Tests to Add

- `internal/release/builder_test.go` — Full build pipeline: desired state -> rendered output -> release directory with correct structure
- `internal/release/ulid_test.go` — ULID generation: uniqueness, lexicographic ordering, time component
- `internal/release/symlink_test.go` — Symlink creation, atomic switch, error handling for missing targets
- `internal/api/release_test.go` — HTTP endpoint: trigger build, verify release created, verify symlink updated
- `internal/release/builder_test.go` (concurrency tests) — Concurrent build requests to same environment are serialized; concurrent builds to different environments run in parallel

### 5.4 Dependencies/Config

- `github.com/oklog/ulid/v2` — ULID generation (widely used, well-maintained)
- E1-S2 renderer package (internal dependency)
- E2-S3 blob store package (internal dependency)
- Standard library: `os`, `path/filepath`, `crypto/sha256`, `io/fs`, `time`, `sync`

## 6. Acceptance Criteria

- [ ] AC-1: `POST /api/v1/websites/{website}/environments/{env}/releases` triggers a full build and returns the release ID (ULID) with HTTP 201
- [ ] AC-2: The release directory contains fully rendered HTML pages at their correct route paths (e.g., `index.html`, `product/index.html`)
- [ ] AC-3: Assets, stylesheets, and scripts are copied into the release directory at their expected paths
- [ ] AC-4: The release directory includes `.manifest.json` (snapshot of input resources), `.build-log.txt`, and `.output-hashes.json`
- [ ] AC-5: During the build, output is written to `releases/<ulid>/tmp/`; this directory is not accessible as the active release
- [ ] AC-6: After successful build, `tmp/` is renamed to the final release directory atomically via `os.Rename`
- [ ] AC-7: The `current` symlink is updated atomically to point to the new release directory
- [ ] AC-8: The `releases` table in the database contains a row with the ULID, environment reference, manifest snapshot, output hashes, and build log
- [ ] AC-9: The environment's `active_release_id` is updated to the new release ULID
- [ ] AC-10: Concurrent build requests to the same environment are serialized (one builds while the other waits); requests to different environments can proceed in parallel
- [ ] AC-11: If the build fails (e.g., renderer error), the temp directory is cleaned up, the release is marked as `failed` in the database, and the `current` symlink is not changed
- [ ] AC-12: Release IDs are ULIDs that sort lexicographically by creation time
- [ ] AC-13: Output file hashes in `.output-hashes.json` match the actual sha256 of the rendered files

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: ULID generation produces unique, time-ordered IDs
- [ ] Unit test: symlink creation points to correct target; atomic switch updates target
- [ ] Unit test: build log captures events with timestamps
- [ ] Integration test: insert desired state into DB, trigger build, verify release directory structure and contents
- [ ] Integration test: verify `.output-hashes.json` hashes match actual file content
- [ ] Integration test: trigger build with invalid state (missing component), verify failure handling and cleanup
- [ ] Integration test: concurrent builds to same environment are serialized (use goroutines + timing)
- [ ] Integration test: verify `current` symlink points to latest successful release

### Manual Tests

- [ ] Apply a sample site via E2-S3, trigger a build, inspect the release directory on disk
- [ ] Verify `current` symlink: `ls -la websites/futurelab/envs/staging/current` points to the ULID directory
- [ ] Trigger two builds in quick succession; verify only one runs at a time
- [ ] Introduce a render error; verify temp directory is cleaned up and symlink unchanged

## 8. Performance / Reliability Considerations

- Full rebuild of a typical site (5-10 pages, ~20 components, ~10 assets) should complete in under 2 seconds
- Large asset copying should be efficient (use hard links where possible on same filesystem; fall back to copy)
- Build lock timeout: if a build takes longer than 60 seconds, consider it failed (configurable)
- Temp directory cleanup on failure must be reliable (deferred cleanup in Go)
- Symlink switch is near-instant for visitors; no request sees a half-built site
- Release directories are never modified after finalization (immutability guarantee)

## 9. Risks & Mitigations

- **Risk:** `os.Rename` across filesystems fails (temp and final must be on same filesystem) — **Mitigation:** Ensure `tmp/` is created inside the release directory's parent (same filesystem). Document this requirement.
- **Risk:** Symlink switch is not truly atomic on all platforms — **Mitigation:** Use the rename-a-temporary-symlink pattern: create a new symlink with a temp name, then `os.Rename` the temp symlink over the `current` symlink. This is atomic on POSIX.
- **Risk:** Renderer from E1-S2 may not be ready — **Mitigation:** Define a clear `Renderer` interface that the builder depends on. Implement a simple stub renderer for testing until E1-S2 is available.
- **Risk:** Disk space exhaustion from accumulated releases — **Mitigation:** Out of scope for this story, but document that a release pruning mechanism is needed (post-v1 or separate story).
- **Risk:** Build lock contention under rapid successive applies — **Mitigation:** Per-environment lock is fine for v1 workload. Queue-based approach can be added later if needed.

## 10. Open Questions

- Should the build endpoint be synchronous (return after build completes) or asynchronous (return immediately with release ID, poll for status)? Recommendation: synchronous in v1 for simplicity; builds are fast for typical sites.
- Should we support hard-linking assets from the blob store into the release directory (saves disk space) or always copy? Recommendation: copy in v1 for simplicity and portability; hard links as optimization later.
- Should the renderer interface be defined in this story or in E1-S2? Recommendation: define the interface in a shared package; E1-S2 implements it, this story consumes it.
- Release directory structure: should routes like `/product` produce `product/index.html` or `product.html`? Recommendation: `product/index.html` for clean URLs with any static file server.

## 11. Research Notes

- **ULID library for Go:** `github.com/oklog/ulid/v2` is the standard choice. ULIDs are 128-bit identifiers that encode timestamp + randomness, are lexicographically sortable, and are URL-safe. Generation: `ulid.New(ulid.Timestamp(time.Now()), entropy)` where `entropy` is a `math/rand` or `crypto/rand` source.
- **Atomic file operations:**
  - `os.Rename` is atomic on POSIX when source and destination are on the same filesystem.
  - For atomic symlink switch: create a temp symlink (`current.tmp -> releases/<ulid>`), then `os.Rename("current.tmp", "current")`. The rename atomically replaces the old symlink.
  - Go's `os.Symlink` creates a symlink; `os.Rename` atomically replaces an existing file/symlink.
- **Symlink patterns:**
  - The `current` symlink is a relative symlink: `current -> releases/<ulid>/` (relative, so it works if the data directory is moved).
  - On Linux/macOS, symlink resolution is transparent to static file servers (Caddy follows symlinks by default).
- **ULID format example:** `01ARZ3NDEKTSV4RRFFQ69G5FAV` — 26 characters, Crockford's Base32, encodes 48-bit timestamp (milliseconds) + 80-bit randomness.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
