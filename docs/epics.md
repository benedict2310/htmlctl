# htmlctl / htmlservd — Epic & Story Map

## Epic 1 — Foundations: Repo schema + local render

**Goal:** deterministic local render + serve.

| # | Story | File |
|---|-------|------|
| 1.1 | Define resource schemas and parser | [E1-S1](stories/E1-S1-resource-schemas-parser.md) |
| 1.2 | Implement deterministic renderer | [E1-S2](stories/E1-S2-deterministic-renderer.md) |
| 1.3 | Component validation engine | [E1-S3](stories/E1-S3-component-validation.md) |
| 1.4 | Local CLI commands (render + serve) | [E1-S4](stories/E1-S4-local-cli-commands.md) |

**Done when:** local `render` + `serve` works for sample `futurelab` site.

---

## Epic 2 — Server daemon: state, releases, and API

**Goal:** htmlservd stores desired state, creates releases, serves via filesystem.

| # | Story | File |
|---|-------|------|
| 2.1 | htmlservd bootstrap + config | [E2-S1](stories/E2-S1-server-bootstrap-config.md) |
| 2.2 | SQLite schema | [E2-S2](stories/E2-S2-sqlite-schema.md) |
| 2.3 | Bundle ingestion (apply) | [E2-S3](stories/E2-S3-bundle-ingestion.md) |
| 2.4 | Release builder | [E2-S4](stories/E2-S4-release-builder.md) |
| 2.5 | Audit log | [E2-S5](stories/E2-S5-audit-log.md) |

**Done when:** `htmlctl apply` creates a release, activates it, and logs it.

---

## Epic 3 — Remote transport + kubectl UX

**Goal:** htmlctl talks to htmlservd securely via SSH tunnel.

| # | Story | File |
|---|-------|------|
| 3.1 | Context config + selection | [E3-S1](stories/E3-S1-context-config.md) |
| 3.2 | SSH tunnel transport | [E3-S2](stories/E3-S2-ssh-tunnel-transport.md) |
| 3.3 | Core remote commands | [E3-S3](stories/E3-S3-core-remote-commands.md) |
| 3.4 | Diff engine | [E3-S4](stories/E3-S4-diff-engine.md) |

**Done when:** end-to-end remote apply/diff/logs works with contexts.

---

## Epic 4 — Promotion and rollback

**Goal:** staging->prod promote without rebuild; rollback support.

| # | Story | File |
|---|-------|------|
| 4.1 | Release history | [E4-S1](stories/E4-S1-release-history.md) |
| 4.2 | Rollback | [E4-S2](stories/E4-S2-rollback.md) |
| 4.3 | Promote (artifact promotion) | [E4-S3](stories/E4-S3-promote.md) |

**Done when:** staging changes promote to prod identically and rollback works.

---

## Epic 5 — Domains + TLS via Caddy

**Goal:** production domains and HTTPS.

| # | Story | File |
|---|-------|------|
| 5.1 | DomainBinding resource | [E5-S1](stories/E5-S1-domain-binding-resource.md) |
| 5.2 | Generate Caddy config snippets | [E5-S2](stories/E5-S2-caddy-config-generation.md) |
| 5.3 | Reload Caddy safely | [E5-S3](stories/E5-S3-caddy-reload.md) |
| 5.4 | `htmlctl domain add/verify` | [E5-S4](stories/E5-S4-domain-cli-commands.md) |

**Done when:** `futurelab.studio` serves prod over HTTPS and `staging.futurelab.studio` serves staging.

---

## Epic 6 — Security Hardening ✓

**Goal:** close all HIGH and MEDIUM findings from the 2026-02-20 security audit.
**Status:** Complete (2026-02-23) — all 16 findings remediated and verified.
**Post-epic audit:** [E6-post-epic-audit-2026-02-23](review-logs/E6-post-epic-audit-2026-02-23.log)

| # | Story | File |
|---|-------|------|
| 6.1 | API authentication layer (Implemented) | [E6-S1](stories/E6-S1-api-authentication.md) |
| 6.2 | Input name validation (path traversal + Caddyfile injection) (Implemented) | [E6-S2](stories/E6-S2-input-name-validation.md) |
| 6.3 | HTML XSS hardening (renderer + component validator) (Implemented) | [E6-S3](stories/E6-S3-html-xss-hardening.md) |
| 6.4 | SSH transport hardening (Implemented) | [E6-S4](stories/E6-S4-ssh-transport-hardening.md) |
| 6.5 | Container & entrypoint security hardening (Implemented) | [E6-S5](stories/E6-S5-container-security-hardening.md) |
| 6.6 | SQL query helper hardening (Implemented) | [E6-S6](stories/E6-S6-sql-query-hardening.md) |
| 6.7 | API error response sanitization (Implemented) | [E6-S7](stories/E6-S7-api-error-sanitization.md) |

**Done when:** all 16 audit findings (Vulns 1–16) are remediated and verified.

**Post-epic fix (2026-02-23):** Data race on `(*Server).listener` between `Start()`
and `Addr()` found during race-detector run and fixed in `internal/server/server.go`
by guarding the field with `sync.RWMutex`. All 428 tests pass clean under `-race`.

---

## Epic 7 — Metadata and Telemetry

**Goal:** add first-class SEO/share metadata rendering and optional site telemetry collection without external infrastructure.
**Status:** In progress (E7-S1 complete, E7-S2 planned)

| # | Story | File |
|---|-------|------|
| 7.1 | Server-rendered SEO and share-card metadata (Implemented) | [E7-S1](stories/E7-S1-server-rendered-seo-metadata.md) |
| 7.2 | Telemetry ingest endpoint for static sites | [E7-S2](stories/E7-S2-telemetry-ingest-endpoint.md) |

**Done when:** pages can emit crawler-visible metadata directly in HTML and operators can collect/query basic page telemetry via htmlservd.
