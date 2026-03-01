# E9-S1 — Environment Backend Data Model

**Epic:** Epic 9 — Environment Backends
**Status:** Planned
**Priority:** P1 (Critical Path — foundation for all E9 stories)
**Estimated Effort:** 1–2 days
**Dependencies:** E2-S2 (SQLite schema), E5-S1 (DomainBinding resource as architectural pattern)
**Target:** `internal/db/`, `internal/backend/`
**Design Reference:** Architectural review 2026-03-01

---

## 1. Objective

Introduce a first-class environment-scoped backend mapping that associates a URL path matcher with an upstream service address. This is the persistence and validation foundation for Epic 9: staging and prod can declare different backend routes for the same static site without changing the promoted release artifact.

## 2. User Story

As an operator, I want to declare that requests to `/api/*` on my staging environment should be forwarded to `https://api-staging.example.com`, and requests on prod forwarded to `https://api.example.com`, so that my static site can use a single relative `/api/` path in its JavaScript regardless of environment.

## 3. Background and Motivation

htmlctl currently treats environments as a combination of:

- release-backed static site content, which is promoted byte-for-byte between environments
- environment-scoped control-plane configuration, such as domain bindings, which is managed separately from releases

Environment backends belong in the second category. They are operational routing state, not site bundle state:

- they must not be part of `htmlctl apply` bundle resources
- they must not be embedded in release manifests or output hashes
- they must not be rebuilt or re-derived during promote

That architectural seam is already established by domain bindings. Environment backends should follow the same pattern: persisted in SQLite, resolved by server-side control-plane code, and consumed by Caddy config generation.

## 4. Scope

### In Scope

- New `environment_backends` SQLite table (migration 006).
- `BackendRow` DB model in `internal/db/models.go`.
- Dedicated backend validation helpers in `internal/backend/`.
- DB queries:
  - `UpsertBackend`
  - `ListBackendsByEnvironment`
  - `DeleteBackendByPathPrefix`
- Strict validation and normalization rules for:
  - backend path matcher syntax
  - upstream URL syntax
- Unit tests for DB queries, migration behaviour, and backend validation helpers.

### Out of Scope

- Caddy config generation (E9-S2).
- HTTP API handlers and CLI commands (E9-S3).
- Declarative backend resources in `website.yaml` or `htmlctl apply`.
- Authentication directives or auth policy composition (`forward_auth`, `basicauth`) — future story.
- Upstream health checks, load balancing, or request/response rewriting.

## 5. Product and Architecture Alignment

- **Environment-scoped control plane:** backends are operational configuration, like domain bindings, not bundle content. This preserves the product model of static releases plus environment-level runtime configuration.
- **Promotion invariant:** promotion remains artifact promotion only. Backend rows are not copied, hashed, or rebuilt as part of release promotion.
- **No new model layer:** backends do not belong in `pkg/model`, which represents locally parsed site bundle resources. Epic 9 is intentionally server-managed and imperative (`htmlctl backend ...` in E9-S3), so the implementation should mirror `internal/domain` and DB-backed server state instead.
- **Deterministic downstream consumption:** E9-S2 depends on stable ordering and unambiguous path matcher semantics. This story must define those rules now rather than leaving Caddy matching behaviour to later interpretation.

## 6. Data Model

### 6.1 New Table — `environment_backends`

```sql
CREATE TABLE IF NOT EXISTS environment_backends (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path_prefix TEXT NOT NULL,
    upstream TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(environment_id, path_prefix)
);
CREATE INDEX IF NOT EXISTS idx_environment_backends_environment_id
    ON environment_backends(environment_id);
```

`ON DELETE CASCADE` is correct: if an environment is removed, its backend configuration is removed with it.

### 6.2 `internal/db/models.go` Addition

```go
type BackendRow struct {
    ID            int64
    EnvironmentID int64
    PathPrefix    string
    Upstream      string
    CreatedAt     string
    UpdatedAt     string
}
```

No `Backend` type should be added to `pkg/model/types.go`. Backends are not site bundle resources and are not parsed from local bundle input.

## 7. Validation and Normalization Contract

Validation should live in a dedicated `internal/backend` package so the same logic can be reused by:

- E9-S2, when generating Caddy config from stored rows
- E9-S3, when validating API input

Do not overload `internal/names`, which is intentionally limited to resource-name validation.

### 7.1 Path Prefix Semantics

For v1, backend path matchers must use one canonical syntax:

- `/<segment>/*` or `/<segment>/<subsegment>/*` for prefix routing

Examples of valid values:

- `/api/*`
- `/api/v1/*`
- `/auth/callback/*`

Examples of invalid values:

- ``
- `/`
- `api/*`
- `/api/`
- `/api`
- `/api/**`
- `/api?x=1`
- `/api/#fragment`
- `/../api/*`

Rules:

- must start with `/`
- maximum length 256 characters
- must not contain `..`
- must not contain query strings or fragments
- must end with `/*`
- must not contain empty path segments
- normalization is conservative: trim surrounding whitespace only; do not silently rewrite `/api/` to `/api/*`

The last rule matters. The API should reject ambiguous forms rather than guessing operator intent.

This is intentionally prefix-only. Exact-path matching and full catch-all routing are out of scope for v1. That keeps the feature aligned with the product vision: static files remain the default handler, and backends are additive path-based integrations rather than a general request-routing language.

### 7.2 Upstream URL Semantics

Upstream validation should also live in `internal/backend`.

Rules:

- must be an absolute `http` or `https` URL
- host is required
- embedded credentials are forbidden
- query string and fragment must be empty
- normalization is conservative: trim surrounding whitespace only; do not rewrite scheme, host, or trailing slash

Examples of valid values:

- `https://api.example.com`
- `http://localhost:8080`
- `https://auth.internal.example.com/base`

Examples of invalid values:

- `api.example.com`
- `ftp://api.example.com`
- `https://user:pass@example.com`
- `https://api.example.com?debug=1`
- `https://api.example.com#frag`

## 8. Implementation Plan

### 8.1 Files to Create

- `internal/backend/validate.go` — backend-specific normalization and validation helpers.
- `internal/backend/validate_test.go` — table-driven tests for path-prefix and upstream validation.
- `internal/db/migrations/006_environment_backends.go` — migration SQL following the existing migration pattern.
- `internal/db/migrations/006_environment_backends_test.go` — migration test following the existing migration test pattern.

### 8.2 Files to Modify

- `internal/db/models.go` — add `BackendRow`.
- `internal/db/queries.go` — add backend CRUD helpers.
- `internal/db/queries_test.go` — add backend query tests.
- `internal/db/migrations/001_initial_schema.go` — register migration 006 in `All()`.

### 8.3 Query Shape

```go
func (q *Queries) UpsertBackend(ctx context.Context, in BackendRow) error
func (q *Queries) ListBackendsByEnvironment(ctx context.Context, environmentID int64) ([]BackendRow, error)
func (q *Queries) DeleteBackendByPathPrefix(ctx context.Context, environmentID int64, pathPrefix string) (bool, error)
```

`DeleteBackendByPathPrefix` returning `(bool, error)` is intentional. It mirrors the existing domain-binding deletion pattern: `true` when a row was deleted, `false` when no matching row exists.

### 8.4 Query Requirements

- `UpsertBackend` uses `ON CONFLICT(environment_id, path_prefix) DO UPDATE`.
- Upsert updates `upstream` and `updated_at`.
- `ListBackendsByEnvironment` must order by `path_prefix` ascending for deterministic downstream config generation.
- Queries must enumerate columns explicitly rather than using `SELECT *`.

## 9. Acceptance Criteria

- [ ] AC-1: Migration 006 creates the `environment_backends` table and index with the schema above; `RunMigrations` remains idempotent.
- [ ] AC-2: `UpsertBackend` inserts a new row and, on conflict, updates `upstream` and `updated_at`.
- [ ] AC-3: `ListBackendsByEnvironment` returns all backends for an environment ordered by `path_prefix` ascending.
- [ ] AC-4: `DeleteBackendByPathPrefix` deletes the matching row and returns `true`; it returns `false` without error when the row does not exist.
- [ ] AC-5: `internal/backend` validation rejects invalid path prefixes, including missing leading slash, ambiguous `/api/`-style forms, `..`, query/fragment content, and values longer than 256 characters.
- [ ] AC-6: `internal/backend` validation accepts valid canonical prefixes, including `/api/*` and `/api/v1/*`.
- [ ] AC-7: upstream validation rejects non-absolute URLs, unsupported schemes, embedded credentials, and URLs with query strings or fragments.
- [ ] AC-8: upstream validation accepts valid absolute `http` and `https` URLs without mutating their meaning.
- [ ] AC-9: `go test -race ./internal/db/... ./internal/backend/...` passes.
- [ ] AC-10: Existing migration tests and unrelated DB tests continue to pass with no regressions.

## 10. Tests to Add

- `internal/backend/validate_test.go`:
  - Accept `/api/*` and `/api/v1/*`.
  - Reject empty string.
  - Reject `/`.
  - Reject missing leading slash.
  - Reject `/api/` and `/api`.
  - Reject `..`, query strings, fragments, and overlong values.
  - Accept valid `http` and `https` upstream URLs.
  - Reject relative URLs, unsupported schemes, credentials, query strings, and fragments.
- `internal/db/queries_test.go`:
  - Upsert inserts new backend row.
  - Upsert updates upstream on conflict.
  - List returns rows ordered by `path_prefix`.
  - Delete removes existing row and returns `true`.
  - Delete on non-existent row returns `false`, no error.
  - List on environment with no backends returns an empty slice.
- `internal/db/migrations/006_environment_backends_test.go`:
  - Migration applies cleanly.
  - Table exists with the correct columns.
  - `UNIQUE(environment_id, path_prefix)` is enforced.
  - `ON DELETE CASCADE` removes backends when an environment is deleted.

## 11. Risks and Mitigations

- **Risk:** ambiguous path syntax leaks into Caddy config generation and produces surprising match behaviour. **Mitigation:** enforce one canonical matcher syntax in this story and reject all ambiguous variants.
- **Risk:** backend validation logic becomes split across DB, server, and Caddy layers. **Mitigation:** centralize it in `internal/backend` and reuse it from E9-S2 and E9-S3.
- **Risk:** future work treats backends as release content and accidentally couples them to promote. **Mitigation:** keep them out of `pkg/model`, bundle manifests, and release hashes; document that invariant explicitly in this story and the follow-on stories.
- **Risk:** migration numbering drifts as Epic 8 website-head work lands first. **Mitigation:** reserve migration 006 here and keep migration references explicit in the story.

## 12. Implementation Readiness

This story is implementation-ready. Its persistence and validation contract is now explicit enough to build without further architectural discovery. Before E9-S2 and E9-S3 are implemented, their examples and validation references should be aligned to the same canonical path-prefix syntax and `internal/backend` helper ownership described here. The core architectural contract is:

- backends are DB-backed environment configuration
- validation lives in `internal/backend`
- no bundle-resource or promotion coupling is introduced
- downstream Caddy rendering can rely on deterministic, already-validated rows
