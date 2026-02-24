# Story Quality Checks

Use this checklist before handing off a story for implementation.

## Preflight Compatibility

- [ ] Story file lives under `docs/stories/<epic>/` with correct filename (`<ID>-<TITLE>.md`).
- [ ] Title uses `# <ID> - <Title>`.
- [ ] Required metadata fields exist and use exact format:
  - `**Status:** ...`
  - `**Priority:** ...`
  - `**Dependencies:** ...`
- [ ] `**Status:**` is not "Complete" or "Implemented".
- [ ] Dependencies list uses parseable IDs (e.g., `F.01`, `A.02`) or `None`.
- [ ] Acceptance Criteria section includes checkbox lines (`- [ ]`).
- [ ] Run: `.claude/skills/implement-story/scripts/preflight.sh <story-path>`.

## Clarity & Scope

- [ ] Objective clearly states the user value and problem solved.
- [ ] Scope is explicit (in-scope and out-of-scope listed).
- [ ] Implementation Plan (Draft) lists concrete files to create/modify.
- [ ] Verification plan includes automated tests and manual checks.

## Architecture Alignment

- [ ] References relevant PRD/ARCHITECTURE sections.
- [ ] Respects component boundaries and concurrency model.
- [ ] For tools: includes confirmation guardrails and audit logging requirements.
- [ ] For permissions: uses correct APIs and documents expected state updates.

## Completeness

- [ ] Risks and mitigations documented.
- [ ] Open questions captured (or explicitly "None").
- [ ] Story added to `docs/stories/README.md` and epic README.
