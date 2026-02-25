# E5-S2 - Generate Caddy Config Snippets

**Epic:** Epic 5 — Domains + TLS via Caddy
**Status:** Implemented
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E5-S1 (DomainBinding resource)
**Target:** Linux server (htmlservd)
**Design Reference:** PRD Section 8 (Domains + TLS), Technical Spec Sections 5 and 8

---

## 1. Objective

Generate valid Caddy configuration from DomainBinding resources so that each bound domain serves the correct environment's `current/` symlink directory over HTTPS with automatic TLS. This bridges the gap between the domain binding data model (E5-S1) and actually serving traffic: without config generation, domain bindings are inert database rows.

## 2. User Story

As an operator, I want htmlservd to automatically generate Caddy configuration from my domain bindings so that each domain serves the correct environment's content with HTTPS, without me having to manually write or maintain Caddyfile blocks.

## 3. Scope

### In Scope

- Go template for Caddy site blocks (Caddyfile format)
- Config generator that reads all DomainBinding records and produces a complete Caddyfile
- Each domain binding produces a site block with `file_server` serving from the environment's `current/` symlink path (e.g., `/var/lib/htmlservd/websites/sample/envs/prod/current`)
- Support multiple domains per environment (multiple site blocks pointing to same root)
- Automatic TLS via Caddy's built-in ACME (default behavior when domain is a public hostname)
- Output written to a configurable file path (e.g., `/etc/caddy/Caddyfile` or `/etc/caddy/conf.d/htmlservd.Caddyfile`)
- Config regeneration function callable from other components (E5-S3 will call it before reload)
- Unit tests for template rendering and config generation

### Out of Scope

- Caddy reload / apply (E5-S3)
- CLI commands (E5-S4)
- Caddy JSON API config format (use Caddyfile format for v1 simplicity)
- Custom TLS certificate configuration (rely on Caddy's automatic ACME)
- Rate limiting, caching headers, or advanced Caddy directives
- Reverse proxy configuration (file_server only for static sites)

## 4. Architecture Alignment

- **Component boundaries:** The config generator lives in the server (htmlservd) as a new `internal/caddy` package. It reads from the DomainBinding store (E5-S1) and writes to the filesystem. It does not interact with Caddy directly — that is E5-S3's responsibility.
- **Data flow:** `DomainBindingStore.List()` -> `caddy.GenerateConfig()` -> write Caddyfile to disk.
- **Templates:** Use Go's `text/template` (not `html/template`) since we are generating Caddy config, not HTML.
- **Filesystem:** The generated Caddyfile path is configured in htmlservd's config (E2-S1). Default: `/etc/caddy/Caddyfile`.
- **Determinism:** Config output must be deterministic — domain blocks sorted alphabetically by domain name so that repeated generation produces identical output when bindings have not changed.
- **PRD references:** PRD Section 8 ("htmlservd writes Caddy snippets"), Technical Spec Section 5 (storage layout showing `current` symlink paths).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/caddy/config.go` — Config generator: reads domain bindings, renders Caddyfile via template, writes to disk
- `internal/caddy/config_test.go` — Unit tests for config generation
- `internal/caddy/template.go` — Caddyfile template definition and helpers
- `internal/caddy/template_test.go` — Template rendering tests

### 5.2 Files to Modify

- `internal/config/config.go` — Add Caddy-related configuration fields (`CaddyfilePath`, `CaddyAdminAPI` for E5-S3)
- `cmd/htmlservd/main.go` — Initialize caddy config generator with store and config

### 5.3 Tests to Add

- `internal/caddy/config_test.go`
  - Single domain binding generates correct site block
  - Multiple domains for same environment generate separate site blocks with same root
  - Multiple domains across different environments generate correct roots
  - Zero domain bindings generates a valid but empty/minimal Caddyfile
  - Output is deterministic (sorted by domain)
  - Config file is written atomically (write to temp, rename)
- `internal/caddy/template_test.go`
  - Template renders valid Caddyfile syntax for a site block
  - Domain with special characters (hyphens, subdomains) renders correctly
  - File server root path uses environment `current` symlink

### 5.4 Dependencies/Config

- No new Go dependencies; uses `text/template` and `os` from standard library
- New config field in htmlservd config: `caddy.caddyfile_path` (default: `/etc/caddy/Caddyfile`)

## 6. Acceptance Criteria

- [x] AC-1: A `caddy.GenerateConfig()` function reads all DomainBinding records from the store and returns a complete Caddyfile as a string.
- [x] AC-2: Each domain binding produces a Caddyfile site block with the domain as the site address and a `file_server` directive with `root` set to the environment's `current/` symlink path (e.g., `/var/lib/htmlservd/websites/{website}/envs/{env}/current`).
- [x] AC-3: Multiple domains bound to the same environment produce separate site blocks, each pointing to the same `current/` directory.
- [x] AC-4: The generated Caddyfile is deterministic: given the same set of domain bindings, the output is byte-identical across invocations (domains sorted alphabetically).
- [x] AC-5: A `caddy.WriteConfig()` function writes the generated Caddyfile to the configured path atomically (write to temp file, then rename).
- [x] AC-6: When no domain bindings exist, the generator produces a valid minimal Caddyfile (empty or with only global options block).
- [x] AC-7: The Caddyfile path is configurable via htmlservd configuration with a sensible default (`/etc/caddy/Caddyfile`).
- [x] AC-8: All unit tests pass, covering single-domain, multi-domain, multi-environment, and zero-binding scenarios.

## 7. Verification Plan

### Automated Tests

- [x] Unit tests for template rendering with various domain/environment combinations
- [x] Unit tests for full config generation from mock DomainBinding store
- [x] Test deterministic output ordering
- [x] Test atomic file write (temp file + rename pattern)
- [x] Test config generation with zero bindings

### Manual Tests

- [ ] Create domain bindings in SQLite, run config generator, inspect output Caddyfile for correctness
- [ ] Validate generated Caddyfile with `caddy validate --config <path>` (requires Caddy installed)
- [ ] Verify `current/` symlink paths in generated config match actual filesystem layout

## 8. Performance / Reliability Considerations

- Config generation reads all domain bindings in a single query — expected to be fast for the small number of bindings in a typical deployment (< 100 rows).
- Atomic file write (write to temp, rename) prevents Caddy from reading a partially-written config file.
- The generator is stateless and idempotent: calling it multiple times with the same bindings produces the same file.

## 9. Risks & Mitigations

- **Risk:** Generated Caddyfile has syntax errors that prevent Caddy from loading. **Mitigation:** E5-S3 will validate before reloading, but we also add a test that runs `caddy validate` in CI if Caddy is available. Template is kept simple (file_server blocks only) to minimize syntax risk.
- **Risk:** File permissions on the generated Caddyfile prevent Caddy from reading it. **Mitigation:** Write with mode 0644; document that htmlservd needs write access to the Caddyfile path.
- **Risk:** `current/` symlink does not exist yet when config is generated (environment has no releases). **Mitigation:** Caddy will return 404 for that domain, which is acceptable. Document that domains should be bound after at least one release is deployed.

## 10. Open Questions

- Should the generated Caddyfile include a global options block (e.g., `email` for ACME registration)? If so, where does the email come from — htmlservd config or a separate setting?
- Should we generate a Caddy JSON config (via Caddy admin API) instead of a Caddyfile? Caddyfile is simpler for v1 but JSON API enables dynamic updates without file writes. Decision: Caddyfile for v1, consider JSON API post-MVP.
- Should the Caddyfile include `try_files` or `handle_errors` directives for SPA-style routing or custom 404 pages? Default: No for v1 (pure static file serving).

---

## Appendix: Example Generated Caddyfile

Given domain bindings:
- `example.com` -> website `sample`, environment `prod`
- `staging.example.com` -> website `sample`, environment `staging`

Generated output:

```
example.com {
	root * /var/lib/htmlservd/websites/sample/envs/prod/current
	file_server
}

staging.example.com {
	root * /var/lib/htmlservd/websites/sample/envs/staging/current
	file_server
}
```

---

## Implementation Summary

Implemented deterministic Caddyfile generation and atomic write support:
- Added `internal/caddy/config.go`:
  - `GenerateConfig([]Site)` for deterministic site block generation (sorted by domain).
  - `WriteConfig(path, content)` for atomic config writes via temp file + rename.
- Added `internal/server/caddy.go` to resolve domain bindings from DB and map them to Caddy roots under `<dataDir>/websites/<website>/envs/<env>/current`.
- Added `CaddyfilePath` config support in `internal/server/config.go` with env override `HTMLSERVD_CADDYFILE_PATH`.
- Added comprehensive tests in:
  - `internal/caddy/config_test.go`
  - `internal/server/caddy_test.go`
  - `internal/server/config_test.go`

## Code Review Findings

`pi` review logs:
- `docs/review-logs/E5-S2-review-pi-2026-02-17-182513.log` (final)

Final review verdict: **Merge**.

Notes from review:
- No P0/P1 issues.
- P2 observations: keep service permissions aligned with configured Caddyfile path and keep helper placement consistent with server package boundaries.

## Completion Status

Implemented, tested, and reviewed.
