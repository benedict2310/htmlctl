# E6-S1 - API Authentication Layer

**Epic:** Epic 6 — Security Hardening
**Status:** Done
**Priority:** P0 (Critical — unauthenticated destructive API)
**Estimated Effort:** 3 days
**Dependencies:** E3-S2 (SSH tunnel transport), E2-S1 (server bootstrap config)
**Target:** Linux server (htmlservd) + CLI client (htmlctl)
**Design Reference:** Security Audit 2026-02-20, Vulns 1 & 2

---

## 1. Objective

Every htmlservd API endpoint is currently reachable by any process that can open a TCP connection to the server — there is no authentication whatsoever. In addition, the `X-Actor` header used for audit logging is read verbatim from the unauthenticated request, allowing any caller to forge any identity in the permanent audit trail.

This story adds a shared-secret authentication middleware that protects all API routes, and derives the audit `Actor` from the verified token rather than a caller-supplied header.

## 2. User Story

As an operator, I want htmlservd to reject requests that do not carry a valid credential so that only authorised clients (htmlctl instances with the correct context token) can deploy, promote, rollback, or modify domain bindings.

## 3. Scope

### In Scope

- A per-context shared secret (bearer token) stored in the htmlctl context config (`~/.htmlctl/config.yaml`) and in htmlservd config (`/etc/htmlservd/config.yaml` or env var).
- An HTTP middleware function (`authMiddleware`) that reads the `Authorization: Bearer <token>` header on every request, constant-time compares it against the configured secret, and returns `401 Unauthorized` on mismatch.
- `authMiddleware` wraps all routes registered in `registerAPIRoutes`; health routes (`/healthz`, `/readyz`) remain unauthenticated.
- The `Actor` field in audit log entries is derived from the verified token identity (e.g., the context name embedded in the token or a separate `X-Actor` header that is only trusted after authentication passes). Unauthenticated requests never reach audit-log-writing code.
- The htmlctl client sends the bearer token on every request via `internal/client/client.go`.
- The context config schema (`internal/config/types.go`) gains an optional `token` field; `htmlctl context set` supports `--token` flag.
- Token generation helper: `htmlctl context token generate` prints a cryptographically random 32-byte hex token suitable for use as a shared secret.
- Unit tests for the middleware: valid token passes, missing token returns 401, wrong token returns 401, health routes bypass auth.
- Integration test: end-to-end apply with and without token.

### Out of Scope

- mTLS or certificate-based authentication (future hardening).
- Per-route authorisation (all authenticated clients have full access for v1).
- Token rotation or expiry (static shared secret for v1).
- OAuth or external identity providers.

## 4. Architecture Alignment

- **Threat model:** htmlservd binds to `127.0.0.1:9400` by default, but the bind address is configurable and a non-loopback bind currently only produces a warning. Authentication closes the gap for misconfigured or container-shared-network deployments.
- **SSH tunnel:** The SSH tunnel (E3-S2) provides transport-layer authentication between `htmlctl` and the host running `htmlservd`. The bearer token adds application-layer authentication independent of transport, ensuring that any other process on the same host cannot call the API.
- **Audit actor:** With authentication in place, the `Actor` field should be derived from the authenticated context name (passed as `X-Actor` after auth middleware validates the token), not trusted blindly. The middleware can enforce that `X-Actor` is only set by authenticated callers, preventing audit log poisoning.
- **Constant-time comparison:** Use `crypto/subtle.ConstantTimeCompare` to prevent timing-based token enumeration.
- **PRD references:** Technical Spec Section 4 (security model), PRD Section 7 (operator authentication).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/server/auth.go` — `authMiddleware(token string) func(http.Handler) http.Handler` using `crypto/subtle.ConstantTimeCompare`; logs rejected attempts at Warn level (without echoing the submitted token).
- `internal/server/auth_test.go` — table-driven tests for all middleware paths.

### 5.2 Files to Modify

- `internal/server/routes.go` — wrap `registerAPIRoutes` handler tree with `authMiddleware`; health routes remain unwrapped.
- `internal/server/config.go` — add `APIToken string` field; validate non-empty at startup (warn or fatal depending on a `--require-auth` flag, default warn for rollout safety).
- `internal/server/server.go` — pass `cfg.APIToken` into `authMiddleware` during server setup.
- `internal/server/routes.go` — update `actorFromRequest` to only read `X-Actor` header after middleware has confirmed authentication; keep as-is for the header read but document that it is now trusted because auth middleware ran.
- `internal/client/client.go` — add `Authorization: Bearer <token>` header to all outgoing requests.
- `internal/config/types.go` — add `Token string` field to `Context` struct.
- `internal/cli/root.go` — read token from active context and pass to client constructor.
- `internal/cli/context_cmd.go` — add `context set --token` and `context token generate` subcommands using `crypto/rand`.

### 5.3 Tests to Add

- `internal/server/auth_test.go`
  - Valid bearer token: request passes through to handler.
  - Missing `Authorization` header: 401 returned, body contains `{"error":"unauthorized"}`.
  - Wrong token value: 401 returned.
  - Correct token prefix but trailing garbage: 401 returned.
  - Health endpoint (`/healthz`): 200 returned without any `Authorization` header.
  - Timing safety: two calls with different wrong tokens complete in similar wall time (best-effort, not a strict test).
- `internal/server/server_test.go` — verify health/readiness endpoints remain unauthenticated.
- `internal/client/client_test.go` — verify bearer header and actor header emission from configured context.
- `internal/cli/context_cmd_test.go` — verify token generation output and context token set behavior.
- `internal/cli/remote_cmd_helpers_test.go` — verify context token is sent on remote API requests.

### 5.4 Dependencies / Config

- No new Go dependencies; uses `crypto/subtle` and `crypto/rand` from stdlib.
- New config fields:
  - htmlservd: `api.token` (string, required for production; startup warning if empty).
  - htmlctl context: `token` (string, read from context config).

## 6. Acceptance Criteria

- [x] AC-1: All routes under `/api/v1/` return `401 Unauthorized` when the `Authorization` header is absent or contains an incorrect bearer token.
- [x] AC-2: Requests with the correct `Authorization: Bearer <token>` header are passed to the handler unchanged.
- [x] AC-3: `/healthz` and `/readyz` endpoints respond normally without any `Authorization` header.
- [x] AC-4: Token comparison uses `crypto/subtle.ConstantTimeCompare`; no token material appears in logs.
- [x] AC-5: `htmlctl` reads the token from the active context config and sends it as a bearer token on every API request.
- [x] AC-6: `htmlctl context set --token <value>` stores the token in the context; `htmlctl context token generate` prints a fresh 32-byte hex token.
- [x] AC-7: The `Actor` field in audit log entries is only set from a request that has already passed authentication; unauthenticated requests cannot write audit entries.
- [x] AC-8: If `api.token` is not configured in htmlservd, a prominent startup warning is logged and the server still starts (allowing zero-downtime rollout).
- [x] AC-9: All existing API integration tests pass when updated to supply the auth header.

## 7. Verification Plan

### Automated Tests

- [x] `internal/server/auth_test.go` — all auth middleware cases pass.
- [x] `go test ./...` — full suite passes with auth-enabled server/client wiring.
- [x] `internal/cli/context_cmd_test.go` — token generate produces a 64-character hex string; token set round-trips through context config.

### Manual Tests

- [ ] Start htmlservd with a configured token; attempt `curl -X POST http://127.0.0.1:9400/api/v1/websites/x/environments/y/apply` — expect 401.
- [ ] Retry with `-H "Authorization: Bearer <correct-token>"` — expect 400 or 200 (no longer 401).
- [ ] Confirm `/healthz` returns 200 without any auth header.
- [ ] Run `htmlctl apply` with context pointing at a server with token — expect success.
- [ ] Run `htmlctl apply` with context missing the token — expect a clear error.

## 8. Performance / Reliability Considerations

- `ConstantTimeCompare` adds negligible overhead (nanoseconds) compared to SSH tunnel latency.
- Tokens are compared in memory — no database or filesystem lookup per request.

## 9. Risks & Mitigations

- **Risk:** Existing deployments break immediately if token is required without a migration path. **Mitigation:** Startup warning (not fatal) when token is unconfigured; document the rollout sequence (generate token → configure server → update client contexts).
- **Risk:** Token stored in plaintext in context config file. **Mitigation:** Document that the context config file should have mode `0600`; the existing config loader already writes with `0600` permissions. Token-at-rest encryption is out of scope for v1.
- **Risk:** Bearer token intercepted in transit. **Mitigation:** Tokens only travel inside the SSH tunnel (encrypted transport), so interception requires SSH compromise. Non-tunnel deployments should enforce TLS.

## 10. Open Questions

- Should an empty `api.token` be a fatal startup error (more secure) or a warning (easier rollout)? Leaning toward warning for v1 with a clear `--require-auth` flag to enforce fatal mode.
- Should the token be per-context or per-server? Per-context allows different clients to use different tokens; per-server is simpler. Decision deferred to implementation — per-context is the more flexible default.

---

## Implementation Summary

- Added server auth middleware in `internal/server/auth.go` that:
  - parses `Authorization: Bearer <token>`,
  - performs constant-time token comparison (digest + `subtle.ConstantTimeCompare`),
  - returns `401 {"error":"unauthorized"}` on failure,
  - marks request context as actor-trusted only after authentication.
- Wrapped all API routes with auth in `internal/server/server.go` while leaving `/healthz`, `/readyz`, and `/version` unauthenticated.
- Added server config support for API token:
  - `api.token` in YAML config,
  - `HTMLSERVD_API_TOKEN` env override,
  - startup warning when unset,
  - `--require-auth` startup gate in `cmd/htmlservd/main.go`.
- Updated actor trust boundary:
  - `actorFromRequest` now only trusts `X-Actor` when middleware marks the request as authenticated.
- Updated client/context wiring:
  - `Context.Token` and `ContextInfo.Token` in `internal/config`,
  - bearer token sent by `internal/client/client.go`,
  - CLI uses context name as actor + context token as bearer credential via `client.NewWithAuth`.
- Added CLI token utilities:
  - `htmlctl context set <name> --token <value>`
  - `htmlctl context token generate`

## Code Review Findings

- No blocking defects found after implementation and verification.
- Full test suite passes with auth changes (`go test ./...`).

## Completion Status

- Implemented and verified.
