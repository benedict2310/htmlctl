# Epic 10 Pre-Implementation Architecture and Security Review

**Date:** 2026-03-01  
**Scope:** `docs/epics.md` Epic 10 and stories `E10-S1` through `E10-S5`

## Result

Epic 10 stories were reviewed before implementation with two goals:

1. remove architecture ambiguity so a developer can follow existing repo seams
2. close obvious security gaps before any code work starts

All five stories were updated to implementation-ready state.

## Architecture Findings Resolved

- **Preview URLs:** pinned previews were clarified as Caddy site blocks rooted at specific release directories, not ad hoc handlers or domain-binding mutations.
- **Git input mode:** Git work stays entirely in `htmlctl`; `htmlservd` remains source-agnostic and continues to consume tar bundles only.
- **Auth policies:** v1 scope was narrowed to environment-scoped Basic Auth with one credential per prefix to avoid unresolved nested route semantics.
- **Retention/GC:** deletion rules now explicitly preserve active releases, rollback targets, and preview-pinned releases; blob sweep is restricted to hash-named files under the blob root.
- **Component fragments:** sidecars were constrained to deterministic external files with a simple same-directory convention instead of introducing a larger bundler/template subsystem; the story now explicitly includes the missing file-level partial-apply work.

## Security Findings Resolved

- **Preview secrecy confusion:** stories now state that preview hostnames are not access control; noindex headers are required.
- **Git credential leakage:** Git-input story now requires URL redaction in surfaced errors and forbids shell interpolation.
- **Auth credential handling:** auth-policy story requires client-side bcrypt hashing and prohibits hash/plaintext disclosure in list output and logs.
- **GC deletion safety:** retention story restricts deletion scope and mark roots to avoid path traversal and accidental non-blob file deletion.
- **Retention consistency:** release deletion now requires quarantine-before-DB-delete so history never points at already-deleted directories.
- **Component-sidecar CSP drift:** component-fragment story requires external files only and rejects relative CSS `url(...)` references.

## Remaining Open Questions

- No blocking open questions remain for the five planned stories.
- Intentional v1 constraints are documented in-story where broader future support is possible:
  - one preview base domain
  - Git via system `git`
  - Basic Auth only
  - manual retention runs
  - one CSS and one JS sidecar per component
