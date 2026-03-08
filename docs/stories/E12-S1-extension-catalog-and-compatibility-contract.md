# E12-S1 — Extension Catalog and Compatibility Contract

**Epic:** Epic 12 — Optional Service Extensions
**Status:** Implemented (2026-03-06)
**Priority:** P1
**Estimated Effort:** 2-3 days
**Dependencies:** E9-S3 (backend CLI), E11-S3 (remote diagnostics/version awareness)
**Target:** `docs/`, `extensions/` (new), release artifacts/docs
**Design Reference:** Post-E9 adoption feedback from FutureLab newsletter rollout (2026-03-05)

---

## 1. Summary

Define an official extension contract for `htmlctl` ecosystems so dynamic companion services (starting with newsletter) can be shipped as optional packages without bloating `htmlctl` core binaries.

## 2. Architecture Context and Reuse Guidance

- Keep `htmlctl` core focused on static deployment control-plane responsibilities; extension services stay outside `cmd/htmlctl` and `cmd/htmlservd`.
- Reuse existing environment-backend model (Epic 9) for runtime routing instead of inventing extension-specific proxy features.
- Reuse existing diagnostics patterns from E11 (`version`, `doctor`) where compatibility checks are needed.
- Treat extension metadata as documentation + packaging contract first; avoid introducing a plugin runtime in this epic.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Extension catalog layout

Create a top-level `extensions/` catalog with one directory per official extension:

- `extensions/<name>/extension.yaml` (metadata + compatibility)
- `extensions/<name>/README.md`
- `extensions/<name>/CHANGELOG.md`
- `extensions/<name>/ops/` (systemd/env/migration/runbooks)
- `extensions/<name>/examples/` (non-secret sample config)

### 3.2 Compatibility contract

`extension.yaml` must define at least:

- extension name and semantic version
- compatible `htmlctl` and `htmlservd` version range
- required external dependencies (for example PostgreSQL)
- required environment variables (secret vs non-secret)
- required backend path prefixes (for example `/newsletter/*`)
- health endpoint(s)

### 3.3 Security baseline for official extensions

Document mandatory invariants:

- loopback-only service listeners by default
- secrets only in server-side env files (never repo committed)
- explicit staging/prod isolation expectations
- required rate limiting for public endpoints
- sanitized error behavior for HTTP 5xx

## 4. File Touch List

### Files to Create

- `extensions/README.md`
- `extensions/schema/extension.schema.yaml`
- `docs/reference/extensions.md`
- `docs/guides/extensions-overview.md`

### Files to Modify

- `docs/epics.md` — add Epic 12 entries
- `docs/technical-spec.md` — add extension model boundary section
- `README.md` — mention optional extensions track

## 5. Implementation Steps

1. Define extension metadata schema and field semantics.
2. Add catalog documentation and contributor rules for official extensions.
3. Add compatibility policy and release expectations.
4. Add extension security baseline and operational guardrails.
5. Add example metadata file for one concrete extension (newsletter in E12-S2).

## 6. Tests and Validation

### Automated

- Schema validation test for `extension.yaml` examples.
- Lint/CI check that every official extension folder has required files.

### Manual

- Validate docs are sufficient for an operator to discover extension compatibility and required backend wiring without reading source code.

## 7. Acceptance Criteria

- [x] AC-1: `extensions/` catalog structure is documented and versioned in repo docs.
- [x] AC-2: A formal `extension.yaml` schema exists with compatibility and security-relevant fields.
- [x] AC-3: Extension docs explicitly state that extensions are optional and not part of `htmlctl`/`htmlservd` core runtime.
- [x] AC-4: Security baseline requirements are documented for all official extensions.
- [x] AC-5: The docs reference Epic 9 backend routing as the canonical integration mechanism for public paths.

## 8. Risks and Open Questions

### Risks

- **Risk:** extension concept drifts into an implicit plugin runtime inside `htmlservd`.  
  **Mitigation:** lock scope to packaging + contract only in E12.
- **Risk:** compatibility ambiguity causes failed operator rollouts.  
  **Mitigation:** require explicit version ranges and tested matrices per extension release.

### Open Questions

- Should extension metadata be consumed by a future `htmlctl extension` command, or remain docs-only in v1? (defer to post-E12 backlog)

## 9. Implementation Notes (2026-03-06)

- Added extension catalog and contract docs:
  - `extensions/README.md`
  - `extensions/schema/extension.schema.yaml`
  - `docs/reference/extensions.md`
  - `docs/guides/extensions-overview.md`
- Added first official extension metadata skeleton:
  - `extensions/newsletter/extension.yaml`
  - `extensions/newsletter/README.md`
  - `extensions/newsletter/CHANGELOG.md`
- Added manifest validation package and tests:
  - `internal/extensionspec/manifest.go`
  - `internal/extensionspec/manifest_test.go`
- Updated product docs/spec boundary:
  - `README.md`
  - `docs/README.md`
  - `docs/technical-spec.md`
- Re-validated in local Docker verification matrix (2026-03-06):
  - `go test ./internal/extensionspec` in `golang:1.24-bookworm` container passed
  - confirms extension manifest contract remains parseable/validated in a clean container environment

## 10. Hardening Update (2026-03-08)

- The original v1 contract shipped as a docs and packaging artifact only.
- Added active compatibility enforcement path:
  - `htmlctl extension validate <extension-dir-or-manifest>`
  - validates manifest structure plus `spec.compatibility.minHTMLCTL`
  - `--remote --context <ctx>` also validates `spec.compatibility.minHTMLSERVD` against the selected remote `htmlservd`
- Added semver comparison helpers in `internal/extensionspec` so compatibility checks are executable rather than advisory-only.
- Updated extension reference and operator docs to treat validation as the pre-routing gate before extension adoption or upgrade.
