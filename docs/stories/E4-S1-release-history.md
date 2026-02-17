# E4-S1 - Release History

**Epic:** Epic 4 — Promotion and rollback
**Status:** Done
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E2-S4 (release builder creates releases), E3-S3 (remote command framework)
**Target:** v1
**Design Reference:** PRD sections 5.4, 9; Technical spec sections 2.7, 4, 5, 9.2

---

## 1. Objective

Operators and agents need visibility into the release history for any environment to make informed decisions about rollback and promotion. This story adds the `htmlctl rollout history` command and the corresponding server API endpoint so users can list all releases for an environment with their status, timestamp, actor, and ID.

## 2. User Story

As an operator or AI agent, I want to view the history of releases deployed to an environment so that I can identify which release is active, review past deployments, and make informed rollback or promotion decisions.

## 3. Scope

### In Scope

- Server API endpoint: `GET /api/v1/websites/{name}/environments/{env}/releases` returning a list of releases
- Each release entry includes: release ID (ULID), timestamp, actor, environment, status (`active` or `previous`)
- Releases returned in reverse chronological order (newest first), leveraging ULID natural sort order
- CLI command: `htmlctl rollout history website/<name> --context <ctx>`
- Tabular output format for the CLI (human-readable table with columns: ID, timestamp, actor, status)
- Pagination support in the API (limit/offset query params) with a sensible default (e.g., 20)

### Out of Scope

- Detailed release contents/diff (future: `rollout inspect`)
- Deleting or pruning old releases (future story)
- Filtering releases by date range or actor (post-MVP enhancement)
- Release comparison between environments (handled by promote story)

## 4. Architecture Alignment

- **Server component (htmlservd):** New HTTP handler registered on the existing `net/http` mux. Queries the SQLite `releases` table (created by E2-S4) joined with audit log entries (E2-S5) for actor information.
- **CLI component (htmlctl):** New `rollout history` subcommand using the remote command framework from E3-S3. Sends HTTP GET to the server and formats the response as a table.
- **Storage:** Reads from `releases` table in SQLite. The `current` symlink is inspected to determine which release is `active`. All other releases get status `previous`.
- **Concurrency:** Read-only operation; no locking required beyond SQLite's default read concurrency.
- **Security:** Follows existing control plane security model (localhost binding, SSH tunnel access per technical spec section 7).

### References
- PRD section 5.4 (rollback prod journey), section 9 (acceptance criteria: audit log records all applies/promotions/rollbacks)
- Technical spec section 2.7 (Release: ULID, manifests, rendered output, hashes)
- Technical spec section 4 (Release pipeline: atomic activation, rollback)
- Technical spec section 5 (Storage layout: releases directory, `current` symlink)
- Technical spec section 9.2 (`htmlctl rollout history website/futurelab --context prod`)

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/server/handler_releases.go` — HTTP handler for `GET /api/v1/websites/{name}/envs/{env}/releases`
- `internal/server/handler_releases_test.go` — Unit tests for the releases handler
- `internal/cli/cmd/rollout_history.go` — CLI command implementation for `htmlctl rollout history`
- `internal/cli/cmd/rollout_history_test.go` — Unit tests for the CLI command
- `internal/model/release_list.go` — Response model for release list API (if not already defined in E2-S4)

### 5.2 Files to Modify

- `internal/server/routes.go` — Register the new releases endpoint
- `internal/cli/cmd/rollout.go` — Register `history` as a subcommand of `rollout`
- `internal/store/release_store.go` — Add `ListReleases(websiteName, envName string, limit, offset int) ([]Release, error)` query method
- `internal/store/release_store_test.go` — Tests for the new query method

### 5.3 Tests to Add

- `internal/store/release_store_test.go` — Test `ListReleases` returns correct order, pagination, and active status detection
- `internal/server/handler_releases_test.go` — Test HTTP response format, pagination params, 404 for unknown website/env
- `internal/cli/cmd/rollout_history_test.go` — Test table formatting, context resolution, error handling

### 5.4 Dependencies/Config

- Depends on SQLite schema from E2-S2 (releases table, audit_log table)
- Depends on release builder from E2-S4 (releases exist to list)
- Depends on remote command framework from E3-S3 (SSH tunnel + HTTP transport)
- Uses `github.com/oklog/ulid/v2` for ULID parsing and display

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl rollout history website/<name> --context <ctx>` outputs a table of releases for the specified environment
- [ ] AC-2: Each row displays: release ID (ULID), timestamp (human-readable), actor, and status (`active` or `previous`)
- [ ] AC-3: The active release (matching the `current` symlink target) is marked with status `active`
- [ ] AC-4: Releases are listed in reverse chronological order (newest first)
- [ ] AC-5: Server API endpoint `GET /api/v1/websites/{name}/environments/{env}/releases` returns JSON array of release objects
- [ ] AC-6: API supports `limit` and `offset` query parameters with default limit of 20
- [ ] AC-7: API returns 404 with descriptive message for unknown website or environment
- [ ] AC-8: Command exits with non-zero code and descriptive error for invalid context or unreachable server

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: `ListReleases` store method returns releases in descending ULID order
- [ ] Unit test: `ListReleases` correctly identifies the active release via symlink inspection
- [ ] Unit test: Pagination with limit/offset works correctly
- [ ] Unit test: HTTP handler returns correct JSON structure
- [ ] Unit test: HTTP handler returns 404 for missing website/env
- [ ] Unit test: CLI formats output as aligned table with correct columns

### Manual Tests

- [ ] Deploy 3+ releases to a staging environment, run `htmlctl rollout history website/futurelab --context staging`, verify output shows all releases with correct active marker
- [ ] Verify ULID-based ordering matches chronological deployment order
- [ ] Verify command works over SSH tunnel to remote server

## 8. Performance / Reliability Considerations

- ULID sorting is lexicographic, which matches chronological order — no additional sort logic needed in SQLite (`ORDER BY id DESC`)
- Default pagination (limit 20) prevents unbounded result sets on environments with many releases
- Read-only operation; no risk of contention with concurrent apply/release operations
- SQLite index on `(website_name, env_name)` in releases table ensures fast lookups

## 9. Risks & Mitigations

- **Risk:** Actor information may not be stored with releases if audit log schema differs from expected. **Mitigation:** Join on audit_log table by release ID, or store actor directly in releases table during E2-S4 implementation.
- **Risk:** `current` symlink may be temporarily broken during an in-progress release. **Mitigation:** Handle `ErrNotExist` for symlink target gracefully; show "activating..." status or omit active marker.
- **Risk:** Large number of releases could slow queries. **Mitigation:** Pagination is built-in; add SQLite index on releases table.

## 10. Open Questions

- Should the table output include a truncated ULID for readability, or show the full 26-character ULID? (Recommendation: full ULID for copy-paste usability, with timestamp column for human scanning.)
- Should `--output json` flag be supported for machine-readable output in v1? (Recommendation: yes, aligns with kubectl patterns and agent workflows.)

## 11. Research Notes

- **kubectl rollout history pattern:** `kubectl rollout history deployment/nginx` shows revision number, change cause. htmlctl adapts this with ULID as revision ID and actor/timestamp as context.
- **ULID sorting:** ULIDs are lexicographically sortable by time, making `ORDER BY id DESC` equivalent to reverse chronological. No need for separate timestamp sort column.
- **Release listing UX:** kubectl uses a compact table; `htmlctl` should follow the same pattern with columns: `RELEASE ID | CREATED | ACTOR | STATUS`.

---

## Implementation Summary

Implemented release history end-to-end across server, client, CLI, and DB:
- Added `htmlctl rollout history website/<name>` in `internal/cli/rollout_cmd.go` with table/json/yaml output and `--limit`/`--offset`.
- Added paginated release history API handling in `internal/server/release.go`:
  - `GET /api/v1/websites/{name}/environments/{env}/releases`
  - default limit 20, capped at 200
  - `active`/`previous` status derivation and actor fallback handling.
- Added client pagination support in `internal/client/client.go`:
  - `ListReleasesPage(...)` for explicit pages
  - `ListReleases(...)` now iterates pages to return full history.
- Added DB query support in `internal/db/queries.go`:
  - `ListReleasesByEnvironmentPage(...)`
  - `ListLatestReleaseActors(...)` for actor enrichment.
- Added/updated tests:
  - `internal/server/release_history_test.go`
  - `internal/server/release_branch_test.go`
  - `internal/server/release_helpers_test.go`
  - `internal/client/client_test.go`
  - `internal/cli/rollout_cmd_test.go`
  - `internal/db/queries_test.go`.

## Code Review Findings

`pi` review logs:
- `docs/review-logs/E4-S1-review-pi-2026-02-16-213124.log`
- `docs/review-logs/E4-S1-review-pi-2026-02-16-213347.log`
- `docs/review-logs/E4-S1-review-pi-2026-02-16-213709.log`

Findings addressed:
- Included the new rollout command implementation file (`internal/cli/rollout_cmd.go`) so the root command registration compiles.
- Cleaned up unused query additions from early draft iterations.
- Verified pagination, actor lookup, and release status derivation behavior via focused tests.

## Completion Status

Implemented, tested, and reviewed. Epic 4 scoped coverage target was met (>85% on Epic 4 files), with this story’s server/client/CLI paths covered by dedicated tests.
