# E1-S3 - Component Validation Engine

**Epic:** Epic 1 - Foundations: Repo schema + local render
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** E1-S1
**Target:** Go CLI (htmlctl)
**Design Reference:** docs/technical-spec.md section 6.1

---

## 1. Objective

Implement a validation engine that enforces structural rules on component HTML fragments before rendering. Components are the primary editing surface for AI agents, so catching malformed HTML early -- before it reaches the renderer or production -- prevents broken layouts, accessibility issues, and security problems. The validation rules are drawn directly from the production safety requirements in the technical spec.

## 2. User Story

As an AI agent editing website components, I want clear validation errors when my component HTML violates structural rules so that I can fix problems before rendering or deploying.

## 3. Scope

### In Scope

- **Single-root element validator**: parse component HTML and verify it contains exactly one root element (no text nodes or multiple elements at root level)
- **Root tag allowlist**: the root element must be one of `section`, `header`, `footer`, `main`, `nav`, `article`, `div` (configurable via a default list that can be overridden)
- **Anchor ID validator**: if a component is used in a page layout (i.e., it is anchor-navigable), its root element must include an `id` attribute matching the component name
- **Script disallow rule**: component HTML must not contain `<script>` tags anywhere in the tree; JavaScript is only permitted via `scripts/site.js`
- **Validation result type**: a structured result containing the component name, pass/fail status, and a list of diagnostic messages with severity (error/warning)
- **Batch validation**: validate all components in a site, collecting all errors before returning (not fail-fast on first error)
- **A `ValidateComponent(component *model.Component, isNavigable bool) []ValidationError` function** for single-component validation with usage context
- **A `ValidateAllComponents(site *model.Site) []ValidationError` function** for batch validation across the entire site
- **Usage-aware batch validation**: `ValidateAllComponents` first computes component usage from page layouts to determine which components are anchor-navigable before applying anchor-ID rules
- **Clear, actionable error messages** that include the component name, the rule violated, and what to fix

### Out of Scope

- Page validation (route normalization, include cycle detection) -- these are simple checks handled inline in the loader (E1-S1)
- Asset validation (filename sanitization, size limits, content-type allowlist) -- deferred to Epic 2 server-side stories
- Bundle validation (hash verification from client manifests) -- deferred to Epic 2
- HTML sanitization or rewriting (we validate and reject; we do not auto-fix)
- CSS validation within components
- Accessibility linting beyond the anchor ID rule (e.g., ARIA attributes, heading levels)
- Custom per-component validation rules (v1 uses a fixed rule set)

## 4. Architecture Alignment

- **Validation rules**: Implements technical-spec.md section 6.1 (Component validation) exactly: single root, tag allowlist, anchor ID, no scripts
- **Production safety**: These validations are a key part of the production safety model described in the PRD section 7 (Risks) -- catching mistakes before they reach production
- **Consumes E1-S1 output**: Takes `*model.Component` and `*model.Site` as inputs from the loader package
- **Package boundary**: Creates `pkg/validator/` which depends on `pkg/model/` from E1-S1; consumed by E1-S4 (CLI) and later by E2-S3 (server-side apply)
- **No concurrency concerns**: Validation is stateless and sequential
- **HTML parsing**: Uses `golang.org/x/net/html` for robust HTML fragment parsing rather than regex

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `pkg/validator/validator.go` - Core validation orchestration: ValidateComponent (with usage context), ValidateAllComponents, ValidationError type
- `pkg/validator/rules.go` - Individual rule implementations: single root, tag allowlist, anchor ID, no scripts
- `pkg/validator/config.go` - Default configuration (allowed root tags list) with option to override

### 5.2 Files to Modify

- `go.mod` - Add `golang.org/x/net` dependency for HTML parsing

### 5.3 Tests to Add

- `pkg/validator/validator_test.go` - Integration tests: validate full components, batch validation across a site
- `pkg/validator/rules_test.go` - Unit tests for each individual rule with valid and invalid inputs
- `testdata/components/valid-section.html` - Valid component with `<section id="name">` root
- `testdata/components/valid-div.html` - Valid component with `<div>` root (no anchor ID needed if not navigable)
- `testdata/components/multi-root.html` - Invalid: two sibling root elements
- `testdata/components/text-root.html` - Invalid: text content at root level before element
- `testdata/components/bad-tag.html` - Invalid: root element is `<span>` (not in allowlist)
- `testdata/components/has-script.html` - Invalid: contains a `<script>` tag
- `testdata/components/nested-script.html` - Invalid: contains a `<script>` tag nested deep in the tree
- `testdata/components/missing-id.html` - Invalid: anchor-navigable component without `id` on root
- `testdata/components/wrong-id.html` - Invalid: anchor-navigable component with `id` not matching component name

### 5.4 Dependencies/Config

- `golang.org/x/net/html` - Go HTML tokenizer and parser; handles HTML fragments correctly including malformed HTML recovery

## 6. Acceptance Criteria

- [ ] AC-1: A component with exactly one root element from the allowlist passes validation
- [ ] AC-2: A component with zero root elements (empty or whitespace-only) fails with a clear error
- [ ] AC-3: A component with multiple root elements fails with an error naming the component and listing the extra elements found
- [ ] AC-4: A component with a root element not in the allowlist (e.g., `<span>`, `<p>`) fails with an error listing the allowed tags
- [ ] AC-5: A component containing a `<script>` tag at any nesting depth fails with an error explaining that scripts are not allowed in components
- [ ] AC-6: An anchor-navigable component whose root element lacks an `id` attribute fails with an error specifying the expected `id` value
- [ ] AC-7: An anchor-navigable component whose root `id` does not match the component name fails with an error showing expected vs actual `id`
- [ ] AC-8: Batch validation (`ValidateAllComponents`) reports errors for all invalid components, not just the first one
- [ ] AC-9: Error messages include the component name, the rule that was violated, and a human-readable description of how to fix it
- [ ] AC-10: The root tag allowlist defaults to `section`, `header`, `footer`, `main`, `nav`, `article`, `div` and can be overridden programmatically
- [ ] AC-11: Valid component HTML with nested elements, attributes, and whitespace passes all rules without false positives
- [ ] AC-12: `ValidateAllComponents` derives navigability from `site.Pages[*].spec.layout[*].include` and only applies anchor-ID checks to referenced components

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: single root `<section>` passes validation
- [ ] Unit test: single root `<div>` passes validation when `isNavigable=false`
- [ ] Unit test: single root `<header>` passes validation
- [ ] Unit test: two sibling `<section>` elements fails with multi-root error
- [ ] Unit test: root `<span>` fails with tag allowlist error
- [ ] Unit test: root `<p>` fails with tag allowlist error
- [ ] Unit test: nested `<script>` tag at depth 3 fails with script disallow error
- [ ] Unit test: `<script>` as direct child of root fails with script disallow error
- [ ] Unit test: component named "pricing" with root `<section id="pricing">` passes anchor ID check when `isNavigable=true`
- [ ] Unit test: component named "pricing" with root `<section>` (no id) fails anchor ID check when `isNavigable=true`
- [ ] Unit test: component named "pricing" with root `<section id="wrong">` fails with expected vs actual id when `isNavigable=true`
- [ ] Unit test: empty HTML content fails with zero-root error
- [ ] Unit test: whitespace-only HTML content fails with zero-root error
- [ ] Unit test: HTML comments at root level are ignored (do not count as root elements)
- [ ] Integration test: batch validate a site with 3 valid and 2 invalid components, verify 2 errors returned
- [ ] Unit test: `ValidateAllComponents` computes usage index from page layouts and passes `isNavigable=true` only for referenced components
- [ ] Unit test: custom allowlist overrides the default (e.g., adding `<aside>` to the list)

### Manual Tests

- [ ] Create a deliberately broken component (multi-root) and verify the error message is clear enough for an agent to fix the issue
- [ ] Validate all components in the sample futurelab site to ensure they all pass

## 8. Performance / Reliability Considerations

- Component HTML fragments are small (typically under 1KB); parsing performance is not a concern
- `golang.org/x/net/html` handles malformed HTML gracefully via browser-like error recovery, which means it will always produce a parse tree even for broken input; the validator must inspect the recovered tree carefully
- Validation is pure and stateless; no I/O beyond reading the already-loaded component HTML from memory

## 9. Risks & Mitigations

- **Risk**: `golang.org/x/net/html` wraps fragments in implicit `<html><head><body>` structure during parsing, which complicates root element counting - **Mitigation**: Parse using `html.ParseFragment()` with a `<body>` context element to get direct children without the implicit wrapper; count only `ElementNode` types among the returned nodes
- **Risk**: HTML comments or whitespace text nodes at root level could be incorrectly counted as root elements - **Mitigation**: Filter node list to only `html.ElementNode` types; skip `TextNode` that is purely whitespace and `CommentNode`
- **Risk**: Self-closing tags like `<br/>` or `<img/>` could appear at root and pass the single-root check but be meaningless as a component root - **Mitigation**: The tag allowlist naturally excludes void elements since `br`, `img`, etc. are not in the allowed set
- **Risk**: Agents may produce components with inline `<style>` tags; unclear if this should be allowed - **Mitigation**: Allow `<style>` in v1 (only `<script>` is disallowed per spec); revisit if CSS scoping is added post-v1

## 10. Open Questions

- Should `<style>` tags be allowed in components, or should CSS be restricted to the style bundle only? (Recommendation: allow for v1, warn but do not error)
- Should the anchor ID validator run only for components actually used in page layouts, or for all components? (Recommendation: validate only when used in a layout, since standalone components may not need IDs)
- Should validation errors include line/column numbers within the component HTML? (Recommendation: nice-to-have but not required for v1; the component files are typically short)

## 11. Research Notes

### Go HTML Parsing (golang.org/x/net/html)

- `html.Parse()` parses a complete HTML document, wrapping in implicit html/head/body
- `html.ParseFragment()` parses an HTML fragment in the context of a given parent element; returns a slice of `*html.Node`
- For component validation, use `html.ParseFragment(reader, &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})` to parse fragments as body content
- Node types: `ElementNode`, `TextNode`, `CommentNode`, `DoctypeNode`, `DocumentNode`
- The `Data` field on `ElementNode` contains the tag name (lowercase)
- Attributes are available via the `Attr` slice on each node

### HTML Fragment Validation Approaches

- Parse fragment, collect root-level ElementNodes, verify count is exactly 1
- Walk the tree recursively to find `<script>` tags at any depth
- Check `node.Attr` for `id` attribute on the root element
- The parser is lenient (like a browser); it will not reject malformed HTML but will produce a best-effort tree. Our validator inspects the resulting tree rather than the raw text.

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
