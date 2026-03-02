# htmlctl / htmlservd — Epic & Story Map

## Epic 1 — Foundations: Repo schema + local render

**Goal:** deterministic local render + serve.

| # | Story | File |
|---|-------|------|
| 1.1 | Define resource schemas and parser | [E1-S1](stories/E1-S1-resource-schemas-parser.md) |
| 1.2 | Implement deterministic renderer | [E1-S2](stories/E1-S2-deterministic-renderer.md) |
| 1.3 | Component validation engine | [E1-S3](stories/E1-S3-component-validation.md) |
| 1.4 | Local CLI commands (render + serve) | [E1-S4](stories/E1-S4-local-cli-commands.md) |

**Done when:** local `render` + `serve` works for sample `sample` site.

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

**Done when:** `example.com` serves prod over HTTPS and `staging.example.com` serves staging.

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
**Status:** Complete (2026-02-23)

| # | Story | File |
|---|-------|------|
| 7.1 | Server-rendered SEO and share-card metadata (Implemented) | [E7-S1](stories/E7-S1-server-rendered-seo-metadata.md) |
| 7.2 | Telemetry ingest endpoint for static sites (Implemented) | [E7-S2](stories/E7-S2-telemetry-ingest-endpoint.md) |

**Done when:** pages can emit crawler-visible metadata directly in HTML and operators can collect/query basic page telemetry via htmlservd.

---

## Epic 8 — DX & Reliability

**Goal:** Fix developer-experience rough edges and reliability gaps surfaced during real-world use.

| # | Story | File |
|---|-------|------|
| 8.1 | SSH auth: fall back to key file when agent key is rejected | [E8-S1](stories/E8-S1-ssh-auth-agent-key-fallback.md) |
| 8.2 | Automatic OG image generation (Implemented) | [E8-S2](stories/E8-S2-og-image-generation.md) |
| 8.3 | Promote metadata host warnings (Implemented) | [E8-S3](stories/E8-S3-promote-metadata-host-warnings.md) |
| 8.4 | Website-scoped favicon support (Implemented) | [E8-S4](stories/E8-S4-website-favicon-support.md) |
| 8.5 | Declarative `robots.txt` generation (Implemented) | [E8-S5](stories/E8-S5-robots-txt-generation.md) |
| 8.6 | Automatic `sitemap.xml` generation (Implemented) | [E8-S6](stories/E8-S6-sitemap-xml-generation.md) |

---

## Epic 9 — Environment Backends

**Goal:** Allow operators to declare per-environment reverse-proxy upstreams so that static sites can integrate dynamic services (auth, APIs) via relative paths, with Caddy routing requests to the correct backend per environment.
**Status:** Complete (2026-03-01)

**Motivation:** Static content is identical across environments after promotion. What differs is routing: staging may proxy `/api/*` to a test service while prod proxies the same prefix to the real one. Backends are environment configuration — not release content — and are managed independently of the promotion flow.

| # | Story | File |
|---|-------|------|
| 9.1 | Environment backend data model + DB schema (Implemented) | [E9-S1](stories/E9-S1-environment-backend-model.md) |
| 9.2 | Backend-aware Caddy site block generation (Implemented) | [E9-S2](stories/E9-S2-backend-caddy-config.md) |
| 9.3 | Backend management API + CLI (`htmlctl backend add/list/remove`) (Implemented) | [E9-S3](stories/E9-S3-backend-api-and-cli.md) |

**Done when:** `htmlctl backend add website/futurelab --env prod --path /api/* --upstream https://api.example.com` proxies live traffic and `htmlctl backend list website/futurelab --env prod` shows the declared backends.

---

## Epic 10 — Review, Automation, and Lifecycle

**Goal:** Extend htmlctl from a static-site deployment control plane into a fuller publishing platform with draft review URLs, reproducible Git-driven deployment, visitor access control, storage lifecycle management, and richer component delivery.
**Status:** Planned

**Motivation:** Epics 1-9 established deterministic rendering, safe release promotion, production TLS/domain routing, telemetry, and environment-specific backends. The next set of features should increase operator leverage without undermining those guarantees: review non-active releases safely, deploy from source control without manual checkout steps, protect selected paths, control storage growth, and let components carry scoped behavior without falling back to large global bundles.

| # | Story | File |
|---|-------|------|
| 10.1 | Preview URLs for draft releases | [E10-S1](stories/E10-S1-preview-urls-draft-releases.md) |
| 10.2 | Git input mode for `apply` | [E10-S2](stories/E10-S2-git-input-apply.md) |
| 10.3 | Path-based auth policies | [E10-S3](stories/E10-S3-path-auth-policies.md) |
| 10.4 | Release retention and storage GC | [E10-S4](stories/E10-S4-release-retention-and-gc.md) |
| 10.5 | Component-scoped CSS/JS fragments | [E10-S5](stories/E10-S5-component-scoped-css-js-fragments.md) |

**Done when:** operators can create expiring preview hosts for specific releases, deploy a site directly from a pinned Git ref, gate selected prefixes with stored auth policies, prune old storage safely without breaking rollback or previews, and ship per-component CSS/JS sidecars deterministically.

---

## Epic 11 — CLI UX Polish

**Goal:** Make `htmlctl` feel more like a coherent operator control plane by fixing command inconsistencies, context ergonomics, diagnostics, and unsafe/default-awkward CLI behavior.
**Status:** Planned

**Motivation:** Epics 8 and 9 improved reliability and added backend management, but the command surface still has avoidable friction: `config` vs `context` is split awkwardly, many remote commands ignore context defaults, `config view` exposes secrets, remote version skew is hard to detect, error output is often less actionable than it should be, and the command inventory does not line up cleanly with what operators expect.

| # | Story | File |
|---|-------|------|
| 11.1 | Safe context lifecycle and config UX | [E11-S1](stories/E11-S1-safe-context-lifecycle-and-config-ux.md) |
| 11.2 | Context-aware defaults for remote commands | [E11-S2](stories/E11-S2-context-aware-defaults-remote-commands.md) |
| 11.3 | Remote diagnostics and version awareness | [E11-S3](stories/E11-S3-remote-diagnostics-and-version-awareness.md) |
| 11.4 | Inventory and workflow guidance polish | [E11-S4](stories/E11-S4-inventory-and-workflow-guidance-polish.md) |

**Done when:** operators can inspect and manage contexts safely, use common remote commands without repeatedly restating the current website/environment, detect client/server skew quickly, and discover resource inventory and next steps from the CLI without reading the source.
