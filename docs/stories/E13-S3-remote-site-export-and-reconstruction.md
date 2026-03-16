# E13-S3 — Remote Site Export and Canonical Source Reconstruction

**Epic:** Epic 13 — Agent-Native Site Discoverability  
**Status:** Proposed  
**Priority:** P1  
**Estimated Effort:** 5-6 days  
**Dependencies:** E2-S3, E2-S4, E3-S3, E10-S5, E13-S1  
**Target:** `internal/blob/`, `internal/server/`, `internal/client/`, `internal/cli/`, `docs/`, `.agent/skills/htmlctl-publish/`  
**Design Reference:** zero-context operator review on 2026-03-15

---

## 1. Summary

Add a supported way to pull the current desired state back out of `htmlservd` as a canonical site tree. This closes the largest agent-ergonomics gap in the current system: the server is the source of truth, but the CLI cannot reconstruct that source state locally.

## 2. Architecture Context and Reuse Guidance

- Reuse desired state, not rendered releases. Export must reconstruct source resources from DB metadata plus blob content; it must not copy files from the active release output because those files are rendered HTML artifacts, not source inputs.
- Reuse existing DB row/query helpers and the blob store:
  - website metadata from `websites`
  - pages from `pages`
  - components from `components`
  - style bundle file refs from `style_bundles.files_json`
  - assets from `assets`
  - branding from `website_icons`
- Add a blob read helper to `internal/blob/` instead of open-coding file reads at call sites.
- Be explicit about fidelity:
  - exported component HTML/CSS/JS and binary assets/branding should be byte-exact
  - exported `website.yaml` and `pages/*.page.yaml` will be canonical regenerated YAML, not original hand-authored formatting/comments
- Preserve security invariants:
  - authenticated API only
  - sanitized 5xx responses
  - no path traversal in exported archive/directory entries

## 3. Proposed Changes and Architecture Improvements

### 3.1 Server-side source export endpoint

Add:

- `GET /api/v1/websites/{website}/environments/{env}/source`

The endpoint should stream a canonical tar.gz site tree containing:

- `website.yaml`
- `pages/*.page.yaml`
- `components/*.html`
- optional `components/*.css`
- optional `components/*.js`
- `styles/*`
- optional `scripts/site.js`
- `assets/**`
- `branding/**`

Implementation rules:

- reconstruct website/page YAML from parsed DB metadata using stable field order and normalized line endings
- read component/style/script/asset/icon bytes from blob storage by hash
- preserve canonical file paths used by `apply`/`manifest`
- stream or buffer safely; do not hold unbounded temporary state if it can be avoided

### 3.2 CLI export workflow

Add:

- `htmlctl site export --context staging -o ./site`

Behavior:

- default output is a directory tree
- optional `--archive <path>.tar.gz`
- refuse to overwrite non-empty output without explicit `--force`
- print clear fidelity notes:
  - YAML regenerated canonically
  - comments and original key formatting are not preserved

### 3.3 Round-trip validation as product contract

The key correctness contract is not “looks plausible”; it is round-trip safety:

1. export desired state
2. run `htmlctl render` locally
3. run `htmlctl diff -f <exported-dir> --context <same-env>`
4. observe no changes

That round-trip should be the required success criterion for this story.

## 4. File Touch List

### Files to Create

- `internal/cli/site_export_cmd_test.go`

### Files to Modify

- `internal/blob/store.go` — add a read helper with hash validation
- `internal/blob/store_test.go`
- `internal/server/resources.go` or a new focused export handler file — implement source export assembly
- `internal/server/routes.go`
- `internal/server/resources_test.go` or dedicated export tests
- `internal/client/client.go` — add source export request helper
- `internal/client/client_test.go`
- `internal/client/types.go` — only if a metadata wrapper is needed alongside the tar stream
- `internal/cli/site_cmd.go` — add `site export`
- `internal/cli/root.go`
- `pkg/model/types.go` or a small shared YAML serialization helper if needed for canonical website/page output
- `docs/technical-spec.md`
- `docs/operations-manual-agent.md`
- `.agent/skills/htmlctl-publish/SKILL.md`
- `.agent/skills/htmlctl-publish/references/deployment-workflows.md`

## 5. Implementation Steps

1. Add blob read support in `internal/blob/`.
2. Build canonical source serializers for:
   - website YAML
   - page YAML
3. Implement server-side source export assembly from DB metadata plus blobs.
4. Expose the export endpoint through the authenticated API router.
5. Add `htmlctl site export`:
   - output directory mode
   - archive mode
   - overwrite safety checks
   - fidelity note in table output
6. Add round-trip tests and docs that define the export contract.

## 6. Tests and Validation

### Automated

- blob read helper rejects invalid hashes and reads valid blobs correctly
- export endpoint returns a tar.gz with expected canonical paths
- exported website/page YAML is deterministic
- exported component sidecars are present only when hashes exist
- binary assets and branding bytes round-trip exactly
- `site export -o <dir>` refuses to clobber a non-empty directory without `--force`
- exporting a site and then diffing it back against the same environment reports no changes
- 5xx failures remain sanitized

### Manual

- run:
  - `htmlctl site export --context staging -o ./exported-site`
  - `htmlctl render -f ./exported-site -o ./dist`
  - `htmlctl diff -f ./exported-site --context staging`
- verify the exported tree is understandable to a fresh operator and round-trips cleanly

## 7. Acceptance Criteria

- [ ] AC-1: `htmlctl site export --context <ctx> -o <dir>` exports a canonical source tree for the selected website/environment.
- [ ] AC-2: Exported component HTML/CSS/JS and binary assets/branding are reconstructed from desired-state blobs, not from rendered release output.
- [ ] AC-3: Exported `website.yaml` and page YAML are deterministic and semantically equivalent to current desired state, even though original comments/formatting are not preserved.
- [ ] AC-4: Exported output round-trips through `render` and `diff` with no reported changes against the same environment.
- [ ] AC-5: CLI output makes fidelity limits explicit so operators do not assume a perfect formatting-preserving backup.

## 8. Risks and Open Questions

### Risks

- **Risk:** developers assume release artifacts can stand in for source export.  
  **Mitigation:** document and test that export is rebuilt from desired state and blobs only.

- **Risk:** canonical YAML regeneration surprises users expecting comments or original key ordering.  
  **Mitigation:** make this explicit in command output and docs; define semantically equivalent round-trip as the contract.

- **Risk:** large exports become memory-heavy if implemented as an in-memory archive.  
  **Mitigation:** prefer streaming tar.gz assembly where practical; if initial implementation buffers, document expected limits and keep code structured for later streaming.

### Open Questions

- Should `site export` allow selecting a historical release instead of current desired state in v1? Recommendation: no. Start with current desired state only; historical release export can come later if needed.
