# E10-S5 — Component-Scoped CSS/JS Fragments

**Epic:** Epic 10 — Review, Automation, and Lifecycle
**Status:** Implemented (2026-03-04)
**Priority:** P2
**Estimated Effort:** 5-6 days
**Dependencies:** E1-S1 (site loader), E1-S2 (renderer), E1-S3 (component validation), E2-S3 (bundle ingestion)
**Target:** `pkg/model/`, `pkg/loader/`, `pkg/renderer/`, `internal/state/`, `internal/db/`, `internal/cli/`
**Design Reference:** PRD 10.3 component CSS/JS roadmap item

---

## 1. Summary

Allow components to carry optional sidecar CSS and JS files so pages can ship local behavior without pushing everything into global `styles/default.css` and `scripts/site.js`.

## 2. Architecture Context and Reuse Guidance

- Preserve the current component-first model. Sidecars belong to the same component name:
  - `components/hero.html`
  - `components/hero.css` (optional)
  - `components/hero.js` (optional)
- Keep output deterministic and CSP-friendly:
  - emit sidecars as external, content-addressed files
  - no inline `<style>` or inline `<script>`
- Reuse the existing renderer static-file pipeline for content-addressed output.
- Reuse current page-layout order to determine CSS/JS injection order.
- Extend the current bundle/apply source builder so file-level apply can package component sidecars as partial updates; do not assume the current directory-only builder is sufficient.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Model and storage

Extend component state to include optional CSS and JS hashes. Keep HTML in the existing `content_hash` field; add:

- `css_hash TEXT NOT NULL DEFAULT ''`
- `js_hash TEXT NOT NULL DEFAULT ''`

in a migration against `components`.

Loader should read optional sidecars from `components/<name>.css` and `components/<name>.js`.

### 3.2 Bundle and apply

Allow `Component` resources to reference:

- required HTML file
- optional CSS sidecar
- optional JS sidecar

Partial apply should work for sidecar files as well as `.html`.

### 3.3 Rendering

For each page:

- collect components in layout order
- dedupe sidecars by component first occurrence
- emit CSS links after global styles
- emit `defer` JS scripts after global `site.js`

Output paths should be deterministic and content-addressed, for example:

- `/components/hero.<hash>.css`
- `/components/hero.<hash>.js`

### 3.4 Validation and scope limits

Keep v1 narrow:

- sidecars are optional
- one CSS file and one JS file per component
- CSS is emitted verbatim
- JS is emitted verbatim
- relative `url(...)` references in component CSS are rejected in v1; authors must use absolute `/assets/...` URLs

## 4. File Touch List

### Files to Create

- `pkg/renderer/component_fragments_test.go`

### Files to Modify

- `pkg/model/types.go`
- `pkg/loader/loader.go`
- `pkg/loader/validate.go`
- `pkg/loader/loader_test.go`
- `pkg/loader/validate_test.go`
- `pkg/renderer/assets.go`
- `pkg/renderer/renderer.go`
- `pkg/renderer/template.go`
- `pkg/renderer/renderer_test.go`
- `internal/bundle/build.go`
- `internal/bundle/manifest.go`
- `internal/bundle/build_test.go`
- `internal/cli/apply_cmd.go`
- `internal/cli/apply_cmd_test.go`
- `internal/state/merge.go`
- `internal/state/merge_test.go`
- `internal/db/migrations/010_component_fragments.go`
- `internal/db/queries.go`
- `internal/db/queries_test.go`
- `pkg/validator/rules.go` and tests if CSS-sidecar validation lives there
- `docs/technical-spec.md`

## 5. Implementation Steps

1. Extend the component model and DB schema with `css_hash` and `js_hash`.
2. Update the loader to read optional sidecars.
3. Extend bundle/apply source resolution so a component sidecar path can be packaged as a partial component update.
4. Update bundle manifest rules so component resources may carry multiple file refs.
5. Update state apply/merge logic to store sidecar hashes in blobs and DB.
6. Extend renderer statics to materialize component CSS/JS files and inject page-specific links/scripts in deterministic order.
7. Add CSS validation rejecting relative `url(...)` references.
8. Emit component JS with `defer` and preserve deterministic script order.

## 6. Tests and Validation

### Automated

- Loader tests:
  - component with only HTML
  - component with HTML+CSS
  - component with HTML+JS
  - component with both sidecars
- Bundle/state tests:
  - component manifest accepts optional sidecars
  - partial apply updates only one sidecar
  - file-level apply of `components/<name>.css` and `components/<name>.js` packages the owning component correctly
- Renderer tests:
  - injected CSS/JS order follows layout order
  - sidecars are deduped by first component occurrence
  - output filenames are content-addressed and deterministic
  - component JS script tags are emitted with `defer`
  - pages without sidecars remain unchanged
- Validation tests:
  - relative `url(...)` in component CSS is rejected

### Manual

- Add one component-local CSS file and confirm only pages using that component load it.
- Add one component-local JS file and confirm only pages using that component load it.
- Reuse the same component twice in one page and confirm sidecars are emitted once.

## 7. Acceptance Criteria

- [ ] AC-1: Components may define optional `.css` and `.js` sidecars next to their `.html` file.
- [ ] AC-2: Sidecars are stored and deployed through the existing bundle/apply pipeline.
- [ ] AC-3: Pages inject component CSS/JS in deterministic order and dedupe duplicate includes.
- [ ] AC-4: Sidecars are emitted as external content-addressed files, not inline tags.
- [ ] AC-5: Relative `url(...)` references in component CSS are rejected in v1.
- [ ] AC-6: Existing sites without sidecars render byte-identically to current behavior.
- [ ] AC-7: Partial apply can update component sidecars without requiring a full-site rewrite.
- [ ] AC-8: Component JS is emitted with `defer` in deterministic order after the global script.
- [ ] AC-9: `go test ./pkg/... ./internal/bundle/... ./internal/state/... ./internal/db/...` passes.

## 8. Risks and Open Questions

### Risks

- **Renderer drift for existing sites.**
  Mitigation: preserve old output exactly when no sidecars are present.
- **CSS sidecars break asset URL expectations.**
  Mitigation: reject relative `url(...)` in v1.
- **Global/script ordering becomes ambiguous.**
  Mitigation: keep fixed order: global styles, component CSS, page content, global script, component JS.

### Open Questions

- None blocking. v1 uses same-directory sidecars and does not introduce template-time JS modules or CSS bundling.
