# E6-S3 - HTML XSS Hardening

**Epic:** Epic 6 — Security Hardening
**Status:** Done
**Priority:** P0 (Critical — stored XSS served to end users)
**Estimated Effort:** 1 day
**Dependencies:** E1-S2 (deterministic renderer), E1-S3 (component validation)
**Target:** htmlctl (renderer) + htmlservd (bundle ingestion validator)
**Design Reference:** Security Audit 2026-02-20, Vulns 3 & 11

---

## 1. Objective

Two independent XSS vectors exist in the HTML generation pipeline:

1. The page renderer uses `text/template` instead of `html/template`, so `Title`, `Description`, and other metadata fields are written verbatim into HTML without escaping. Attacker-controlled page YAML produces a rendered HTML file that executes arbitrary JavaScript in visitor browsers.
2. The component validator checks for `<script>` elements but does not check inline event handler attributes (`onclick`, `onerror`, `onload`, etc.). A component with `<img src="x" onerror="payload">` passes all validation rules and is stored and served.

This story fixes both vectors: switch the renderer to `html/template` and extend the validator to reject `on*` attributes.

## 2. User Story

As a site visitor, I want the HTML pages served by htmlctl to not execute attacker-supplied JavaScript, and as an operator, I want the component validator to catch all common XSS vectors — not just `<script>` tags.

## 3. Scope

### In Scope

- Change `pkg/renderer/template.go` import from `"text/template"` to `"html/template"`.
- Cast `PageData.ContentHTML` to `template.HTML` so that intentional raw HTML (the assembled component body) is not double-escaped.
- Verify that `Title`, `Description`, `StyleHrefs`, `ScriptSrc`, and any other string fields auto-escape through the new template engine.
- Extend `pkg/validator/rules.go` `containsScript` (or a renamed `containsUnsafeHTML`) to walk all element attribute names and return `true` (unsafe) for any attribute whose name matches `^on\w+$` (case-insensitive), rejecting the component.
- Update validator error messages to distinguish between `<script>` element rejection and event handler attribute rejection.
- Update all renderer and validator tests to cover the new behaviour.
- Document in the validator that `ContentHTML` is intentionally unescaped and that all other fields go through auto-escaping.

### Out of Scope

- Content Security Policy header enforcement (separate infrastructure concern; can be added as a Caddy directive later).
- Sanitising existing stored component HTML (stored blobs are immutable; a re-validation step would require a separate migration story).
- Blocking CSS-based injection via `style` attributes (lower risk; deferred to future hardening).
- Escaping `ContentHTML` (intentionally raw — architectural constraint).

## 4. Architecture Alignment

- **`text/template` vs `html/template`:** `html/template` is a drop-in replacement for `text/template` that adds context-aware auto-escaping. The only required code change beyond the import is casting intentional raw HTML values to `template.HTML` to opt them out of escaping.
- **Validator depth:** The existing `containsScript` DFS walk already traverses the full element tree. Adding an attribute check at each element node requires only a sibling loop over `node.Attr`, making the change minimal and low-risk.
- **`on*` attribute pattern:** HTML event handler attributes are defined as any attribute beginning with `on` followed by one or more word characters (`^on\w+$`). This pattern covers all current and future HTML event attributes without requiring an explicit allowlist.
- **PRD references:** PRD Section 3 (component model, security constraints), Technical Spec Section 3 (renderer).

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- None — all changes are modifications to existing files.

### 5.2 Files to Modify

- `pkg/renderer/template.go`
  - Change `import "text/template"` → `import "html/template"`.
  - In the template data struct population, cast the `ContentHTML` field: `ContentHTML: template.HTML(assembled)`.
  - Verify the `defaultPageTemplate` constant is syntactically valid for `html/template` (it is, since `{{.ContentHTML}}` with a `template.HTML` value renders unescaped).
- `pkg/validator/rules.go`
  - In `containsScript` (or rename to `containsUnsafeHTML`), add an attribute check loop: for each element node, iterate `node.Attr` and return `true` if any `Key` matches `(?i)^on\w+$`.
  - Update the function's doc comment and rename if the semantics have changed.
- `pkg/validator/rules_test.go`
  - Add test cases: `<section id="x"><img src="x" onerror="evil()"></section>` → rejected.
  - Add test cases: `<section id="x" onclick="evil()">text</section>` → rejected.
  - Confirm `<section id="x"><a href="page.html">link</a></section>` → still accepted.
- `pkg/renderer/renderer_test.go`
  - Add test: page with `Title: "</title><script>alert(1)</script>"` → rendered HTML contains escaped version, not the raw script tag.
  - Add test: page with `Description: '" onload="evil()'` → rendered attribute is escaped, not injected.
- `pkg/renderer/template_test.go`
  - Verify that `ContentHTML` containing actual HTML tags renders unescaped (intentional).
  - Verify that `Title` containing `<script>` is escaped in output.

### 5.3 Tests to Add

See §5.2 — tests are co-located with their source files.

### 5.4 Dependencies / Config

- No new Go dependencies (`html/template` is in the standard library alongside `text/template`).
- No config changes.

## 6. Acceptance Criteria

- [x] AC-1: `pkg/renderer/template.go` imports `html/template`; `text/template` is no longer imported in the renderer package.
- [x] AC-2: A page with `spec.title: '</title><script>alert(1)</script>'` produces a rendered HTML file where the script tag is HTML-escaped (e.g., `&lt;script&gt;`) and does not execute in a browser.
- [x] AC-3: A page with `spec.description: '" onload="evil()'` produces a rendered HTML file where the attribute injection is escaped and does not produce an `onload` attribute.
- [x] AC-4: `ContentHTML` (assembled from component HTML files) renders unescaped in the output so that valid HTML markup in components is preserved.
- [x] AC-5: The component validator rejects any component containing an element with an attribute matching `(?i)^on\w+$` (e.g., `onerror`, `onclick`, `onload`, `onmouseover`).
- [x] AC-6: The validator still accepts components with non-event attributes (e.g., `href`, `src`, `class`, `id`, `style`).
- [x] AC-7: All existing renderer and validator tests continue to pass.

## 7. Verification Plan

### Automated Tests

- [x] Renderer tests: `</title><script>` in Title is escaped in output.
- [x] Renderer tests: `ContentHTML` with raw HTML renders correctly.
- [x] Validator tests: `onerror` attribute rejected; `href` attribute accepted.
- [x] Full renderer + validator integration: a complete site bundle with a safe component renders without error.

### Manual Tests

- [ ] Create a page YAML with a malicious title; run `htmlctl render`; inspect output HTML to confirm script is escaped.
- [ ] Submit a component with `<img onerror="...">` via apply; confirm the server returns a validation error.
- [ ] Verify that an existing valid site (e.g., `testdata/valid-site`) still renders identically before and after the change.

## 8. Performance / Reliability Considerations

- `html/template` parsing is slightly slower than `text/template` due to context analysis, but this is a one-time cost at template parse time (template is compiled once at package init). Rendering throughput is unaffected.
- The attribute walk in the validator adds O(attributes) work per element, which is negligible for typical component HTML sizes.

## 9. Risks & Mitigations

- **Risk:** Existing valid components that happen to use event attributes (e.g., `<button onclick="...">`) are now rejected. **Mitigation:** This is the intended behaviour — inline event handlers are not supported in the security model. Document the restriction; components should use external JS files referenced via `spec.scripts`.
- **Risk:** `ContentHTML` cast to `template.HTML` could introduce double-escaping bugs if the assembled HTML is itself HTML-escaped at an earlier stage. **Mitigation:** Audit the assembly pipeline to confirm `ContentHTML` contains raw HTML, not pre-escaped HTML. Add a round-trip test.

## 10. Open Questions

- Should `style` attributes be inspected for `expression()` or `url(javascript:...)` patterns? This is a lower-risk attack surface and is deferred to a follow-up hardening story.
- Should the validator reject `<a href="javascript:...">` links? Worth discussing — add to a follow-up issue if confirmed.

---

## 11. Implementation Summary

- Switched renderer templating to `html/template` and changed `pageTemplateData.ContentHTML` to `template.HTML` so component markup remains unescaped while metadata is auto-escaped.
- Updated renderer page assembly to cast stitched component HTML at the trust boundary (`ContentHTML: template.HTML(contentHTML)`).
- Replaced script-only validator traversal with unsafe HTML detection that rejects:
  - `<script>` elements (`script-disallow`)
  - inline event handler attributes matching `(?i)^on\w+$` (`event-handler-disallow`)
- Added tests for metadata escaping and trusted content rendering:
  - `pkg/renderer/template_test.go`
  - `pkg/renderer/renderer_test.go`
- Added validator tests for `onerror`/`onclick` rejection and safe attribute acceptance:
  - `pkg/validator/rules_test.go`
- Updated architecture docs to codify event-handler rejection in component validation:
  - `docs/technical-spec.md`
  - `docs/epics.md`

## 12. Completion Status

- Implemented and verified.
