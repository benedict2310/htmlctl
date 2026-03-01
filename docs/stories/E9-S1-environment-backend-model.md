# E9-S1 — Environment Backend Data Model

**Epic:** Epic 9 — Environment Backends
**Status:** Planned
**Priority:** P1 (Critical Path — foundation for all E9 stories)
**Estimated Effort:** 1–2 days
**Dependencies:** E2-S2 (SQLite schema), E5-S1 (DomainBinding resource as schema pattern)
**Target:** `internal/db/`, `pkg/model/`, `internal/names/`
**Design Reference:** Architecture discussion 2026-03-01

---

## 1. Objective

Introduce a first-class `Backend` resource that associates a URL path prefix with an upstream service address, scoped to a specific environment. This is the data layer for environment-specific dynamic routing: staging and prod can declare different backends for the same path prefix without affecting each other's static content.

## 2. User Story

As an operator, I want to declare that requests to `/api/*` on my staging environment should be forwarded to `https://api-staging.example.com`, and requests on prod forwarded to `https://api.example.com`, so that my static site can use a single relative `/api/` path in its JavaScript regardless of environment.

## 3. Background and Motivation

htmlctl currently serves only static files. A common next step for operators is to add dynamic functionality — authentication, user data APIs, form submissions — without abandoning the static-first model. The correct architectural seam is environment-scoped reverse proxy configuration: the static content is identical across environments (the promotion invariant), but the *routing rules* differ per environment.

A `Backend` is not promoted — it is environment configuration, not release content. Operators manage backends independently from releases, exactly as they manage domain bindings.

## 4. Scope

### In Scope

- New `environment_backends` SQLite table (migration 005).
- `Backend` model type in `pkg/model/types.go`.
- `BackendRow` DB model in `internal/db/models.go`.
- DB queries: `UpsertBackend`, `ListBackendsByEnvironment`, `DeleteBackendByPathPrefix`.
- Validation:
  - `path_prefix` must start with `/`, must not contain `..`, max 256 chars.
  - `upstream` must be a valid absolute `http` or `https` URL with no embedded credentials.
- Unit tests for DB queries and validation logic.

### Out of Scope

- Caddy config generation (E9-S2).
- HTTP API handlers and CLI commands (E9-S3).
- Forward auth / authentication policies (future story).

## 5. Architecture Alignment

- **Pattern:** mirrors `domain_bindings` — a separate table, environment-scoped, not part of the release artifact, not promoted.
- **Uniqueness:** `(environment_id, path_prefix)` — one upstream per path per environment. Upsert semantics allow operators to update an upstream without removing and re-adding.
- **Validation location:** `internal/names` for path prefix (new `ValidateBackendPathPrefix` function). Upstream URL validation lives in `internal/server` alongside other apply-time checks.
- **No impact on promotion invariant:** backends are never included in release snapshots or output hashes. `promote` copies static artifacts only.

## 6. Data Model

### 6.1 New Table — `environment_backends`

```sql
CREATE TABLE IF NOT EXISTS environment_backends (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path_prefix TEXT NOT NULL,
    upstream    TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(environment_id, path_prefix)
);
CREATE INDEX IF NOT EXISTS idx_environment_backends_environment_id
    ON environment_backends(environment_id);
```

`ON DELETE CASCADE` is correct here: if an environment is deleted, its backends go with it.

### 6.2 `pkg/model/types.go` Addition

```go
// Backend declares a reverse-proxy upstream for a specific URL path prefix
// within an environment. It is environment configuration, not release content,
// and is never included in the promotion artifact.
type Backend struct {
    PathPrefix string `yaml:"pathPrefix" json:"pathPrefix"`
    Upstream   string `yaml:"upstream"   json:"upstream"`
}
```

### 6.3 `internal/db/models.go` Addition

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

## 7. Implementation Plan

### 7.1 Files to Create

- `internal/db/migrations/005_environment_backends.go` — migration SQL as a `const` following the existing pattern.

### 7.2 Files to Modify

- `pkg/model/types.go` — add `Backend` struct.
- `internal/db/models.go` — add `BackendRow`.
- `internal/db/queries.go` — add `UpsertBackend`, `ListBackendsByEnvironment`, `DeleteBackendByPathPrefix`.
- `internal/db/migrations/001_initial_schema.go` (`All()` slice) — register migration 005.
- `internal/db/queries_test.go` — new tests for backend queries.

### 7.3 New Queries

```go
func (q *Queries) UpsertBackend(ctx context.Context, in BackendRow) error
func (q *Queries) ListBackendsByEnvironment(ctx context.Context, environmentID int64) ([]BackendRow, error)
func (q *Queries) DeleteBackendByPathPrefix(ctx context.Context, environmentID int64, pathPrefix string) (bool, error)
```

Upsert uses `ON CONFLICT(environment_id, path_prefix) DO UPDATE SET upstream=excluded.upstream, updated_at=...`.

### 7.4 Validation

Add to `internal/names` (or a new `internal/validate` package if it grows):

```go
// ValidateBackendPathPrefix reports an error if pathPrefix is not a valid
// backend path prefix. It must start with '/', must not contain '..', and
// must be at most 256 characters.
func ValidateBackendPathPrefix(pathPrefix string) error
```

Upstream URL validation (absolute http/https, no credentials) lives in `internal/server` alongside other handler-level validation, not in `internal/names`.

## 8. Acceptance Criteria

- [ ] AC-1: Migration 005 creates the `environment_backends` table with the schema above; `RunMigrations` is idempotent.
- [ ] AC-2: `UpsertBackend` inserts a new row and, on conflict, updates `upstream` and `updated_at`.
- [ ] AC-3: `ListBackendsByEnvironment` returns all backends for an environment ordered by `path_prefix` (deterministic for Caddy config generation in E9-S2).
- [ ] AC-4: `DeleteBackendByPathPrefix` deletes the matching row and returns `true`; returns `false` (not an error) when the row does not exist.
- [ ] AC-5: `ValidateBackendPathPrefix` rejects strings that do not start with `/`, contain `..`, or exceed 256 characters; accepts valid prefixes like `/`, `/api/`, `/api/v1/*`.
- [ ] AC-6: `go test -race ./internal/db/... ./pkg/model/...` passes.
- [ ] AC-7: Existing migration tests and all other tests continue to pass — no regression.

## 9. Tests to Add

- `internal/db/queries_test.go`:
  - Upsert inserts new backend row.
  - Upsert updates upstream on conflict.
  - List returns rows ordered by `path_prefix`.
  - Delete removes existing row, returns `true`.
  - Delete on non-existent row returns `false`, no error.
  - List on environment with no backends returns empty slice.
- `internal/db/migrations/005_environment_backends_test.go` (follow existing migration test pattern):
  - Migration applies cleanly.
  - Table exists with correct columns after migration.
  - `UNIQUE(environment_id, path_prefix)` constraint is enforced.
  - `ON DELETE CASCADE` removes backends when environment is deleted.

## 10. Risks and Mitigations

- **Risk:** Future migration adds a column to `environment_backends`, breaking existing `SELECT *` queries. **Mitigation:** queries enumerate columns explicitly (consistent with existing pattern in `queries.go`).
- **Risk:** Operator sets `upstream` to a loopback address that is valid on the server but unreachable — silent misconfiguration. **Mitigation:** warn on apply (similar to `warnLocalhostMetadataURLs`); Caddy will return 502 in practice, which is visible.
