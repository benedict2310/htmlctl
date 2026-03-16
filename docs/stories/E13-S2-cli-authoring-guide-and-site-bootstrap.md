# E13-S2 — CLI Authoring Guide and Site Bootstrap

**Epic:** Epic 13 — Agent-Native Site Discoverability  
**Status:** Proposed  
**Priority:** P1  
**Estimated Effort:** 3-4 days  
**Dependencies:** E1-S1 through E1-S4, E10-S5, E13-S1  
**Target:** `internal/cli/`, `docs/`, `.agent/skills/htmlctl-publish/`  
**Design Reference:** zero-context operator review on 2026-03-15

---

## 1. Summary

Move the core authoring model out of out-of-band docs and into the CLI by adding a built-in site guide and a minimal site bootstrap flow. A fresh operator should be able to learn the page/component/design model and scaffold a valid starting site from the terminal alone.

## 2. Architecture Context and Reuse Guidance

- Reuse the canonical source-layout rules already documented in:
  - `docs/technical-spec.md`
  - `docs/operations-manual-agent.md`
  - `.agent/skills/htmlctl-publish/SKILL.md`
- Reuse `pkg/loader`, `pkg/validator`, and `pkg/model` constraints as the source of truth. Do not create a second divergent schema description in code.
- Improve the help of existing commands instead of replacing them:
  - `apply`
  - `diff`
  - `render`
  - `get`
  - `status`
- Keep bootstrap deterministic and minimal. `site init` should emit a small valid site tree that immediately passes `htmlctl render`.

## 3. Proposed Changes and Architecture Improvements

### 3.1 New `site` guidance commands

Add a new CLI group focused on authoring guidance:

- `htmlctl site explain`
- `htmlctl site init <dir>`

`site explain` should answer the zero-context questions that are currently buried in docs:

- canonical site skeleton
- page vs component model
- page-body component guidance
- component validator constraints
- supported sidecars:
  - `components/<name>.css`
  - `components/<name>.js`
- where website metadata belongs:
  - `website.yaml`
  - `branding/`
  - `styles/`
  - `assets/`
- valid partial-apply paths
- generated-artifact caveats:
  - OG image generation
  - favicon/robots/sitemap/llms.txt/structured data
  - `promote` byte-for-byte behavior

Prefer high-signal text output for humans, with optional `json|yaml` for agents that want machine-readable summaries.

### 3.2 `site init`

Add:

- `htmlctl site init <dir> --template minimal`

The emitted skeleton should include:

- `website.yaml`
- `pages/index.page.yaml`
- `components/hero.html`
- `styles/tokens.css`
- `styles/default.css`

Optional niceties:

- `scripts/site.js` omitted by default unless explicitly requested
- comments kept minimal
- ASCII only

The generated site must pass:

- `htmlctl render -f <dir> -o <dir>/dist`

### 3.3 Help text overhaul for core authoring commands

Upgrade the built-in help of:

- `apply`
- `diff`
- `render`
- `get`
- `status`

Minimum improvements:

- `Long` description with context-aware behavior explained
- examples for full-site apply, file-level apply, and `--from-git`
- explicit supported file paths for partial apply
- examples showing active-context fallback
- explicit “use `inspect` for deep resource details” hint in `get --help`

### 3.4 Keep docs and skill synchronized

This story should reduce dependence on docs, not eliminate them. Update docs and the publish skill to mirror the CLI language so agents see the same terms everywhere:

- reusable section component
- page-body component
- composed page
- website-level metadata
- runtime environment config vs bundle content

## 4. File Touch List

### Files to Create

- `internal/cli/site_cmd.go` — root `site` command plus `explain` and `init`
- `internal/cli/site_cmd_test.go`

### Files to Modify

- `internal/cli/root.go`
- `internal/cli/apply_cmd.go`
- `internal/cli/diff_cmd.go`
- `internal/cli/render.go`
- `internal/cli/get_cmd.go`
- `internal/cli/status_cmd.go`
- `internal/cli/render_test.go`
- `internal/cli/apply_cmd_test.go`
- `internal/cli/diff_cmd_test.go`
- `docs/technical-spec.md`
- `docs/operations-manual-agent.md`
- `docs/README.md`
- `.agent/skills/htmlctl-publish/SKILL.md`
- `.agent/skills/htmlctl-publish/references/commands.md`
- `.agent/skills/htmlctl-publish/references/resource-schemas.md`

## 5. Implementation Steps

1. Add a new `site` command group and register it from `root.go`.
2. Implement `site explain` with a compact, deterministic summary of the authoring model.
3. Implement `site init <dir> --template minimal`:
   - fail if target already exists and is non-empty unless `--force` is explicitly chosen
   - emit a minimal valid site tree
   - keep generated naming and content deterministic
4. Expand help text and examples for `apply`, `diff`, `render`, `get`, and `status`.
5. Align docs and the publish skill with the command wording and examples.

## 6. Tests and Validation

### Automated

- `site explain` prints the expected sections and supports `json|yaml`
- `site init` creates the expected files with deterministic contents
- `site init` refuses to overwrite a non-empty target by default
- a freshly initialized site passes `htmlctl render`
- `apply --help`, `diff --help`, `render --help`, and `get --help` include the new examples and guidance

### Manual

- hand a fresh operator only:
  - `htmlctl site explain`
  - `htmlctl site init ./site --template minimal`
  - `htmlctl render -f ./site -o ./dist`
- verify they can understand the supported page/component/design model without opening the docs first

## 7. Acceptance Criteria

- [ ] AC-1: `htmlctl site explain` documents the supported site skeleton, page/component model, sidecars, website metadata locations, and generated-artifact caveats.
- [ ] AC-2: `htmlctl site init <dir> --template minimal` creates a minimal valid site that renders successfully without manual fixes.
- [ ] AC-3: `apply`, `diff`, `render`, `get`, and `status` help output includes actionable examples and no longer hides key agent-facing behavior behind vague phrasing like “supported site file path”.
- [ ] AC-4: CLI terminology matches docs and `.agent/skills/htmlctl-publish/` guidance closely enough that an operator does not have to translate between competing vocabularies.

## 8. Risks and Open Questions

### Risks

- **Risk:** guidance drifts from the actual loader/validator rules.  
  **Mitigation:** derive command output from shared constants/types where practical and add tests that assert important phrases/examples.

- **Risk:** `site init` turns into a large templating system.  
  **Mitigation:** ship one minimal template only in v1; defer theme catalogs and richer starters.

- **Risk:** help text becomes verbose without becoming clearer.  
  **Mitigation:** optimize for fast comprehension and concrete examples; keep long-form docs out of `--help`.

### Open Questions

- Should `site explain` default to human text only, or support structured output on day one? Recommendation: support `json|yaml` from the start because the target user is often another agent.
