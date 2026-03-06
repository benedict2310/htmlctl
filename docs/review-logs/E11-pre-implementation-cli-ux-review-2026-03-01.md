# Epic 11 Pre-Implementation CLI UX Review

**Date:** 2026-03-01  
**Scope:** Epic 11 and stories `E11-S1` through `E11-S4`

## Result

The mini-epic is intentionally narrow and operator-focused. It does not change deployment semantics; it improves safety, discoverability, and command consistency on top of the existing control plane.

## Architecture Findings Resolved

- `context` vs `config` work was kept additive and backward-compatible rather than renaming command groups.
- context-aware defaults are CLI-only and do not require API changes.
- remote diagnostics reuse existing health/version endpoints instead of inventing a new diagnostics API.
- inventory cleanup reuses current domain/backend list endpoints and keeps task-specific commands intact.

## Security Findings Resolved

- `config view` is explicitly changed to redact tokens by default, with opt-in secret display.
- diagnostics commands must never print token material.
- backend safety warnings are advisory only and do not weaken server-side validation.

## UX/Error-State Findings Resolved

- config/context stories now require fix-oriented recovery hints for missing config, unknown context, and duplicate context creation.
- context-default stories now require actionable errors when the active context lacks website or environment information.
- diagnostics now require per-layer failure reporting plus concrete next steps.
- inventory/workflow polish now includes kubectl-style guidance for malformed refs and unsupported inventory requests.

## Remaining Open Questions

- No blocking open questions remain.
- Intentional v1 constraints:
  - old command names remain for compatibility
  - context-default behavior is opt-in by omission, not a hard behavior change for explicit refs
  - diagnostics remain read-only
