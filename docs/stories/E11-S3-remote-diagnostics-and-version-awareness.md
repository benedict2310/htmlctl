# E11-S3 — Remote Diagnostics and Version Awareness

**Epic:** Epic 11 — CLI UX Polish  
**Status:** Completed  
**Priority:** P1  
**Estimated Effort:** 2-3 days  
**Dependencies:** E2-S1 (health/version endpoints), E8-S3 (promote warnings), E8/E9 operator drift findings  
**Target:** `internal/cli/`, `internal/client/`, `internal/server/health.go` reuse  
**Design Reference:** repeated operator need to distinguish local CLI build from remote server capabilities

---

## 1. Summary

Add first-class diagnostics so operators can quickly answer: which context am I targeting, is the tunnel/auth working, and are my local CLI and remote server on compatible versions?

## 2. Architecture Context and Reuse Guidance

- Reuse existing unauthenticated `/healthz`, `/readyz`, and `/version` endpoints.
- Do not add a new server diagnostics API unless existing health/version endpoints are insufficient.
- Diagnostics must never print bearer tokens.

## 3. Proposed Changes and Architecture Improvements

Add:

- `htmlctl version --remote`
- `htmlctl doctor`

`version --remote`:

- prints local CLI version
- queries remote `/version` through the configured transport
- emits both values in table/json/yaml output

`doctor`:

- validates config/context resolution
- checks SSH transport establishment
- checks `/healthz`, `/readyz`, `/version`
- prints resolved website/environment/server target
- prints actionable failure hints
- ends with a short “Next steps” section when any check fails

## 4. File Touch List

### Files to Create

- `internal/cli/doctor_cmd.go`
- `internal/cli/doctor_cmd_test.go`

### Files to Modify

- `internal/client/client.go` — add health/version request helpers
- `internal/client/types.go`
- `internal/cli/version.go`
- `internal/cli/version_test.go`
- `internal/cli/root.go`
- `README.md`
- `docs/technical-spec.md`
- `.agent/skills/htmlctl-publish/references/commands.md` — add `version --remote` and `doctor`

## 5. Implementation Steps

1. Add client helpers for `/healthz`, `/readyz`, and `/version`.
2. Extend `version` with `--remote`.
3. Add `doctor` command with deterministic checks and clear failure messages.
4. Support structured output for both commands.
5. Standardize diagnostic failure messages so they identify the failed layer:
   - config
   - SSH transport
   - auth
   - server health/readiness
   - version skew
6. Update `.agent/skills/htmlctl-publish/references/commands.md`: add a Diagnostics section covering `version --remote` and `doctor` with example output and common failure hints.

## 6. Tests and Validation

### Automated

- `version` prints local version only by default.
- `version --remote` prints local and remote versions.
- `doctor` reports config-resolution failure cleanly.
- `doctor` reports transport failure cleanly.
- `doctor` reports health/readiness/version success path.
- `doctor` failure output includes fix-oriented next steps for each failed layer.
- no output includes token material.

### Manual

- Run `htmlctl doctor --context staging` against a healthy server and against a broken known-hosts entry.

## 7. Acceptance Criteria

- [x] AC-1: `htmlctl version --remote` reports both local CLI and remote server version.
- [x] AC-2: `htmlctl doctor` checks config, transport, health, readiness, and version using the selected context.
- [x] AC-3: Diagnostics output is available in table/json/yaml.
- [x] AC-4: Failure messages are actionable and do not expose token material.
- [x] AC-5: `doctor` groups failures by layer and includes a next-step hint for each failed check.
- [x] AC-6: `.agent/skills/htmlctl-publish/references/commands.md` has a Diagnostics section covering `version --remote` and `doctor`.

## 8. Risks and Open Questions

### Risks

- **Doctor becomes an unbounded grab-bag.**
  Mitigation: keep v1 to transport and basic server reachability/version only.

### Open Questions

- None blocking. v1 diagnostics stay read-only.
