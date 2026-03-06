# E8-S7 — `llms.txt` Generation and Auto-Generated JSON-LD Structured Data

**Epic:** Epic 8 — DX & Reliability
**Status:** Implemented (2026-03-06)
**Priority:** P2 (Medium — natural completion of the discoverability stack from E8-S5/S6)
**Estimated Effort:** 2–3 days
**Dependencies:** E8-S4 (first-class `Website` desired state), E8-S5 (robots.txt / `publicBaseURL`), E8-S6 (sitemap.xml / page inclusion rules)
**Target:** `pkg/model`, `pkg/loader`, `internal/release`, docs
**Design Reference:** [llmstxt.org](https://llmstxt.org/) spec; schema.org Organization/WebSite/Article types

---

## 1. Objective

Complete the discoverability stack by generating two remaining artifacts:

1. **`/llms.txt`** — a plain-text markdown file at the site root that gives LLM crawlers, RAG pipelines, and AI agents a concise, structured overview of the site's pages and purpose, following the emerging [llms.txt convention](https://llmstxt.org/).
2. **Website-level auto-generated JSON-LD** — structured data blocks automatically emitted into every page `<head>` describing the site as an `Organization` and `WebSite` according to schema.org, so search engines and AI systems understand who publishes the content.

Per-page manual `jsonLD` entries (already supported via `spec.head.jsonLD` in the page schema from E7-S1) are not changed by this story. This story adds automation at the website level and a new generated file artifact.

---

## 2. User Stories

**As an operator**, I want an `/llms.txt` file generated automatically from my site's page manifest and website metadata, so AI systems and LLM-powered search can understand my site's content without crawling every page.

**As an operator**, I want website-level structured data (`Organization`, `WebSite` schema) injected into every page head automatically, so my site has correct entity markup without me writing boilerplate JSON-LD in every page YAML.

---

## 3. Scope

### In Scope

- Generate `/llms.txt` from website metadata and page desired state.
- Generate website-level `Organization` + `WebSite` JSON-LD blocks and inject them into every page `<head>` during release rendering.
- Respect existing inclusion rules from E8-S6: pages with `robots: noindex` / `robots: none` are excluded from `llms.txt`.
- Reuse `publicBaseURL` from `spec.seo` for all URL generation.
- Keep both artifacts deterministic and promote-stable (generated at build time, not rebuilt on promote).

### Out of Scope

- Auto-generated per-page `Article` / `TechArticle` / `BlogPosting` JSON-LD (operator-controlled via existing `spec.head.jsonLD`).
- `llms-full.txt` variant (extended context file; defer to a future story).
- Custom `llms.txt` templates or operator-supplied sections beyond what is derivable from desired state.
- `BreadcrumbList` or `SiteNavigationElement` schema types.

---

## 4. Architecture Context and Reuse Guidance

- Both features are **release artifacts**, not page HTML renderer features. Generate them in `internal/release`, following the same pattern as `robots.go` and `sitemap.go`.
- `llms.txt` depends on `publicBaseURL` (E8-S5) and the same page inclusion rules as `sitemap.xml` (E8-S6). Reuse `shouldExcludeFromSitemap` (or promote it to a shared helper if it is currently unexported).
- Website-level JSON-LD is injected into page HTML. The renderer already handles per-page `jsonLD` entries; the website-level blocks should be prepended from a new `website.Spec.Head.SchemaOrg` configuration field, passed through the same template data path as `WebsiteIconsHTML`.
- Both features require `publicBaseURL` to be set. Add validation guards accordingly.
- Promotion invariant from E4-S3 still applies: no rebuild, no environment-specific rewrite.

---

## 5. Proposed Design

### 5.1 `llms.txt` Format

Follow the [llmstxt.org](https://llmstxt.org/) convention:

```
# <website name>

> <website description>

## Pages

- [<page title>](<URL>): <page description>
- ...
```

Rules:
- Header uses `website.Metadata.Name` (or a new `spec.seo.displayName` field — see §5.3).
- Description uses a new `spec.seo.description` field; falls back to empty line if absent.
- Pages section lists all crawlable pages (same inclusion rules as sitemap.xml, ordered by URL).
- Per-page description uses `page.Spec.Description` when non-empty; omits the colon clause when empty.
- LF line endings, UTF-8, no CRLF.
- Generated when `spec.seo.llmsTxt.enabled: true`.

### 5.2 Website-Level JSON-LD

Auto-inject two schema.org blocks into every page `<head>` when `spec.seo.structuredData.enabled: true`:

**`Organization`:**
```json
{
  "@context": "https://schema.org",
  "@type": "Organization",
  "name": "<displayName or metadata.name>",
  "url": "<publicBaseURL>"
}
```

**`WebSite`:**
```json
{
  "@context": "https://schema.org",
  "@type": "WebSite",
  "name": "<displayName or metadata.name>",
  "url": "<publicBaseURL>"
}
```

Injection rules:
- Emitted as `<script type="application/ld+json">` tags in page `<head>`, before per-page `jsonLD` entries.
- Only when `publicBaseURL` is set and `spec.seo.structuredData.enabled: true`.
- Operator can suppress per-page auto-injection by adding a manual `jsonLD` entry with type `Organization` or `WebSite` (do not duplicate; check existing entries).

### 5.3 Website Schema Changes

Extend `WebsiteSEO` in `pkg/model/types.go`:

```go
type WebsiteSEO struct {
    PublicBaseURL  string               `yaml:"publicBaseURL"            json:"publicBaseURL,omitempty"`
    DisplayName    string               `yaml:"displayName,omitempty"    json:"displayName,omitempty"`
    Description    string               `yaml:"description,omitempty"    json:"description,omitempty"`
    Robots         *WebsiteRobots       `yaml:"robots,omitempty"         json:"robots,omitempty"`
    Sitemap        *WebsiteSitemap      `yaml:"sitemap,omitempty"        json:"sitemap,omitempty"`
    LLMsTxt        *WebsiteLLMsTxt      `yaml:"llmsTxt,omitempty"        json:"llmsTxt,omitempty"`
    StructuredData *WebsiteStructuredData `yaml:"structuredData,omitempty" json:"structuredData,omitempty"`
}

type WebsiteLLMsTxt struct {
    Enabled bool `yaml:"enabled" json:"enabled"`
}

type WebsiteStructuredData struct {
    Enabled bool `yaml:"enabled" json:"enabled"`
}
```

`DisplayName` and `Description` are website-scoped metadata used by both features. They live in `seo` because they are discovery-facing; `metadata.name` remains the technical resource name.

Example `website.yaml`:
```yaml
spec:
  seo:
    publicBaseURL: https://example.com
    displayName: "Example — Modern Web Publishing"
    description: "Build notes, architecture breakdowns, and product thinking."
    robots:
      enabled: true
    sitemap:
      enabled: true
    llmsTxt:
      enabled: true
    structuredData:
      enabled: true
```

### 5.4 Release Output

| Artifact | Path | Condition |
|----------|------|-----------|
| `llms.txt` | `/llms.txt` | `spec.seo.llmsTxt.enabled == true` + `publicBaseURL` set |
| Organization JSON-LD | injected into every page `<head>` | `spec.seo.structuredData.enabled == true` + `publicBaseURL` set |
| WebSite JSON-LD | injected into every page `<head>` | same |

---

## 6. File Touch List

### Files to Modify

- `pkg/model/types.go` — add `WebsiteLLMsTxt`, `WebsiteStructuredData`; add `DisplayName`, `Description` to `WebsiteSEO`.
- `pkg/model/types_test.go` — YAML/JSON roundtrip coverage.
- `pkg/loader/validate.go` — validate new fields; require `publicBaseURL` when `llmsTxt.enabled` or `structuredData.enabled`.
- `pkg/loader/validate_test.go` — add coverage.
- `internal/release/builder.go` — call `injectWebsiteStructuredData` and materialize `llms.txt` during release build.
- `internal/release/builder_test.go` — verify presence, content, and page injection.
- `internal/server/promote_test.go` — verify promote parity includes generated `llms.txt`.
- `docs/technical-spec.md` — document `llms.txt`, `structuredData`, `displayName`, `description`.

### Files to Create

- `internal/release/llmstxt.go` — `GenerateLLMsTxt` helper.
- `internal/release/llmstxt_test.go` — unit tests.
- `internal/release/structured_data.go` — website `Organization` / `WebSite` block generation and injection helper.
- `internal/release/structured_data_test.go` — unit tests for generation, ordering, and duplicate suppression.

---

## 7. Implementation Steps

1. Add `WebsiteLLMsTxt`, `WebsiteStructuredData`, `DisplayName`, `Description` to `WebsiteSEO`.
2. Add loader validation (require `publicBaseURL` when enabled).
3. Implement `GenerateLLMsTxt` in `internal/release/llmstxt.go`; reuse sitemap inclusion rules.
4. Implement website-level JSON-LD block generation in `pkg/renderer/head.go` or a new `internal/release/structureddata.go` helper.
5. Wire both into release materialization (after `sitemap.xml`, before OG images).
6. Extend renderer template to accept and emit `WebsiteSchemaHTML`.
7. Update docs.

---

## 8. Acceptance Criteria

- [x] AC-1: `Website` resource supports `spec.seo.llmsTxt.enabled`, `spec.seo.structuredData.enabled`, `spec.seo.displayName`, and `spec.seo.description`.
- [x] AC-2: When `llmsTxt.enabled` and `publicBaseURL` are set, release output contains a deterministic `/llms.txt` following the llmstxt.org format.
- [x] AC-3: `llms.txt` includes only crawlable pages (excludes `robots: noindex` / `robots: none`), ordered by URL.
- [x] AC-4: Per-page description is included in `llms.txt` when `spec.description` is non-empty.
- [x] AC-5: When `structuredData.enabled` and `publicBaseURL` are set, every rendered page `<head>` contains `Organization` and `WebSite` JSON-LD blocks derived from website metadata.
- [x] AC-6: Website-level JSON-LD blocks appear before per-page `jsonLD` entries in the rendered `<head>`.
- [x] AC-7: Both artifacts are generated at build time and promoted byte-identical (no rebuild on promote).
- [x] AC-8: Omitting `publicBaseURL` with either feature enabled produces a validation error at apply time.
- [x] AC-9: `go test -race ./...` passes.
- [x] AC-10: Technical spec is updated.

---

## 9. Tests and Validation

### Automated

- Unit test: `llms.txt` format matches spec for site with multiple pages.
- Unit test: `noindex` pages are excluded from `llms.txt`.
- Unit test: pages without `spec.description` are included without the description clause.
- Unit test: `llms.txt` is omitted when disabled or `publicBaseURL` is missing.
- Unit test: `Organization` and `WebSite` JSON-LD appear in page `<head>` output.
- Unit test: website-level JSON-LD precedes per-page `jsonLD` entries.
- Unit test: JSON output is valid and fields derive correctly from `displayName` / `metadata.name` fallback.
- Unit test: promote preserves `llms.txt` and page HTML unchanged.
- `go test -race ./...`

### Manual

- Apply a site with both features enabled; verify `/llms.txt` lists expected pages with correct URLs.
- Mark a page `robots: noindex`; verify it is absent from `llms.txt`.
- View page source; verify website-level JSON-LD blocks appear before page-specific `jsonLD`.
- Promote staging → prod; verify artifacts are byte-identical.

---

## 10. Risks and Mitigations

- **Risk:** `llms.txt` spec is still evolving; format may change.
  **Mitigation:** generate only the stable core sections (header, description, pages list). Avoid vendor-specific extensions in v1.

- **Risk:** website-level JSON-LD duplicates a manually authored Organization block.
  **Mitigation:** check for existing `Organization`/`WebSite` entries in `spec.head.jsonLD` before auto-injecting; skip auto-injection for types already present.

- **Risk:** `DisplayName` and `Description` fields are easy to overlook.
  **Mitigation:** fall back gracefully (`metadata.name` for display name; omit description clause when empty). Log a build-time note when `llmsTxt` is enabled but `description` is absent.

## 11. Implementation Notes (2026-03-06)

- Added website SEO schema support:
  - `spec.seo.displayName`
  - `spec.seo.description`
  - `spec.seo.llmsTxt.enabled`
  - `spec.seo.structuredData.enabled`
- Added validation guards in loader normalization:
  - max-length checks for display fields
  - `publicBaseURL` required when `sitemap`, `llmsTxt`, or `structuredData` generation is enabled
- Added release helpers:
  - `internal/release/llmstxt.go` for deterministic `/llms.txt` generation reusing sitemap inclusion/canonical logic
  - `internal/release/structured_data.go` for website-level JSON-LD generation and per-page prepend injection before page JSON-LD
- Builder integration:
  - website structured data injection runs before page rendering
  - `llms.txt` is materialized as part of release artifacts
- Promote parity coverage:
  - `internal/server/promote_test.go` asserts generated `llms.txt` is present in promoted artifact hashing path

Independent review gate:
- `codex review --uncommitted` completed with no actionable findings.

Verification evidence:
- `go test ./...` passed
- `go test -race ./...` passed
- Local Docker E2E with `htmlservd-ssh:e8s7` + `htmlctl:e8s7` passed (`E2E_OK`)
  - verified `/llms.txt` content and `noindex` exclusion
  - verified `Organization` and `WebSite` JSON-LD exist and appear before page-level JSON-LD in rendered HTML
- Re-validated in combined extension matrix (2026-03-06):
  - local Docker deploy with `spec.seo.llmsTxt.enabled` + `spec.seo.structuredData.enabled`
  - `/llms.txt` contained only crawlable pages with deterministic URL ordering
  - rendered page source kept website-level JSON-LD before page-level JSON-LD
