# E2-S5 - Audit Log

**Epic:** Epic 2 — Server daemon: state, releases, and API
**Status:** Done
**Priority:** P1 (High)
**Estimated Effort:** 2 days
**Dependencies:** E2-S2 (audit_log table in SQLite schema), E2-S4 (release events to log)
**Target:** v1
**Design Reference:** [Technical Spec - Sections 7, 9.2](../technical-spec.md)

---

## 1. Objective

Implement structured audit logging for all state-changing operations in `htmlservd`. Every apply, release build, activation, rollback, and promotion must produce a durable audit entry capturing who did what, when, to which environment, and with what result. This provides the accountability and traceability required for a production-safe control plane.

## 2. User Story

As an operator, I want every state-changing operation recorded in a structured audit log so that I can review the history of changes to any environment, understand who made each change and what it contained, and diagnose issues after the fact using `htmlctl logs`.

## 3. Scope

### In Scope

- Audit logger service: a Go interface and implementation that writes structured entries to the `audit_log` table
- Audit entry fields:
  - `actor` — identity of the caller (SSH principal, user ID, or "system" for internal ops)
  - `timestamp` — RFC 3339 timestamp of the operation
  - `environment_id` — which environment was affected
  - `operation` — operation type enum: `apply`, `release.build`, `release.activate`, `rollback`, `promote`, `domain.add`, `domain.remove`
  - `resource_summary` — human-readable summary of what changed (resource names, content hashes)
  - `release_id` — ULID of the release involved, if applicable
  - `metadata_json` — additional structured data (e.g., apply mode, resource count, source info)
- Integration points: hook audit logging into existing operations:
  - Bundle ingestion (E2-S3): log after successful apply
  - Release builder (E2-S4): log after release build, log after activation
- HTTP API endpoint for retrieving audit logs: `GET /api/v1/websites/{website}/environments/{env}/logs`
  - Pagination: `?limit=N&offset=M` or cursor-based
  - Filtering: `?operation=apply`, `?since=<timestamp>`, `?until=<timestamp>`
  - Default: most recent entries first (descending timestamp)
- HTTP API endpoint for all-environment logs: `GET /api/v1/websites/{website}/logs`
- Structured JSON response format for log entries
- Log entry formatting for CLI consumption (used by `htmlctl logs`)

### Out of Scope

- Real-time log streaming / WebSocket subscription (post-v1)
- Log rotation or archival (SQLite handles growth; pruning is post-v1)
- External log forwarding (syslog, log aggregators) (post-v1)
- Authentication-based actor resolution (in v1, actor is passed as a header or defaults to "local")
- Audit entries for read-only operations (get, list, diff)
- CLI implementation of `htmlctl logs` (Epic 3, E3-S3)

## 4. Architecture Alignment

- **Audit log requirements (Tech Spec Section 7):** The spec mandates audit log records containing: actor identity, timestamp, environment, resource change summary (hashes), and release ID activated. This story implements all specified fields.
- **CLI design (Tech Spec Section 9.2):** `htmlctl logs` retrieves audit entries. This story provides the server-side API that the CLI will call.
- **Database schema (E2-S2):** The `audit_log` table is already defined in E2-S2. This story implements the Go service that writes to and reads from that table.
- **Concurrency:** Audit log writes are append-only and do not require the per-environment build lock. They can be written within the same database transaction as the operation they record, or immediately after.
- **Security model:** Audit entries are immutable once written. The API provides read-only access. As of E6-S1, API endpoints are protected by bearer-token middleware when `api.token` is configured.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/audit/logger.go` — `AuditLogger` interface and SQLite-backed implementation
- `internal/audit/entry.go` — `AuditEntry` struct, operation type constants, builder/helper methods
- `internal/audit/query.go` — Query helpers: list entries with filtering, pagination, ordering
- `internal/api/logs.go` — HTTP handlers for `GET .../logs` endpoints

### 5.2 Files to Modify

- `internal/api/apply.go` — Add audit log call after successful bundle ingestion
- `internal/api/release.go` — Add audit log calls after release build and activation
- `internal/api/routes.go` — Register logs endpoint routes
- `internal/server/server.go` — Initialize `AuditLogger`, inject into API handlers
- `internal/db/queries.go` — Add insert and query functions for audit_log table (if not already in the logger package)

### 5.3 Tests to Add

- `internal/audit/logger_test.go` — Write entries, read them back, verify all fields persisted correctly
- `internal/audit/query_test.go` — Filtering by operation, time range, environment; pagination; ordering
- `internal/api/logs_test.go` — HTTP endpoint: list logs, filter by operation, paginate, verify JSON response format
- Integration tests in `internal/api/apply_test.go` and `internal/api/release_test.go` — Verify audit entries created after apply and release operations

### 5.4 Dependencies/Config

- No new external dependencies (uses `database/sql`, `encoding/json`, `time` from stdlib)
- Configuration: optional `audit.max-query-limit` setting (default 1000 entries per query)

## 6. Acceptance Criteria

- [ ] AC-1: Every successful `apply` operation produces an audit entry with operation `apply`, the actor, environment, and a resource summary listing changed resource names and hashes
- [ ] AC-2: Every successful release build produces an audit entry with operation `release.build` and the release ULID
- [ ] AC-3: Every release activation produces an audit entry with operation `release.activate`, the release ULID, and the previous active release ULID (if any)
- [ ] AC-4: Audit entries include all required fields: actor, timestamp, environment_id, operation, resource_summary, release_id, metadata_json
- [ ] AC-5: `GET /api/v1/websites/{website}/environments/{env}/logs` returns audit entries for that environment in reverse chronological order as a JSON array
- [ ] AC-6: `GET /api/v1/websites/{website}/logs` returns audit entries across all environments for that website
- [ ] AC-7: The `?limit=N&offset=M` query parameters control pagination; default limit is 50
- [ ] AC-8: The `?operation=<type>` query parameter filters entries by operation type
- [ ] AC-9: The `?since=<timestamp>&until=<timestamp>` query parameters filter entries by time range (RFC 3339 format)
- [ ] AC-10: Audit entries are immutable: once written, they cannot be updated or deleted via the API
- [ ] AC-11: The `AuditLogger` interface is clean and injectable, allowing other subsystems to log audit events with a single method call
- [ ] AC-12: If audit logging fails (e.g., database error), the primary operation still succeeds but the error is logged to the server's operational log

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: create an `AuditLogger`, write an entry, read it back, verify all fields match
- [ ] Unit test: write multiple entries, query with limit/offset, verify correct pagination
- [ ] Unit test: query with operation filter, verify only matching entries returned
- [ ] Unit test: query with time range filter, verify correct entries returned
- [ ] Unit test: verify entries are ordered by timestamp descending
- [ ] Integration test: perform an apply via the API, query the logs endpoint, verify an `apply` audit entry exists
- [ ] Integration test: build a release via the API, query logs, verify `release.build` and `release.activate` entries exist
- [ ] Integration test: verify logs endpoint returns correct JSON structure with all fields

### Manual Tests

- [ ] Apply a sample site, then `curl .../logs` to see the audit entry
- [ ] Build multiple releases, verify each appears in the log with correct release IDs
- [ ] Filter logs by operation type, verify correct filtering
- [ ] Test pagination with many entries

## 8. Performance / Reliability Considerations

- Audit log writes are append-only INSERTs, which are fast in SQLite (especially in WAL mode)
- Audit logging should not slow down the critical path of apply/release operations. Target: < 5ms per audit write.
- Query performance: index on `(environment_id, timestamp)` and `(operation, timestamp)` ensures fast filtered queries
- Maximum query limit (1000 entries) prevents excessive memory usage on large log tables
- For typical deployments, the audit log table will contain thousands of entries, not millions. No special partitioning needed in v1.

## 9. Risks & Mitigations

- **Risk:** Audit log table grows unbounded — **Mitigation:** Acceptable for v1 (metadata-only rows are small, ~500 bytes each). Add pruning/archival in a future story.
- **Risk:** Audit write failure silently drops entries — **Mitigation:** Log audit write failures to the operational log (`slog.Error`). Do not fail the primary operation. Consider a retry mechanism in the future.
- **Risk:** Actor identity could be spoofed if caller identity is trusted before authentication — **Mitigation:** As of E6-S1, `X-Actor` is only trusted after auth middleware validates the request; unauthenticated requests cannot write audit entries.
- **Risk:** Clock skew causes out-of-order timestamps — **Mitigation:** Use server-local time consistently. ULIDs on releases already provide ordering; audit timestamps are supplementary.

## 10. Open Questions

- Should audit entries be written within the same database transaction as the operation, or in a separate transaction? Recommendation: same transaction when possible (ensures consistency); separate transaction for release activation (which involves symlink operations outside the DB).
- Should the `resource_summary` be a structured JSON object or a human-readable string? Recommendation: human-readable string for v1 (easier to display in CLI); `metadata_json` carries structured data for programmatic access.
- Should there be a `GET /api/v1/audit` endpoint for global (all-website) audit queries? Recommendation: not in v1; per-website is sufficient.
- How should actor identity work with SSH tunnels? Recommendation: the CLI sets `X-Actor`; the server only trusts it after API auth middleware validates the bearer token.

## 11. Research Notes

- **Structured logging patterns:** The audit log is distinct from operational logging (`slog`). Operational logs go to stderr for the operator/systemd; audit logs go to the database for querying by the CLI. Both should be structured (JSON), but they serve different purposes.
- **Audit trail best practices:**
  - Append-only: never update or delete audit entries.
  - Include before/after state (or hashes) for change operations.
  - Timestamps should be UTC in ISO 8601 / RFC 3339 format.
  - Actor identity should be captured at the API boundary, not deep in business logic.
  - Pagination is essential for large audit logs; cursor-based pagination is more robust than offset-based for concurrent appends, but offset is simpler for v1.
- **API design for log retrieval:**
  ```
  GET /api/v1/websites/futurelab/environments/staging/logs?limit=20&operation=apply

  Response:
  {
    "entries": [
      {
        "id": 42,
        "actor": "local",
        "timestamp": "2026-02-15T10:30:00Z",
        "environment": "staging",
        "operation": "apply",
        "resourceSummary": "Updated components/pricing (sha256:a1b2...), styles/default.css (sha256:c3d4...)",
        "releaseId": null,
        "metadata": {"mode": "partial", "resourceCount": 2}
      }
    ],
    "total": 156,
    "limit": 20,
    "offset": 0
  }
  ```
- **Go interface pattern:** Define `AuditLogger` as a minimal interface so it can be mocked in tests and swapped for different backends in the future:
  ```go
  type AuditLogger interface {
      Log(ctx context.Context, entry AuditEntry) error
      Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, int, error)
  }
  ```

---

## Implementation Summary

- Added `internal/audit/` subsystem:
  - `entry.go` defines audit entry/filter/result contracts and operation constants.
  - `logger.go` implements SQLite-backed insert/query logic with filtering (`operation`, `since`, `until`) and pagination (`limit`, `offset`).
  - `async.go` adds bounded async write buffering, graceful close, idle-wait support, and concurrency-safe shutdown behavior.
- Integrated audit writes into state-changing APIs:
  - `internal/server/apply.go` writes `apply` audit entries (non-dry-run) with actor, resource summary, and metadata.
  - `internal/server/release.go` writes `release.build` and `release.activate` entries with release IDs.
  - Actor is sourced from `X-Actor` (default: `local`), and is only trusted after authentication middleware passes (E6-S1).
- Added log retrieval APIs in `internal/server/logs.go`:
  - `GET /api/v1/websites/{website}/environments/{env}/logs`
  - `GET /api/v1/websites/{website}/logs`
  - Supports `limit`, `offset`, `operation`, `since`, and `until`.
- Added/extended tests:
  - `internal/audit/logger_test.go`
  - `internal/server/logs_test.go`
  - Existing apply/release endpoint tests exercise audit integration paths.

## Code Review Findings

- Ran `pi` review for E2-S5 (`docs/review-logs/E2-S5-review-pi-*.log`) and fixed reported high-severity issues:
  - Fixed async logger shutdown race (`send on closed channel`) with explicit lock-guarded close/send semantics.
  - Fixed timestamp ordering risk by storing fixed-width UTC timestamp strings for lexicographic sort stability.
- Additional design notes accepted for v1 scope:
  - Audit events are environment-scoped in current schema/API (no website-global audit rows without `environment_id`).
  - Async mode is intentionally best-effort under sustained DB contention (drops are surfaced in server logs; primary operations still succeed).

## Completion Status

- Implemented and verified with automated tests (`go test ./...` in Docker `golang:1.24`).
