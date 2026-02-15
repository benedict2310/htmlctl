# E1-S4 - Local CLI Commands (render + serve)

**Epic:** Epic 1 - Foundations: Repo schema + local render
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E1-S1, E1-S2, E1-S3
**Target:** Go CLI (htmlctl)
**Design Reference:** docs/technical-spec.md section 9.2, docs/prd.md section 5 (journey 1)

---

## 1. Objective

Wire the resource loader, validator, and renderer into a usable CLI binary (`htmlctl`) with two local commands: `render` and `serve`. This completes Epic 1 by delivering the first user journey from the PRD -- local preview. An agent (or developer) can point `htmlctl render` at a site directory, get deterministic static output, and immediately preview it with `htmlctl serve`. This is the integration point where all prior E1 stories come together into a working tool.

## 2. User Story

As an AI agent managing a website, I want to run `htmlctl render -f ./site -o ./dist` to produce static HTML output and `htmlctl serve ./dist --port 8080` to preview it locally, so that I can verify changes before deploying to staging.

## 3. Scope

### In Scope

- **CLI framework setup**: initialize `htmlctl` binary using `spf13/cobra` for command parsing, help text, and flag handling
- **`htmlctl render` command**: accepts `-f` (source site directory) and `-o` (output directory) flags; loads the site (E1-S1), validates all components (E1-S3), renders to output (E1-S2); exits with non-zero code on validation or render errors
- **`htmlctl serve` command**: accepts a positional argument for the directory to serve and `--port` flag (default 8080); starts a simple HTTP static file server; logs requests to stdout; runs until interrupted (Ctrl+C / SIGINT)
- **`htmlctl version` command**: prints the binary version (set at build time via ldflags)
- **Root command with help**: `htmlctl` with no arguments prints usage and available commands
- **Error output**: validation errors and render errors printed to stderr with clear formatting; exit code 1 on failure
- **Render summary output**: on success, print number of pages rendered and output directory path to stdout
- **Signal handling for serve**: graceful shutdown on SIGINT/SIGTERM
- **`main.go` entry point**: minimal main package that invokes the cobra root command

### Out of Scope

- Remote commands (diff, apply, status, promote, rollback, logs) -- these require Epic 2 and Epic 3
- Context/config management (`~/.htmlctl/config.yaml`) -- deferred to E3-S1
- Domain commands -- deferred to Epic 5
- Watch mode / auto-reload on file changes (nice-to-have, not v1)
- HTTPS serving (local serve is HTTP only; HTTPS is via Caddy in production)
- Colored terminal output or progress bars (keep it simple for agent consumption)
- Dry-run mode for render (could add later)

## 4. Architecture Alignment

- **CLI design**: Follows technical-spec.md section 9.2 exactly: `htmlctl render -f ./site -o ./dist` and `htmlctl serve ./dist --port 8080`
- **User journey**: Implements PRD section 5, journey 1 (Local preview: render + serve locally)
- **Integration point**: Wires together `pkg/loader` (E1-S1), `pkg/validator` (E1-S3), and `pkg/renderer` (E1-S2) into the CLI execution pipeline
- **Package boundary**: Creates `cmd/htmlctl/` for the binary entry point and `internal/cli/` for command definitions; commands call into the existing `pkg/` packages
- **Go binary**: Per technical-spec.md section 10, prefer Go for the CLI
- **Agent-friendly output**: The CLI is designed for AI agents (PRD section 4); output should be structured, parseable, and not decorative

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `cmd/htmlctl/main.go` - Entry point: calls cobra root command Execute()
- `internal/cli/root.go` - Root cobra command with version flag and usage help
- `internal/cli/render.go` - `render` subcommand: parse flags, load site, validate, render, report results
- `internal/cli/serve.go` - `serve` subcommand: parse flags, start HTTP file server, handle graceful shutdown
- `internal/cli/version.go` - `version` subcommand: print version info (set via ldflags at build time)
- `Makefile` - Build targets: `build`, `test`, `lint`; passes version via `-ldflags`

### 5.2 Files to Modify

- `go.mod` - Add `github.com/spf13/cobra` dependency

### 5.3 Tests to Add

- `internal/cli/render_test.go` - Integration tests: run render command against testdata fixtures, verify exit code and output directory contents
- `internal/cli/serve_test.go` - Integration tests: start serve command, make HTTP request, verify response, shut down
- `internal/cli/root_test.go` - Test: running with no args prints usage without error
- `internal/cli/version_test.go` - Test: version command prints version string
- `testdata/valid-site/` - Reuse fixture from E1-S1 (complete valid site for end-to-end render test)
- `testdata/invalid-site/` - Fixture with validation errors to test error reporting

### 5.4 Dependencies/Config

- `github.com/spf13/cobra` - CLI framework (standard in Go ecosystem; used by kubectl, hugo, docker CLI)
- Go standard library `net/http` for static file serving
- Go standard library `os/signal` for graceful shutdown

## 6. Acceptance Criteria

- [ ] AC-1: `htmlctl render -f ./site -o ./dist` loads the site directory, validates components, renders HTML, and writes output to `./dist`
- [ ] AC-2: `htmlctl render` exits with code 0 on success, printing the number of pages rendered and output path
- [ ] AC-3: `htmlctl render` exits with code 1 on validation errors, printing all errors to stderr before exiting
- [ ] AC-4: `htmlctl render` exits with code 1 if the source directory does not exist or is missing `website.yaml`, with a clear error message
- [ ] AC-5: `htmlctl render` exits with code 1 if `-f` flag is omitted, printing usage help
- [ ] AC-6: `htmlctl serve ./dist --port 8080` starts an HTTP server serving static files from `./dist` on port 8080
- [ ] AC-7: `htmlctl serve` responds to HTTP requests with correct content types (HTML, CSS, JS, images)
- [ ] AC-8: `htmlctl serve` logs each HTTP request to stdout (method, path, status code)
- [ ] AC-9: `htmlctl serve` shuts down gracefully on SIGINT (Ctrl+C) without error
- [ ] AC-10: `htmlctl serve` exits with code 1 if the directory argument is missing or the directory does not exist
- [ ] AC-11: `htmlctl serve --port 0` picks a random available port and prints the actual port to stdout
- [ ] AC-12: `htmlctl` with no arguments prints usage listing available commands (render, serve, version)
- [ ] AC-13: `htmlctl version` prints the version string
- [ ] AC-14: `htmlctl render -f ./site -o ./dist` produces the same output as calling `loader.LoadSite()` + `validator.ValidateAllComponents()` + `renderer.Render()` directly (the CLI is a thin wrapper, not a separate code path)

## 7. Verification Plan

### Automated Tests

- [ ] Integration test: run `htmlctl render -f testdata/valid-site -o /tmp/test-output` and verify exit code 0 and output files exist
- [ ] Integration test: run `htmlctl render -f testdata/invalid-site -o /tmp/test-output` and verify exit code 1 and stderr contains validation error messages
- [ ] Integration test: run `htmlctl render` without `-f` flag and verify exit code 1 and stderr contains usage help
- [ ] Integration test: start `htmlctl serve /tmp/test-output --port 0`, make HTTP GET to `/`, verify 200 response with HTML content
- [ ] Integration test: start serve, make HTTP GET to `/nonexistent`, verify 404 response
- [ ] Integration test: start serve, send SIGINT, verify clean shutdown (no error on stderr)
- [ ] Unit test: version command output contains a version string
- [ ] Unit test: root command with no args exits with code 0 and prints help text

### Manual Tests

- [ ] Build `htmlctl` binary, render the sample futurelab site, serve it, and open in a browser to verify the full local preview workflow
- [ ] Verify that render + serve together complete the "local preview" user journey from the PRD

## 8. Performance / Reliability Considerations

- The `render` command should complete in under 1 second for a typical site (tens of pages, tens of components)
- The `serve` command should handle concurrent HTTP requests efficiently (Go's `net/http` handles this natively with goroutines)
- Graceful shutdown should wait for in-flight HTTP requests to complete (use `http.Server.Shutdown()` with a timeout)
- The serve command should bind to `127.0.0.1` by default (not `0.0.0.0`) for security in local preview

## 9. Risks & Mitigations

- **Risk**: Port conflicts if default port 8080 is already in use - **Mitigation**: Print a clear error message when `listen` fails; support `--port 0` for automatic port selection; print the actual bound address on startup
- **Risk**: Cobra adds binary size and dependency surface - **Mitigation**: Cobra is the standard Go CLI framework, well-maintained, and already used by kubectl (our design inspiration); the dependency is justified
- **Risk**: Integration tests involving HTTP servers and signal handling can be flaky - **Mitigation**: Use `--port 0` in tests for automatic port allocation; use context cancellation instead of OS signals in test code; set timeouts on all test HTTP requests
- **Risk**: File permission errors when writing to output directory - **Mitigation**: Check write permissions early; provide a clear error message if the output directory cannot be created

## 10. Open Questions

- Should `htmlctl render` default `-o` to `./dist` if not specified? (Recommendation: yes, for convenience; `./dist` is a conventional output directory name)
- Should `htmlctl serve` support a `--open` flag to automatically open the browser? (Recommendation: no, agents do not use browsers; keep the CLI agent-focused)
- Should render print a machine-parseable summary (JSON) in addition to human-readable output? (Recommendation: defer; add `--output json` flag in a later story if needed)

## 11. Research Notes

### Go CLI Frameworks (spf13/cobra)

- Cobra provides subcommand routing, flag parsing, help generation, and shell completion
- Used by kubectl, docker CLI, hugo, gh (GitHub CLI) -- all kubectl-style tools
- Commands are defined as `cobra.Command` structs with `RunE` functions that return errors
- Persistent flags (on root) apply to all subcommands; local flags apply to one command
- Version info is typically injected via `ldflags`: `go build -ldflags "-X main.version=1.0.0"`
- Cobra automatically generates `--help` and `-h` flags and prints usage on error

### Static File Serving in Go

- `http.FileServer(http.Dir(path))` serves a directory as static files with correct MIME types
- Handles `index.html` directory listings automatically (serves `index.html` for directory paths)
- `http.Server{Addr: addr, Handler: handler}` gives control over shutdown
- `server.Shutdown(ctx)` performs graceful shutdown, waiting for active connections
- Use `net.Listen("tcp", ":0")` to get an OS-assigned free port, then `listener.Addr()` to discover it
- Request logging can be done with a simple middleware wrapping `http.Handler`

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
