# E1-S2 - Implement Deterministic Renderer

**Epic:** Epic 1 - Foundations: Repo schema + local render
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 3 days
**Dependencies:** E1-S1
**Target:** Go CLI (htmlctl)
**Design Reference:** docs/technical-spec.md sections 3, 2.1, 2.5

---

## 1. Objective

Implement the publish-time stitching renderer that composes parsed resource schemas into complete, static HTML pages. The renderer must be fully deterministic: given the same inputs, it must produce byte-identical output every time, on every platform. This determinism is a core product requirement -- it underpins reliable diffs, promotion (staging to prod without rebuild), and hash-based integrity checks.

## 2. User Story

As an AI agent managing a website, I want to render a site directory into static HTML output so that I can preview changes locally and trust that the same input always produces the same output.

## 3. Scope

### In Scope

- Base template implementation: a built-in `default` template providing the `<html><head>...</head><body><main>{{content}}</main>...</body></html>` structure
- Style injection: inject `tokens.css` and `default.css` into `<head>` as `<link>` tags in a stable, deterministic order
- Script injection: inject a `<script>` tag at end of `<body>` only if `scripts/site.js` exists; the injected `src` points to the content-addressed script output path
- Component stitching: concatenate component HTML fragments into `<main>` in the order specified by the page's `spec.layout` includes
- Page title and description injection into `<head>` via `<title>` and `<meta name="description">` tags
- Output file generation: `/index.html` for route `/`, and `/<route>/index.html` for all other routes (e.g., `/product` produces `/product/index.html`)
- Asset copying: emit both content-addressed asset files and original `assets/...` compatibility paths in the output directory
- Style copying: emit content-addressed CSS files and inject those hashed paths in `<link>` tags
- Script copying: emit content-addressed `site.js` output when present and inject that hashed path in `<script src>`
- Determinism guarantees: stable ordering of all injections, normalized LF line endings, no timestamps or random values in output, content-addressed static filenames (sha256-based)
- A `Render(site *model.Site, outputDir string) error` function that produces the complete output directory
- Clean output: remove and recreate the output directory on each render to avoid stale files

### Out of Scope

- Multiple base templates (v1 has only `default`; post-MVP roadmap item)
- Component CSS/JS fragments (post-v1 per technical spec section 2.4)
- Minification or bundling of CSS/JS
- Server-side rendering or dynamic content
- Release creation, immutable release packaging (Epic 2)
- Remote rendering on htmlservd (Epic 2)
- Image optimization or responsive image generation

## 4. Architecture Alignment

- **Publish-time stitching**: Follows the composition model in technical-spec.md section 3.1 -- pages are layouts referencing components, renderer produces full HTML
- **Determinism requirements**: Implements all four guarantees from technical-spec.md section 3.2: stable ordering, normalized LF, no time-dependent output, content-addressed asset names
- **Base template**: The built-in `default` template matches the structure described in section 3.1: `<html><head>...</head><body><main>{{content}}</main>...</body></html>`
- **Consumes E1-S1 output**: Takes `*model.Site` as input, which contains parsed Website, Pages, Components, StyleBundle, scripts, and assets
- **Package boundary**: This story creates `pkg/renderer/` which depends on `pkg/model/` from E1-S1
- **No concurrency concerns**: Rendering is sequential; pages are rendered one at a time

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `pkg/renderer/renderer.go` - Core render function: takes Site, produces output directory
- `pkg/renderer/template.go` - Built-in `default` base template definition and HTML generation
- `pkg/renderer/assets.go` - Asset copying with content-addressed filenames (sha256 hashing)
- `pkg/renderer/determinism.go` - Helpers for LF normalization, stable ordering, content hashing

### 5.2 Files to Modify

- None (new package; depends on E1-S1 types but does not modify them)

### 5.3 Tests to Add

- `pkg/renderer/renderer_test.go` - Integration tests: render a full site, verify output files exist and contain expected content
- `pkg/renderer/template_test.go` - Unit tests: base template produces correct HTML structure, injects title/description/styles/scripts correctly
- `pkg/renderer/assets_test.go` - Unit tests: content-addressed filename generation, asset copying
- `pkg/renderer/determinism_test.go` - Unit tests: LF normalization, idempotent render (render twice, compare byte-for-byte)
- `testdata/valid-site/` - Reuse fixture from E1-S1 (or extend it with assets)
- `testdata/site-with-assets/` - Fixture with images/SVGs in assets/ to test content-addressed copying
- `testdata/site-no-scripts/` - Fixture without scripts/site.js to test optional script handling

### 5.4 Dependencies/Config

- Go standard library `html/template` or `text/template` for base template rendering
- Go standard library `crypto/sha256` for content-addressed asset names
- Go standard library `os`, `io`, `path/filepath` for file output
- No new external dependencies beyond what E1-S1 introduces

## 6. Acceptance Criteria

- [ ] AC-1: Rendering a site with route `/` produces `<outputDir>/index.html`
- [ ] AC-2: Rendering a site with route `/product` produces `<outputDir>/product/index.html`
- [ ] AC-3: Output HTML contains the base template structure: `<!DOCTYPE html>`, `<html>`, `<head>`, `<body>`, `<main>`
- [ ] AC-4: `<head>` contains `<title>` with the page's `spec.title` value
- [ ] AC-5: `<head>` contains `<meta name="description">` with the page's `spec.description` value
- [ ] AC-6: `<head>` contains `<link>` tags for `tokens.css` and `default.css` in that stable order
- [ ] AC-7: Component HTML fragments appear inside `<main>` in the exact order specified by the page's `spec.layout`
- [ ] AC-8: When `scripts/site.js` is present, a `<script>` tag is injected at the end of `<body>` and its `src` points to the content-addressed script path (`/scripts/site-<hash>.js`)
- [ ] AC-9: When `scripts/site.js` is absent, no script tag is injected
- [ ] AC-10: Running `Render()` twice on the same input produces byte-identical output (determinism)
- [ ] AC-11: All output files use LF line endings (no CRLF)
- [ ] AC-12: Asset files are copied to the output directory with content-addressed filenames (sha256-based)
- [ ] AC-13: CSS files are copied to the output directory and link tags reference the correct paths
- [ ] AC-14: Output directory is cleaned before each render (no stale files from previous runs)
- [ ] AC-15: Asset files also exist at their original `assets/...` paths so existing component HTML references (for example `/assets/logo.svg`) remain valid without HTML rewriting

## 7. Verification Plan

### Automated Tests

- [ ] Integration test: render the valid-site testdata fixture, verify all expected output files exist
- [ ] Integration test: verify output HTML structure matches expected template (doctype, html, head, body, main)
- [ ] Unit test: title and description injection into head
- [ ] Unit test: style link injection order (tokens.css before default.css)
- [ ] Unit test: component stitching order matches layout spec
- [ ] Unit test: script injection present/absent based on site.js existence and injected path uses hashed filename
- [ ] Determinism test: render same site twice to separate output dirs, compare all files byte-for-byte
- [ ] Unit test: LF normalization strips CR characters
- [ ] Unit test: content-addressed asset filename uses sha256 of file content
- [ ] Integration test: original asset compatibility paths (for example `assets/logo.svg`) are present alongside hashed outputs
- [ ] Integration test: multi-page site produces correct directory structure (index.html + route/index.html)

### Manual Tests

- [ ] Render the sample futurelab site and open the output in a browser to verify it displays correctly
- [ ] Verify that running render on macOS and Linux produces identical output (if cross-platform environment available)

## 8. Performance / Reliability Considerations

- Sites are small (tens of pages, tens of components); rendering should complete in milliseconds
- File I/O errors during output (permission denied, disk full) should produce clear error messages
- Content-addressed asset hashing uses sha256 which is fast for typical asset sizes (< 10MB each)
- Output directory cleanup (remove + recreate) must handle read-only files gracefully

## 9. Risks & Mitigations

- **Risk**: Go `html/template` auto-escapes content, which could mangle component HTML fragments - **Mitigation**: Use `text/template` for the base template or mark component content as `template.HTML` (safe) since component HTML is trusted admin content per the PRD
- **Risk**: File system ordering differs across OS/FS (e.g., HFS+ vs ext4 readdir order) could break determinism - **Mitigation**: Always sort file lists explicitly; never rely on directory iteration order
- **Risk**: Asset content-addressing changes filenames, breaking hardcoded references in component HTML - **Mitigation**: For v1, assets referenced in HTML use original paths; content-addressed names are for cache-busting in link/script tags only. Document this clearly.
- **Risk**: Large binary assets (videos) slow down hashing - **Mitigation**: Not a v1 concern; site assets are expected to be small images/SVGs

## 10. Open Questions

- Should the base template be a Go-embedded string or loaded from a file? (Recommendation: Go-embedded via `embed` package for zero-config simplicity)
- Should content-addressed filenames apply to CSS/JS as well, or only to assets? (Recommendation: apply to all static files for cache-busting consistency)
- Should the renderer produce a manifest file listing all outputs with their hashes? (Useful for later release/promotion stories; could add now cheaply)

## 11. Research Notes

### Go text/template vs html/template

- `html/template` auto-escapes HTML content, which is undesirable for trusted component fragments
- `text/template` gives full control; since all content is admin-authored and trusted, this is safe
- Alternative: use `html/template` with `template.HTML` type to mark trusted content
- Recommendation: use `text/template` for simplicity and clarity

### Deterministic File Output Patterns

- Always sort pages by route before rendering to ensure stable file creation order
- Use `\n` (LF) explicitly; never use `fmt.Println` which may use platform line endings
- Use `bytes.Buffer` to assemble output in memory, then write atomically
- Content-addressed naming: `<name>-<sha256prefix>.<ext>` (e.g., `hero-a1b2c3d4.jpg`)
- Truncate sha256 to first 8-12 hex characters for readable filenames

---

## Implementation Summary

- Implemented deterministic renderer in `pkg/renderer` with:
  - deterministic page rendering order,
  - base template composition,
  - stable stylesheet/script injection,
  - LF-normalized, atomic output writes.
- Implemented content-addressed static outputs (styles, script, assets).
- Compatibility fix: assets are now written in two forms:
  - hashed path (for deterministic/cache-friendly outputs), and
  - original path under `assets/` (so existing component HTML references like `/assets/logo.svg` keep working without HTML rewriting).

## Code Review Findings

- Initial implementation wrote only hashed asset names, which could break hardcoded component asset URLs.
- Fixed by preserving original asset paths in output while retaining hashed copies.

## Completion Status

- Implemented and validated with renderer unit/integration tests.
