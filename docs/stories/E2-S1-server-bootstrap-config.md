# E2-S1 - htmlservd Bootstrap + Config

**Epic:** Epic 2 — Server daemon: state, releases, and API
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E1-S1 (shared resource model / schemas)
**Target:** v1
**Design Reference:** [Technical Spec - Sections 5, 7, 10](../technical-spec.md)

---

## 1. Objective

Stand up the `htmlservd` server daemon with configuration loading, data directory initialization, HTTP API skeleton, and graceful shutdown. This is the foundational story for all server-side functionality -- every subsequent Epic 2 story depends on a running, configurable daemon process.

## 2. User Story

As a self-hosted operator, I want `htmlservd` to start reliably from a config file, bind only to localhost by default, initialize its data directories, and shut down gracefully on signals so that I have a secure, predictable foundation for the website control plane.

## 3. Scope

### In Scope

- `htmlservd` binary entry point (`cmd/htmlservd/main.go`)
- Configuration file loading (YAML) with defaults and environment variable overrides
- Config struct covering: bind address, port, data directory, log level
- Default bind address: `127.0.0.1:9400` (localhost-only, security model)
- Base data directory initialization (`/var/lib/htmlservd/` or configurable path)
  - Create subdirectories: `blobs/sha256/`, `websites/`
  - Create or open `db.sqlite` (placeholder; actual schema is E2-S2)
- HTTP server startup using `net/http` with a health check endpoint (`GET /healthz`)
- Graceful shutdown on SIGINT and SIGTERM (context cancellation, drain in-flight requests)
- Structured logging (JSON format) with configurable log level
- Version/build info endpoint (`GET /version`)

### Out of Scope

- SQLite schema creation (E2-S2)
- API routes beyond `/healthz` and `/version` (later stories)
- TLS termination (handled by Caddy front proxy, Epic 5)
- Authentication / token-based auth (post-v1)
- systemd unit file creation (separate deliverable)
- Remote access / SSH tunnel setup (Epic 3)

## 4. Architecture Alignment

- **Security model (Tech Spec Section 7):** `htmlservd` binds to `127.0.0.1` only by default. Remote access is via SSH tunnel or private network reverse proxy. This story enforces that default.
- **Storage layout (Tech Spec Section 5):** The server initializes the directory tree under `/var/lib/htmlservd/` (or configured `data-dir`). This story creates the skeleton; later stories populate it.
- **Implementation notes (Tech Spec Section 10):** Standard `net/http` for API. Go binary for daemon.
- **Resource model (Tech Spec Section 2):** Config references the resource model from E1-S1 for shared type definitions. The server imports the same Go packages.
- **Concurrency:** The HTTP server uses Go's standard concurrency model. Graceful shutdown uses `context.Context` propagation and `http.Server.Shutdown()`.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `cmd/htmlservd/main.go` — Entry point: parse flags, load config, start server, handle signals
- `internal/server/server.go` — HTTP server setup, router registration, graceful lifecycle
- `internal/server/config.go` — Config struct, YAML loading, defaults, env var overrides
- `internal/server/health.go` — `/healthz` and `/version` handlers
- `internal/server/datadir.go` — Data directory initialization and validation

### 5.2 Files to Modify

- `go.mod` — Add dependencies (YAML parser: `gopkg.in/yaml.v3`)

### 5.3 Tests to Add

- `internal/server/config_test.go` — Config loading: defaults, file override, env var override, invalid config
- `internal/server/server_test.go` — Server start/stop lifecycle, health endpoint returns 200
- `internal/server/datadir_test.go` — Directory creation, idempotent re-initialization, permission errors

### 5.4 Dependencies/Config

- `gopkg.in/yaml.v3` — YAML config parsing
- `log/slog` — Structured logging (Go stdlib, no external dep)
- Standard library: `net/http`, `os/signal`, `context`, `os`, `path/filepath`

## 6. Acceptance Criteria

- [ ] AC-1: `htmlservd` starts and listens on `127.0.0.1:9400` by default when no config file is provided
- [ ] AC-2: `htmlservd --config /path/to/config.yaml` loads bind address, port, data directory, and log level from the file
- [ ] AC-3: Environment variables (`HTMLSERVD_BIND`, `HTMLSERVD_PORT`, `HTMLSERVD_DATA_DIR`, `HTMLSERVD_LOG_LEVEL`) override config file values
- [ ] AC-4: On startup, the data directory tree is created if it does not exist (`blobs/sha256/`, `websites/`)
- [ ] AC-5: `GET /healthz` returns HTTP 200 with `{"status":"ok"}`
- [ ] AC-6: `GET /version` returns HTTP 200 with build version information
- [ ] AC-7: Sending SIGINT or SIGTERM causes the server to drain in-flight requests and exit cleanly within 10 seconds
- [ ] AC-8: If the configured port is already in use, the server exits with a clear error message and non-zero exit code
- [ ] AC-9: Structured JSON logs are written to stderr with configurable log level (debug, info, warn, error)
- [ ] AC-10: The server refuses to bind to `0.0.0.0` unless explicitly configured (logged warning when binding to non-loopback address)

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for config loading (defaults, file, env vars, precedence)
- [ ] Unit tests for data directory initialization (creation, idempotency, error handling)
- [ ] Integration test: start server, hit `/healthz`, verify 200 response
- [ ] Integration test: start server, send SIGTERM, verify clean shutdown

### Manual Tests

- [ ] Run `htmlservd` with no arguments; confirm it binds to `127.0.0.1:9400`
- [ ] Run `htmlservd --config sample.yaml` with custom port; confirm binding
- [ ] Verify data directory tree is created at configured location
- [ ] `curl http://127.0.0.1:9400/healthz` returns OK
- [ ] Kill process with `kill -TERM`; confirm graceful exit in logs

## 8. Performance / Reliability Considerations

- Server should start in under 500ms (no heavy initialization in this story)
- Graceful shutdown timeout: 10 seconds maximum before force exit
- Data directory initialization must be idempotent (safe to restart)
- Health check endpoint must respond in under 10ms

## 9. Risks & Mitigations

- **Risk:** Config file format changes between stories — **Mitigation:** Keep config struct extensible; use YAML with optional fields and sensible defaults.
- **Risk:** Data directory permissions on different OS/environments — **Mitigation:** Check and report permission errors clearly at startup; do not silently fail.
- **Risk:** Port conflicts in development — **Mitigation:** Clear error message when port is in use; configurable port via flag/env/config.

## 10. Open Questions

- Should we support a `--foreground` / `--daemon` flag, or always run in foreground (letting systemd/launchd manage daemonization)? Recommendation: foreground only in v1.
- Should config support TOML as an alternative to YAML? Recommendation: YAML only in v1 for consistency with resource manifests.
- Default port number: `9400` is proposed; confirm no conflicts with common services.

## 11. Research Notes

- **Go daemon patterns:** Use `os/signal.NotifyContext` for clean signal handling. Run HTTP server in a goroutine, block on context cancellation, call `server.Shutdown(ctx)`.
- **net/http server setup:** Use `http.Server` struct directly (not `http.ListenAndServe`) to control timeouts (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`).
- **Config file loading:** `gopkg.in/yaml.v3` is the standard choice. Load defaults -> overlay file -> overlay env vars in that precedence order.
- **Structured logging:** Go 1.21+ includes `log/slog` in stdlib. Use `slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: configuredLevel})`.

---

## Implementation Summary

- Implemented `htmlservd` daemon bootstrap and lifecycle:
  - `cmd/htmlservd/main.go` for flag parsing (`--config`), config loading, logger init, and signal-driven run loop.
  - `internal/server/server.go` for HTTP server startup, `/healthz` + `/version` route wiring, graceful shutdown, and port bind handling.
- Implemented config pipeline in `internal/server/config.go`:
  - defaults (`127.0.0.1:9400`, `/var/lib/htmlservd`, `info`),
  - YAML file overlay,
  - environment variable override precedence (`HTMLSERVD_*`),
  - validation and listen-address helpers.
- Implemented data-dir bootstrap in `internal/server/datadir.go`:
  - creates `blobs/sha256/`, `websites/`, and `db.sqlite` placeholder file.
- Implemented structured JSON logging via `log/slog` with configurable levels.
- Updated `Makefile` build target to produce both `bin/htmlctl` and `bin/htmlservd`.

## Code Review Findings

- No critical issues found during implementation pass.
- Added explicit tests for:
  - config precedence and invalid env/file input,
  - data-dir creation/idempotency/error handling,
  - server health/version endpoints,
  - graceful cancellation shutdown path,
  - occupied-port startup failures.

## Completion Status

- Implemented and validated with automated tests (`go test ./...`).
