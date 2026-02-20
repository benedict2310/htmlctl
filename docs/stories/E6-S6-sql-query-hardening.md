# E6-S6 - SQL Query Helper Hardening

**Epic:** Epic 6 — Security Hardening
**Status:** Pending
**Priority:** P1 (High — latent SQL injection in internal helpers)
**Estimated Effort:** 0.5 days
**Dependencies:** E2-S2 (SQLite schema)
**Target:** htmlservd (database layer)
**Design Reference:** Security Audit 2026-02-20, Vuln 10

---

## 1. Objective

Three private database helper functions in `internal/db/queries.go` construct SQL statements using `fmt.Sprintf` to interpolate `table` and `column` name arguments directly into query strings. No allowlist or parameterization guards these structural identifiers. All current call sites pass hard-coded literal strings, so there is no live injection today — but the helpers form a reusable internal API with no runtime protection, one refactoring step away from a SQL injection vulnerability.

This story adds a compile-time-visible allowlist inside each helper so that any future call with a variable or user-influenced argument fails fast with a clear error rather than executing attacker-controlled SQL.

## 2. User Story

As a maintainer, I want any attempt to call the internal SQL helper functions with a table or column name that is not in an explicit allowlist to return an error immediately, so that future refactoring cannot accidentally introduce SQL injection by passing a variable string to these helpers.

## 3. Scope

### In Scope

- Add a `validTableColumns` allowlist map inside `queries.go` (or a package-level `var`) that enumerates every permitted `(table, column)` pair used by the three helpers.
- At the top of each helper (`deleteByWebsiteNotIn`, `deleteByWebsiteSetDifference`, `deleteByWebsiteAndKey`), check the `table` and `column` arguments against the allowlist and return `fmt.Errorf("invalid table/column: %q/%q", table, column)` if either is not allowed.
- Update the helpers' doc comments to document the allowlist requirement.
- Add unit tests for the allowlist: valid pairs pass; arbitrary strings return errors.

### Out of Scope

- Rewriting the helpers to use typed methods per entity (that would be a larger refactor; this story adds a runtime guard as the minimum safe fix).
- Changes to any other query methods (all other methods in `queries.go` already use `?` parameterization for all values).
- Changes to migration SQL (static DDL, not user-influenced).

## 4. Architecture Alignment

- **Why not just use typed methods?** Typed per-entity methods would be the ideal long-term solution and would eliminate the helpers entirely. However, that refactor touches multiple call sites and risks introducing regressions during a security-focused sprint. The allowlist guard is a minimal, low-risk fix that closes the vulnerability without changing the calling convention.
- **Allowlist as documentation:** The allowlist doubles as explicit documentation of which `(table, column)` combinations the helpers are intended to support, making future additions visible in code review.
- **Parameterization note:** SQL structural identifiers (`table` and `column` names) cannot be passed as `?` bind parameters in standard SQL — only value parameters can. The allowlist is therefore the correct mitigation for structural injection, not `?` parameterization.
- **PRD references:** Technical Spec Section 5 (database design).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- None — all changes are in existing files.

### 5.2 Files to Modify

- `internal/db/queries.go`
  - Add a package-level `var allowedTableColumns = map[string]map[string]bool{ ... }` covering all valid `(table → set of allowed columns)` pairs for the three helpers. Based on current callers:
    - `pages`: `name`
    - `components`: `name`
    - `scripts`: `name` (if applicable — audit all call sites)
    - `assets`: `filename` (if applicable — audit all call sites)
  - In `deleteByWebsiteNotIn(ctx, table, column, ...)`: check `allowedTableColumns[table][column]` before constructing the query; return error if not found.
  - In `deleteByWebsiteSetDifference(ctx, table, column, ...)`: same check.
  - In `deleteByWebsiteAndKey(ctx, table, column, ...)`: same check.
  - Update doc comments for all three helpers to reference the allowlist.

### 5.3 Tests to Add

- `internal/db/queries_test.go`
  - `deleteByWebsiteNotIn` with a valid `(table, column)` pair: executes without error (existing tests cover this).
  - `deleteByWebsiteNotIn` with an invalid table (`"sqlite_master"`): returns error immediately without executing SQL.
  - `deleteByWebsiteNotIn` with valid table but invalid column (`"1=1; DROP TABLE pages; --"`): returns error immediately.
  - Same two error cases for `deleteByWebsiteSetDifference` and `deleteByWebsiteAndKey`.

### 5.4 Dependencies / Config

- No new dependencies.
- No config changes.

## 6. Acceptance Criteria

- [ ] AC-1: Each of the three SQL helper functions checks `table` and `column` against an explicit allowlist at the start of the function body, before any `fmt.Sprintf` call.
- [ ] AC-2: Any call with a table or column not in the allowlist returns a descriptive error without executing any SQL.
- [ ] AC-3: All existing callers continue to work (they all use allowlisted literal strings).
- [ ] AC-4: Unit tests confirm that invalid table and column names return errors; valid pairs execute normally.
- [ ] AC-5: The allowlist is defined once (not duplicated per helper) and is clearly readable in a code review.

## 7. Verification Plan

### Automated Tests

- [ ] Existing query tests pass unchanged.
- [ ] New tests: invalid `table` and `column` values return errors without touching the database.

### Manual Tests

- [ ] Code review: confirm `fmt.Sprintf` calls for SQL construction only occur after the allowlist check passes.
- [ ] `make test` passes without regression.

## 8. Performance / Reliability Considerations

- Map lookup on a small static map is O(1) and adds negligible overhead to database operations.

## 9. Risks & Mitigations

- **Risk:** A legitimate future caller needs a new `(table, column)` pair that is not in the initial allowlist. **Mitigation:** The allowlist is easy to extend; the compile-visible map makes additions explicit and reviewable. Document the allowlist extension process in the helpers' doc comment.
- **Risk:** The allowlist is defined incorrectly and blocks a legitimate call site, causing a runtime error in production. **Mitigation:** The unit tests verify all current call sites against the allowlist; any missing entry would cause a test failure before the change ships.

## 10. Open Questions

- Should the helpers be deprecated in favour of typed per-entity methods in a follow-up refactor story? Yes — log as a future tech-debt item. The allowlist fix is the immediate security gate; typed methods are the clean long-term solution.
