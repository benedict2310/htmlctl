# E11-S2 — Context-Aware Defaults for Remote Commands

**Epic:** Epic 11 — CLI UX Polish  
**Status:** Completed  
**Priority:** P1  
**Estimated Effort:** 2-3 days  
**Dependencies:** E3-S1 (context config), E3-S3 (core remote commands), E9-S3 (backend CLI)  
**Target:** `internal/cli/`, `internal/client/`  
**Design Reference:** CLI UX review after Epics 8 and 9

---

## 1. Summary

Reduce repetitive command usage by letting common remote commands default website and environment from the active context while preserving explicit `website/<name>` and `--env` overrides.

## 2. Architecture Context and Reuse Guidance

- The active context already contains `website` and `environment`; reuse that data instead of adding new config fields.
- Keep explicit refs valid for scripts and multi-website operators.
- Do not change wire APIs for this story. This is CLI argument-resolution work only.

## 3. Proposed Changes and Architecture Improvements

Target commands:

- `status`
- `logs`
- `rollout history`
- `rollout undo`
- `backend add|list|remove`

Behavior:

- If `website/<name>` is omitted, use `rt.ResolvedContext.Website`.
- If `--env` is omitted on backend commands, use `rt.ResolvedContext.Environment`.
- Help text must show both forms:
  - explicit
  - context-default
- If the resolved context is missing a website or environment, fail with an actionable error that names the missing field and suggests `htmlctl context set ...`.

Examples:

- `htmlctl status`
- `htmlctl logs`
- `htmlctl rollout history`
- `htmlctl backend list`

still resolve to the active context target.

## 4. File Touch List

### Files to Modify

- `internal/cli/status_cmd.go`
- `internal/cli/logs_cmd.go`
- `internal/cli/rollout_cmd.go`
- `internal/cli/backend_cmd.go`
- `internal/cli/remote_helpers.go`
- corresponding `*_test.go` files
- `README.md`
- `docs/technical-spec.md`
- `.agent/skills/htmlctl-publish/references/commands.md` — update `status`, `logs`, `rollout`, and `backend` examples to show both explicit and context-default forms

## 5. Implementation Steps

1. Add shared helpers to resolve website/env from explicit args or context defaults.
2. Update target commands to accept zero or one positional website ref where appropriate.
3. Update backend commands so `--env` defaults from context.
4. Update help/usage text and tests.
5. Improve resolution errors so they explain whether the command needs a website ref, an environment, or a context fix.
6. Update `.agent/skills/htmlctl-publish/references/commands.md`: revise `status`, `logs`, `rollout history/undo`, and `backend` examples to show both the explicit `website/<name>` form and the context-default (no-arg) form.

## 6. Tests and Validation

### Automated

- `status` with no args uses context website.
- `logs` with no args uses context website.
- `rollout history/undo` with no args use context website.
- backend commands default `--env` from context.
- explicit args/flags override context defaults.
- missing context defaults produce fix-oriented error messages.

### Manual

- Run common staging workflows without repeating `website/<name>` and `--env`.

## 7. Acceptance Criteria

- [x] AC-1: `status`, `logs`, and `rollout` commands can operate without an explicit website ref when the context provides one.
- [x] AC-2: backend commands default `--env` from the active context.
- [x] AC-3: Explicit website refs and `--env` overrides still work and win over context defaults.
- [x] AC-4: Help output documents the defaulting behavior clearly.
- [x] AC-5: Missing default website/environment state produces actionable recovery guidance.
- [x] AC-6: `.agent/skills/htmlctl-publish/references/commands.md` shows both explicit and context-default forms for `status`, `logs`, `rollout`, and `backend` commands.

## 8. Risks and Open Questions

### Risks

- **Implicit targeting surprises operators.**
  Mitigation: help output and success output should echo the resolved website/environment.

### Open Questions

- None blocking. The story keeps explicit targeting as the highest-precedence path.
