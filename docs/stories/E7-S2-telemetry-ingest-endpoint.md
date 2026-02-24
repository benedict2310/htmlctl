# E7-S2 - Telemetry Ingest Endpoint for Static Sites

**Epic:** Epic 7 — Metadata and Telemetry
**Status:** Implemented (2026-02-23)
**Priority:** P2 (Medium — observability and product analytics)
**Estimated Effort:** 3-4 days
**Dependencies:** E2-S1 (server bootstrap), E2-S2 (SQLite schema), E5-S2 (Caddy config generation), E5-S3 (Caddy reload), E6-S7 (error sanitization)
**Target:** htmlservd + caddy integration
**Design Reference:** `docs/technical-spec.md` sections 4, 7, and 8

---

## 1. Objective

Add an optional, platform-level telemetry ingest endpoint so static pages served by htmlservd can emit lightweight product events (for example `page_view`, `cta_click`) without requiring a separate analytics backend.

## 2. User Story

As an operator, I want a built-in telemetry endpoint that my static pages can post events to, so I can inspect engagement signals while keeping deployment and operations inside the htmlctl/htmlservd stack.

## 3. Scope

### In Scope

- Add a public ingest endpoint for browser events:
  - `POST /collect/v1/events`
- Add telemetry storage table in SQLite with retention-friendly columns.
- Add host/domain resolution so events are attributed to website/environment.
- Add strict payload validation and limits (size, batch count, attribute limits).
- Route telemetry path through generated Caddy config to htmlservd.
- Provide authenticated read API for operators:
  - `GET /api/v1/websites/{website}/environments/{env}/telemetry/events`
  - pagination + basic filters (`event`, `since`, `until`, `limit`, `offset`)
- Add tests for auth boundaries, validation, and sanitized errors.

### Out of Scope

- Frontend analytics SDK packaging (sites can use `navigator.sendBeacon` directly).
- Real-time dashboards/UI.
- Third-party vendor export pipelines (Datadog, Segment, GA).
- Cross-domain CORS collection (v1 is same-origin only; use `sendBeacon` with default `text/plain` body transport).

## 4. Architecture Context and Reuse Guidance

- Treat this as **platform capability** in htmlservd, not app-specific logic.
- Keep current security invariants intact:
  - `/api/v1/*` remains bearer-authenticated.
  - public ingest route must return sanitized 5xx errors and never leak internals.
- Reuse existing patterns:
  - route registration in `internal/server/server.go` + `routes.go`
  - query style and migration model in `internal/db/*`
  - Caddy generation in `internal/caddy/config.go`.

### Library/Version Decision (via `gh` research)

- v1 ingest endpoint should use Go stdlib + existing SQLite layer (no mandatory telemetry dependency).
- If OpenTelemetry export is added in a follow-up, pin to latest stable versions verified via `gh` on 2026-02-23:
  - `open-telemetry/opentelemetry-go`: `v1.40.0` (latest)
  - `open-telemetry/opentelemetry-go-contrib`: `v0.65.0` line (latest train)
- Caddy upstream latest is `v2.11.1`, but this repo currently pins `2.8.4`; this story does not require a Caddy version bump.

## 5. Proposed Changes

### 5.1 API Design

Public ingest payload:

```json
{
  "events": [
    {
      "name": "page_view",
      "path": "/ora",
      "occurredAt": "2026-02-23T11:15:00Z",
      "sessionId": "b2f1d4d9-...",
      "attrs": {
        "referrer": "https://futurelab.studio/",
        "source": "landing"
      }
    }
  ]
}
```

Validation limits (v1 defaults):
- request body <= 64KB
- max 50 events per request
- `name` max 64 chars, `[a-zA-Z0-9][a-zA-Z0-9_-]*`
- `path` must normalize to `/...`
- max 16 attrs per event; key<=64, value<=256

### 5.2 Event Attribution

- Determine website/environment from request host:
  - domain binding match when present
- unresolved hosts are rejected with 400 (no preview fallback in v1)
- Store resolved environment ID; reject unresolved hosts with 400.

### 5.3 Caddy Integration

Generated site blocks include telemetry pass-through:
- `handle /collect/v1/events* { reverse_proxy 127.0.0.1:<htmlservd-port> }`
- Keep `file_server` behavior unchanged for other paths.

### 5.4 Storage Model

New table — **migration 004** (E7-S1 consumes migration 003 for `pages.head_json`; see Section 11.2):

```sql
CREATE TABLE IF NOT EXISTS telemetry_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    event_name  TEXT NOT NULL,
    path        TEXT NOT NULL,
    occurred_at TEXT NULL,
    received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    session_id  TEXT NULL,
    attrs_json  TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_telemetry_events_env_received
    ON telemetry_events(environment_id, received_at);
CREATE INDEX IF NOT EXISTS idx_telemetry_events_env_name_received
    ON telemetry_events(environment_id, event_name, received_at);
```

File: `internal/db/migrations/004_telemetry_events.go` — register as `Version: 4` in `All()`.

Note: `DEFAULT now` is **not valid SQLite syntax** and would be stored as the literal string `"now"`. The correct form shown above uses `strftime(...)`. All existing migrations use this same pattern.

## 6. File Touch List

### Files to Create

- `internal/db/migrations/004_telemetry_events.go` — telemetry table + indexes (Version 4; E7-S1 takes Version 3).
- `internal/server/telemetry.go` — ingest + query handlers and request/response DTOs.
- `internal/server/telemetry_test.go` — endpoint behavior coverage.

### Files to Modify

- `internal/server/server.go` — register public telemetry route outside `/api/v1` auth middleware.
- `internal/server/routes.go` — add authenticated telemetry read route under website/environment path.
- `internal/server/config.go` — optional telemetry config knobs (enable flag, max body/events, retention days).
- `internal/db/models.go` — telemetry row types.
- `internal/db/queries.go` + tests — insert/list telemetry queries.
- `internal/caddy/config.go` + tests — emit `handle` + `reverse_proxy` for ingest endpoint.
- `docker/htmlservd-entrypoint.sh` — preview bootstrap Caddyfile includes telemetry proxy stanza.
- `docs/technical-spec.md` — document endpoint, payload, and retention.

## 7. Implementation Steps

1. Add DB migration and query/model support for telemetry events.
2. Implement public ingest handler with strict validation and safe defaults.
3. Implement authenticated list endpoint for operators.
4. Add host-to-environment resolution helper using domain bindings and preview fallback.
5. Update Caddy config generation and bootstrap Caddyfile path handling.
6. Add tests for success, invalid payloads, host resolution failures, and 5xx sanitization.
7. Document API contract and operator guidance.

## 8. Tests and Validation

### Automated

- Handler tests:
  - valid ingest accepted (`202`) and persisted
  - oversize body / too many events rejected (`400`)
  - invalid name/path rejected (`400`)
  - unresolved host rejected (`400`)
  - DB failure returns sanitized `500` without internal details
- Auth tests:
  - ingest route is reachable without bearer token
  - telemetry read route requires bearer token like other `/api/v1/*`
- Caddy config tests:
  - generated config includes telemetry proxy stanza and retains file server.

### Manual

- `curl -X POST http://<domain>/collect/v1/events ...` stores events.
- `htmlctl` or direct API call fetches stored events for target environment.
- Confirm static pages still serve unchanged and ingest path routes to API.

## 9. Acceptance Criteria

- [x] AC-1: `POST /collect/v1/events` ingests valid same-origin telemetry events and stores them in SQLite.
- [x] AC-2: Ingest endpoint enforces payload size, batch count, and field validation limits.
- [x] AC-3: Ingest events are correctly attributed to website/environment via host/domain mapping.
- [x] AC-4: `GET /api/v1/websites/{website}/environments/{env}/telemetry/events` returns paginated events and remains bearer-authenticated.
- [x] AC-5: Any telemetry ingest/query 5xx response is sanitized (no DB paths/schema leakage).
- [x] AC-6: Caddy-generated and bootstrap configs route `/collect/v1/events` to htmlservd while keeping static file serving intact.
- [x] AC-7: Technical spec and operator docs document endpoint usage and limits.
- [x] AC-8: Telemetry events older than `retentionDays` are deleted by a background cleanup loop (when `retentionDays > 0`).

## 10. Risks and Open Questions

- **Risk:** public endpoint abuse/spam.
  - **Mitigation:** strict payload limits, lightweight validation, optional feature flag, and retention window.
- **Risk:** attribution errors when host is not bound.
  - **Mitigation:** explicit 400 with actionable error; include preview fallback only when configured.
- **Risk:** endpoint growth into full analytics platform.
  - **Mitigation:** keep v1 minimal (ingest + query), leave dashboards/export to follow-up stories.
- **Open question:** Should v1 aggregate counters instead of raw event rows?
  - Proposed: store raw rows first for flexibility; add rollups in a later story if needed.

---

## 11. Architectural Review Notes

### 11.1 Route Registration Pattern — Where the Public Route Goes

**Gap:** The story says "register public telemetry route outside `/api/v1` auth middleware" but does not show the exact wiring location.

**Finding:** `internal/server/server.go` (`New()`) builds two muxes:
```go
mux := http.NewServeMux()          // top-level
registerHealthRoutes(mux, version) // /healthz, /readyz, /version — no auth
apiMux := http.NewServeMux()
registerAPIRoutes(apiMux, srv)
mux.Handle("/api/v1/", authMiddleware(cfg.APIToken, logger)(apiMux))
```

**Required fix:** The public ingest handler must be registered directly on `mux` (top-level), NOT on `apiMux`. The correct pattern, to be added in `New()` immediately after `registerHealthRoutes`:

```go
// Public telemetry ingest — no auth, must stay outside /api/v1.
if cfg.Telemetry.Enabled {
    mux.HandleFunc("/collect/v1/events", srv.handleTelemetryIngest)
}
```

The authenticated read route is registered via `registerAPIRoutes` → `registerTelemetryAPIRoutes(apiMux, srv)` in `internal/server/routes.go`, following the same pattern as `handleDomains`. The `handleWebsiteAPI` dispatcher in `routes.go` must gain a new `strings.HasSuffix(r.URL.Path, "/telemetry/events")` case.

### 11.2 Migration Number Conflict

**Gap:** Story says "migration 003 or 004 depending on ordering." E7-S1 (`internal/db/migrations/003_pages_head_metadata.go`) already claims Version 3.

**Finding:** The current `All()` function in `internal/db/migrations/001_initial_schema.go` returns Versions 1 and 2. The next available version is 3. E7-S1 takes Version 3. This story must use **Version 4**.

**Required fix (already applied in Section 5.4 and the file touch list):** Migration file is `internal/db/migrations/004_telemetry_events.go` and the `Migration` struct must be `{Version: 4, Name: "telemetry_events", UpSQL: telemetryEventsSchemaSQL}`.

### 11.3 SQLite `DEFAULT now` Bug

**Gap (already fixed in Section 5.4):** `received_at TEXT NOT NULL DEFAULT now` is invalid SQLite. SQLite does not recognise `now` as a function in a DEFAULT expression; it would store the literal string `"now"`.

**Correct syntax** (matching the existing schema convention from `001_initial_schema.go` and `002_domain_bindings.go`):
```sql
received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
```
The existing migrations use `%Y-%m-%dT%H:%M:%fZ` (with milliseconds). Either is acceptable; be consistent. The `%S` (without fractional) form is shown in the corrected DDL above for human readability, but `%f` is equally correct.

### 11.4 Host Resolution Architecture

**Gap:** The story says "determine website/environment from request host" but gives no data flow details.

**Finding:** `internal/db/queries.go` already has `GetDomainBindingByDomain(ctx, domain)` which joins `domain_bindings`, `environments`, and `websites` and returns a `DomainBindingResolvedRow` containing `EnvironmentID`, `WebsiteName`, and `EnvironmentName`. This is exactly the lookup needed.

**Required pattern for the ingest handler:**
1. Extract host from `r.Host` (strip port with `net.SplitHostPort` if present; fall back to `r.Host` if no port).
2. Call `dbpkg.NewQueries(s.db).GetDomainBindingByDomain(ctx, host)`.
3. On `sql.ErrNoRows`: check if preview fallback is configured (see 11.5); otherwise return 400 "host not bound to any environment".
4. On other DB error: call `s.writeInternalAPIError`.
5. Use `row.EnvironmentID` as the FK for the telemetry insert.

The host extraction helper should be a small private function in `telemetry.go` (e.g. `resolveHostToEnvironment`) rather than inline in the handler, to make it independently testable.

### 11.5 Attribution for Preview Mode

**Gap:** "fallback to preview website/env for bootstrap preview mode" — this is not defined anywhere in the codebase.

**Finding:** Preview mode is a **Caddy bootstrap concept** only. `docker/htmlservd-entrypoint.sh` writes a single-site Caddyfile for `HTMLSERVD_PREVIEW_WEBSITE`/`HTMLSERVD_PREVIEW_ENV`; with telemetry enabled it now includes `/collect/v1/events` reverse proxy routing. htmlservd still has no concept of a dedicated "preview environment ID", so attribution must come from domain bindings.

**Required fix:** Two concrete options — the story must pick one:

- **Option A (recommended):** Remove the preview fallback entirely. Require explicit host/domain binding for attribution; return 400 for any unrecognised host and document this in the operator guide.
- **Option B:** Add a `Telemetry.PreviewEnvironmentID int64` config field. The entrypoint resolves the environment row ID at startup and passes it as an env var. The handler uses it only when domain lookup returns `sql.ErrNoRows` AND the config field is non-zero.

Option A is simpler and eliminates the security surface (see Section 12.7). If the story author chooses Option B, add the env var and config field to `internal/server/config.go` and `docker/htmlservd-entrypoint.sh`.

### 11.6 Caddy Config Generation — Where `handle` Goes

**Gap:** `internal/caddy/config.go` currently uses a pure `strings.Builder` loop, writing each site as `{domain} { root * {root}\n\tfile_server\n}`. There is no existing mechanism for per-site extra directives.

**Required changes to `internal/caddy/config.go`:**
1. Add a `TelemetryPort int` field to `ConfigOptions` (0 = disabled).
2. In `GenerateConfigWithOptions`, when `opts.TelemetryPort > 0`, append the reverse_proxy block inside each site block before the closing brace:
   ```
   handle /collect/v1/events* {
       reverse_proxy 127.0.0.1:<port>
   }
   ```
3. The `file_server` line must come **after** `handle` in Caddy's directive ordering to avoid catching the ingest path.
4. Add `caddy_test.go` assertions: generated config contains the `handle` stanza when port is set, and does NOT contain it when port is zero.

The `generateCaddyConfig` method in `internal/server/caddy.go` must pass a non-zero telemetry proxy port (resolved from explicit `s.cfg.Port` or the active listener address after startup).

### 11.7 Request Body Size Limit

**Gap:** The story specifies 64KB but does not prescribe the Go implementation.

**Finding:** `internal/server/apply.go` uses `maxApplyBundleBytes = 50 * 1024 * 1024` with `http.MaxBytesReader`. That is the precedent.

**Required fix:** At the top of `handleTelemetryIngest`, before calling `json.NewDecoder`:
```go
r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
```
After `json.Decode`, check `errors.As(err, new(*http.MaxBytesError))` and return 413 (not 400) with message "request body too large".

### 11.8 Pagination Pattern

**Finding:** The existing pagination pattern uses `limit` and `offset` query parameters (see `parseListReleasesPagination` in `internal/server/release.go`): default limit 20, max limit 200.

**Required fix:** The telemetry list endpoint must use the same param names (`limit`, `offset`) with the same clamping logic. The story already lists these in Section 3 but does not specify the defaults or cap. Set: default limit = 100, max limit = 1000 (events are small rows). Add a `parseListTelemetryPagination` function in `telemetry.go` following the same pattern as `parseListReleasesPagination`.

### 11.9 `occurred_at` Client-Supplied Trust Boundary

**Gap:** The story does not specify what to do with far-future or far-past timestamps.

**Required fix:**
1. Parse `occurred_at` using `time.Parse(time.RFC3339, v)` (or `time.RFC3339Nano`). Reject unparseable values with 400 "invalid occurredAt: must be ISO 8601 / RFC 3339".
2. Enforce a skew window: reject timestamps more than **24 hours in the future** from server `time.Now().UTC()`. Return 400 "occurredAt is too far in the future".
3. Enforce a floor: reject timestamps more than **30 days in the past**. Return 400 "occurredAt is too far in the past". (The 30-day window matches a reasonable retention period.)
4. Re-serialize the parsed `time.Time` to UTC RFC3339 before storing, never storing the raw client string.

### 11.10 `session_id` Format Constraint

**Gap:** The story says "`session_id TEXT NULL`" with no format or length constraint.

**Required fix:** Define a concrete constraint: `session_id` is an optional, arbitrary client-generated identifier. Accept any string of 1–128 characters matching `^[a-zA-Z0-9_\-]{1,128}$` when present. Reject values that are non-empty but fail this pattern with 400. Store as-is after validation. Do not attempt to parse as UUID.

### 11.11 Response Body on Ingest

**Gap:** The story says 202 Accepted but does not specify the response body.

**Required fix:** Return a minimal JSON body:
```json
{"accepted": 3}
```
where the integer is the count of events successfully persisted. Use `writeJSON(w, http.StatusAccepted, map[string]int{"accepted": count})`. Empty body (204) is tempting but makes it harder for clients to detect partial acceptance in future versions.

### 11.12 Config Struct — Missing YAML Key and Defaults

**Gap:** Section 5 refers to an "enable flag" but gives no YAML key name, env var name, or Go struct definition.

**Required fix:** Add to `internal/server/config.go`:
```go
type TelemetryConfig struct {
    Enabled       bool `yaml:"enabled"`
    MaxBodyBytes  int  `yaml:"maxBodyBytes"`
    MaxEvents     int  `yaml:"maxEvents"`
    RetentionDays int  `yaml:"retentionDays"`
}
```
Add `Telemetry TelemetryConfig yaml:"telemetry,omitempty"` to `Config`. Defaults: `Enabled: false`, `MaxBodyBytes: 65536`, `MaxEvents: 50`, `RetentionDays: 90`. Add env vars: `HTMLSERVD_TELEMETRY_ENABLED`, `HTMLSERVD_TELEMETRY_RETENTION_DAYS`. Include `Telemetry.Enabled` validation: if disabled and the route is not registered, Caddy must not emit the `handle` stanza either.

### 11.13 `attrs` Key Validation

**Gap:** The story says keys up to 64 chars but does not specify the allowed character set.

**Required fix:** Keys must match `^[a-zA-Z0-9_][a-zA-Z0-9_-]*$` (same pattern as `internal/names` but allowing underscore as first char for conventional JS property names like `_ga`). Values may be arbitrary UTF-8 up to 256 bytes (no character-level constraint, but validate byte length). Reject events with keys that fail this pattern with 400.

### 11.14 File Touch List — Missing Items

The following files are missing from the touch list in Section 6 and must be added:

- `internal/server/caddy.go` — `generateCaddyConfig` must pass `TelemetryPort` to `caddy.ConfigOptions`.
- `internal/caddy/caddy_test.go` (or `internal/server/caddy_test.go`) — new assertions for telemetry `handle` stanza.
- `internal/db/migrations/001_initial_schema.go` — the `All()` function must gain the Version 4 entry for `telemetry_events`. (This file registers all migrations.)

---

## 12. Security Review Notes

### 12.1 Public Endpoint Abuse / DDoS

**Gap:** The story lists "strict payload limits" as the sole mitigation. There is no mention of per-IP or per-environment rate limiting.

**Findings and recommended mitigations:**
- **Caddy layer rate limiting:** Add `rate_limit` or `header` plugin configuration to the `handle` block. Caddy's built-in `rate_limit` (available via `caddy-ratelimit` plugin) can enforce per-IP limits. Specify a concrete limit (e.g. 60 requests/minute per IP) in the Caddy template.
- **Per-environment write budget:** In the telemetry query layer, consider a DB-level check: if `COUNT(*) WHERE environment_id = ? AND received_at > strftime(...)` exceeds a configurable cap (e.g. 100,000 rows), return 429 "telemetry quota exceeded for this environment". This prevents a single rogue environment from filling the entire disk.
- **Minimum viable mitigation for v1:** The `http.MaxBytesReader(64KB)` + 50-event cap prevent any single request from consuming excessive resources. This is acceptable for v1, but the story must acknowledge it and add an AC or open question about rate limiting.

### 12.2 `path` Field Injection

**Gap:** `path` is stored in SQLite and returned via the read API. The story only says it "must normalize to `/...`" with no further constraint.

**Findings:**
- Null bytes (`\x00`) in a TEXT column are accepted by SQLite but can truncate C-string processing in some tools.
- CRLF sequences (`\r\n`) enable log injection if the field is ever logged raw.
- Path traversal sequences (`../`) have no SQLite impact but are semantically wrong.

**Required fix:** After confirming the path starts with `/`, apply:
1. Parse with `url.PathUnescape` and check for errors (rejects malformed percent-encoding).
2. Apply `path.Clean` to resolve `..` and double slashes.
3. If the cleaned result does not start with `/`, return 400.
4. Reject paths containing null bytes (`strings.ContainsRune(p, 0)`).
5. Enforce a maximum length of 1024 bytes.
6. Store the cleaned path, not the raw client-supplied string.

### 12.3 `session_id` Injection

**Gap:** Arbitrary strings stored in DB with no documented character constraint. Risk of log injection if the field appears in server-side structured log output.

**Required fix:**
- Validate against `^[a-zA-Z0-9_\-]{1,128}$` before storage (specified in Section 11.10).
- In the ingest handler, never log `session_id` in server-side log lines. If the field must appear in structured logs (e.g. for debugging), mask it: log only the first 8 chars followed by `...`.

### 12.4 `attrs` Value Injection

**Gap:** Attr values up to 256 chars with no character constraint. If attrs are rendered in any future HTML context (dashboard, API response wrapped in HTML), `</script>` or `<img onerror=...>` sequences in values could cause XSS.

**Findings:** In v1, attrs are returned as JSON in API responses and never directly rendered as HTML. However:
- The `attrs_json` column stores the Go `json.Marshal` output of `map[string]string`, which correctly escapes special characters in JSON. The JSON itself is safe.
- Risk surfaces if a future story renders attrs in a Go `html/template` context and uses `template.JS` or raw output.

**Required fix for v1:**
- Add a note to the story and the technical spec: attr values must not be assumed safe for HTML rendering. Any future consumer of `attrs_json` must parse the JSON and pass values through `html/template`'s auto-escaping, never use `template.HTML` or `template.JS` casts on attr values.
- Consider stripping null bytes from attr values as a defensive measure.

### 12.5 `event_name` Validation — Length Conflict

**Gap:** The story says `name` max 64 chars with pattern `[a-zA-Z0-9][a-zA-Z0-9_-]*`. `internal/names.ValidateResourceName` enforces the same pattern but max 128 chars. If the implementation calls `validateResourceName` (which wraps `names.ValidateResourceName`), the 64-char limit is silently ignored.

**Required fix:** Do NOT call `validateResourceName` for event names. Implement a dedicated `validateEventName` function in `telemetry.go`:
```go
var eventNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func validateEventName(name string) error {
    if !eventNameRE.MatchString(name) {
        return fmt.Errorf("event name must match [a-zA-Z0-9][a-zA-Z0-9_-]* and be at most 64 characters")
    }
    return nil
}
```
The regex `{0,63}` after the first character enforces a total max length of 64 without a separate length check.

### 12.6 Timestamp Injection (`occurred_at`)

**Gap:** `occurred_at` is user-supplied ISO 8601. Storing the raw string without parsing allows malformed values (e.g. `'; DROP TABLE telemetry_events; --`, or a 10,000-char string) to reach the DB column.

**Required fix:** Always parse with `time.Parse(time.RFC3339, ...)` and re-serialize to `time.UTC().Format(time.RFC3339)` before binding as a query parameter. See Section 11.9 for full constraints. This prevents any injection risk from the timestamp field.

### 12.7 Host Header Attribution — Spoofing Risk

**Gap:** Attribution relies on `r.Host`. When htmlservd sits behind Caddy with reverse_proxy, Caddy sets the `Host` header to the original request host by default, which is correct. However, `r.Host` can be spoofed in some deployment topologies (e.g. direct port access bypassing Caddy, or misconfigured reverse proxy).

**Findings:**
- In the standard deployment, Caddy is the only process bound to external ports. htmlservd listens on `127.0.0.1:9400` and is not directly reachable from the internet. `r.Host` will therefore always be the value Caddy forwarded, which equals the original request `Host` header after Caddy's SNI/TLS validation.
- If htmlservd is ever bound to a non-loopback address (it already logs a warning for this), `r.Host` is entirely attacker-controlled and attribution is meaningless.

**Required fix:**
- The `isLoopbackHost` warning in `server.go` is already present. Add a note to the story and operator docs: when telemetry is enabled, htmlservd MUST be bound to loopback only. The Caddy config is the authoritative source of host identity.
- Optionally: support a `X-Forwarded-Host` header, but only when it is sent by Caddy (which sets it via `header_up`). This is out of scope for v1 but should be noted as a follow-up.
- Reject `Host` values that are not present in `domain_bindings` (already specified: return 400 "host not bound") — this naturally prevents cross-environment injection.

### 12.8 CORS — `Content-Type: application/json` Triggers Preflight

**Gap:** Section 3 says "v1 keeps same-origin/simple POST usage" and CORS is out of scope. However, a `POST` with `Content-Type: application/json` is **not** a simple request under the Fetch/CORS spec. Browsers will issue a preflight `OPTIONS /collect/v1/events` before posting, which will receive a 404 (or 405) and block the actual POST.

**Impact:** `navigator.sendBeacon` sends `Content-Type: text/plain; charset=UTF-8` by default (a simple request), which does NOT trigger a preflight. If the story intends beacon-only usage, same-origin is fine. But the story shows a JSON payload, which implies `Content-Type: application/json` via `fetch()`.

**Required fix — pick one and document it:**
- **Option A (beacon):** Accept `Content-Type: text/plain` (sendBeacon default). Parse the body as JSON regardless of content type. This is a common analytics endpoint pattern. No preflight is triggered on same-origin or cross-origin calls from first-party pages.
- **Option B (fetch with CORS):** Handle `OPTIONS /collect/v1/events` with appropriate `Access-Control-Allow-*` headers. If the ingest URL is on the same domain as the page (Caddy routes it), then same-origin rules apply and no CORS headers are needed. If the ingest URL ever differs from the page origin (e.g. a centralized `collect.example.com`), full CORS handling is required.

The story must explicitly state which option is chosen and add a test: confirm that a simulated `OPTIONS` preflight against the ingest handler returns an appropriate response (either 200 with CORS headers, or 405 with a clear note about same-origin-only).

### 12.9 Auth Boundary Regression Test

**Gap:** The story says "ingest route is reachable without bearer token" in Section 8 but does not specify the exact test assertion.

**Required test (must exist in `telemetry_test.go`):**
```
TestTelemetryIngestNoAuthRequired:
  - POST /collect/v1/events with a valid payload and NO Authorization header
  - Assert HTTP 202 (or 400 for invalid host, but NOT 401)
  - Assert the response does NOT contain "unauthorized"

TestTelemetryReadRequiresAuth:
  - GET /api/v1/websites/foo/environments/bar/telemetry/events with NO Authorization header
  - Assert HTTP 401
```
These two tests together verify the auth boundary did not regress. The server-level test infrastructure already exists in `internal/server/auth_test.go` — follow the same pattern.

### 12.10 Error Message Leakage — 400 vs 5xx Distinction

**Finding:** The story correctly uses `writeAPIError` for 400s and `writeInternalAPIError` for 500s, matching codebase convention. `writeAPIError` sends the message string directly (acceptable for client errors). `writeInternalAPIError` logs internally and sends a generic message.

**Required clarification for the implementation:**
- Validation errors (bad `name`, `path`, oversized body, unparseable `occurredAt`): use `writeAPIError(w, http.StatusBadRequest, "descriptive message about the field", nil)`. Include the field name in the message but NOT internal Go struct names, column names, or SQL fragment.
- Host resolution failure (domain not found): `writeAPIError(w, http.StatusBadRequest, "host is not bound to any environment", nil)`. Do NOT include the raw `r.Host` value in the error message (it is attacker-controlled).
- DB errors: always `s.writeInternalAPIError(w, r, "telemetry ingest failed", err)` — no column names, table names, or SQL in the message string.

### 12.11 SQLite Injection via `attrs_json`

**Finding:** `attrs_json` will be produced by `json.Marshal(map[string]string{...})` in Go and then bound as a `?` parameter in the INSERT statement. Go's `json.Marshal` does not produce SQL-injectable output, and the `?` binding prevents SQL injection regardless of content.

**Status:** No injection risk exists as long as `attrs_json` is always bound as a parameter (never string-interpolated into the query). Add a code review note: the INSERT for `telemetry_events` must use `?` for `attrs_json`, consistent with all other queries in `internal/db/queries.go`.

### 12.12 Retention / Storage Growth

**Gap:** The story mentions "retention days" as a config knob but specifies no enforcement mechanism.

**Findings:** Without a cleanup job, the `telemetry_events` table grows unboundedly. SQLite does not support scheduled jobs or TTL expiry.

**Required fix:** Add to the implementation steps and acceptance criteria:
- A background goroutine in `Server` that runs a DELETE query on a configurable interval (e.g. every hour):
  ```sql
  DELETE FROM telemetry_events
  WHERE received_at < strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-' || ? || ' days')
  ```
  where the bind parameter is `cfg.Telemetry.RetentionDays`.
- The goroutine must be started in `Server.Start()` and stopped in `Server.Shutdown()`.
- When `RetentionDays` is 0, skip the cleanup entirely (unlimited retention mode for testing).
- Add AC-8: "Telemetry events older than `retentionDays` are automatically deleted by a background cleanup job."
