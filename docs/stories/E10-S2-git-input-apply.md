# E10-S2 — Git Input Mode for `apply`

**Epic:** Epic 10 — Review, Automation, and Lifecycle
**Status:** Implemented (2026-03-03)
**Priority:** P1
**Estimated Effort:** 3-4 days
**Dependencies:** E2-S3 (bundle ingestion), E3-S3 (remote apply), E6-S4 (SSH transport hardening)
**Target:** `internal/cli/`, `internal/bundle/`, `internal/client/`
**Design Reference:** PRD 10.1 Git input roadmap item

---

## 1. Summary

Allow operators and CI jobs to deploy directly from a pinned Git source without manually checking out files into the working directory first. The server remains Git-agnostic; `htmlctl` resolves the source locally, builds the existing tar bundle, and sends the same authenticated apply/release requests it already uses.

## 2. Architecture Context and Reuse Guidance

- Keep Git client logic on the CLI side. `htmlservd` must not gain repository/network access.
- Reuse the existing bundle path:
  - resolve source to a temporary site directory
  - run `bundle.BuildTarFromDir`
  - call `ApplyBundle`
  - call `CreateRelease`
- Reuse current `apply --dry-run` behavior by making Git resolution produce a temporary directory that can feed existing diff/apply logic.
- Use the system `git` binary through `exec.CommandContext`. Do not shell out via `sh -c`.
- Require an explicit `--ref` for determinism. No floating-branch deploys in v1.

## 3. Proposed Changes and Architecture Improvements

### 3.1 CLI UX

Add mutually exclusive source modes to `htmlctl apply`:

- local path: `htmlctl apply -f ./site --context staging`
- Git source: `htmlctl apply --from-git <repo> --ref <rev> [--subdir site] --context staging`

Valid `--from-git` inputs:

- local repository path
- remote `https://...`
- remote `ssh://...`
- scp-style `git@host:org/repo.git`

`--dry-run` must work with Git input.

### 3.2 Provenance metadata

Extend bundle manifest with optional source metadata:

```json
"source": {
  "type": "git",
  "repo": "git@github.com:org/repo.git",
  "ref": "a1b2c3d4",
  "subdir": "site"
}
```

Server-side apply audit metadata should include the source block when present.

### 3.3 Git resolution flow

1. Create temp dir.
2. Initialize/fetch the requested repo/ref without evaluating shell.
3. Check out detached `FETCH_HEAD`.
4. Resolve optional `--subdir`.
5. Feed the resulting directory to the existing bundle builder.

Keep the implementation minimal:

- no sparse checkout in v1
- no `diff --from-git` in this story
- no server-side cache of repos

## 4. File Touch List

### Files to Create

- `internal/cli/git_source.go`
- `internal/cli/git_source_test.go`

### Files to Modify

- `internal/bundle/manifest.go` — optional `source` field
- `internal/bundle/manifest_test.go`
- `internal/bundle/build.go` — helper for source metadata injection
- `internal/bundle/build_test.go`
- `internal/cli/apply_cmd.go`
- `internal/cli/apply_cmd_test.go`
- `internal/server/apply.go` — include bundle source metadata in audit entry
- `docs/technical-spec.md`
- `README.md`

## 5. Implementation Steps

1. Add bundle manifest `Source` metadata with validation for `type=git`, `repo`, `ref`, and optional `subdir`.
2. Implement Git resolution helper:
   - normalize repo input
   - fetch requested ref into a temp clone/worktree
   - return site directory path and source metadata
3. Update `apply` CLI flag parsing:
   - exactly one of `--from` or `--from-git`
   - `--ref` required with `--from-git`
4. Reuse existing local validation and bundle build path after Git checkout.
5. Add source metadata to the bundle manifest before upload.
6. Include source metadata in apply audit metadata server-side when present.

## 6. Tests and Validation

### Automated

- Local temp repo tests:
  - apply from pinned commit succeeds
  - `--subdir` selects nested site root
  - missing ref fails with clear error
  - non-site subdir fails local validation before upload
  - `--from` and `--from-git` together fail fast
  - `--dry-run` works from Git source
- Manifest tests:
  - source metadata round-trips and validates
- Audit tests:
  - apply audit metadata includes repo/ref when bundle source metadata is present

### Manual

- Deploy from a pinned commit SHA on a local repo clone.
- Deploy from a private remote repo using existing SSH agent credentials.
- Confirm the same repo/ref produces the same release output as a normal local checkout.

## 7. Acceptance Criteria

- [ ] AC-1: `htmlctl apply --from-git <repo> --ref <rev>` deploys the requested Git revision without requiring a manual checkout first.
- [ ] AC-2: `--dry-run` works with Git input and uses the same diff behavior as local directories.
- [ ] AC-3: The server remains Git-agnostic; only the CLI resolves repositories.
- [ ] AC-4: Bundle/apply audit metadata includes Git source provenance when provided.
- [ ] AC-5: `--from` and `--from-git` are mutually exclusive and `--ref` is mandatory for Git mode.
- [ ] AC-6: Errors do not leak credentials embedded in repository URLs.
- [ ] AC-7: `go test ./internal/cli/... ./internal/bundle/...` passes.

## 8. Risks and Open Questions

### Risks

- **Credentials leak in error strings.**
  Mitigation: redact userinfo/tokens from repo URLs before surfacing CLI errors.
- **Git binary missing on the operator machine.**
  Mitigation: fail with a clear prerequisite error; do not attempt a partial pure-Go Git implementation.
- **Remote refs change.**
  Mitigation: require explicit `--ref`; no branch-name-only deploys.

### Open Questions

- None blocking. v1 relies on the system `git` binary and existing SSH/credential helpers rather than adding new auth flags.
