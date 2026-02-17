# Domain Hardening Notes

This document tracks post-E5 hardening changes for domain operations.

## 1. Rollback Metadata Preservation

When `DELETE /api/v1/domains/{domain}` succeeds in DB but Caddy reload fails, rollback now restores:

- original `id`
- original `created_at`
- original `updated_at`

Implementation:

- `internal/db/queries.go`: `RestoreDomainBinding(...)`
- `internal/server/domains.go`: delete compensation path uses restore, not fresh insert

Regression coverage:

- `internal/db/queries_test.go`: `TestRestoreDomainBindingPreservesIdentity`
- `internal/server/domains_error_test.go`: metadata assertions after failed delete reload

## 2. Same-Domain Concurrency Control

Domain create/delete operations now serialize on a per-domain lock stripe so concurrent operations on the same domain cannot interleave.

Implementation:

- `internal/server/server.go`: `domainLockStripes`
- `internal/server/apply.go`: `domainLock(...)` and index hashing
- `internal/server/domains.go`: create/delete acquire domain lock

Regression coverage:

- `internal/server/domains_error_test.go`: `TestDomainsSameDomainDeleteCreateSerialized`

## 3. First-Deploy UX

`htmlctl apply` now emits an explicit first-deploy hint when `previousReleaseId` is absent:

- confirms first deploy completion
- points operator to next domain-binding command

Implementation:

- `internal/cli/apply_cmd.go`
- `internal/cli/apply_cmd_test.go`
