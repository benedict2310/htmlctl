# E9-S2 — Backend-Aware Caddy Site Block Generation

**Epic:** Epic 9 — Environment Backends
**Status:** Planned
**Priority:** P1 (Critical Path)
**Estimated Effort:** 1 day
**Dependencies:** E9-S1 (backend data model), E5-S2 (Caddy config generation)
**Target:** `internal/caddy/`, `internal/server/caddy.go`
**Design Reference:** Architecture discussion 2026-03-01

---

## 1. Objective

Extend Caddyfile generation to include `reverse_proxy` directives for any backends declared on an environment, so that path-prefixed requests are forwarded to the operator-configured upstream while all other paths continue to be served from the static release.

## 2. User Story

As an operator, I want Caddy to automatically route `/api/*` to my backend service when I declare a backend for my environment, so that my static site can call relative API paths without any manual Caddyfile editing.

## 3. Background

E5-S2 generates Caddyfile site blocks with a single `file_server` directive. E9-S1 introduces `environment_backends` — per-environment reverse-proxy declarations. This story wires the two together: when an environment has backends, the generated site block gains `reverse_proxy` directives placed *before* `file_server` so that Caddy's directive ordering evaluates them first.

The static serving invariant is preserved: `file_server` remains the terminal handler. Backends only intercept the declared path prefixes.

## 4. Scope

### In Scope

- Extend `internal/caddy` `Site` struct with a `Backends []Backend` field.
- Update the Caddyfile template to emit `reverse_proxy {path_prefix} {upstream}` directives, one per backend, sorted by `path_prefix` for determinism.
- Update `internal/server/caddy.go` to load backends from DB and attach them to each `Site` before passing to the generator.
- Unit tests covering site blocks with zero, one, and multiple backends; mixed environments.

### Out of Scope

- API handlers or CLI commands for managing backends (E9-S3).
- Authentication directives (`forward_auth`, `basicauth`) — future story.
- Path rewriting or header manipulation.
- Upstream health checks or load balancing (multiple upstreams per prefix).

## 5. Architecture Alignment

- **Caddy directive order:** `reverse_proxy` must appear before `file_server` within a site block. Caddy's default directive order places `reverse_proxy` before `file_server`, so no explicit `order` global directive is needed — but the template must emit `reverse_proxy` before `file_server` for clarity.
- **Determinism:** backends are sorted by `path_prefix` ascending before template rendering. `ListBackendsByEnvironment` (E9-S1) already returns rows in `path_prefix` order; the caddy layer sorts again defensively.
- **Promotion invariant:** this story adds no release artifact changes. Generated Caddyfile is server configuration, not a release file.
- **Caddyfile format:** continues to use the Caddyfile DSL (not Caddy JSON API) for consistency with E5-S2.

## 6. Generated Output

### Site Block With No Backends (Current Behaviour, Unchanged)

```caddy
example.com {
    root * /var/lib/htmlservd/websites/sample/envs/prod/current
    file_server
}
```

### Site Block With Backends

```caddy
example.com {
    root * /var/lib/htmlservd/websites/sample/envs/prod/current
    reverse_proxy /api/* https://api.example.com
    reverse_proxy /auth/* https://auth.example.com
    file_server
}
```

Multiple domains bound to the same environment each get the same set of backend directives.

## 7. Implementation Plan

### 7.1 Files to Modify

- `internal/caddy/config.go` — extend `Site` struct; update template.
- `internal/caddy/config_test.go` — add backend rendering tests.
- `internal/server/caddy.go` — load backends per environment from DB and attach to `Site`.
- `internal/server/caddy_test.go` — add tests for backend-aware config generation via server helper.

### 7.2 `internal/caddy` Changes

**Extend `Site` struct:**

```go
type Backend struct {
    PathPrefix string
    Upstream   string
}

type Site struct {
    Domain   string
    Root     string
    Backends []Backend // sorted by PathPrefix; empty = static only
}
```

**Update template** (extend the existing site block template):

```
{{ .Domain }} {
    root * {{ .Root }}
    {{- range .Backends }}
    reverse_proxy {{ .PathPrefix }} {{ .Upstream }}
    {{- end }}
    file_server
}
```

**Sorting:** before rendering, sort `site.Backends` by `PathPrefix` ascending. `GenerateConfig` receives `[]Site`; it is the caller's responsibility to populate `Backends` in order (or sort defensively inside `GenerateConfig`).

### 7.3 `internal/server/caddy.go` Changes

The existing `buildCaddySites` (or equivalent) function resolves domain bindings from DB and maps them to `caddy.Site`. Extend it to also call `q.ListBackendsByEnvironment(ctx, env.ID)` and populate `site.Backends` for each site.

Group by environment ID to avoid N+1: load backends for all environments in a single pass before iterating domain bindings.

## 8. Acceptance Criteria

- [ ] AC-1: A site with no backends generates a Caddyfile block identical to the current output (regression-free).
- [ ] AC-2: A site with one backend generates a block containing exactly one `reverse_proxy` directive before `file_server`.
- [ ] AC-3: A site with multiple backends generates `reverse_proxy` directives sorted by `path_prefix` ascending.
- [ ] AC-4: Two domain bindings for the same environment both include the same set of backend directives.
- [ ] AC-5: Generated Caddyfile is deterministic: identical input always produces identical output.
- [ ] AC-6: `go test ./internal/caddy/... ./internal/server/...` passes; `go test -race` passes.
- [ ] AC-7: No changes to the existing `file_server` behaviour or domain-binding semantics.

## 9. Tests to Add

- `internal/caddy/config_test.go`:
  - Site with zero backends renders unchanged site block.
  - Site with one backend renders correct `reverse_proxy` directive before `file_server`.
  - Site with multiple backends renders them sorted by path prefix.
  - Two sites in same config, one with backends and one without, both render correctly.
- `internal/server/caddy_test.go`:
  - Environment with one backend produces correct site block.
  - Multiple domain bindings on the same environment share the same backend directives.

## 10. Risks and Mitigations

- **Risk:** `reverse_proxy` directive with a trailing slash on `path_prefix` vs without (e.g. `/api/` vs `/api/*`) behaves differently in Caddy. **Mitigation:** document the expected format in AC-5 of E9-S1 and validate accordingly; add a template test with each variant.
- **Risk:** Caddy fails to reload after generating a config with an unreachable upstream. **Mitigation:** Caddy accepts the config and returns 502 at request time — this is expected and acceptable. Caddy does not validate upstream reachability at config load time.
