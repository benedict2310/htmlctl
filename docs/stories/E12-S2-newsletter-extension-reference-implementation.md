# E12-S2 — Newsletter Extension Reference Implementation

**Epic:** Epic 12 — Optional Service Extensions
**Status:** Implemented (2026-03-06)
**Priority:** P1
**Estimated Effort:** 3-4 days
**Dependencies:** E12-S1
**Target:** `extensions/newsletter/` (new)
**Design Reference:** FutureLab `P4-S2` foundation implementation (2026-03-05)

---

## 1. Summary

Create an official `htmlctl-newsletter` reference extension package that implements newsletter foundation capabilities as a standalone service, reusable by any `htmlctl` operator.

## 2. Architecture Context and Reuse Guidance

- Reuse the proven FutureLab service foundation shape (loopback listeners, env-driven config, systemd units, PostgreSQL migrations).
- Keep all newsletter runtime code in extension scope (`extensions/newsletter/`), not in `internal/server` or `cmd/htmlservd`.
- Reuse Epic 9 backends for public routing (`/newsletter/*`), not new gateway mechanisms.
- Keep compatibility boundaries explicit: extension depends on `htmlservd` networking/routing, but `htmlservd` does not depend on extension internals.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Extension service package

Provide a deployable service with:

- `serve` command with `/healthz`
- `migrate` command for schema setup
- migrations for `subscribers`, `verification_tokens`, `campaigns`, `campaign_sends`
- strict loopback bind validation

### 3.2 Extension metadata and examples

Ship `extension.yaml` declaring:

- required env vars
- required database setup
- expected backend path prefixes
- min compatible `htmlctl/htmlservd` versions

### 3.3 Security-by-default behavior

Enforce and document:

- loopback-only addresses
- no plaintext secret output in logs
- separate staging/prod credentials and DBs
- token hash storage (no raw token persistence)

## 4. File Touch List

### Files to Create

- `extensions/newsletter/extension.yaml`
- `extensions/newsletter/README.md`
- `extensions/newsletter/service/` (Go module)
- `extensions/newsletter/service/cmd/htmlctl-newsletter/main.go`
- `extensions/newsletter/service/internal/...`
- `extensions/newsletter/service/internal/migrate/sql/001_foundation.sql`

### Files to Modify

- `docs/reference/extensions.md` — add newsletter entry
- `docs/guides/extensions-overview.md` — add newsletter quickstart

## 5. Implementation Steps

1. Copy and normalize FutureLab foundation service into `extensions/newsletter/service`.
2. Create `extension.yaml` metadata with compatibility range and required integration points.
3. Add tests for config validation and `/healthz` behavior.
4. Add migration tests for idempotent schema setup.
5. Add packaging instructions for release artifact generation.

## 6. Tests and Validation

### Automated

- `go test ./...` under `extensions/newsletter/service`
- config tests reject non-loopback bind addresses
- migration tests verify table set and idempotency

### Manual

- local run with PostgreSQL + loopback listeners
- verify `/healthz` on staging/prod sample configs

## 7. Acceptance Criteria

- [x] AC-1: `extensions/newsletter` exists as a standalone extension package with metadata and docs.
- [x] AC-2: The reference service can run in staging/prod mode with loopback-only listeners and separate DB URLs.
- [x] AC-3: Foundation schema migrations apply cleanly and idempotently.
- [x] AC-4: Health endpoint returns `200 OK`.
- [x] AC-5: No code changes are required in `cmd/htmlctl` or `cmd/htmlservd` to run the extension.

## 8. Risks and Open Questions

### Risks

- **Risk:** extension code diverges from real-world adopter needs.  
  **Mitigation:** keep extension minimal, validated by pilot adoption (E12-S4).
- **Risk:** accidental coupling to one site's assumptions.  
  **Mitigation:** remove site-specific hostnames from defaults and force explicit env values where needed.

### Open Questions

- Should reference extension releases be built from this monorepo or mirrored to a dedicated `htmlctl-newsletter` repo later?

## 9. Implementation Notes (2026-03-06)

- Added newsletter reference service module under `extensions/newsletter/service`:
  - `cmd/htmlctl-newsletter/main.go`
  - `internal/config/config.go`
  - `internal/migrate/migrate.go`
  - `internal/migrate/sql/001_foundation.sql`
  - `internal/server/server.go`
- Generalized FutureLab service foundation into extension scope:
  - renamed binary/command to `htmlctl-newsletter`
  - removed site-specific defaults
  - required explicit `NEWSLETTER_PUBLIC_BASE_URL` and `NEWSLETTER_RESEND_API_KEY`
  - enforced loopback listener validation
- Added tests:
  - config validation tests for loopback/public URL/env requirements
  - migration tests for apply/skip behavior and required schema clauses
  - health and `/newsletter` + `/newsletter/*` handler tests
- Hardened migration runner for concurrent startups:
  - transaction-level migration claim via `INSERT ... ON CONFLICT DO NOTHING`
  - no race between migration existence check and migration recording
- Updated extension docs:
  - `extensions/newsletter/README.md`
  - `extensions/newsletter/CHANGELOG.md`
  - `docs/reference/extensions.md`
  - `docs/guides/extensions-overview.md`
- Verification evidence:
  - `go test ./...` in `extensions/newsletter/service`
  - `go test ./...` at repository root
  - independent `codex review` loops with all findings fixed
  - Docker E2E apply + domain routing check (`E2E_OK`)
  - Re-validated (2026-03-06):
    - local PostgreSQL container + host-loopback `htmlctl-newsletter serve`
    - `/healthz` returned `200`
    - routed `/newsletter/verify` via htmlservd backend returned expected `501` placeholder
