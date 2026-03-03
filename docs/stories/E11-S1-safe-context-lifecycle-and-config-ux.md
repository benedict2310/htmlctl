# E11-S1 — Safe Context Lifecycle and Config UX

**Epic:** Epic 11 — CLI UX Polish  
**Status:** Completed  
**Priority:** P1  
**Estimated Effort:** 2-3 days  
**Dependencies:** E3-S1 (context config), E6-S1 (auth)  
**Target:** `internal/cli/`, `internal/config/`  
**Design Reference:** CLI UX review after Epics 8 and 9

---

## 1. Summary

Unify the `config` and `context` experience so operators can list, create, update, and switch contexts predictably, while making config inspection safe by default.

## 2. Architecture Context and Reuse Guidance

- Reuse the existing YAML config format and `internal/config.Load/Save`.
- Do not introduce a second config file or migration path.
- Preserve existing commands for backwards compatibility where possible; add aliases before removing anything.
- Security requirement: config inspection must not print bearer tokens by default.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Command shape

Add context lifecycle commands under `context`:

- `htmlctl context list`
- `htmlctl context use <name>` as the preferred alias for `config use-context`
- `htmlctl context create <name> --server ... --website ... --environment ... [--port ...] [--token ...]`

Keep:

- `config current-context`
- `config use-context`

but update help text to point operators at the `context` group.

### 3.2 Safe config view

Change `htmlctl config view` so tokens are redacted by default:

- `token: <redacted>`

Add explicit opt-in:

- `htmlctl config view --show-secrets`

Never print secrets in table/help/error output unless `--show-secrets` is set.

### 3.3 Actionable config/context errors

Common failure paths must tell the operator what to do next:

- missing config file:
  - mention the expected path
  - suggest `htmlctl context create ...`
- unknown context name:
  - print the requested name
  - suggest `htmlctl context list`
- duplicate context on create:
  - suggest `htmlctl context use <name>` or `htmlctl context set <name>`

## 4. File Touch List

### Files to Modify

- `internal/cli/config_cmd.go`
- `internal/cli/config_cmd_test.go`
- `internal/cli/context_cmd.go`
- `internal/cli/context_cmd_test.go`
- `internal/config/types.go`
- `internal/config/loader.go`
- `internal/config/resolve.go` if helper reuse is needed
- `README.md`
- `docs/technical-spec.md`
- `.agent/skills/htmlctl-publish/references/commands.md` — add `context create` and `config view --show-secrets`

## 5. Implementation Steps

1. Add a redaction helper for config output that preserves structure but masks tokens.
2. Add `config view --show-secrets`.
3. Add `context list` and `context use`.
4. Add `context create` with validation against existing context-name/server/environment rules.
5. Update help text so `config` is primarily inspection-oriented and `context` is primarily mutation-oriented.
6. Upgrade config/context error strings to include the next likely recovery command.
7. Update `.agent/skills/htmlctl-publish/references/commands.md`: add `context create` usage and document the `config view --show-secrets` flag.

## 6. Tests and Validation

### Automated

- `config view` redacts tokens by default.
- `config view --show-secrets` prints the real token.
- `context list` shows all contexts and current-context marker.
- `context use` switches active context.
- `context create` writes a valid new context and rejects duplicate names.
- missing config / unknown context / duplicate create errors include actionable next steps.

### Manual

- Create a new staging context from scratch.
- Verify `config view` is safe to paste/share by default.

## 7. Acceptance Criteria

- [x] AC-1: `htmlctl context list` and `htmlctl context use <name>` exist and work against the current config file.
- [x] AC-2: `htmlctl context create` can create a valid context entry without manual YAML editing.
- [x] AC-3: `htmlctl config view` redacts bearer tokens by default.
- [x] AC-4: `htmlctl config view --show-secrets` exists for explicit secret inspection.
- [x] AC-5: Existing `config use-context` continues to work.
- [x] AC-6: No command/help/error path prints tokens unintentionally.
- [x] AC-7: Common config/context failure states suggest the next likely recovery command.
- [x] AC-8: `.agent/skills/htmlctl-publish/references/commands.md` documents `context create` and `config view --show-secrets`.

## 8. Risks and Open Questions

### Risks

- **Breaking existing scripts.**
  Mitigation: keep existing `config use-context`; add aliases rather than rename destructively.
- **False sense of secrecy from partial redaction.**
  Mitigation: redact the full token field, not just a prefix.

### Open Questions

- None blocking. The story keeps old commands and adds preferred aliases.
