# E12-S5 — Telemetry Collector Extension for Browser Event Ingest

**Epic:** Epic 12 — Optional Service Extensions
**Status:** Done (2026-03-09)
**Priority:** P1
**Estimated Effort:** 3 days
**Dependencies:** E7-S2, E9-S3, E12-S4
**Target:** `extensions/telemetry-collector/`, `docs/guides/`, `docs/reference/`, `.agent/skills/htmlctl-publish/`
**Design Reference:** Futurelab SEO measurement rollout

---

## 1. Summary

Ship an official telemetry collector extension that accepts same-origin browser events on a site-owned path, validates a narrow public event contract, and forwards those events server-side into htmlservd's authenticated telemetry ingest API without exposing bearer tokens to browser JavaScript.

## 2. Architecture Context and Reuse Guidance

- Reuse Epic 7 telemetry storage/querying as-is. Do **not** weaken core telemetry auth or add anonymous browser ingest directly to `htmlservd`.
- Reuse the Epic 12 extension packaging model established by `extensions/newsletter/`:
  - `extension.yaml`
  - `service/` Go module
  - `ops/` installer + env examples + systemd units
  - adopter guide in `docs/guides/`
- Reuse the newsletter service structure for config loading, loopback-only bind validation, graceful shutdown, rate limiting, and sanitized error handling.
- Reuse Epic 9 backend routing. Public telemetry ingress must be routed through an explicit backend prefix rather than becoming a special-case core runtime path.
- Preserve the current htmlservd security model:
  - `POST /collect/v1/events` remains bearer-authenticated
  - browser traffic never receives the htmlservd bearer token
  - environment attribution continues to come from the forwarded site host

## 3. Proposed Changes and Architecture Improvements

### 3.1 Official telemetry collector extension

Add a new official extension package:

- extension name: `telemetry-collector`
- runtime binary: `htmlctl-telemetry-collector`
- public backend prefix: `/site-telemetry/*`

The extension service should:

- expose `POST /site-telemetry/v1/events`
- expose `GET /healthz`
- validate same-origin browser events against one configured public base URL per environment
- validate a narrow allowed-event set before forwarding:
  - `page_view`
  - `link_click`
  - `cta_click`
  - `newsletter_signup`
- apply lightweight per-client rate limiting to public ingest
- forward accepted events to htmlservd `POST /collect/v1/events` with bearer auth
- preserve the original public site host when forwarding so htmlservd can attribute telemetry to the correct environment

### 3.2 No core auth relaxation

Do **not** change htmlservd telemetry ingest to allow unauthenticated browser POSTs.

The collector extension is the browser-facing trust boundary. Core telemetry remains a trusted-ingest sink and authenticated reporting API.

### 3.3 Futurelab pilot adoption

Adopt the official collector on Futurelab as the first real deployment:

- update site JS to post to `/site-telemetry/v1/events`
- install staging/prod collector services
- route `/site-telemetry/*` via Epic 9 backends
- verify staging event ingest end-to-end by querying htmlservd telemetry events
- only then deploy prod

### 3.4 Documentation and skill integration

Document:

- extension contract
- install/upgrade/runbook
- Futurelab-style site wiring
- telemetry reporting workflow via htmlservd API

Update `htmlctl-publish` skill guidance so agents know:

- browser telemetry should go through the collector extension, not direct htmlservd ingest
- which backend path to use
- how to verify capture after cutover

## 4. File Touch List

### Files to Add

- `extensions/telemetry-collector/extension.yaml`
- `extensions/telemetry-collector/README.md`
- `extensions/telemetry-collector/CHANGELOG.md`
- `extensions/telemetry-collector/service/go.mod`
- `extensions/telemetry-collector/service/go.sum`
- `extensions/telemetry-collector/service/cmd/htmlctl-telemetry-collector/main.go`
- `extensions/telemetry-collector/service/internal/config/config.go`
- `extensions/telemetry-collector/service/internal/config/config_test.go`
- `extensions/telemetry-collector/service/internal/server/server.go`
- `extensions/telemetry-collector/service/internal/server/server_test.go`
- `extensions/telemetry-collector/service/internal/server/rate_limit.go`
- `extensions/telemetry-collector/service/internal/server/rate_limit_test.go`
- `extensions/telemetry-collector/service/README.md`
- `extensions/telemetry-collector/ops/setup-telemetry-collector-extension.sh`
- `extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-staging.service`
- `extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-prod.service`
- `extensions/telemetry-collector/ops/env/staging.env.example`
- `extensions/telemetry-collector/ops/env/prod.env.example`
- `docs/guides/telemetry-collector-extension-hetzner.md`
- `docs/review-logs/E12-telemetry-collector-extension-<date>.md`

### Files to Modify

- `docs/epics.md`
- `docs/reference/extensions.md`
- `docs/guides/extensions-overview.md`
- `.agent/skills/htmlctl-publish/SKILL.md`
- `.agent/skills/htmlctl-publish/references/deployment-workflows.md`

### Futurelab adopter files (pilot validation)

- `/Users/bene/Dev-Source-NoBackup/futurelab-website/site/scripts/site.js`
- `/Users/bene/Dev-Source-NoBackup/futurelab-website/docs/seo/futurelab-world-class-seo-plan-2026-03-09.md`
- `/Users/bene/Dev-Source-NoBackup/futurelab-website/docs/stories/P2-S1-landing-page-applied.md`

## 5. Implementation Steps

1. Add the `telemetry-collector` extension manifest and package skeleton.
2. Implement env-driven service config:
   - environment (`staging|prod`)
   - loopback bind addr
   - public base URL
   - htmlservd base URL
   - htmlservd bearer token
   - allowed event names
3. Implement the HTTP service:
   - `GET /healthz`
   - `POST /site-telemetry/v1/events`
   - same-origin validation
   - public event allowlist validation
   - request-size / batch-size validation
   - per-client rate limiting
   - forwarding to htmlservd with bearer auth and preserved public host
   - sanitized 5xx responses
4. Add installer assets and env examples for staging/prod services.
5. Update extension docs and `htmlctl-publish` skill references.
6. Build and install the collector on Futurelab staging.
7. Update Futurelab site JS to post to `/site-telemetry/v1/events`.
8. Add staging backend route and verify end-to-end ingestion by querying htmlservd telemetry events.
9. Promote/deploy the same path to prod after staging verification.
10. Capture findings and evidence in a review log.

## 6. Tests and Validation

### Automated

- `go test ./...` in `extensions/telemetry-collector/service`
- `go test -race ./...` in `extensions/telemetry-collector/service`
- `go vet ./...` in `extensions/telemetry-collector/service`
- `bash -n extensions/telemetry-collector/ops/setup-telemetry-collector-extension.sh`
- `git diff --check`

Required test coverage:

- config validation:
  - invalid bind addr rejected
  - invalid public base URL rejected
  - missing htmlservd token rejected
- server behavior:
  - health endpoint
  - unsupported methods
  - invalid content type
  - invalid origin / host mismatch
  - disallowed event names rejected
  - invalid body rejected
  - rate limit triggers `429`
  - upstream success returns `202`
  - upstream `4xx` passthrough remains sanitized
  - upstream `5xx` returns sanitized collector error
  - forwarding preserves original host for htmlservd attribution

### Manual / E2E

- Futurelab staging:
  - collector service healthy on loopback
  - backend route `/site-telemetry/*` active
  - `site.js` points to `/site-telemetry/v1/events`
  - posting a valid event to `https://staging.futurelab.studio/site-telemetry/v1/events` returns `202`
  - querying htmlservd telemetry API shows the stored event under `futurelab/staging`
- Futurelab prod:
  - same verification sequence after staging sign-off

## 7. Acceptance Criteria

- [x] AC-1: Official `telemetry-collector` extension package exists with manifest, service module, installer assets, and docs.
- [x] AC-2: Public browser events can be posted to `/site-telemetry/v1/events` and are forwarded server-side into htmlservd telemetry ingest without exposing the bearer token to browsers.
- [x] AC-3: The collector validates same-origin requests, enforces a narrow allowed-event set, and rate-limits public ingest.
- [x] AC-4: htmlservd core telemetry auth remains unchanged; no anonymous browser ingest is added directly to core.
- [x] AC-5: Futurelab staging captures a real telemetry event end-to-end and the event is queryable through htmlservd telemetry reporting.
- [x] AC-6: Futurelab prod is deployed only after staging E2E verification succeeds.
- [x] AC-7: Extension docs and `htmlctl-publish` skill references describe installation, routing, verification, and reporting.

## 8. Risks and Open Questions

### Risks

- **Risk:** forwarding to htmlservd loses environment attribution if the original site host is not preserved.  
  **Mitigation:** set the forwarded request host explicitly to the validated public site host and cover this in tests.

- **Risk:** the browser-facing collector becomes an abuse path.  
  **Mitigation:** narrow event allowlist, same-origin validation, request size limits, per-client rate limiting, and loopback-only runtime policy.

- **Risk:** Futurelab site JS changes deploy before the collector backend exists.  
  **Mitigation:** stage collector service and backend first, then apply site JS update, then verify end-to-end before prod.

### Open Questions

- Should the collector remain a standalone extension long-term, or should htmlctl eventually provide an official telemetry-collector installer shortcut for common hosts?

---

## 9. Completion Notes (2026-03-09)

- Added the official `telemetry-collector` extension package under `extensions/telemetry-collector/` with:
  - manifest
  - loopback-only Go service
  - installer/env/systemd assets
  - Hetzner runbook
- Collector public contract:
  - `GET /healthz`
  - `POST /site-telemetry/v1/events`
- Collector enforcement:
  - configured public-origin match
  - allowed event allowlist
  - request body and batch-size limits
  - per-client rate limiting
  - sanitized upstream 5xx handling
- Forwarding behavior:
  - server-side `Authorization: Bearer <token>` into htmlservd `POST /collect/v1/events`
  - original public host preserved for environment attribution
  - no bearer token exposed to browser JS
- Futurelab adoption:
  - installed `htmlctl-telemetry-collector-staging` and `htmlctl-telemetry-collector-prod`
  - enabled htmlservd telemetry sink on the host
  - added `/site-telemetry/*` backends for staging and prod
  - updated `site/scripts/site.js` to post to `/site-telemetry/v1/events`
  - verified staging public `202` ingest plus stored-event query
  - promoted the same site asset to prod and verified the same flow there
