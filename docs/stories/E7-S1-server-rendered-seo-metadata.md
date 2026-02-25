# E7-S1 - Server-Rendered SEO and Share-Card Metadata

**Epic:** Epic 7 — Metadata and Telemetry
**Status:** Implemented (2026-02-23)
**Priority:** P1 (High — SEO/share correctness)
**Estimated Effort:** 2-3 days
**Dependencies:** E1-S1 (resource schemas), E1-S2 (renderer), E2-S2 (SQLite schema), E2-S4 (release builder), E6-S3 (HTML hardening)
**Target:** loader + state + release + renderer
**Design Reference:** `docs/technical-spec.md` sections 2.3 and 3.1, production migration of `example.com/ora`

---

## 1. Objective

Move page metadata generation from client-side JavaScript to server-rendered HTML output so crawlers and social scrapers reliably receive canonical tags, Open Graph, Twitter cards, and JSON-LD without JS execution.

## 2. User Story

As an operator publishing pages with `htmlctl`, I want SEO and share-card metadata to be rendered directly into each page's `<head>`, so search engines and social platforms can index and preview pages correctly.

## 3. Scope

### In Scope

- Extend page schema with optional structured `spec.head` metadata.
- Persist `spec.head` in desired state, DB rows, and release build artifacts.
- Render metadata into HTML `<head>` using `html/template`-safe escaping.
- Support:
  - canonical URL
  - standard `<meta name="...">` entries (`keywords`, `robots`, `author`, `application-name`)
  - Open Graph properties (`og:*`)
  - Twitter properties (`twitter:*`)
  - JSON-LD blocks (`<script type="application/ld+json">`)
- Add deterministic ordering rules for all rendered head tags.
- Add fixture/update docs with an `/ora` example using server-rendered metadata.

### Out of Scope

- Arbitrary raw HTML injection into `<head>`.
- Dynamic runtime metadata mutation via browser JS.
- Full custom template engine (keep built-in `default` template path).

## 4. Architecture Context and Reuse Guidance

- Reuse existing page pipeline; do not create a parallel metadata path:
  - parse in `pkg/model` + `pkg/loader`
  - persist via `internal/state/merge.go`, `internal/db/queries.go`, and release rebuild path in `internal/release/builder.go`
  - render in `pkg/renderer/template.go`.
- Keep security invariants from Epic 6:
  - all metadata must be escaped by `html/template`
  - JSON-LD payload must be validated/serialized server-side, never copied as raw untrusted HTML.
- Keep deterministic output invariants from Epic 1:
  - stable tag ordering
  - LF-only output
  - deterministic serialization for JSON-LD.

### Library/Version Decision (via `gh` research)

- No new third-party rendering library is needed; standard library `html/template` and `encoding/json` are sufficient and safer.
- Keep dependency footprint unchanged for this story.

## 5. Proposed Changes

### 5.1 Schema and Persistence

Add optional page-level metadata object:

```yaml
spec:
  head:
    canonicalURL: https://example.com/ora
    meta:
      keywords: Ora, macOS voice assistant
      robots: index,follow
      author: Benedict
      application-name: Ora
    openGraph:
      type: website
      url: https://example.com/ora
      siteName: Sample Studio
      locale: en_US
      title: Ora for macOS | Local Voice Assistant
      description: Local-first voice assistant for macOS...
      image: /assets/ora/og-image.jpg
    twitter:
      card: summary_large_image
      url: https://example.com/ora
      title: Ora for macOS | Local Voice Assistant
      description: A privacy-first macOS assistant running fully on your device.
      image: /assets/ora/og-image.jpg
    jsonLD:
      - id: ora-softwareapplication
        payload:
          "@context": https://schema.org
          "@type": SoftwareApplication
          name: Ora
```

Persist this in `pages.head_json` as canonical JSON.

### 5.2 Rendering

- Render metadata in this order for determinism:
1. canonical
2. standard `meta[name]` entries (sorted by name)
3. Open Graph properties (fixed key order)
4. Twitter properties (fixed key order)
5. JSON-LD scripts (input order, stable JSON key ordering where possible)

### 5.3 Documentation

- Update technical spec page schema and rendering sections.
- Include guidance that social metadata must be defined in manifests, not injected at runtime.

## 6. File Touch List

### Files to Create

- `internal/db/migrations/003_pages_head_metadata.go` — add `pages.head_json TEXT NOT NULL DEFAULT '{}'`.

  **Migration numbering note:** E7-S2 (telemetry) also claims migration `003`. Since E7-S1 is P1 and E7-S2 is P2, this story should own `003`. E7-S2 must be updated to use `004`. The two stories must not be merged or implemented in parallel without coordinating the migration sequence. See Section 11.4 for full detail.

### Files to Modify

- `pkg/model/types.go` — add typed `PageHead` fields.
- `pkg/model/types_test.go` — schema roundtrip coverage.
- `pkg/loader/loader_test.go` — parse new `spec.head` fixture.
- `internal/db/models.go` — add `HeadJSON` to `PageRow`.
- `internal/db/queries.go` — include `head_json` in insert/upsert/list/select queries.
- `internal/db/queries_test.go` — verify persistence and retrieval.
- `internal/state/merge.go` + tests — write `head_json` into desired state.
- `internal/release/builder.go` + tests — restore `spec.head` from stored JSON when rebuilding site tree.
- `pkg/renderer/template.go` — extend template data and render head tags.
- `pkg/renderer/renderer_test.go` + `pkg/renderer/template_test.go` — deterministic and escaping assertions.
- `docs/technical-spec.md` — schema/rendering docs.

## 7. Implementation Steps

1. Define `PageHead` types in `pkg/model` and wire YAML parsing.
2. Add DB migration and query/model updates (`head_json`).
3. Update state merge + release builder to roundtrip `head_json`.
4. Extend renderer template data model for head tags and JSON-LD.
5. Implement deterministic head tag generation helpers.
6. Add docs and fixtures for `/ora` metadata.
7. Run full tests + race detector.

## 8. Tests and Validation

### Automated

- Unit tests for YAML parsing of `spec.head`.
- DB query tests for `head_json` insert/upsert/list.
- State/release roundtrip test ensuring `spec.head` survives apply -> DB -> release build.
- Renderer tests:
  - expected tags exist
  - metadata values are escaped
  - JSON-LD emitted once with valid JSON
  - deterministic output across runs.

### Manual

- Apply sample site containing `/ora` metadata.
- `curl -s http://<host>/ora/ | rg '<meta|canonical|ld\+json'` confirms server-rendered tags.
- Validate OG/Twitter fields with a scraper simulator that does not execute JS.

## 9. Acceptance Criteria

- [x] AC-1: `Page` resource supports optional `spec.head` metadata and parses successfully.
- [x] AC-2: Metadata survives apply/release pipeline (loader -> DB -> release builder -> renderer) without loss.
- [x] AC-3: `/ora` output HTML contains canonical + OG + Twitter + JSON-LD tags in server-rendered head.
- [x] AC-4: Metadata rendering uses safe escaping and passes XSS-focused tests.
- [x] AC-5: Output ordering is deterministic and stable across repeated renders.
- [x] AC-6: Technical spec is updated to document the schema and rendering behavior.

## 10. Risks and Open Questions

- **Risk:** schema expansion touches DB + release reconstruction path; easy to regress if only renderer is updated.
  - **Mitigation:** require explicit roundtrip test through state and release builder.
- **Risk:** JSON-LD canonicalization can become nondeterministic if map iteration order leaks.
  - **Mitigation:** central helper that serializes in stable order; add determinism tests.
- **Open question:** Should relative URLs in metadata (`/assets/...`) be auto-expanded to absolute URLs per domain?
  - Proposed: keep user-provided values unchanged in v1, document recommended absolute URLs.

---

## 11. Architectural Review Notes

### 11.1 Type Model: Missing Concrete Struct Definitions

**Gap:** The story says "add typed `PageHead` fields" to `pkg/model/types.go` but does not specify the actual Go struct layout. The YAML example has nested objects (`openGraph`, `twitter`, `jsonLD`) and a free-form map (`meta`). Without explicit struct definitions the implementer must guess field names, YAML tags, and JSON tags, risking tag mismatches across the YAML-to-DB-to-YAML roundtrip.

**Current state:** `PageSpec` in `pkg/model/types.go` has `yaml:` tags on every existing field. `PageLayoutItem` additionally carries `json:` tags (used when marshaling layout to DB). The new `Head` field must follow the same dual-tag convention since it will be marshaled to JSON for DB storage and unmarshaled from YAML when read from the page file.

**Recommended fix:** Define the structs explicitly in `pkg/model/types.go` as follows (canonical struct layout for implementer reference):

```go
// PageHead holds optional SEO and share-card metadata for a page.
type PageHead struct {
    CanonicalURL string            `yaml:"canonicalURL" json:"canonicalURL,omitempty"`
    Meta         map[string]string `yaml:"meta"         json:"meta,omitempty"`
    OpenGraph    *OpenGraph        `yaml:"openGraph"    json:"openGraph,omitempty"`
    Twitter      *TwitterCard      `yaml:"twitter"      json:"twitter,omitempty"`
    JSONLD       []JSONLDBlock     `yaml:"jsonLD"       json:"jsonLD,omitempty"`
}

// OpenGraph holds og:* property values in fixed declaration order.
type OpenGraph struct {
    Type        string `yaml:"type"        json:"type,omitempty"`
    URL         string `yaml:"url"         json:"url,omitempty"`
    SiteName    string `yaml:"siteName"    json:"siteName,omitempty"`
    Locale      string `yaml:"locale"      json:"locale,omitempty"`
    Title       string `yaml:"title"       json:"title,omitempty"`
    Description string `yaml:"description" json:"description,omitempty"`
    Image       string `yaml:"image"       json:"image,omitempty"`
}

// TwitterCard holds twitter:* property values in fixed declaration order.
type TwitterCard struct {
    Card        string `yaml:"card"        json:"card,omitempty"`
    URL         string `yaml:"url"         json:"url,omitempty"`
    Title       string `yaml:"title"       json:"title,omitempty"`
    Description string `yaml:"description" json:"description,omitempty"`
    Image       string `yaml:"image"       json:"image,omitempty"`
}

// JSONLDBlock is a single JSON-LD script block.
// Payload is stored as raw JSON to preserve arbitrary schema.org structure.
type JSONLDBlock struct {
    ID      string                 `yaml:"id"      json:"id"`
    Payload map[string]interface{} `yaml:"payload" json:"payload"`
}
```

Then add `Head *PageHead \`yaml:"head" json:"head,omitempty"\`` to `PageSpec`.

Using pointer types for `OpenGraph` and `TwitterCard` ensures that absent blocks serialize as `null` (omitted with `omitempty`) rather than empty objects, keeping `head_json` compact and its zero value as `{}` when `Head` itself is nil.

### 11.2 Loader Parsing: Automatic but Requires Correct Tags

**Gap:** The story is silent on whether the loader needs changes. In practice, `loadPages` in `pkg/loader/loader.go` calls `yaml.Unmarshal` into `model.Page`, which means `spec.head` will parse automatically once `PageSpec.Head` has the correct `yaml:"head"` tag. No changes to `loader.go` itself are required; however, `pkg/loader/validate.go` (`ValidateSite`) currently validates route uniqueness and component references but does nothing with metadata. This story should specify whether `ValidateSite` validates head metadata (e.g., empty canonical URL is allowed, but a malformed URL is not).

**Recommended fix:** Add a `validatePageHead` helper inside `pkg/loader/validate.go` called from `ValidateSite` that:
1. If `Head` is nil, skips (optional field).
2. If `Head.CanonicalURL` is non-empty, validates it has a scheme of `https://` or `http://` (no `javascript:` or `data:` — see Section 12.3).
3. If `Head.OpenGraph` or `Head.Twitter` contain non-empty URL/image fields, apply the same URL scheme check.
4. Validates `meta` keys do not exceed a reasonable length limit (see Section 12.7).

Add `pkg/loader/validate.go` to the file touch list.

### 11.3 State Merge: Three Sites That Must All Change Together

**Gap:** The story identifies `internal/state/merge.go` as needing change but does not describe the specific code paths that must be updated. Reading the actual code reveals three distinct spots:

1. **Change detection** (line 220 in `merge.go`): The `if existing, ok := pageByName[res.Name]; !ok { ... } else if existing.ContentHash != ref.Hash || ...` comparison does not include `HeadJSON`. If head metadata changes but `content_hash` does not, the diff engine will report no change and the upsert will not update the DB. The comparison must add `|| existing.HeadJSON != string(headJSON)`.

2. **UpsertPage call** (lines 224-233): The `dbpkg.PageRow` struct literal passed to `UpsertPage` must include `HeadJSON: string(headJSON)`.

3. **UpsertPage SQL** (queries.go lines 91-106): The `INSERT` and `ON CONFLICT DO UPDATE SET` clauses must include `head_json`. The story lists this file but the gap is that the upsert's `SET` clause must include `head_json=excluded.head_json`; without it a second apply of changed metadata would silently not update the DB.

**Recommended fix:** The story's implementation step 3 must explicitly call out all three locations. Add this as an explicit sub-requirement: "The upsert query `ON CONFLICT DO UPDATE SET` clause must include `head_json=excluded.head_json`."

### 11.4 Migration Numbering Conflict

**Gap (confirmed):** The current migration sequence is:
- `001_initial_schema.go` (version 1)
- `002_domain_bindings.go` (version 2)

E7-S1 proposes `003_pages_head_metadata.go` (version 3).
E7-S2 also proposes `003_telemetry_events.go` (version 3 — see E7-S2 Section 5.4 and 6).

Both files are registered in `All()` in `001_initial_schema.go`. Two migrations with the same version number will either panic or silently apply only one, depending on the migration runner implementation. This is a hard conflict.

**Recommended fix:**
- E7-S1 keeps `003_pages_head_metadata.go` with version 3.
- E7-S2 must be updated to use `004_telemetry_events.go` with version 4.
- Both story files must note this dependency: E7-S2 is blocked on E7-S1 for migration numbering.
- A note has already been added to Section 6 of this story. The corresponding update to E7-S2 Section 5.4 and 6 must be made when that story is implemented.

### 11.5 Release Builder: Two Separate Reconstruction Gaps

**Gap:** The release builder in `internal/release/builder.go` reconstructs page documents from DB rows in two places, and the story does not distinguish them:

1. **`materializeSource` (lines 445-455):** Builds a `model.Page` struct and marshals it to YAML for the intermediate source directory. It currently sets `Spec` to `{Route, Title, Description, Layout}` only. If `HeadJSON` is not deserialized back into `Spec.Head` at this step, the loader (`LoadSite`) will parse a page YAML with no head field, and the renderer will produce pages with no metadata even though the DB has it stored. This is the critical roundtrip gap.

2. **`desiredState.Manifest()` (lines 319-367):** Builds the release manifest snapshot JSON. It currently omits head data from the page entries. While omitting it from the manifest is arguably acceptable (the manifest is informational), including a `headPresent: bool` or `headHash` field would make manifest diffs meaningful. This is lower priority but worth noting.

**Recommended fix for item 1:** In `materializeSource`, after unmarshaling `layout_json` into `layout`, also unmarshal `row.HeadJSON` into a `*model.PageHead` value and assign it to `pageDoc.Spec.Head`. Pattern:

```go
var head *model.PageHead
if strings.TrimSpace(row.HeadJSON) != "" && row.HeadJSON != "{}" {
    var h model.PageHead
    if err := json.Unmarshal([]byte(row.HeadJSON), &h); err != nil {
        return fmt.Errorf("parse head_json for page %q: %w", row.Name, err)
    }
    head = &h
}
pageDoc := model.Page{
    ...
    Spec: model.PageSpec{
        ...
        Head: head,
    },
}
```

### 11.6 Renderer: Template Data Extension and Injection Point

**Gap:** The story says "extend template data and render head tags" but the current `pageTemplateData` struct and `defaultPageTemplate` constant in `pkg/renderer/template.go` have no head field and no injection point in the template. The `<head>` block in the template is a hardcoded string constant with no placeholder between the `<meta name="description">` tag and the closing `</head>`. Adding metadata requires either:

(a) A new `template.HTML` field (`HeadMetaHTML`) containing the pre-rendered metadata tags, built by a helper function that uses `html/template` correctly, or
(b) Extending `pageTemplateData` with typed `*model.PageHead` and writing inline template logic for each tag category.

Option (a) is safer: the helper function renders each tag through a mini-template (ensuring attribute values are auto-escaped) and returns `template.HTML`. The outer template simply does `{{.HeadMetaHTML}}`. This keeps the main template simple.

**Recommended fix:** Define `HeadMetaHTML template.HTML` in `pageTemplateData`. Add a `renderHeadMeta(head *model.PageHead) (template.HTML, error)` function in a new file `pkg/renderer/head.go` that executes a separate `html/template` for each tag category (canonical, meta, OG, Twitter) and uses `json.Marshal` for JSON-LD. The result is concatenated and returned as `template.HTML`. The template injection point in `defaultPageTemplate` becomes `{{- if .HeadMetaHTML }}{{ .HeadMetaHTML }}{{- end }}` placed between `<meta name="description">` and `{{- range .StyleHrefs }}`.

The file touch list should be updated to add `pkg/renderer/head.go` as a new file.

### 11.7 Deterministic Ordering: OG and Twitter Map Iteration Risk

**Gap:** Section 5.2 says "Open Graph properties (fixed key order)" and "Twitter properties (fixed key order)" but the current proposal uses typed structs (`OpenGraph`, `TwitterCard`) with named fields. Since struct field iteration is done via reflection in templates, and `html/template` iterates struct fields in declaration order, the ordering IS deterministic as long as the fields are declared in the canonical order in the Go struct. However, the `meta map[string]string` field IS a Go map, and map iteration in Go is explicitly randomized. Section 5.2 correctly requires "sorted by name" for `meta` entries, but the implementation must use `sort.Strings(keys)` explicitly — a range over the map in a template would be nondeterministic.

**Recommended fix:** Confirm in the story that the renderer helper function for `meta` entries sorts keys with `sort.Strings` before rendering. Do not rely on `html/template`'s `range` over a map (which is also nondeterministic). Instead, convert the map to a `[]struct{ Name, Content string }` sorted by `Name` before passing it to any template.

Additionally, `JSONLDBlock.Payload` is `map[string]interface{}`. When this map is serialized with `json.Marshal`, Go's `encoding/json` sorts map keys alphabetically as of Go 1.12+. This is documented stdlib behavior, so JSON-LD key ordering IS deterministic via `json.Marshal` without additional work. However, the story should explicitly state this so the implementer knows they do not need a custom JSON serializer.

### 11.8 File Touch List: Missing Files

The following files are missing from the Section 6 file touch list:

- `pkg/renderer/head.go` (new file) — the head metadata rendering helper. The story currently says "extend `pkg/renderer/template.go`" but the renderer logic for 5 tag categories is substantial enough to warrant a dedicated file, consistent with the existing `assets.go` separation.
- `pkg/loader/validate.go` — needs a `validatePageHead` helper for URL scheme validation (see 11.2).
- `internal/db/migrations/001_initial_schema.go` — the `All()` function in this file must be updated to register migration 003. This is where new migrations are added; the story does not call it out.

The diff engine (`internal/diff/`) does not need changes — it operates on file hashes only and is unaffected by new page fields.

---

## 12. Security Review Notes

### 12.1 XSS via Metadata Values: Auto-escaping Coverage

**Assessment:** When metadata string values (`title`, `description`, `canonicalURL`, OG/Twitter fields) are placed in `html/template` attribute contexts (e.g., `content="{{.Value}}"`) or text contexts (e.g., `<title>{{.Title}}</title>`), `html/template` auto-escapes them correctly. This provides baseline XSS protection for standard fields.

**Risk remaining:** The `renderHeadMeta` helper function (see Section 11.6) must itself use `html/template` for rendering each attribute value — it must not use `fmt.Sprintf` or `strings.Builder` to construct raw HTML strings and then cast them to `template.HTML`. Any such cast bypasses auto-escaping. Every `<meta name="..." content="...">`, `<link rel="canonical" href="...">`, and `<meta property="..." content="...">` tag must be produced by executing an `html/template` template, not string concatenation.

**Recommended enforcement:** In `pkg/renderer/head.go`, define mini-templates at package init time (using `template.Must`) for each tag type. For example:

```go
var metaNameTmpl = template.Must(template.New("meta-name").Parse(
    `  <meta name="{{.Name}}" content="{{.Content}}">` + "\n",
))
var metaPropertyTmpl = template.Must(template.New("meta-property").Parse(
    `  <meta property="{{.Property}}" content="{{.Content}}">` + "\n",
))
var canonicalTmpl = template.Must(template.New("canonical").Parse(
    `  <link rel="canonical" href="{{.URL}}">` + "\n",
))
```

Add a test that feeds `"><script>alert(1)</script>` as a metadata field value and asserts the rendered output contains no unescaped `<script>` tag.

### 12.2 JSON-LD Script Injection: The `</script>` Terminator Attack

**Gap (concrete, high severity):** This is the most critical security gap in the story. The `payload` field of `JSONLDBlock` is `map[string]interface{}`, populated from operator-controlled YAML. If any string value within the payload contains `</script>`, it will terminate the `<script type="application/ld+json">` block early when naively embedded in HTML, allowing injection of arbitrary HTML after the closing tag.

Example attack payload in YAML:
```yaml
jsonLD:
  - id: malicious
    payload:
      name: "foo</script><script>alert(document.cookie)</script>"
```

`json.Marshal` on the map produces: `{"name":"foo</script><script>alert(document.cookie)</script>"}`. If this string is placed inside `<script type="application/ld+json">...</script>` via a naive `template.HTML` cast, the browser HTML parser terminates the script block at the first `</script>`.

**The story says** "serialize server-side, never copied as raw untrusted HTML" but does not specify HOW to safely embed JSON inside a `<script>` tag in `html/template`. This is a known and non-trivial problem.

**Recommended fix:** Use `json.Marshal` followed by `htmlEscapeJSON` that replaces `<`, `>`, and `&` with their Unicode escape equivalents (`\u003c`, `\u003e`, `\u0026`). Go's `encoding/json` already does this by default: `json.Marshal` escapes `<`, `>`, and `&` to `\u003c`, `\u003e`, `\u0026` respectively (this is the default behavior of `json.Marshal` for string values in Go's standard library, as documented and confirmed since Go 1.0). Therefore the `</script>` sequence in a string value becomes `\u003c/script\u003e` in the JSON output, which is safe to embed inside a `<script>` tag.

**The story must explicitly state:** "JSON-LD payloads must be serialized with `json.Marshal` (which escapes `<`, `>`, `&` by default) and the result cast to `template.HTML` only after this serialization. Do not use `json.NewEncoder` with `SetEscapeHTML(false)`. Add a renderer test that feeds a payload containing `</script>` and asserts the output does not contain an unescaped `</script>`."

### 12.3 URL Field Validation: javascript: and data: Scheme Injection

**Gap (high severity):** The `canonicalURL`, `OpenGraph.URL`, `OpenGraph.Image`, `TwitterCard.URL`, and `TwitterCard.Image` fields accept arbitrary strings. If an operator sets `canonicalURL: javascript:alert(1)`, the rendered HTML becomes:

```html
<link rel="canonical" href="javascript:alert(1)">
```

`html/template` will auto-escape `"` and `<` inside attribute values, but it does NOT reject `javascript:` scheme URLs placed in `href` or `src` attributes — it only does context-aware escaping, which for a string in an `href` attribute position means encoding special characters but not scheme-filtering.

Note: `html/template` does filter `javascript:` in certain URL contexts (it inserts `#ZgotmplZ` for known-unsafe schemes in `href`/`src` attribute positions via its URL normalizer). However, this behavior is context-dependent and may not apply to all attribute positions used for metadata (e.g., `<meta content="javascript:...">` is a text attribute, not a URL attribute, so no URL normalization occurs).

**Recommended fix:** Add explicit URL scheme validation in the `validatePageHead` function (see Section 11.2). For any field semantically intended as a URL (`canonicalURL`, `url`, `image` in OG/Twitter), validate that the value, after `strings.TrimSpace`, either:
- is empty (field is optional), or
- starts with `https://` or `http://`.

Reject with a loader validation error if a non-empty URL field contains a `javascript:`, `data:`, or other non-HTTP scheme. This check belongs in `pkg/loader/validate.go` so it fires at apply time, before DB persistence.

### 12.4 Meta Map Key Injection

**Gap:** The `meta map[string]string` field uses operator-controlled YAML keys as the `name` attribute value in `<meta name="..." content="...">`. A key like `"><script>` or `x" onmouseover="` would be injected directly into the attribute value position.

**Assessment:** This is mitigated by `html/template` auto-escaping IF the key is rendered in an attribute value context (i.e., `<meta name="{{.Name}}" ...>` via a template). The auto-escaper will HTML-encode `"`, `>`, and `<` in attribute value positions, so `"><script>` becomes `&#34;&gt;&lt;script&gt;`, which is inert.

**Risk remaining:** The mitigation holds ONLY if the mini-template approach (Section 12.1) is used. If the implementer uses `fmt.Sprintf` to build `<meta name="%s" content="%s">` and casts to `template.HTML`, the key is not escaped and injection succeeds.

**Recommended fix:** Enforce the mini-template approach (Section 12.1) as a non-negotiable implementation constraint. Additionally, add an explicit test in `pkg/renderer/template_test.go` that passes a `meta` key containing `"><script>alert(1)</script>` and asserts the rendered output does not contain an unescaped `<script>` tag.

### 12.5 Open Graph Property Name Injection

**Assessment:** In the proposed typed-struct approach (Section 11.1), OG property names like `og:title`, `og:description` are hardcoded in the template as string literals — they are not derived from user input. There is therefore no injection risk for OG property names as long as the renderer uses the typed `OpenGraph` struct rather than a `map[string]string` for OG properties. The story must explicitly rule out a map-based OG implementation.

**Recommended fix:** Confirm in the implementation notes that `OpenGraph` and `TwitterCard` must be typed structs with fixed fields. A future "custom OG property" feature that uses a map would require the same escaping discipline applied to `meta`.

### 12.6 JSON-LD @context and @type: Arbitrary Schema Injection

**Assessment:** The story allows arbitrary `payload` content including any `@context` and `@type` values. This is inherent to JSON-LD's purpose. The security concern is limited to the script injection vector addressed in Section 12.2 (mitigated by `json.Marshal`'s default HTML escaping). The semantic risk of a malformed or adversarial JSON-LD block is limited to search-engine indexing quality — it does not create a browser-side execution vector when the JSON is properly escaped.

**One residual risk:** If `Payload` contains a deeply nested map with very large numbers of keys or deeply nested structures, `json.Marshal` could produce very large output, inflating page size.

**Recommended fix:** Add a validation rule in `validatePageHead` that limits:
- maximum number of JSON-LD blocks per page: 5 (configurable constant)
- maximum depth and key count: use a simple byte-count limit on the JSON-serialized payload — reject if `len(json.Marshal(block.Payload)) > 16384` (16 KB per block).

### 12.7 Input Length Limits: Missing Field-Level Caps

**Gap:** No length limits are specified for any metadata field value. Large values in `title` (which becomes `<title>` text content), `description`, OG fields, or `meta` values would inflate every rendered page on each request served by Caddy. A `meta["keywords"]` value of 1 MB would be embedded in every page HTML response.

**Recommended fix:** Add the following length limits, enforced in `validatePageHead` at load/apply time:

| Field | Max length |
|---|---|
| `canonicalURL` | 2048 chars |
| Each `meta` key | 128 chars |
| Each `meta` value | 512 chars |
| Number of `meta` entries | 32 |
| OG/Twitter string fields | 512 chars each |
| `jsonLD[].id` | 128 chars |
| JSON-serialized `jsonLD[].payload` | 16384 bytes |
| Number of JSON-LD blocks | 5 |

These limits match common SEO tool constraints and prevent memory/response-size DoS from oversized metadata.

### 12.8 Persistence XSS: JSON in Terminal Output

**Assessment:** When `head_json` is read from the DB and displayed via CLI commands (e.g., a future `htmlctl get page` command), the JSON string value may contain HTML characters (`<`, `>`, `&`, `"`). In a terminal context these are inert — terminals do not render HTML. However, if `head_json` is ever embedded in a web-rendered response (e.g., a future admin UI or the existing API response JSON), the JSON string is already double-encoded (the metadata strings are JSON-encoded, then the whole JSON object is JSON-encoded again in the API response body), which is safe.

**No action required** for this specific concern in v1. The risk would materialize only if a future feature renders `head_json` values as raw HTML without re-escaping, which the existing `html/template` discipline would prevent. Note this as a follow-up invariant: "Any future web-rendered display of page metadata values must pass through `html/template` escaping, not raw string interpolation."
