# E5-S1 - DomainBinding Resource

**Epic:** Epic 5 — Domains + TLS via Caddy
**Status:** Implemented
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E2-S2 (SQLite schema), E2-S1 (server framework)
**Target:** Linux server (htmlservd)
**Design Reference:** PRD Section 8 (Domains + TLS), Technical Spec Section 2.2

---

## 1. Objective

Introduce the DomainBinding resource so that htmlservd can map custom domain names to environments. This is the foundational data model for the entire Domains + TLS epic: without domain bindings persisted in the database, Caddy config generation, reload, and CLI commands have nothing to operate on.

## 2. User Story

As an operator, I want to bind a custom domain (e.g., `example.com`) to a specific environment (e.g., `prod`) so that Caddy can later be configured to serve that environment's content on that domain with automatic TLS.

## 3. Scope

### In Scope

- `DomainBinding` Go struct with fields: `ID`, `Domain`, `WebsiteName`, `EnvironmentName`, `CreatedAt`, `UpdatedAt`
- SQLite `domain_bindings` table with schema migration
- CRUD operations: Create, Read (single + list), Delete (re-bind is `remove` + `add` in v1)
- Server API endpoints: `POST /api/v1/domains`, `GET /api/v1/domains`, `GET /api/v1/domains/{domain}`, `DELETE /api/v1/domains/{domain}`
- Domain name validation:
  - Valid hostname format (RFC 1123)
  - No duplicate domain bindings
  - Prevent binding the same domain to multiple environments
- Unit tests for validation logic and database operations
- Integration tests for API endpoints

### Out of Scope

- Caddy config generation (E5-S2)
- Caddy reload (E5-S3)
- CLI commands for domain management (E5-S4)
- DNS verification / reachability checks
- Wildcard domain support
- SSL certificate management

## 4. Architecture Alignment

- **Component boundaries:** DomainBinding lives in the server (htmlservd) resource layer alongside Website, Environment, and Release resources. It follows the same pattern: Go struct -> SQLite persistence -> HTTP API.
- **Database:** Uses the existing SQLite database (`db.sqlite`) established in E2-S2. The `domain_bindings` table references `websites` and `environments` tables via foreign keys.
- **API pattern:** Follows the same `net/http` handler pattern established in E2-S1.
- **Concurrency:** Database writes use the existing SQLite connection with proper locking (single-writer model from E2-S2).
- **Audit logging:** Domain binding changes (create, update, delete) must be recorded in the audit log (E2-S5).
- **PRD references:** PRD Section 8 ("Domain binding per environment"), Technical Spec Section 2.2 (Environment resource includes "domain bindings").

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/model/domain_binding.go` — DomainBinding struct and validation methods
- `internal/store/domain_binding_store.go` — SQLite CRUD operations for domain_bindings table
- `internal/api/domain_handler.go` — HTTP handlers for domain binding CRUD
- `internal/model/domain_binding_test.go` — Unit tests for domain validation
- `internal/store/domain_binding_store_test.go` — Database operation tests
- `internal/api/domain_handler_test.go` — API integration tests

### 5.2 Files to Modify

- `internal/store/migrations.go` — Add `domain_bindings` table migration
- `internal/api/router.go` — Register domain binding routes
- `internal/store/store.go` — Add DomainBindingStore interface/initialization (if using a store registry pattern)

### 5.3 Tests to Add

- `internal/model/domain_binding_test.go`
  - Valid domain names accepted (e.g., `example.com`, `staging.example.com`, `my-site.example.com`)
  - Invalid domain names rejected (empty string, spaces, underscores in labels, trailing dots, IP addresses, too-long labels)
  - Domain normalization (lowercased, trimmed)
- `internal/store/domain_binding_store_test.go`
  - Create binding and retrieve it
  - List bindings (all, by website, by environment)
  - Delete binding
  - Duplicate domain rejected (UNIQUE constraint)
  - Foreign key validation (website/environment must exist)
- `internal/api/domain_handler_test.go`
  - POST creates binding and returns 201
  - POST with invalid domain returns 400 with error message
  - POST with duplicate domain returns 409
  - GET lists all bindings
  - GET single binding returns 200
  - GET non-existent binding returns 404
  - DELETE removes binding and returns 204
  - DELETE non-existent binding returns 404

### 5.4 Dependencies/Config

- No new Go dependencies required; domain validation uses standard library (`net` package, regex)
- SQLite migration added to existing migration list

## 6. Acceptance Criteria

- [x] AC-1: A `DomainBinding` resource can be created via `POST /api/v1/domains` with `domain`, `website`, and `environment` fields, and is persisted in the `domain_bindings` SQLite table.
- [x] AC-2: Domain names are validated against RFC 1123 hostname rules; invalid domains are rejected with a 400 response and descriptive error message.
- [x] AC-3: Duplicate domain bindings (same domain string) are rejected with a 409 Conflict response.
- [x] AC-4: All domain bindings can be listed via `GET /api/v1/domains`, optionally filtered by website or environment query parameters.
- [x] AC-5: A single domain binding can be retrieved via `GET /api/v1/domains/{domain}` returning 200, or 404 if not found.
- [x] AC-6: A domain binding can be deleted via `DELETE /api/v1/domains/{domain}` returning 204, or 404 if not found.
- [x] AC-7: Domain strings are normalized to lowercase before storage and comparison.
- [x] AC-8: Domain binding create and delete operations are recorded in the audit log.
- [x] AC-9: All unit and integration tests pass.

## 7. Verification Plan

### Automated Tests

- [x] Unit tests for domain name validation (valid/invalid patterns, normalization)
- [x] Unit tests for DomainBinding struct methods
- [x] Integration tests for SQLite store CRUD operations
- [x] Integration tests for HTTP API handlers (all CRUD endpoints, error cases)
- [x] Test that duplicate domain insertion returns appropriate error

### Manual Tests

- [ ] Start htmlservd, POST a domain binding via curl, verify it appears in GET response
- [ ] Attempt to POST an invalid domain (e.g., `not a domain!`) and verify 400 error
- [ ] Attempt to POST a duplicate domain and verify 409 error
- [ ] DELETE a domain binding and verify it no longer appears in list

## 8. Performance / Reliability Considerations

- Domain lookups should be indexed in SQLite (UNIQUE index on `domain` column) for fast lookup during Caddy config generation (E5-S2).
- The `domain_bindings` table is expected to be small (tens of rows at most), so no pagination is required for v1.
- Foreign key constraints ensure referential integrity (domain bindings cannot reference non-existent websites/environments).

## 9. Risks & Mitigations

- **Risk:** Domain validation edge cases (IDN/punycode domains, very long subdomains). **Mitigation:** Start with ASCII-only RFC 1123 validation; document that IDN support is post-v1.
- **Risk:** Orphaned domain bindings if an environment is deleted. **Mitigation:** Use `ON DELETE CASCADE` foreign key constraint, or validate environment existence on binding creation and flag orphans.
- **Risk:** Race condition if two concurrent requests try to bind the same domain. **Mitigation:** SQLite UNIQUE constraint on `domain` column provides atomic conflict detection.

## 10. Open Questions

- Should domain bindings support an optional `path_prefix` field for path-based routing (e.g., `example.com/blog` -> different environment)? Default: No for v1.
- Should we store the domain binding status (e.g., `pending`, `active`, `error`) to track whether Caddy has successfully picked up the binding? Useful for E5-S3/S4 but may be premature here.

---

## Implementation Summary

Implemented domain binding resource end-to-end:
- Added domain binding schema migration (`internal/db/migrations/002_domain_bindings.go`) and wired migration version 2 into migration bootstrap/tests.
- Added domain model/validation in `internal/domain/binding.go` (lowercasing + RFC 1123 hostname checks).
- Added DB models and CRUD/query methods for domain bindings in `internal/db/models.go` and `internal/db/queries.go`.
- Added HTTP API in `internal/server/domains.go` with routes:
  - `POST /api/v1/domains`
  - `GET /api/v1/domains`
  - `GET /api/v1/domains/{domain}`
  - `DELETE /api/v1/domains/{domain}`
- Added domain create/delete audit operations (`domain.add`, `domain.remove`) and wired audit writes.
- Added server/DB/domain tests for validation, CRUD, duplicate handling, filters, method/path branches, and audit branches.

## Code Review Findings

`pi` review logs:
- `docs/review-logs/E5-S1-review-pi-2026-02-17-181802.log` (final)

Final review verdict: **Ready**.

Notes from review:
- No P0/P1 issues.
- P2 observation: re-bind uses remove+add in v1 (not a dedicated update endpoint).

## Completion Status

Implemented, tested, and reviewed.
