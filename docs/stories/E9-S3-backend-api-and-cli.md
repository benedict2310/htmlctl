# E9-S3 — Backend Management API and CLI

**Epic:** Epic 9 — Environment Backends
**Status:** Planned
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E9-S1 (data model), E9-S2 (Caddy config), E3-S3 (core remote commands pattern), E6-S1 (auth middleware)
**Target:** `internal/server/`, `internal/client/`, `cmd/htmlctl/`
**Design Reference:** Architecture discussion 2026-03-01

---

## 1. Objective

Expose backend management as authenticated HTTP API endpoints on `htmlservd` and implement the corresponding `htmlctl backend` subcommands, so operators can add, list, and remove environment backends from the CLI without touching the server directly.

## 2. User Story

As an operator, I want to run `htmlctl backend add website/futurelab --env prod --path /api/* --upstream https://api.futurelab.studio` and have Caddy immediately start proxying those requests, so that I can wire dynamic services into my static site without editing server config files.

## 3. Scope

### In Scope

- Three HTTP API endpoints (all authenticated via Bearer token):
  - `POST   /api/v1/websites/{website}/environments/{env}/backends`
  - `GET    /api/v1/websites/{website}/environments/{env}/backends`
  - `DELETE /api/v1/websites/{website}/environments/{env}/backends/{path_prefix}`
- Caddy config regeneration and reload after each `POST` and `DELETE`.
- Server-side validation of `path_prefix` and `upstream`.
- Three CLI subcommands: `htmlctl backend add`, `htmlctl backend list`, `htmlctl backend remove`.
- Audit log entries for add and remove operations.
- Unit tests for handlers; integration-style tests using the existing in-process test server pattern.

### Out of Scope

- Batch backend declarations via `htmlctl apply` (i.e. backends in `website.yaml`). Backends are managed imperatively, not as part of the site bundle — they are environment configuration, not site content.
- Forward auth / whole-site authentication policies (future story).
- Backend health checks or status reporting.

## 4. Architecture Alignment

- **Auth:** all three endpoints require `Authorization: Bearer <token>` — consistent with all `/api/v1/*` routes.
- **Caddyfile regeneration:** add/remove both trigger the existing `s.reloadCaddy()` path (same as domain-bind operations). No new reload infrastructure needed.
- **Validation ownership:** request validation delegates to `internal/backend` helpers introduced in E9-S1. The API layer reports validation failures, but does not define its own matcher semantics.
- **Audit log:** `add` logs operation `"backend.add"`, `remove` logs `"backend.remove"`. Summary includes `path_prefix` and `upstream` (for add) or `path_prefix` (for remove).
- **Error responses:** all 5xx responses go through `writeInternalAPIError` — no internal paths or schema details exposed to clients.
- **CLI pattern:** follows `htmlctl domain` command structure — subcommands under a parent `backend` command, context-aware, machine-parseable output.

## 5. API Design

### POST `/api/v1/websites/{website}/environments/{env}/backends`

Request body (JSON):
```json
{ "pathPrefix": "/api/*", "upstream": "https://api.example.com" }
```

Response `201 Created`:
```json
{ "pathPrefix": "/api/*", "upstream": "https://api.example.com", "createdAt": "..." }
```

Validation errors → `400 Bad Request` with `errors` array.
Already-exists → `200 OK` with updated row (upsert semantics).

### GET `/api/v1/websites/{website}/environments/{env}/backends`

Response `200 OK`:
```json
{
  "backends": [
    { "pathPrefix": "/api/*", "upstream": "https://api.example.com", "createdAt": "...", "updatedAt": "..." }
  ]
}
```

Empty environment → `200 OK` with `"backends": []`.

### DELETE `/api/v1/websites/{website}/environments/{env}/backends?path={path_prefix}`

`path_prefix` is URL-encoded in the `path` query parameter (e.g. `path=%2Fapi%2F*`). Using a query parameter avoids the slash-normalization problem that arises when path prefixes containing `/` are embedded in URL path segments.

Response `204 No Content` on success.
Not found → `404 Not Found`.

## 6. CLI Design

```
htmlctl backend add website/futurelab --env prod --path /api/* --upstream https://api.example.com
htmlctl backend list website/futurelab --env prod
htmlctl backend remove website/futurelab --env prod --path /api/*
```

**`backend add` output:**
```
backend /api/* -> https://api.example.com added to futurelab/prod
```

**`backend list` output (table):**
```
PATH PREFIX   UPSTREAM                          CREATED
/api/*        https://api.example.com           2026-03-01T12:00:00Z
/auth/*       https://auth.example.com          2026-03-01T12:05:00Z
```

**`backend remove` output:**
```
backend /api/* removed from futurelab/prod
```

All commands respect `--context` and `--output json` flags consistent with the rest of the CLI.

## 7. Implementation Plan

### 7.1 Files to Create

- `internal/server/backends.go` — `handleBackends` dispatcher; `handleBackendAdd`, `handleBackendList`, `handleBackendRemove` sub-functions.
- `internal/server/backends_test.go` — handler tests.
- `cmd/htmlctl/backend.go` — `backendCmd`, `backendAddCmd`, `backendListCmd`, `backendRemoveCmd`.

### 7.2 Files to Modify

- `internal/server/server.go` — register the three new routes.
- `internal/client/client.go` — add `AddBackend`, `ListBackends`, `RemoveBackend` methods.
- `cmd/htmlctl/main.go` — register `backendCmd`.

### 7.3 Route Registration

All three operations share a single route; dispatch on `r.Method` inside the handler:

```go
mux.HandleFunc("/api/v1/websites/{website}/environments/{env}/backends", s.requireAuth(s.handleBackends))
```

`DELETE` reads the path prefix from the `path` query parameter, but should parse `r.URL.RawQuery` with `url.ParseQuery` rather than relying on `r.URL.Query()`. That preserves the slash-safe query-parameter design while still allowing the handler to reject malformed query encoding with `400` instead of silently discarding it. A missing or empty `path` parameter also returns `400`.

### 7.4 Validation (Server-Side)

- `path_prefix`: delegate to `backend.ValidatePathPrefix` (E9-S1). Accept only the canonical prefix matcher forms defined there, such as `/api/*`.
- `upstream`: delegate to `backend.ValidateUpstreamURL` (E9-S1). Require absolute `http` or `https`, non-empty host, no credentials, and no query string or fragment. Use `writeAPIError(w, 400, ...)` on failure.

### 7.5 Reload Caddy After Mutation

After a successful `UpsertBackend` or `DeleteBackendByPathPrefix`, call the existing Caddyfile-regenerate-and-reload sequence (same path as `domain bind/unbind`). Reload errors are logged server-side and do not fail the API response — the data change is committed regardless.

## 8. Acceptance Criteria

- [ ] AC-1: `POST` with valid body inserts or updates the backend and returns `201` (or `200` on upsert) with the stored row.
- [ ] AC-2: `POST` with invalid `path_prefix` (no leading `/`, contains `..`, too long) returns `400` with a descriptive error message that contains no internal path or schema detail.
- [ ] AC-2a: `POST` with ambiguous or non-canonical `path_prefix` forms such as `/api/` or `/api` returns `400`.
- [ ] AC-3: `POST` with invalid `upstream` (non-http/https scheme, embedded credentials, empty host) returns `400`.
- [ ] AC-4: `GET` returns all backends for the environment ordered by `path_prefix`; returns empty list (not `404`) when none exist.
- [ ] AC-5: `DELETE` returns `204` on success and `404` when the prefix does not exist.
- [ ] AC-5a: `DELETE` returns `400` when the `path` query parameter is missing, empty, or malformed.
- [ ] AC-6: `POST` and `DELETE` trigger Caddyfile regeneration; Caddy reload failure does not roll back the DB change.
- [ ] AC-7: All three endpoints reject unauthenticated requests with `401`.
- [ ] AC-8: `htmlctl backend add` / `list` / `remove` call the correct endpoints and display human-readable output; `--output json` emits machine-parseable JSON.
- [ ] AC-9: `POST` and `DELETE` create audit log entries with the correct operation name and resource summary.
- [ ] AC-10: `go test -race ./internal/server/... ./cmd/htmlctl/...` passes.

## 9. Tests to Add

- `internal/server/backends_test.go`:
  - Add backend: valid request → 201 + correct body.
  - Add backend: invalid path prefix → 400 with no internal detail.
  - Add backend: ambiguous path prefix (`/api/`, `/api`) → 400.
  - Add backend: invalid upstream (bad scheme, credentials) → 400.
  - Add backend: unauthenticated → 401.
  - Add backend: upsert existing → 200 with updated upstream.
  - List backends: empty environment → 200 with `"backends": []`.
  - List backends: populated environment → 200 ordered by path prefix.
  - Remove backend: existing → 204.
  - Remove backend: not found → 404.
  - Remove backend: missing, empty, or malformed `path` query parameter → 400.
  - Remove backend: unauthenticated → 401.
  - 5xx responses contain no internal error detail (sanitization invariant).

## 10. Risks and Mitigations

- **Risk:** Path prefixes contain literal `/` which cannot be safely round-tripped through a URL path segment. **Mitigation:** resolved by design — `DELETE` uses a query parameter (`?path=...`) rather than a path segment. Parse `r.URL.RawQuery` with `url.ParseQuery` so malformed query encoding is rejected explicitly instead of being silently ignored.
- **Risk:** Caddy reload fails after a backend is added because the upstream is unreachable or the config is syntactically wrong. **Mitigation:** Caddy validates syntax at reload time (not upstream reachability). Log the reload error; surface it in the CLI output as a warning so the operator knows to check. The backend is persisted either way.
- **Risk:** Operator adds a backend with a path prefix that shadows a legitimate static file path (e.g. `/styles/*`). **Mitigation:** this is operator intent, not a bug. Document the behaviour: `reverse_proxy` takes priority over `file_server` for the declared prefix. Future story could add a warning for suspicious prefixes.
