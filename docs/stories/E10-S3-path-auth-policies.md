# E10-S3 — Path-Based Auth Policies

**Epic:** Epic 10 — Review, Automation, and Lifecycle
**Status:** Implemented (2026-03-03)
**Priority:** P1
**Estimated Effort:** 4-5 days
**Dependencies:** E5-S2 (Caddy generation), E6-S1 (API auth), E9-S2 (backend-aware routing)
**Target:** `internal/db/`, `internal/server/`, `internal/caddy/`, `internal/client/`, `internal/cli/`
**Design Reference:** PRD 10.2 visitor auth roadmap item

---

## 1. Summary

Add environment-scoped auth policies that protect selected path prefixes with HTTP Basic Auth. This provides a first useful gating mechanism for docs, private previews, and semi-private site areas without introducing a separate auth service.

## 2. Architecture Context and Reuse Guidance

- Treat auth policies like backends: runtime environment config, not bundle content. Credentials differ per environment and must not be promoted with releases.
- Reuse canonical path-prefix validation from `internal/backend` so auth and backend routing follow the same matcher shape.
- Reuse Caddy as the enforcement point. `htmlservd` should configure policies, not proxy credentials itself.
- Keep v1 scope to **one credential per protected prefix** and **Basic Auth only**.
- Never store plaintext passwords. The CLI hashes with bcrypt and the server stores only the hash.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Policy model

Add `auth_policies` table:

- `id INTEGER PRIMARY KEY`
- `environment_id INTEGER NOT NULL`
- `path_prefix TEXT NOT NULL`
- `username TEXT NOT NULL`
- `password_hash TEXT NOT NULL`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

Unique constraint on `(environment_id, path_prefix)`.

Validation:

- `path_prefix` must be canonical `/<segment>/*`
- `username` non-empty, ASCII, max 128 chars
- `password_hash` must be bcrypt and within accepted cost bounds

### 3.2 API and CLI

Endpoints:

- `POST /api/v1/websites/{website}/environments/{env}/auth-policies`
- `GET /api/v1/websites/{website}/environments/{env}/auth-policies`
- `DELETE /api/v1/websites/{website}/environments/{env}/auth-policies?path=...`

CLI:

- `htmlctl authpolicy add website/<name> --env prod --path /docs/* --username reviewer --password-stdin`
- `htmlctl authpolicy list website/<name> --env prod`
- `htmlctl authpolicy remove website/<name> --env prod --path /docs/*`

`list` must never return the password hash.

### 3.3 Caddy route generation

Extend `internal/caddy` from simple site-level directives to deterministic route blocks:

- exact protected backend prefix:
  - `basic_auth`
  - `reverse_proxy`
- exact protected static prefix:
  - `basic_auth`
  - `root *`
  - `file_server`
- remaining unprotected backends
- final unprotected `file_server`

To keep generation unambiguous in v1:

- reject overlapping auth policy prefixes within one environment
- allow exact prefix match between one auth policy and one backend
- do not attempt nested/overlapping auth trees

## 4. File Touch List

### Files to Create

- `internal/server/auth_policies.go`
- `internal/server/auth_policies_test.go`
- `internal/cli/authpolicy_cmd.go`
- `internal/cli/authpolicy_cmd_test.go`

### Files to Modify

- `internal/db/migrations/009_auth_policies.go`
- `internal/db/models.go`
- `internal/db/queries.go`
- `internal/db/queries_test.go`
- `internal/caddy/config.go`
- `internal/caddy/config_test.go`
- `internal/server/caddy.go`
- `internal/server/routes.go`
- `internal/client/client.go`
- `internal/client/types.go`
- `internal/cli/root.go`
- `docs/technical-spec.md`

## 5. Implementation Steps

1. Add migration 009 and query helpers.
2. Add validation helpers for username and bcrypt hash format/cost.
3. Implement server handlers with authenticated CRUD and audit logging.
4. Implement CLI commands:
   - read password from stdin without echoing it back
   - hash locally with bcrypt before sending to the server
5. Extend Caddy config generation to emit deterministic protected route blocks.
6. Reject overlapping auth-policy prefixes before writing config.
7. Trigger Caddy reload after add/remove using the same path as domain/backend mutations.
8. Emit audit operations `authpolicy.add` and `authpolicy.remove` without serializing password hash material.

## 6. Tests and Validation

### Automated

- DB tests:
  - add/list/remove policies
  - unique path constraint
- Server tests:
  - unauthenticated requests return `401`
  - invalid path/user/hash return `400`
  - list omits hash material
  - delete missing path returns `404`
  - audit entries omit password hash material
  - sanitized 5xx responses
- Caddy tests:
  - protected backend prefix emits `basic_auth` before `reverse_proxy`
  - protected static prefix emits `basic_auth` before `file_server`
  - overlapping prefixes are rejected
- CLI tests:
  - `--password-stdin` required for add
  - hashes are not printed in table/json output

### Manual

- Add `/docs/*` policy and verify browser prompts for credentials.
- Add policy on a path with a backend and verify auth happens before proxying.
- Remove policy and verify the route becomes public again.

## 7. Acceptance Criteria

- [ ] AC-1: Operators can add, list, and remove Basic Auth policies per environment and path prefix.
- [ ] AC-2: Passwords are hashed client-side and never stored or returned in plaintext.
- [ ] AC-3: Matching requests are challenged by Caddy before reaching `reverse_proxy` or `file_server`.
- [ ] AC-4: Auth policies remain environment-scoped and are not affected by release promotion.
- [ ] AC-5: Overlapping auth prefixes are rejected with a clear `400`.
- [ ] AC-6: `htmlctl authpolicy list` omits hash material from human and structured output.
- [ ] AC-7: All API routes remain bearer-authenticated and 5xx responses stay sanitized.
- [ ] AC-8: Add/remove operations are audit-logged without storing or emitting password hash material in summaries/metadata.
- [ ] AC-9: `go test -race ./internal/server/... ./internal/caddy/... ./internal/cli/...` passes.

## 8. Risks and Open Questions

### Risks

- **Credential leakage via logs or JSON output.**
  Mitigation: never log or serialize the bcrypt hash outside DB/Caddy generation code paths.
- **Ambiguous route precedence with overlapping prefixes.**
  Mitigation: reject overlaps in v1 instead of trying to infer nested auth behavior.
- **Operator expects multi-user or federated auth.**
  Mitigation: explicitly scope v1 to one bcrypt credential per prefix; forward-auth/OIDC is separate future work.

### Open Questions

- None blocking. v1 is intentionally Basic Auth only.
