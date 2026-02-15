# E2-S2 - SQLite Schema

**Epic:** Epic 2 — Server daemon: state, releases, and API
**Status:** Not Started
**Priority:** P0 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E2-S1 (server bootstrap + config provides data directory and DB file path)
**Target:** v1
**Design Reference:** [Technical Spec - Sections 2, 5](../technical-spec.md)

---

## 1. Objective

Define and implement the SQLite database schema that stores all resource metadata, environment state, release records, and audit log entries for `htmlservd`. This schema is the single source of truth for the server's declarative state and is a prerequisite for bundle ingestion, release building, and audit logging.

## 2. User Story

As the htmlservd daemon, I need a well-structured, migration-managed SQLite database so that all resource metadata, environment state, release history, and audit entries are persisted reliably and queryable by every server subsystem.

## 3. Scope

### In Scope

- SQLite driver selection and integration (`modernc.org/sqlite` for CGO-free builds)
- Database connection management (single writer, WAL mode for concurrent reads)
- Schema definition for all core tables:
  - `websites` — website metadata and spec
  - `environments` — per-website environment instances (staging, prod)
  - `pages` — page definitions with route and layout
  - `components` — component metadata and content hash references
  - `style_bundles` — style bundle metadata and file hash references
  - `assets` — asset metadata and content-addressed hash references
  - `releases` — immutable release records per environment
  - `audit_log` — structured audit entries for all state-changing operations
- Schema migration framework (versioned, forward-only migrations)
- Initial migration (v1 schema creation)
- Database opening/initialization integrated with server startup (E2-S1)
- Indexes for common query patterns (by website, by environment, by release)
- Foreign key enforcement enabled

### Out of Scope

- Blob storage on filesystem (E2-S3 handles content-addressed blob files)
- Populating data via API (E2-S3 bundle ingestion)
- Release creation logic (E2-S4)
- Audit log writing logic (E2-S5)
- Database backup/restore tooling
- Multi-database or database replication

## 4. Architecture Alignment

- **Resource model (Tech Spec Section 2):** The schema directly maps the resource model: Website, Environment, Page, Component, StyleBundle, Asset, Release. Each resource type gets its own table with metadata fields matching the spec.
- **Storage layout (Tech Spec Section 5):** `db.sqlite` lives at `<data-dir>/db.sqlite`. Blobs are stored on the filesystem (not in SQLite) for efficiency. The database stores only metadata and content hashes that reference blob files.
- **Immutable releases (Tech Spec Section 2.7):** The `releases` table stores immutable snapshots. Once a release row is inserted, it is never updated (only the environment's `active_release_id` pointer changes).
- **Audit log (Tech Spec Section 7):** The `audit_log` table captures actor, timestamp, environment, resource change summary, and activated release ID.
- **Concurrency:** SQLite in WAL mode supports concurrent readers with a single writer. The server uses a single `*sql.DB` connection pool. Write operations that span multiple tables use transactions.
- **Implementation notes (Tech Spec Section 10):** Use `modernc.org/sqlite` (pure Go, no CGO) for portability. Alternative: `mattn/go-sqlite3` if CGO is acceptable.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/db/db.go` — Database open, WAL mode setup, foreign keys, connection pool config
- `internal/db/migrate.go` — Migration runner: version tracking table, ordered migration execution
- `internal/db/migrations/001_initial_schema.go` — Initial schema DDL (all tables, indexes, constraints)
- `internal/db/models.go` — Go structs for database rows (Website, Environment, Page, Component, StyleBundle, Asset, Release, AuditEntry)
- `internal/db/queries.go` — Basic CRUD query helpers (insert, get-by-id, list-by-website, etc.)

### 5.2 Files to Modify

- `internal/server/server.go` — Integrate DB initialization into server startup; pass `*sql.DB` to subsystems
- `internal/server/config.go` — Add database-specific config options (e.g., `db-path` override, WAL mode toggle)
- `go.mod` — Add `modernc.org/sqlite` dependency

### 5.3 Tests to Add

- `internal/db/db_test.go` — Database open/close, WAL mode verification, foreign key enforcement
- `internal/db/migrate_test.go` — Migration execution, idempotent re-run, version tracking
- `internal/db/migrations/001_initial_schema_test.go` — Schema creation, table existence, column types, constraints
- `internal/db/queries_test.go` — Insert and query each resource type; foreign key constraint violations; unique constraint enforcement

### 5.4 Dependencies/Config

- `modernc.org/sqlite` — Pure-Go SQLite driver (no CGO required)
- Standard library: `database/sql`, `embed` (for migration SQL if using embedded files)

## 6. Acceptance Criteria

- [ ] AC-1: On first startup, `htmlservd` creates `db.sqlite` in the configured data directory with WAL mode enabled
- [ ] AC-2: The `websites` table stores name, default style bundle reference, and base template
- [ ] AC-3: The `environments` table stores website reference, environment name (staging/prod), and active release ID (nullable FK to releases)
- [ ] AC-4: The `pages` table stores route, title, description, layout (JSON array of includes), and website reference
- [ ] AC-5: The `components` table stores name, scope, content hash (sha256), and website reference
- [ ] AC-6: The `style_bundles` table stores bundle name, file references with hashes, and website reference
- [ ] AC-7: The `assets` table stores original filename, content type, size, content hash (sha256), and website reference
- [ ] AC-8: The `releases` table stores release ID (ULID), environment reference, manifest snapshot (JSON), output hashes, build log, status, and created_at timestamp; rows are never updated after insertion
- [ ] AC-9: The `audit_log` table stores actor, timestamp, environment, operation type, resource change summary, and release ID reference
- [ ] AC-10: Foreign key constraints are enforced (e.g., deleting a website cascades or blocks based on policy)
- [ ] AC-11: A migration version table (`schema_migrations`) tracks applied migrations; re-running startup does not re-apply migrations
- [ ] AC-12: All tables have appropriate indexes for query patterns: lookup by website, by environment, by release, by timestamp
- [ ] AC-13: Concurrent read access works while a write transaction is in progress (WAL mode)

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: open database, verify WAL mode is active (`PRAGMA journal_mode`)
- [ ] Unit test: run initial migration, verify all tables exist with correct columns
- [ ] Unit test: insert a website, environment, page, component, style bundle, asset, release, and audit entry; query them back
- [ ] Unit test: verify foreign key constraint prevents orphaned environment (invalid website reference)
- [ ] Unit test: verify migration idempotency (run migrations twice, no error)
- [ ] Unit test: verify unique constraints (duplicate website name, duplicate route per website)
- [ ] Integration test: start server, verify DB file created and schema applied

### Manual Tests

- [ ] Start `htmlservd`, inspect `db.sqlite` with `sqlite3` CLI tool; verify tables and schema
- [ ] Restart `htmlservd` against existing `db.sqlite`; verify no migration errors
- [ ] Check WAL mode: `PRAGMA journal_mode;` returns `wal`

## 8. Performance / Reliability Considerations

- WAL mode provides significantly better concurrent read performance vs. default journal mode
- Single writer with transaction batching for multi-table operations
- Connection pool: max 1 writer connection, up to 4 reader connections
- Keep blob data on filesystem (not in SQLite) to avoid database bloat
- Index design targets common access patterns: list resources by website, get release by ID, query audit log by time range
- SQLite database should remain under 100MB for typical deployments (metadata only, no blobs)

## 9. Risks & Mitigations

- **Risk:** `modernc.org/sqlite` has different performance characteristics vs. `mattn/go-sqlite3` — **Mitigation:** Benchmark during implementation; the metadata-only workload is lightweight. Switch to `mattn/go-sqlite3` only if CGO is acceptable and performance requires it.
- **Risk:** Schema changes in later stories break existing migrations — **Mitigation:** Forward-only migration strategy; never modify existing migrations, only add new ones.
- **Risk:** Concurrent writes cause `SQLITE_BUSY` errors — **Mitigation:** Use a single writer connection with retry logic and appropriate busy timeout (`_busy_timeout=5000`).
- **Risk:** Storing layout as JSON in the pages table loses relational queryability — **Mitigation:** Layout is always read as a whole; no need to query individual includes. JSON is sufficient.

## 10. Open Questions

- Should we use `database/sql` directly or use a lightweight query builder (e.g., `sqlc`, `squirrel`)? Recommendation: `database/sql` directly for v1 simplicity; evaluate `sqlc` if query volume grows.
- Should we embed migration SQL as Go strings or use `embed` with `.sql` files? Recommendation: Go functions with DDL strings for type safety and easier testing.
- Cascade delete policy: should deleting a website cascade-delete all environments/resources, or require explicit cleanup first? Recommendation: require explicit cleanup (RESTRICT) for safety.

## 11. Research Notes

- **Go SQLite drivers:**
  - `modernc.org/sqlite` — Pure Go, no CGO. Slightly slower for heavy writes but perfectly adequate for metadata workloads. Excellent portability (cross-compile easily).
  - `mattn/go-sqlite3` — CGO-based, faster for bulk operations, requires C compiler. Less portable.
  - Recommendation: `modernc.org/sqlite` for v1 (portability wins; metadata workload is light).
- **Schema migration patterns:** Keep it simple: a `schema_migrations` table with a `version` integer column. Each migration is a Go function. On startup, run all migrations with version > current. No rollback migrations in v1 (forward-only).
- **SQLite best practices for concurrent access:**
  - Enable WAL mode: `PRAGMA journal_mode=WAL;`
  - Set busy timeout: `PRAGMA busy_timeout=5000;`
  - Enable foreign keys: `PRAGMA foreign_keys=ON;`
  - Use a connection pool with `SetMaxOpenConns(5)` (1 writer + readers)
  - For writes, serialize through a single goroutine or mutex-guarded write path.

### Proposed DDL (summary)

```sql
CREATE TABLE websites (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    default_style_bundle TEXT NOT NULL DEFAULT 'default',
    base_template TEXT NOT NULL DEFAULT 'standard',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE environments (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id        INTEGER NOT NULL REFERENCES websites(id),
    name              TEXT NOT NULL, -- 'staging' or 'prod'
    active_release_id TEXT,          -- ULID, nullable (no release yet)
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(website_id, name)
);

CREATE TABLE pages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id  INTEGER NOT NULL REFERENCES websites(id),
    name        TEXT NOT NULL,
    route       TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    layout_json TEXT NOT NULL DEFAULT '[]',  -- JSON array of includes
    content_hash TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(website_id, name),
    UNIQUE(website_id, route)
);

CREATE TABLE components (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id   INTEGER NOT NULL REFERENCES websites(id),
    name         TEXT NOT NULL,
    scope        TEXT NOT NULL DEFAULT 'global',
    content_hash TEXT NOT NULL,  -- sha256 of HTML content (references blob)
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(website_id, name)
);

CREATE TABLE style_bundles (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id  INTEGER NOT NULL REFERENCES websites(id),
    name        TEXT NOT NULL,
    files_json  TEXT NOT NULL DEFAULT '[]',  -- JSON array of {filename, hash}
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(website_id, name)
);

CREATE TABLE assets (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id   INTEGER NOT NULL REFERENCES websites(id),
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    content_hash TEXT NOT NULL,  -- sha256 (references blob)
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(website_id, filename)
);

CREATE TABLE releases (
    id              TEXT PRIMARY KEY,  -- ULID
    environment_id  INTEGER NOT NULL REFERENCES environments(id),
    manifest_json   TEXT NOT NULL,     -- snapshot of all resource manifests
    output_hashes   TEXT NOT NULL DEFAULT '{}',  -- JSON map of output file -> hash
    build_log       TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'building', -- building, active, superseded, failed
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
    -- NOTE: rows are never updated after insertion (immutable releases)
    --       status is set once at creation time or finalization
);

CREATE TABLE audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    actor           TEXT NOT NULL DEFAULT 'system',
    timestamp       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    environment_id  INTEGER REFERENCES environments(id),
    operation       TEXT NOT NULL,  -- apply, promote, rollback, activate, etc.
    resource_summary TEXT NOT NULL DEFAULT '',  -- human-readable change summary with hashes
    release_id      TEXT,           -- ULID of release involved, if any
    metadata_json   TEXT NOT NULL DEFAULT '{}'  -- additional structured data
);
```

---

## Implementation Summary

- Implemented SQLite integration in `internal/db` using `modernc.org/sqlite` (CGO-free):
  - `internal/db/db.go` for DB open/config, WAL/foreign-key PRAGMAs, and helper checks.
  - `internal/db/migrate.go` for forward-only migration runner and `schema_migrations` tracking.
  - `internal/db/migrations/001_initial_schema.go` for v1 schema (websites, environments, pages, components, style_bundles, assets, releases, audit_log, indexes).
  - `internal/db/models.go` and `internal/db/queries.go` for row models and CRUD helpers.
- Integrated DB initialization and migrations into server startup:
  - `internal/server/server.go` now opens DB, runs migrations, and closes DB on shutdown.
  - `internal/server/config.go` now supports `dbPath` and `dbWAL` (+ env overrides `HTMLSERVD_DB_PATH`, `HTMLSERVD_DB_WAL`).
- Added and executed tests:
  - `internal/db/db_test.go`
  - `internal/db/migrate_test.go`
  - `internal/db/migrations/001_initial_schema_test.go`
  - `internal/db/queries_test.go`
  - extended server tests for DB startup integration.

## Code Review Findings

- No blocking defects found in the implemented E2-S2 scope.
- Follow-up considerations for later stories:
  - Add explicit write-path transaction boundaries where multi-table invariants matter (E2-S3/E2-S4).
  - Expand query layer as API handlers land (pagination/filtering for releases/audit log).

## Completion Status

- Implemented and validated with automated tests (`go test ./...`).
