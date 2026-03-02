# E11-S4 — Inventory and Workflow Guidance Polish

**Epic:** Epic 11 — CLI UX Polish  
**Status:** Planned  
**Priority:** P2  
**Estimated Effort:** 3-4 days  
**Dependencies:** E3-S3 (core remote commands), E5-S4 (domain CLI), E9-S3 (backend CLI)  
**Target:** `internal/cli/`, `internal/output/`, `README.md`, `docs/technical-spec.md`  
**Design Reference:** command-surface review after Epics 8 and 9

---

## 1. Summary

Make the command surface easier to discover by aligning inventory commands with operator expectations and by standardizing next-step guidance after domain/backend/deploy operations.

## 2. Architecture Context and Reuse Guidance

- Reuse existing API endpoints; this is primarily CLI composition and output work.
- Keep command additions small and backwards-compatible.
- Avoid creating parallel concepts unless there is a strong reason. Prefer clearer aliases/help over brand-new abstractions.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Inventory surface cleanup

Extend `get` to support:

- `domains`
- `backends`

using the existing list endpoints, so operators have one discoverable inventory command family:

- `htmlctl get websites`
- `htmlctl get environments`
- `htmlctl get releases`
- `htmlctl get domains`
- `htmlctl get backends`

Retain `domain list` and `backend list` as task-specific subcommands.

### 3.2 Guided success output

Standardize post-success guidance:

- `apply` already suggests `domain add` for first deploy
- `domain add` already suggests `domain verify`
- add similar guidance for backend add:
  - explain that routing changed immediately
  - suggest `backend list`
  - suggest checking the live URL

### 3.3 Guided error output

Add fix-oriented, kubectl-style error follow-through for common operator mistakes:

- unsupported `get` resource:
  - print supported resource types
- invalid website ref:
  - show expected `website/<name>` format
- backend path shadow-risk warning:
  - explain that static content under the prefix will be hidden
- domain verify DNS failure:
  - keep existing actionable DNS/TLS hints and align formatting with backend/domain/apply messages

### 3.4 Suspicious-prefix warnings

Add non-blocking CLI warnings for obviously risky backend prefixes when using human/table output:

- `/styles/*`
- `/scripts/*`
- `/assets/*`
- `/favicon.*`

These are warnings only; the operation still proceeds.

## 4. File Touch List

### Files to Modify

- `internal/cli/get_cmd.go`
- `internal/cli/get_cmd_test.go`
- `internal/cli/domain_cmd.go`
- `internal/cli/backend_cmd.go`
- `internal/cli/backend_cmd_test.go`
- `internal/output/formatter.go` only if shared helper changes are needed
- `README.md`
- `docs/technical-spec.md`
- `.agent/skills/htmlctl-publish/references/commands.md` — add `get domains`, `get backends`; update `get` supported-resource-types list
- `.agent/skills/htmlctl-publish/SKILL.md` — update the Safety Checklist and Workflow Decision table to reflect all E11 additions (context-aware defaults, doctor, `get domains`/`get backends`, suspicious-prefix warnings)
- Sync `.agent/skills/htmlctl-publish/` → `~/.claude/skills/htmlctl-publish/` after all content is final

## 5. Implementation Steps

1. Extend `get` resource normalization and dispatch for `domains` and `backends`.
2. Reuse existing list endpoints and formatters.
3. Add backend success guidance and suspicious-prefix warnings in table mode.
4. Improve common CLI error text so unsupported inventory requests and malformed refs tell the operator exactly how to recover.
5. Update help/docs to explain when to use `get` vs task-specific subcommands.
6. Update `.agent/skills/htmlctl-publish/references/commands.md`: add `get domains` and `get backends` examples; update the supported-resource-types list.
7. Update `.agent/skills/htmlctl-publish/SKILL.md`: reflect context-aware defaulting in the Safety Checklist and workflow examples; add `doctor` to the pre-flight checks; update the Workflow Decision table if needed.
8. Sync the final `.agent/skills/htmlctl-publish/` tree to `~/.claude/skills/htmlctl-publish/`.

## 6. Tests and Validation

### Automated

- `get domains` and `get backends` produce table/json/yaml output.
- backend add prints guidance on success.
- suspicious backend prefixes print warnings in table mode only.
- unsupported `get` types and malformed refs return actionable errors.
- structured output remains machine-parseable and warning-free.

### Manual

- Use `get domains` and `get backends` on a live context and confirm output is intuitive.

## 7. Acceptance Criteria

- [ ] AC-1: `htmlctl get domains` and `htmlctl get backends` are supported.
- [ ] AC-2: `domain list` and `backend list` remain available.
- [ ] AC-3: `backend add` prints useful next-step guidance in table mode.
- [ ] AC-4: obviously risky backend prefixes emit warnings without blocking the operation.
- [ ] AC-5: common inventory/ref errors are actionable and name the expected input or next command.
- [ ] AC-6: structured output remains deterministic and machine-parseable.
- [ ] AC-7: `.agent/skills/htmlctl-publish/references/commands.md` lists `get domains` and `get backends` and has an accurate supported-resource-types list.
- [ ] AC-8: `.agent/skills/htmlctl-publish/SKILL.md` reflects context-aware defaulting in its workflow examples and safety checklist.
- [ ] AC-9: `.agent/skills/htmlctl-publish/` is synced to `~/.claude/skills/htmlctl-publish/`.

## 8. Risks and Open Questions

### Risks

- **`get` becomes too broad and confusing again.**
  Mitigation: keep it to listable operator inventory objects only.

### Open Questions

- None blocking. This story prefers additive aliases and guidance over command removal.
