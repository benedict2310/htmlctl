# E12-S4 — Newsletter Extension Adoption Validation and Docs Integration

**Epic:** Epic 12 — Optional Service Extensions
**Status:** Implemented (2026-03-06)
**Priority:** P1
**Estimated Effort:** 2 days
**Dependencies:** E12-S3
**Target:** `docs/guides/`, `.agent/skills/htmlctl-publish/`, `docs/review-logs/`
**Design Reference:** FutureLab pilot adoption track

---

## 1. Summary

Validate the newsletter extension end-to-end in a real adopter workflow and fold the resulting guidance into official docs and `htmlctl-publish` skill references.

## 2. Architecture Context and Reuse Guidance

- Reuse existing first-deploy and backend verification patterns from docs.
- Keep extension onboarding docs aligned with `htmlctl` operational safety model (promote, rollback, backend verification).
- Reuse skill maintenance flow: update `.agent/skills/htmlctl-publish`, then sync to `~/.claude/skills/htmlctl-publish`.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Pilot validation checklist

Define a reproducible validation sequence:

- install extension on staging/prod host
- wire `/newsletter/*` via `htmlctl backend add`
- verify signup/verify routing contracts
- verify failure modes (upstream down, wrong env mapping)

### 3.2 Documentation hardening

Add concrete operator runbooks:

- quickstart for new users
- production cutover checklist
- extension upgrade path

### 3.3 Skill integration

Update `htmlctl-publish` references with:

- extension-aware workflow decision guidance
- newsletter backend wiring examples
- safety checklist additions for dynamic service extensions

## 4. File Touch List

### Files to Modify

- `docs/guides/extensions-overview.md`
- `docs/guides/newsletter-extension-vps.md`
- `docs/operations-manual-agent.md`
- `.agent/skills/htmlctl-publish/SKILL.md`
- `.agent/skills/htmlctl-publish/references/commands.md`
- `.agent/skills/htmlctl-publish/references/deployment-workflows.md`
- `docs/review-logs/` (new E12 adoption validation log)

## 5. Implementation Steps

1. Execute pilot checklist against one real adopter (FutureLab).
2. Capture evidence and failure modes in a review log.
3. Update guides and operations manual with stable procedures.
4. Update `htmlctl-publish` skill refs and sync to `~/.claude/skills/htmlctl-publish`.

## 6. Tests and Validation

### Automated

- doc link checks where configured
- smoke script for backend add/list/remove and health probe commands

### Manual

- real host validation proving extension installation + routing + verification flow readiness.

## 7. Acceptance Criteria

- [x] AC-1: A documented pilot run proves the extension can be installed and routed via Epic 9 backends.
- [x] AC-2: Operator guides include an end-to-end cutover checklist and rollback steps.
- [x] AC-3: `htmlctl-publish` skill docs are updated and synced for extension-aware publishing workflows.
- [x] AC-4: Post-pilot review log records lessons, security observations, and follow-up backlog items.
- [x] AC-5: Extension onboarding docs are sufficient for a new operator to complete setup without reading source code.

## 8. Risks and Open Questions

### Risks

- **Risk:** docs drift from shipped extension assets.  
  **Mitigation:** require every extension release to update docs + skill references in same change set.
- **Risk:** adopters treat extension guidance as production-safe without security validation.  
  **Mitigation:** include explicit security verification gate before public routing activation.

### Open Questions

- Should future epics include a lightweight `htmlctl extension doctor` command for compatibility checks?

## 9. Implementation Notes (2026-03-06)

- Updated operator docs:
  - `docs/guides/extensions-overview.md`
  - `docs/guides/newsletter-extension-vps.md`
  - `docs/operations-manual-agent.md`
- Updated `htmlctl-publish` skill references:
  - `.agent/skills/htmlctl-publish/SKILL.md`
  - `.agent/skills/htmlctl-publish/references/commands.md`
  - `.agent/skills/htmlctl-publish/references/deployment-workflows.md`
- Added adoption validation log:
  - `docs/review-logs/E12-newsletter-extension-adoption-validation-2026-03-06.md`
- Synced updated skill docs to:
  - `~/.claude/skills/htmlctl-publish/`

Independent review findings fixed during this story gate:
- service config now rejects non-numeric/out-of-range `NEWSLETTER_HTTP_ADDR` ports
- migration SQL splitting is SQL-aware (quoted strings/comments/dollar-quoted blocks)
- installer now renders unit files with configured `NEWSLETTER_BIN_PATH`
- extension manifest aligns with runtime defaulting (`NEWSLETTER_HTTP_ADDR` optional)

Verification evidence:
- `go test ./...` (repo root) passed
- `go test ./...` in `extensions/newsletter/service` passed
- `bash -n extensions/newsletter/ops/setup-newsletter-extension.sh` passed
- Docker E2E deploy + backend smoke/failure drill passed (`E2E_OK`)
- Re-validated in local Docker matrix (2026-03-06):
  - backend route `/newsletter/*` verified via `/newsletter/verify` probe (current runtime now returns `400` on missing-token probes instead of the earlier placeholder behavior)
  - controlled failure drill (service stopped) returned expected upstream failure (`502`)
  - guidance/docs corrected to use `/newsletter/verify` because `/newsletter/*` does not include bare `/newsletter`
