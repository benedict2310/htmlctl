# E6-S2 - Input Name Validation

**Epic:** Epic 6 — Security Hardening
**Status:** Done
**Priority:** P0 (Critical — path traversal + Caddyfile injection)
**Estimated Effort:** 2 days
**Dependencies:** E2-S3 (bundle ingestion), E5-S2 (Caddy config generation)
**Target:** Linux server (htmlservd)
**Design Reference:** Security Audit 2026-02-20, Vulns 7, 8 & 9

---

## 1. Objective

Website names, environment names, component names, and page names are accepted from API callers and stored in the database, then later used to construct filesystem paths and Caddyfile configuration. None of these names are currently validated beyond a non-empty check, allowing `..` path traversal sequences and Caddyfile metacharacters (newlines, `{`, `}`) to flow through to file writes and config generation.

This story enforces a strict `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` allowlist on all resource names at every API entry point, closing path traversal and config injection vectors in a single, composable validation layer.

## 2. User Story

As a server operator, I want htmlservd to reject resource names that contain path separators, whitespace, or special characters so that no client-supplied name can escape the intended data directory or inject directives into the Caddy configuration.

## 3. Scope

### In Scope

- A shared `validateResourceName(name string) error` function enforcing `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` (1–128 characters, starts with alphanumeric, allows hyphens and underscores).
- Validation applied at all API parse points:
  - `parseApplyPath` — website name + environment name.
  - `parseReleasePath`, `parseRollbackPath`, `parsePromotePath` — website name + environment name.
  - `handleCreateDomain` — domain name already validated by `domain.Normalize()`; verify coverage.
- Validation applied at bundle ingestion (state merge) for component names and page names stored in the database (`internal/state/merge.go`).
- Secondary assertion in `internal/caddy/config.go` `GenerateConfigWithOptions`: if any assembled root path contains `\n`, `{`, or `}`, return an error before writing the Caddyfile (defence-in-depth; primary defence is input validation above).
- Unit tests for `validateResourceName` covering valid names, names with `..`, `/`, newlines, `{`, `}`, empty string, names exceeding max length.
- Integration test: apply request with traversal-style names returns `400 Bad Request`.

### Out of Scope

- Renaming existing resources stored in the database (migration of pre-existing invalid names is out of scope; invalid names in DB will surface as errors at build time).
- Validating domain names (already handled by `domain.Normalize()`).
- Changes to the blob hash validation (already enforced by regex in `blob/store.go`).

## 4. Architecture Alignment

- **Single point of enforcement:** A shared `validateResourceName` function in a new `internal/server/validate.go` (or inline in the parse helpers) ensures consistent enforcement across all handlers.
- **Defence in depth:** The Caddyfile generator adds a secondary check so that even a name that somehow bypasses the primary validation cannot produce a malformed config file.
- **Filesystem safety:** `filepath.Join` normalises `..` components, which means a name like `../../etc` that passes through `filepath.Join` can escape the data root. The allowlist prevents `..` entirely because dots are not in the allowed character set (only alphanumerics, hyphens, underscores).
- **Component/page names in state merge:** `internal/state/merge.go` currently applies only `TrimSpace` before storing component and page names. Adding `validateResourceName` here ensures names in the database are always safe to use in `filepath.Join` inside `materializeSource`.
- **PRD references:** Technical Spec Section 5 (bundle ingestion), Section 6 (release builder filesystem layout).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/server/validate.go` — `validateResourceName(name string) error` with the regex and length check; exported for use in state merge as well.

### 5.2 Files to Modify

- `internal/server/apply.go` — call `validateResourceName` for website and environment name in `parseApplyPath`; return `400` with a descriptive message on failure.
- `internal/server/release.go` — same validation in `parseReleasePath`.
- `internal/server/promote.go` — same validation in `parsePromotePath`.
- `internal/server/rollback.go` — same validation in `parseRollbackPath`.
- `internal/state/merge.go` — call `validateResourceName` for each component name and page name before `InsertComponent` / `InsertPage`; return error to caller if invalid.
- `internal/caddy/config.go` — in `GenerateConfigWithOptions`, after assembling `root`, check `strings.ContainsAny(root, "\n{}")` and return an error if true.
- `internal/release/builder.go` — add a `sanitizeResourceName` call before `filepath.Join` in `materializeSource` for component (`row.Name`) and page (`row.Name`) paths as a belt-and-suspenders guard (mirrors existing `sanitizeRelPath` used for asset filenames).

### 5.3 Tests to Add

- `internal/server/validate_test.go`
  - Valid: `my-site`, `site1`, `production`, `staging-v2`, `a`.
  - Invalid: empty string, `..`, `../etc`, `foo/bar`, `foo bar`, `foo\nbar`, `foo{bar}`, 129-character name, name starting with `-`.
- `internal/server/apply_test.go` — HTTP tests: website name `../../etc` → 400; env name `prod\nreverse_proxy` → 400; valid names → proceed normally.
- `internal/state/merge_test.go` (or new test) — component name `../evil` rejected; page name with newline rejected.
- `internal/caddy/config_test.go` — `GenerateConfigWithOptions` with a root containing `\n` returns an error and does not write a file.

### 5.4 Dependencies / Config

- No new Go dependencies.
- No config changes — validation is always enabled.

## 6. Acceptance Criteria

- [x] AC-1: `POST /api/v1/websites/{w}/environments/{e}/apply` returns `400 Bad Request` when `{w}` or `{e}` contains `.`, `/`, `\n`, `{`, `}`, or any character outside `[a-zA-Z0-9_-]`.
- [x] AC-2: The same validation is enforced consistently across all path-parse helpers (`parseReleasePath`, `parsePromotePath`, `parseRollbackPath`).
- [x] AC-3: Bundle apply requests whose manifests contain component names or page names with path-unsafe characters are rejected at the state-merge layer with a descriptive error.
- [x] AC-4: `GenerateConfigWithOptions` returns an error (without writing the file) if any assembled root path contains a newline, `{`, or `}`.
- [x] AC-5: `materializeSource` in the release builder uses validated names for component and page file paths, mirroring the existing `sanitizeRelPath` pattern used for asset filenames.
- [x] AC-6: All existing tests continue to pass (existing test names are already safe).
- [x] AC-7: `validateResourceName` is the single canonical implementation; no ad-hoc string checks are duplicated across handlers.

## 7. Verification Plan

### Automated Tests

- [x] `validateResourceName` unit tests: all valid/invalid cases pass.
- [x] Handler-level tests: traversal-style website and env names return 400.
- [x] State merge tests: invalid component/page names are rejected before DB write.
- [x] Caddy config test: newline in root path is caught.

### Manual Tests

- [ ] `curl -X POST http://127.0.0.1:9400/api/v1/websites/..%2F..%2Fetc/environments/passwd/apply` — expect 400 with a name-validation error message.
- [ ] Apply a bundle with a manifest containing `"name": "../evil"` for a component — expect apply to fail with a descriptive error.
- [ ] Apply a bundle with valid names — expect normal success path unaffected.

## 8. Performance / Reliability Considerations

- Regex compilation is done once at package init via `regexp.MustCompile`; matching is O(n) on name length with a hard 128-character cap.
- The additional validation adds negligible latency to API requests.

## 9. Risks & Mitigations

- **Risk:** Legitimate existing deployments use names with characters outside the allowlist (e.g., dots in environment names like `v1.2`). **Mitigation:** Audit existing names before enforcing; if needed, expand the allowlist to include dots with the constraint that `..` is still rejected (or require at least one non-dot character between dots).
- **Risk:** The secondary Caddyfile check in `GenerateConfigWithOptions` could cause a reload to fail if a DB entry was stored before validation was added. **Mitigation:** Add a migration or startup check that logs a warning for any stored names that would not pass the new validation.

## 10. Open Questions

- Should dots be allowed in names (e.g., for versioned environment names like `v1.0`)? Initial proposal: no, to keep `..` impossible without needing lookahead rules. Revisit if operator feedback requires it.
- Max name length: 128 characters proposed. Confirm against any existing UI or CLI length constraints.

---

## 11. Implementation Summary

- Added canonical resource-name validation in `internal/names/validate.go` and server wrapper in `internal/server/validate.go`.
- Applied validation to API path parse points and payload fields:
  - `internal/server/apply.go`
  - `internal/server/release.go`
  - `internal/server/rollback.go`
  - `internal/server/promote.go`
  - `internal/server/domains.go`
- Enforced component/page name validation in state merge (`internal/state/merge.go`).
- Added release builder guard for component/page filenames in `internal/release/builder.go` via `sanitizeResourceName`.
- Added Caddy root-character defense in `internal/caddy/config.go` (`\n`, `{`, `}` rejected).
- Added tests:
  - `internal/server/validate_test.go`
  - `internal/server/apply_test.go`
  - `internal/server/path_parse_test.go`
  - `internal/server/promote_test.go`
  - `internal/server/domains_error_test.go`
  - `internal/state/merge_test.go`
  - `internal/release/builder_test.go`
  - `internal/caddy/config_test.go`
- Verification run: `go test ./...` (pass).
