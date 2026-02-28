# Repository Guidelines

> This file is the single source of truth. `CLAUDE.md` is a symlink to it.

## Project Overview

`htmlctl` / `htmlservd` is a Go static-site deployment system:
- `htmlctl` — CLI tool (client) for applying bundles, managing releases, rollback, promotion, and domain bindings.
- `htmlservd` — Server daemon that stores desired state in SQLite, builds releases, and reloads Caddy for TLS termination.

The full implementation is complete through **Epic 6 (Security Hardening)**. See `docs/epics.md` for the epic/story map.

## Repository Layout

```
cmd/
  htmlctl/        CLI entry point
  htmlservd/      Daemon entry point
internal/
  audit/          Structured audit log (SQLite-backed)
  blob/           Content-addressable blob store (SHA-256)
  bundle/         Bundle ingestion and validation
  caddy/          Caddyfile generation and safe reload
  cli/            Shared CLI helpers and output formatting
  client/         HTTP client for htmlservd API (Bearer auth)
  config/         Config file parsing (YAML, env overrides)
  db/             SQLite query layer and migrations
  diff/           Desired-state diff engine
  domain/         Domain binding logic
  names/          Resource name validation (allowlist regex)
  output/         Machine-parseable output formatting
  release/        Release builder and activation
  server/         HTTP API handlers, auth middleware, error helpers
  state/          Desired-state manifest management
  transport/      SSH tunnel transport (host-key verification, agent socket)
pkg/
  loader/         YAML resource loader
  model/          Shared data model types
  renderer/       Deterministic HTML renderer (html/template)
  validator/      Component and page validation (XSS, event-handler checks)
docs/
  prd.md                 Product requirements
  technical-spec.md      Architecture and API spec
  epics.md               Epic & story map (source of truth for delivery status)
  stories/               Story files: E<epic>-S<story>-<slug>.md
  review-logs/           Post-implementation and security review logs
  guides/                Operator guides
  reference/             Reference documentation
docker/                  Dockerfile and entrypoint scripts for htmlservd container
scripts/                 Utility scripts
testdata/                Fixtures used by tests
.agent/skills/           Contributor automation and shared skills (htmlctl-publish, etc.)
```

## Build & Test Commands

```bash
# Build all binaries
go build ./...
# or
make build

# Run all tests
go test ./...
# or
make test

# Run with race detector (required before merging server changes)
go test -race ./...

# Static analysis
go vet ./...

# Lint (if golangci-lint is installed)
make lint
```

Docker-based verification (matches CI environment, `golang:1.24`):
```bash
docker run --rm -it -v "$PWD":/work -w /work golang:1.24 bash -lc \
  'export PATH=/usr/local/go/bin:$PATH; go test -race ./...'
```

Notes:
- **Always run `go test -race ./...`** before merging any changes to `internal/server/` — the race detector caught a real production concurrency bug (see `docs/review-logs/E6-post-epic-audit-2026-02-23.log`).
- For manual SSH-tunnel end-to-end testing (`htmlctl` → SSH → `htmlservd`), prefer host execution; containers require SSH agent forwarding, `known_hosts`, and a reachable SSH server.
- Preflight check: `.agent/skills/implement-story/scripts/preflight.sh <story-file> --quiet --no-color`
- Project map: `.agent/skills/implement-story/scripts/project-map.sh --summary --no-color`

## Coding Style & Conventions

- **Go formatting:** `gofmt` defaults, lowercase package names, `_test.go` suffixes.
- **Error handling:** wrap with `fmt.Errorf("context: %w", err)`; use `writeInternalAPIError` for all HTTP 5xx responses (never expose internal error text to clients).
- **Input validation:** all resource names must pass `internal/names` validation (`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`, max 128 chars) before database writes.
- **SQL:** use `?` parameterization for all values; structural identifiers (table/column names) must be checked against the allowlist in `internal/db/queries.go` before interpolation.
- **Templates:** use `html/template` (never `text/template`) for any HTML rendering; mark trusted raw HTML explicitly with `template.HTML`.
- **Concurrency:** guard shared server fields with `sync.RWMutex`; use `crypto/subtle.ConstantTimeCompare` for token comparison.
- **Multiword filenames:** follow existing project convention (for example, `rollback_history.go`, `domain_verify.go`).
- **CLI output:** keep deterministic and machine-parseable; see `docs/technical-spec.md`.

## Testing Guidelines

- Prefer table-driven unit tests with `_test.go` suffixes in the same package.
- Add targeted integration tests for CLI/API behaviour changes.
- Prioritize safety-critical scenarios: deterministic rendering, atomic release activation, rollback, promotion hash parity, and auth boundary correctness.
- Every new HTTP handler must have a corresponding sanitized-error test (verify 5xx responses contain no internal paths, IDs, or schema details).
- Acceptance criteria in story files define the minimum test coverage — untested ACs are considered incomplete.

## Security Guidelines

Epic 6 addressed 16 findings from the 2026-02-20 security audit. Maintain these invariants:

| Area | Invariant |
|------|-----------|
| Authentication | All `/api/v1/*` routes require `Authorization: Bearer <token>`; health routes are unauthenticated |
| Actor trust | `X-Actor` header is trusted only after authentication via `actorTrusted` context flag |
| Token comparison | Always use `crypto/subtle.ConstantTimeCompare`; never log token material |
| Input names | Validate with `internal/names` before any DB write or path construction |
| HTML rendering | Use `html/template`; reject `on*` event-handler attributes at validation time |
| SSH transport | Always derive host-key callback from `known_hosts`; validate agent socket ownership |
| Error responses | HTTP 5xx: generic message only (log full error server-side via `writeInternalAPIError`) |
| SQL identifiers | Check table/column against `allowedDeleteTargets` allowlist before interpolation |
| Container | Non-root UID 10001; `restrict,port-forwarding` on authorized_keys; `PermitTunnel no` |

## Commit & Pull Request Guidelines

- Commit message format: `<type>(<scope>): <imperative summary>`
- Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`
- Branch naming: `feat/<story-id>-<short-name>`, `fix/<issue>-<short-name>`, `docs/<topic>`
- PRs must reference the linked story file, list acceptance criteria status, and include test evidence (commands run + outcomes).
- Include screenshots only when changing rendered site output.
- Do not use destructive git operations (`--force`, `reset --hard`, `clean -f`) without explicit approval.
- Run `go test -race ./...` and confirm clean before requesting review.
