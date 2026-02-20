# E2-S3 - Bundle Ingestion (Apply)

**Epic:** Epic 2 — Server daemon: state, releases, and API
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 4 days
**Dependencies:** E2-S1 (server bootstrap + config), E2-S2 (SQLite schema for resource metadata)
**Target:** v1
**Design Reference:** [Technical Spec - Sections 2, 4, 5, 6](../technical-spec.md)

---

## 1. Objective

Implement the server-side HTTP endpoint that accepts a resource bundle (tar archive with manifest and files) from `htmlctl apply`, verifies content integrity via hash checking, stores file blobs in content-addressed storage, and merges the desired state into the environment's current state in the database. This is the primary write path for getting content into the server.

## 2. User Story

As an operator using `htmlctl apply`, I want the server to accept my bundle of resource manifests and files, verify their integrity, store them efficiently, and merge them into the environment's desired state so that my changes are persisted and ready for the next release build.

## 3. Scope

### In Scope

- HTTP API endpoint: `POST /api/v1/websites/{website}/environments/{env}/apply`
- Bundle format definition: tar archive containing:
  - `manifest.json` — declares all resources and their content hashes
  - Resource files (components, styles, scripts, assets, page definitions)
- Bundle upload handling (multipart or raw body with content-type)
- Manifest parsing and validation
- Hash verification: compute sha256 of each received file, compare against manifest hashes
- Content-addressed blob storage: write files to `<data-dir>/blobs/sha256/<hash>`
  - Deduplicate: skip write if blob already exists with matching hash
- State merge logic: upsert resource metadata into database tables
  - Full apply: manifest declares complete desired state; resources not in manifest are removed
  - Partial apply: manifest declares only changed resources; merge into existing state
  - Distinguish via `manifest.mode: "full" | "partial"` field
- Auto-create website and environment records if they do not exist
- Validation of resources before accepting (delegate to validation engine from E1-S3 if available, otherwise basic checks)
- Dry-run mode: validate and report what would change without persisting (`?dry_run=true`)
- Response body: structured JSON with accepted resources, hashes, and any warnings

### Out of Scope

- Release building after apply (E2-S4 handles rendering and release creation)
- Triggering a release automatically after apply (will be wired in E2-S4)
- Client-side bundle creation (handled by `htmlctl` CLI, Epic 3)
- Component HTML validation (E1-S3, referenced but not implemented here)
- Audit log writing (E2-S5; this story prepares the hook but does not implement the audit subsystem)
- Authentication / authorization at endpoint level (delivered in E6-S1)
- Bundle compression (gzip/zstd; accept uncompressed tar in v1)

## 4. Architecture Alignment

- **Release pipeline (Tech Spec Section 4):** Bundle ingestion is step 1 of the pipeline: "Receive bundle (manifests + files)." This story implements receiving and persisting; the rendering/release steps are E2-S4.
- **Storage layout (Tech Spec Section 5):** Blobs stored at `<data-dir>/blobs/sha256/<hash>`. Database stores metadata with hash references. This story creates blobs and updates DB.
- **Validation (Tech Spec Section 6):** Bundle validation requires verifying hashes from the client bundle manifest. This story implements hash verification. Component/page/asset validation rules are applied here if the validation engine is available.
- **Agent-friendly partial apply (Tech Spec Section 9.3):** The server merges partial applies into the last known desired state. The manifest includes a `mode` field to distinguish full vs. partial.
- **Concurrency:** Apply operations for the same website must be serialized because desired-state resources are website-scoped in v1. Use a per-website mutex to prevent concurrent applies from corrupting shared state.
- **Security model (Tech Spec Section 7):** Endpoint is localhost-bound by default and, as of E6-S1, protected by bearer-token middleware when API auth is configured.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/api/routes.go` — HTTP router setup, route registration
- `internal/api/apply.go` — Apply endpoint handler: parse bundle, verify, store, merge
- `internal/bundle/bundle.go` — Bundle format definition, tar reading, manifest parsing
- `internal/bundle/manifest.go` — Manifest struct, JSON parsing, validation
- `internal/blob/store.go` — Content-addressed blob store: write, read, exists, path helpers
- `internal/state/merge.go` — State merge logic: full apply (replace all), partial apply (upsert changed)

### 5.2 Files to Modify

- `internal/server/server.go` — Register API routes, pass dependencies (DB, blob store) to handlers
- `internal/db/queries.go` — Add queries for upserting resources, listing resources by website/env, deleting removed resources
- `go.mod` — No new dependencies expected (tar and sha256 are stdlib)

### 5.3 Tests to Add

- `internal/bundle/bundle_test.go` — Bundle reading: valid tar, missing manifest, corrupt files
- `internal/bundle/manifest_test.go` — Manifest parsing: valid JSON, missing fields, invalid hashes
- `internal/blob/store_test.go` — Blob write, deduplication, read-back, path calculation
- `internal/api/apply_test.go` — End-to-end apply: valid bundle accepted, hash mismatch rejected, partial merge works, full apply replaces state, dry-run returns changes without persisting
- `internal/state/merge_test.go` — Full vs. partial merge logic, resource removal on full apply, upsert behavior

### 5.4 Dependencies/Config

- Standard library only: `archive/tar`, `crypto/sha256`, `encoding/json`, `io`, `net/http`, `os`, `path/filepath`
- No external dependencies for this story

## 6. Acceptance Criteria

- [ ] AC-1: `POST /api/v1/websites/{website}/environments/{env}/apply` accepts a tar archive body and returns 200 on success
- [ ] AC-2: The manifest (`manifest.json` in the tar root) declares resources with their content hashes and an apply mode (`full` or `partial`)
- [ ] AC-3: Each file in the bundle is hashed with sha256; if any hash does not match the manifest, the entire apply is rejected with HTTP 400 and a list of mismatched files
- [ ] AC-4: Verified file blobs are stored at `<data-dir>/blobs/sha256/<hash>`; duplicate blobs (same hash) are not re-written
- [ ] AC-5: In `full` mode, the environment's desired state is replaced entirely: resources not in the manifest are removed from the database
- [ ] AC-6: In `partial` mode, only resources declared in the manifest are upserted; existing resources not mentioned are preserved
- [ ] AC-7: If the website or environment does not exist, they are auto-created
- [ ] AC-8: `?dry_run=true` query parameter causes validation and diff calculation without persisting any changes; response includes what would change
- [ ] AC-9: Concurrent apply requests to the same environment are serialized (second request waits or returns 409 Conflict)
- [ ] AC-10: The response body is JSON containing: accepted resource list with hashes, apply mode used, and any warnings
- [ ] AC-11: Invalid tar archives or missing manifests return HTTP 400 with descriptive error messages
- [ ] AC-12: Blob storage directory is created on-demand if it does not exist

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: parse a valid manifest JSON, verify all fields extracted correctly
- [ ] Unit test: create a tar archive in memory, read it back via bundle parser, verify file contents
- [ ] Unit test: hash verification passes for matching content, fails for mismatched content
- [ ] Unit test: blob store writes file, deduplicates on second write, reads back correctly
- [ ] Unit test: full merge replaces all resources; partial merge upserts only declared resources
- [ ] Integration test: POST a valid bundle to the apply endpoint, verify 200 response and database state
- [ ] Integration test: POST a bundle with a hash mismatch, verify 400 response with error details
- [ ] Integration test: POST with `?dry_run=true`, verify no database changes
- [ ] Integration test: concurrent applies to same environment are serialized (no data corruption)

### Manual Tests

- [ ] Create a sample tar bundle manually, POST it with curl, verify blob files created
- [ ] Apply twice with same content, verify blob deduplication (file not re-written)
- [ ] Apply partial update (single component change), verify only that component updated in DB
- [ ] Apply full update missing a component, verify component removed from DB

## 8. Performance / Reliability Considerations

- Bundle size limit: 50MB default (configurable). Reject larger bundles with HTTP 413.
- Stream tar reading: do not buffer entire bundle in memory. Process entries as they are read.
- Blob writes use atomic write pattern: write to temp file, rename to final path (prevents partial writes on crash).
- Per-environment mutex prevents concurrent apply corruption but should not block applies to different environments.
- Hash computation is streaming (feed bytes to sha256 hasher during read, not after).

## 9. Risks & Mitigations

- **Risk:** Large bundles with many assets could cause slow applies — **Mitigation:** Stream processing, size limits, and progress logging. Optimize blob deduplication (check existence before write).
- **Risk:** Partial apply mode could leave inconsistent state if manifest is malformed — **Mitigation:** Validate manifest completely before starting any writes. Use a database transaction: all-or-nothing.
- **Risk:** Disk full during blob writes — **Mitigation:** Check write errors, clean up partial files, return clear error to client.
- **Risk:** Race condition between hash check and blob write — **Mitigation:** Atomic write (temp file + rename). If blob already exists, skip.

## 10. Open Questions

- Should the bundle format use tar or zip? Recommendation: tar (streaming-friendly, simpler in Go stdlib). Zip could be added later.
- Should partial apply support deleting specific resources (e.g., "remove component X")? Recommendation: yes, via a `deleted` field in the manifest resource entry.
- Maximum number of resources per bundle? Recommendation: no hard limit in v1; rely on bundle size limit.
- Should the apply endpoint trigger a release build automatically, or require a separate API call? Recommendation: separate call in v1 for explicitness; auto-trigger can be added later.

## 11. Research Notes

- **Content-addressed storage patterns:** Git uses SHA-1 (migrating to SHA-256) for its object store. The pattern is: hash content -> use hash as filename -> deduplication is automatic. For htmlservd, use `sha256` directory under `blobs/`. File path: `blobs/sha256/<full-hex-hash>`. No need for fan-out (e.g., `ab/cdef...`) at v1 scale.
- **Bundle format design:** A tar archive with a `manifest.json` at the root. The manifest lists every resource with its type, name, and sha256 hash. Files are stored in the tar at paths matching their resource type: `components/header.html`, `styles/default.css`, `assets/hero.jpg`, `pages/index.page.yaml`.
- **Hash verification:** Compute sha256 while reading each tar entry (streaming). Compare against manifest. If any mismatch, reject the entire bundle before writing blobs to disk.
- **Atomic file writes in Go:** Write to a temp file in the same directory (`<hash>.tmp`), then `os.Rename` to final path. This is atomic on POSIX filesystems. Check for existing blob before writing (skip if hash matches).

### Proposed Manifest Format

```json
{
  "apiVersion": "htmlctl.dev/v1",
  "kind": "Bundle",
  "mode": "partial",
  "website": "futurelab",
  "resources": [
    {
      "kind": "Component",
      "name": "pricing",
      "file": "components/pricing.html",
      "hash": "sha256:a1b2c3d4..."
    },
    {
      "kind": "StyleBundle",
      "name": "default",
      "files": [
        {"file": "styles/tokens.css", "hash": "sha256:e5f6..."},
        {"file": "styles/default.css", "hash": "sha256:7a8b..."}
      ]
    },
    {
      "kind": "Page",
      "name": "index",
      "file": "pages/index.page.yaml",
      "hash": "sha256:9c0d..."
    },
    {
      "kind": "Asset",
      "name": "hero.jpg",
      "file": "assets/hero.jpg",
      "hash": "sha256:1e2f...",
      "contentType": "image/jpeg",
      "size": 245760
    }
  ]
}
```

---

## Implementation Summary

- Added bundle parsing and validation in `internal/bundle/`:
  - `manifest.go` validates `mode`, resource schema, file paths, and sha256 hash format.
  - `bundle.go` reads tar streams, requires `manifest.json`, verifies referenced file hashes, and reports missing/mismatched files.
- Added content-addressed blob storage in `internal/blob/store.go`:
  - Stores blobs under `<data-dir>/blobs/sha256/<hash>`.
  - Deduplicates writes when blobs already exist.
- Added state merge logic in `internal/state/merge.go`:
  - Auto-creates website/environment rows.
  - Supports `partial` upsert behavior and `full` replace behavior (deletes resources not present in manifest).
  - Supports `dry_run` via transaction rollback.
- Added apply endpoint in `internal/server/apply.go`:
  - `POST /api/v1/websites/{website}/environments/{env}/apply`
  - Enforces 50MB request limit, validates bundle, serializes applies per website, and returns structured JSON.
- Extended DB query layer in `internal/db/queries.go`:
  - Added environment lookup, upsert/list/delete helpers for pages, components, style bundles, and assets.
- Added tests:
  - `internal/bundle/*_test.go` for manifest + tar/hash validation.
  - `internal/blob/store_test.go` for blob write/dedupe behavior.
  - `internal/server/apply_test.go` for success path, hash mismatch, dry-run no-persist, full-mode deletion, partial asset deletion, and hash normalization.
  - `internal/db/queries_test.go` for large keep-list deletion behavior (SQLite variable-limit safe path).

## Code Review Findings

- Ran `pi` review three times during implementation (`docs/review-logs/E2-S3-review-pi-*.log`) and addressed reported high-severity issues:
  - Fixed Asset/Script partial deletion identity mismatch by enforcing canonical `name == file` and requiring `file` on deleted Asset/Script resources.
  - Replaced unbounded per-environment lock map with bounded striped locks to avoid unbounded memory growth.
  - Expanded locking scope to website-level (not environment-level) to prevent cross-environment races on shared website resources.
  - Added missing partial deletion test coverage (`deleted: true` in partial mode).
  - Hardened website/environment auto-create against concurrent insert races by re-reading after insert conflict.
  - Added large keep-list deletion fallback to avoid SQLite variable-limit failures in full apply.
  - Normalized stored content hashes to canonical `sha256:<hex>` format.
  - Added deterministic MIME fallback mapping for common web asset extensions when host MIME database is missing.
  - Tightened manifest schema: `Component`/`Page`/`Asset`/`Script` resources must reference exactly one file entry.
- Remaining low-priority review notes were either accepted as intentional for v1 scope or are tracked for later stories (e.g., blob permission policy and blob GC lifecycle).

## Completion Status

- Implemented and validated with automated tests (`go test ./...` in Docker `golang:1.24`).
