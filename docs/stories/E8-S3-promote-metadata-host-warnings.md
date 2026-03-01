# E8-S3 — Promote Metadata Host Warnings

**Epic:** Epic 8 — DX & Reliability  
**Status:** Implemented (2026-03-01)  
**Priority:** P2 (Medium — reduces silent prod metadata mistakes)  
**Estimated Effort:** 0.5–1 day  
**Dependencies:** E4-S3 (promote), E7-S1 (head metadata), E8-S2 (OG auto-injection, optional but recommended)  
**Target:** `internal/server/promote.go` + promote API/CLI response contracts  
**Design Reference:** E8-S2 follow-up decision — 2026-02-26

---

## 1. Objective

Add non-blocking warnings during `promote` when source release metadata likely points to staging hosts instead of target/prod hosts.

This must not rewrite artifacts, must not rebuild, and must preserve the existing artifact-promotion invariant.

## 2. User Story

As an operator promoting staging to prod, I want the system to warn me if canonical/OG/Twitter URLs still point to staging-like hosts, so I can catch metadata drift early without blocking deployment.

## 3. Scope

### In Scope

- Detect host mismatches in source release metadata during promote success path.
- Surface warnings in:
  - promote API response,
  - `htmlctl promote` table output,
  - structured JSON/YAML output.
- Keep warnings non-blocking (promotion still succeeds if hash checks pass).
- Keep behavior deterministic and bounded (stable ordering, capped warning list).

### Out of Scope

- Rewriting manifest/content during promotion.
- Blocking or failing promotion because of metadata host mismatch.
- New DB schema/migrations.

## 4. Detection Rules (v1)

Inspect metadata URL fields from promoted source release manifest:

- `head.canonicalURL`
- `head.openGraph.url`
- `head.openGraph.image`
- `head.twitter.url`
- `head.twitter.image`

Rules:

1. Parse absolute `http(s)` URLs only.
2. Ignore relative URLs and empty fields.
3. Build host sets:
   - source env bound domains (`ListDomainBindings(website, sourceEnv)`),
   - target env bound domains (`ListDomainBindings(website, targetEnv)`).
4. Emit warning when URL host is not in target host set and either:
   - host is in source host set, or
   - target env is `prod` and host contains `staging` token (fallback heuristic when bindings are incomplete).

Warning format should include enough context for operators:

- page name,
- field path (for example `canonicalURL`, `openGraph.image`),
- detected host,
- target environment name.

## 5. Architecture and File Targets

### 5.1 Server

**Modify:** `internal/server/promote.go`

- After successful `release.Promote(...)`, compute warnings from source release manifest + domain bindings.
- Return warnings in response payload.
- Never convert warning-analysis failures into API errors; log and continue with empty warnings.

**Create:** `internal/server/promote_warnings.go`

- `collectPromoteMetadataHostWarnings(...)` helper.
- Manifest parsing + host analysis logic.
- Stable/deterministic warning ordering.
- Hard cap warning count (for example 20) to keep responses bounded.

### 5.2 Client/CLI Contracts

**Modify:** `internal/client/types.go`

- Add `Warnings []string \`json:"warnings,omitempty" yaml:"warnings,omitempty"\`` to `PromoteResponse`.

**Modify:** `internal/cli/promote_cmd.go`

- Table output: print warnings after success line using deterministic prefix like `Warning: ...`.
- Structured output: unchanged serialization now includes `warnings` field.

## 6. Tests and Validation

### Automated

**Create:** `internal/server/promote_warnings_test.go`

- manifest URL extraction and host matching logic.
- ignores relative URLs.
- deterministic ordering + warning cap behavior.

**Modify:** `internal/server/promote_test.go`

- success response includes warnings when staging-like host detected.
- success response has no warnings when hosts match target domains.
- malformed/empty manifest in source release does not fail promote endpoint.

**Modify:** `internal/cli/promote_cmd_test.go`

- table output prints warning lines when response includes warnings.
- JSON output includes `warnings` array.

### Manual

- Promote staging -> prod where page metadata still references `staging.example.com`.
- Verify promote succeeds and warning text is visible in CLI output.
- Verify `--output json` includes warnings.

## 7. Acceptance Criteria

- [ ] AC-1: Successful promote can return non-empty `warnings` without affecting `hashVerified`.
- [ ] AC-2: Warnings are produced for staging-like hosts in promoted metadata when target is prod.
- [ ] AC-3: Warnings do not modify artifacts and do not trigger rebuild.
- [ ] AC-4: CLI table mode surfaces warnings clearly after success summary.
- [ ] AC-5: JSON/YAML outputs include `warnings` deterministically.
- [ ] AC-6: If warning analysis fails internally (parse/query issue), promote still returns success (no warning-based failure path).
- [ ] AC-7: `go test -race ./internal/server/... ./internal/cli/... ./internal/client/...` passes.

## 8. Risks and Mitigations

- **Risk:** false positives where host mismatch is intentional.  
  **Mitigation:** warnings are advisory only, never blocking.

- **Risk:** missing domain bindings reduce signal quality.  
  **Mitigation:** use fallback heuristic only for obvious staging token cases.

- **Risk:** noisy output for large sites.  
  **Mitigation:** cap warning count and include concise wording.

## 9. Open Questions

- None for v1 of warning behavior.

---
