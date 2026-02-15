# E3-S1 - Context Config + Selection

**Epic:** Epic 3 — Remote transport + kubectl UX
**Status:** Not Started
**Priority:** P1 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E1-S4 (CLI framework)
**Target:** htmlctl v1
**Design Reference:** Technical Spec Section 9.1 (Config / contexts)

---

## 1. Objective

Provide a kubectl-style context configuration system so that htmlctl commands can target different remote servers, websites, and environments without repeating connection details on every invocation. This is the foundational layer for all remote operations — every subsequent remote command depends on resolving a context to a server URL, website name, and environment.

## 2. User Story

As an operator or AI agent, I want to configure named contexts (e.g., "staging", "prod") in a config file so that I can switch between remote targets with `--context <name>` instead of passing server/website/environment on every command.

## 3. Scope

### In Scope

- Define config file format (`~/.htmlctl/config.yaml`) with `current-context` and `contexts` list
- Each context contains: `name`, `server` (ssh:// URL), `website`, `environment`
- Load and parse config file at CLI startup
- `--context` flag on all remote commands to override `current-context`
- Resolve context to a `ContextInfo` struct usable by transport and command layers
- Error handling: missing config file, missing context, malformed YAML
- Support `HTMLCTL_CONFIG` environment variable to override default config path
- `htmlctl config view` — print current config
- `htmlctl config current-context` — print active context name
- `htmlctl config use-context <name>` — switch current-context

### Out of Scope

- XDG config path resolution (use `~/.htmlctl/` only in v1; XDG can be added later)
- `htmlctl config set-context` interactive editing (users edit YAML directly in v1)
- Token-based auth fields in context (post-v1)
- Config file locking for concurrent writes
- Config file creation wizard / init command

## 4. Architecture Alignment

- **PRD Section 9.1**: Defines the config format with `current-context`, `contexts[]` containing `name`, `server`, `website`, `environment`.
- **Technical Spec Section 7**: Control plane security model — server URLs use `ssh://` scheme; config must preserve this for the SSH tunnel transport layer (E3-S2).
- **CLI Framework (E1-S4)**: The config loader integrates with the CLI framework established in E1-S4. The `--context` flag must be registered as a persistent/global flag available to all remote subcommands.
- **Component boundary**: Config loading is a standalone package (`internal/config/`) that exports a `Config` struct and loader function. It has no dependency on transport or server packages.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/config/types.go` — `Config`, `Context` struct definitions
- `internal/config/loader.go` — Load config from file path, resolve home dir, parse YAML
- `internal/config/resolve.go` — Resolve context by name (explicit or current-context), return `ContextInfo`
- `cmd/htmlctl/config_cmd.go` — `config view`, `config current-context`, `config use-context` subcommands

### 5.2 Files to Modify

- `cmd/htmlctl/root.go` — Add `--context` persistent flag, wire config loader into command tree
- `go.mod` — Add `gopkg.in/yaml.v3` dependency (if not already present)

### 5.3 Tests to Add

- `internal/config/loader_test.go` — Test loading valid config, missing file, malformed YAML, empty contexts
- `internal/config/resolve_test.go` — Test resolving by name, resolving current-context, missing context error, `--context` override
- `cmd/htmlctl/config_cmd_test.go` — Integration tests for config subcommands

### 5.4 Dependencies/Config

- `go.mod` — `gopkg.in/yaml.v3` for YAML parsing
- No CGO dependencies

## 6. Acceptance Criteria

- [ ] AC-1: Config file at `~/.htmlctl/config.yaml` is loaded and parsed on CLI startup without error when valid
- [ ] AC-2: Each context entry contains `name`, `server`, `website`, and `environment` fields; missing required fields produce a clear error message
- [ ] AC-3: `--context <name>` flag overrides `current-context` for any remote command
- [ ] AC-4: When no `--context` flag is provided, `current-context` from the config file is used
- [ ] AC-5: When the config file is missing, remote commands fail with a helpful error message suggesting config file creation
- [ ] AC-6: When a referenced context name does not exist in the contexts list, a clear error is returned listing available contexts
- [ ] AC-7: `HTMLCTL_CONFIG` environment variable overrides the default config path
- [ ] AC-8: `htmlctl config view` prints the loaded config in YAML format
- [ ] AC-9: `htmlctl config current-context` prints the active context name
- [ ] AC-10: `htmlctl config use-context <name>` updates `current-context` in the config file and confirms the change
- [ ] AC-11: Config file with `server: ssh://root@yourserver` correctly preserves the SSH URL for downstream transport use

## 7. Verification Plan

### Automated Tests

- [ ] Unit tests for YAML parsing with valid, invalid, and edge-case config files
- [ ] Unit tests for context resolution (by name, by current-context, missing context)
- [ ] Unit tests for `HTMLCTL_CONFIG` env var override
- [ ] Unit tests for `use-context` write-back to file
- [ ] Integration test: CLI loads config and `--context` flag correctly selects context

### Manual Tests

- [ ] Create a sample `~/.htmlctl/config.yaml` and run `htmlctl config view` to confirm output
- [ ] Run `htmlctl config use-context prod` and verify the file is updated
- [ ] Run a remote command with `--context staging` and verify the correct context is resolved
- [ ] Run with a missing config file and verify the error message is helpful

## 8. Performance / Reliability Considerations

- Config file is small (typically < 1KB); parsing latency is negligible.
- Config is loaded once per CLI invocation; no caching or hot-reload needed.
- `use-context` writes back the entire file; no partial update. This is acceptable for v1 given the small file size and single-user access pattern.

## 9. Risks & Mitigations

- **Risk:** Config file format drift between versions. **Mitigation:** Use a simple, flat YAML schema with no nesting beyond contexts list. Add an `apiVersion` field for future migration support.
- **Risk:** Home directory resolution fails in non-standard environments (containers, CI). **Mitigation:** Always support `HTMLCTL_CONFIG` env var as escape hatch; use `os.UserHomeDir()` with fallback.
- **Risk:** `use-context` clobbers manual comments in YAML. **Mitigation:** Document this limitation; v1 accepts it. Future versions could use a round-trip YAML library.

## 10. Open Questions

- Should the config support multiple servers per context (e.g., for HA setups)? **Tentative answer:** No, single server per context in v1.
- Should we add an `apiVersion` field to the config file for forward compatibility? **Tentative answer:** Yes, add `apiVersion: htmlctl.dev/v1` but do not enforce it in v1.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
