# E8-S6 — Automatic `sitemap.xml` Generation

**Epic:** Epic 8 — DX & Reliability  
**Status:** In Progress (implemented locally, awaiting review/commit)  
**Priority:** P2 (Medium — essential crawl/discovery feature for production SEO)  
**Estimated Effort:** 1.5–2.5 days  
**Dependencies:** E8-S4 (website-scoped favicon support / first-class `Website` desired state), E8-S5 (declarative `robots.txt` generation)  
**Target:** `pkg/model`, `pkg/loader`, `internal/state`, `internal/db`, `internal/release`, docs  
**Design Reference:** PRD deterministic artifact promotion + existing canonical URL model from E7-S1

---

## 1. Objective

Generate a deterministic `/sitemap.xml` from declared site pages and website SEO metadata so operators get strong default crawl coverage without maintaining a hand-written XML artifact.

The system should automate URL discovery aggressively while preserving operator control over canonical URLs and indexability. It must not derive sitemap hosts from environment bindings or rebuild during promote.

## 2. User Story

As an operator publishing a site with `htmlctl`, I want `sitemap.xml` to be generated automatically from my page routes and canonical metadata, so search engines discover my crawlable pages without me manually maintaining a parallel XML file.

## 3. Scope

### In Scope

- Add website-level sitemap settings to `website.yaml`.
- Generate root `/sitemap.xml` from page desired state.
- Automatically include crawlable pages by default.
- Exclude pages marked `noindex` / `none` in `head.meta.robots`.
- Prefer explicit per-page canonical URLs when valid for sitemap generation.
- Derive page URLs from `publicBaseURL + route` when canonical URL is absent.
- Append a `Sitemap:` line to generated `robots.txt` when both features are enabled.
- Keep output deterministic and promote-stable.

### Out of Scope

- Sitemap index files.
- Image/video/news sitemap extensions.
- `lastmod`, `changefreq`, or `priority` in v1.
- Environment-specific sitemap host rewriting.
- Multiple generated sitemap files for large sites.

## 4. Architecture Context and Reuse Guidance

- Reuse page metadata already supported by E7-S1:
  - `spec.head.canonicalURL`
  - `head.meta["robots"]`
- Reuse website SEO metadata from E8-S5:
  - `spec.seo.publicBaseURL`
  - `spec.seo.robots`
- Generate `sitemap.xml` in `internal/release`, not `pkg/renderer`, because it is a release artifact derived from the site model rather than page HTML.
- Keep promotion invariant from E4-S3:
  - generated XML must depend only on declarative website/page state,
  - not on current environment bindings or request host,
  - promote copies the same bytes unchanged.

## 5. Proposed Design

### 5.1 Website Schema

Extend website SEO config:

```yaml
spec:
  seo:
    publicBaseURL: https://example.com
    robots:
      enabled: true
    sitemap:
      enabled: true
```

Suggested Go model:

- `WebsiteSEO`
  - `PublicBaseURL string`
  - `Robots *WebsiteRobots`
  - `Sitemap *WebsiteSitemap`
- `WebsiteSitemap`
  - `Enabled bool`

This keeps sitemap settings intentionally small in v1.

### 5.2 URL Selection Rules

For each page, compute an effective sitemap URL:

1. If `spec.head.canonicalURL` is an absolute `http(s)` URL whose scheme+host match `publicBaseURL`, use it.
2. If `canonicalURL` is empty, derive URL from `publicBaseURL` + normalized page route.
3. If `canonicalURL` is relative, ignore it for sitemap URL purposes and derive from `publicBaseURL` + route.
4. If `canonicalURL` is absolute but points to a different host than `publicBaseURL`, skip the page and log a non-fatal build warning.

Rationale:

- automation defaults to route discovery,
- operators retain control via canonical URLs,
- cross-host or staging-host leakage does not silently poison the sitemap.

### 5.3 Inclusion Rules

Include a page by default unless page robots metadata indicates it should not be indexed.

Exclude when `head.meta["robots"]` contains either token:

- `noindex`
- `none`

Token matching rules:

- case-insensitive
- comma-separated token parsing
- ignore surrounding whitespace

No additional page-level sitemap DSL is introduced in v1.

### 5.4 Output Shape

Generate a single XML sitemap:

- XML declaration included
- `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`
- one `<url><loc>...</loc></url>` per included page

Ordering:

- pages sorted by normalized route, then page name

Use `encoding/xml` for escaping and deterministic emission.

### 5.5 `robots.txt` Integration

If:

- `robots.enabled == true`, and
- `sitemap.enabled == true`

append exactly one line to generated `robots.txt`:

```text
Sitemap: https://example.com/sitemap.xml
```

This line uses `publicBaseURL`, not domain bindings.

## 6. File Touch List

### Files to Modify

- `pkg/model/types.go`
  - Add typed website sitemap settings.
- `pkg/model/types_test.go`
  - Add YAML/JSON roundtrip coverage.
- `pkg/loader/validate.go`
  - Validate sitemap config and `publicBaseURL` coupling.
- `pkg/loader/validate_test.go`
  - Add sitemap validation coverage.
- `internal/db/models.go`
  - Extend website SEO persistence as needed.
- `internal/db/queries.go`
  - Persist/retrieve sitemap settings.
- `internal/db/queries_test.go`
  - Add sitemap persistence coverage.
- `internal/state/merge.go`
  - Persist sitemap config and detect changes.
- `internal/state/merge_test.go`
  - Cover website sitemap updates.
- `internal/release/builder.go`
  - Materialize `/sitemap.xml` and integrate generated sitemap line into `robots.txt`.
- `internal/release/builder_test.go`
  - Verify presence, ordering, exclusion rules, and robots integration.
- `docs/technical-spec.md`
  - Document `sitemap.xml` generation behavior and `publicBaseURL` dependence.

### Files to Create

- `internal/release/sitemap.go`
  - Sitemap URL selection and XML generation helper.
- `internal/release/sitemap_test.go`
  - Unit tests for inclusion/exclusion, ordering, and XML escaping.

## 7. Implementation Steps

1. Add website sitemap settings to the typed `Website` model.
2. Validate that sitemap generation requires a valid `publicBaseURL`.
3. Implement helper functions:
   - parse page robots tokens,
   - compute effective sitemap URLs,
   - generate deterministic XML.
4. Materialize `/sitemap.xml` during release creation when enabled.
5. Extend `robots.txt` generation to append the generated sitemap line when both features are enabled.
6. Update docs and fixtures.

## 8. Tests and Validation

### Automated

- Unit test: sitemap config parses from `website.yaml`.
- Unit test: sitemap generation requires valid `publicBaseURL`.
- Unit test: pages are ordered deterministically by route/name.
- Unit test: pages with `robots: noindex` or `robots: none` are excluded.
- Unit test: empty/relative canonical URLs fall back to `publicBaseURL + route`.
- Unit test: absolute canonical URLs on a mismatched host are skipped with warning behavior.
- Unit test: generated XML is well-formed and escaped.
- Unit test: `robots.txt` includes `Sitemap:` line when both features are enabled.
- Unit test: promote preserves generated sitemap bytes exactly.
- `go test -race ./...`

### Manual

- Apply a site with `publicBaseURL` and sitemap enabled; verify `/sitemap.xml` exists and lists expected pages.
- Mark one page `robots: noindex`; verify it is omitted from the sitemap.
- Verify generated `robots.txt` includes the sitemap line when enabled.
- Promote staging -> prod and verify the promoted artifact contains identical `sitemap.xml`.

## 9. Acceptance Criteria

- [ ] AC-1: `Website` resource supports typed `spec.seo.sitemap` settings.
- [ ] AC-2: When sitemap generation is enabled, release output contains a deterministic root `/sitemap.xml`.
- [ ] AC-3: Pages are included automatically from declared routes unless excluded by `robots` noindex semantics.
- [ ] AC-4: Explicit canonical URLs are used when valid for the configured canonical public host; otherwise URLs derive from `publicBaseURL + route`.
- [ ] AC-5: Pages with cross-host absolute canonical URLs are skipped rather than emitted with incorrect hosts.
- [ ] AC-6: Generated `robots.txt` appends a single `Sitemap:` line when both robots and sitemap generation are enabled.
- [ ] AC-7: Website sitemap configuration is part of desired state and survives apply -> DB -> release roundtrip.
- [ ] AC-8: Promote copies `sitemap.xml` and `robots.txt` unchanged as part of the immutable release artifact.
- [ ] AC-9: No environment-binding-derived or request-time sitemap rewrite path is introduced.

## 10. Risks and Open Questions

- **Risk:** operators may expect sitemap hosts to follow staging/prod domains automatically.
  **Mitigation:** document that crawl artifacts use canonical website metadata, not environment bindings, because promotion must remain byte-identical.

- **Risk:** some teams may want richer sitemap metadata (`lastmod`, `priority`).
  **Mitigation:** defer until a future story with a clear non-time-dependent data source.

- **Open question:** none blocking for v1.

## 11. Architectural Review Notes

> Added after codebase review. These notes resolve ambiguities and provide implementation-ready guidance.

### 11.1 Current State Baseline

- `PageSpec.Route` (string) — normalized route already exists, stored in `pages.route` column.
- `PageHead.CanonicalURL` (string) — exists from E7-S1, stored in `pages.head_json` JSON blob.
- `PageHead.Meta` (map[string]string) — exists from E7-S1; robots directives live at `head.meta["robots"]`.
- Pages are queried during release build via `q.ListPagesByWebsite(ctx, website.ID)` — ordered by **name** (alphabetically). Sitemap needs a separate sort by URL after computing effective sitemap URLs; do not rely on query ordering.
- Existing precedent for build-time cross-host URL warning: `warnLocalhostMetadataURLs()` in `builder.go`. Use the same log pattern (`log.Addf(...)`) for cross-host canonical skip warnings.
- `publicBaseURL` and robots config come from `WebsiteRow.SeOJSON` (added by E8-S5, migration 006). No additional DB column is needed for this story.

### 11.2 DB Layer

This story requires **no new migrations** and **no new DB columns**. All website SEO data (including `spec.seo.sitemap.enabled`) is stored in the `seo_json` column introduced by E8-S5. Sitemap settings are embedded in the `WebsiteSEO` struct and round-trip through that column.

Update the file touch list accordingly: `internal/db/models.go` and `internal/db/queries.go` do not need changes beyond what E8-S5 already introduced.

### 11.3 Go Model Types

Add to `pkg/model/types.go` (extending `WebsiteSEO` from E8-S5):

```go
type WebsiteSitemap struct {
    Enabled bool `yaml:"enabled" json:"enabled"`
}

// Add Sitemap field to WebsiteSEO:
type WebsiteSEO struct {
    PublicBaseURL string          `yaml:"publicBaseURL" json:"publicBaseURL,omitempty"`
    Robots        *WebsiteRobots  `yaml:"robots,omitempty"  json:"robots,omitempty"`
    Sitemap       *WebsiteSitemap `yaml:"sitemap,omitempty" json:"sitemap,omitempty"`
}
```

### 11.4 URL Selection Rule Specification

The story's four cases map to these exact fields:

| Case | Condition | URL Used |
|------|-----------|----------|
| 1 | `head.CanonicalURL` is absolute, scheme+host match `publicBaseURL` | `head.CanonicalURL` verbatim |
| 2 | `head.CanonicalURL` is empty | `publicBaseURL` + `page.Spec.Route` |
| 3 | `head.CanonicalURL` is non-empty but relative | `publicBaseURL` + `page.Spec.Route` |
| 4 | `head.CanonicalURL` is absolute but different host | skip; log warning via `log.Addf(...)` |

**Host matching:** Parse both URLs with `net/url`. Compare `parsed.Host` values using `strings.EqualFold` — RFC 3986 hostnames are case-insensitive. Ports are included in `.Host` as returned by `url.Parse`, so `example.com:443` ≠ `example.com`.

**`publicBaseURL` normalization:** use the same `normalizePublicBaseURL` helper defined in E8-S5 (see E8-S5 §11.5). Normalization strips trailing slash from path. Route is then appended without double-slash: `strings.TrimRight(baseURL.Path, "/") + route`.

**Validation:** When `spec.seo.sitemap.enabled == true`, `publicBaseURL` must be non-empty. Enforce this in `pkg/loader/validate.go` alongside the existing `publicBaseURL` format validation from E8-S5.

### 11.5 Robots Token Parsing (Exact Spec)

```go
// shouldExcludeFromSitemap returns true if the page's robots meta indicates
// it should not be indexed. Matching is case-insensitive, comma-delimited,
// and requires exact token equality (no substring matching).
func shouldExcludeFromSitemap(robotsMeta string) bool {
    for _, raw := range strings.Split(robotsMeta, ",") {
        token := strings.ToLower(strings.TrimSpace(raw))
        if token == "noindex" || token == "none" {
            return true
        }
    }
    return false
}
```

`robotsMeta` comes from `page.Spec.Head.Meta["robots"]` (empty string if key absent → include the page).

### 11.6 Deterministic Ordering

Sort by **effective sitemap URL** string (lexicographic) after computing all URLs and filtering exclusions. Do not sort by `(route, name)` tuple — canonical URL rewrites make the route-based key unreliable.

```go
sort.Slice(entries, func(i, j int) bool {
    return entries[i].Loc < entries[j].Loc
})
```

This produces a stable, human-readable ordering (alphabetical by URL path).

### 11.7 XML Output

Use `encoding/xml` with the struct tag approach. `xml.Marshal` does **not** emit the XML declaration by default — prepend `xml.Header` (the constant from the stdlib):

```go
type sitemapURLSet struct {
    XMLName xml.Name      `xml:"urlset"`
    Xmlns   string        `xml:"xmlns,attr"`
    URLs    []sitemapURL  `xml:"url"`
}

type sitemapURL struct {
    Loc string `xml:"loc"`
}

data, err := xml.MarshalIndent(urlset, "", "  ")
// Prepend declaration:
out := append([]byte(xml.Header), data...)
```

`xml.Header` is `"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"`. URL characters that require XML escaping (`<`, `>`, `&`) are handled automatically by `encoding/xml`.

### 11.8 `robots.txt` Integration Contract

E8-S5's `GenerateRobotsText` already accepts a `sitemapURL string` parameter (see E8-S5 §11.11). In `builder.go`:

```go
// After materializing sitemap:
var sitemapURL string
if sitemapEnabled {
    sitemapURL = normalizedPublicBaseURL + "/sitemap.xml"
}
robotsTxt := robots.GenerateRobotsText(website.Spec.SEO.Robots, sitemapURL)
```

The sitemap line is therefore appended by the existing `GenerateRobotsText` helper, not by new sitemap-specific code. This keeps the integration clean and avoids circular dependencies between `robots.go` and `sitemap.go`.

### 11.9 Sitemap Generation Function (in `internal/release/sitemap.go`)

```go
// GenerateSitemap produces a deterministic sitemap.xml from declared pages.
// Returns (nil, nil) when sitemap is not enabled or publicBaseURL is empty.
// Non-fatal per-page warnings (cross-host canonical) are written to log.
func GenerateSitemap(
    seo *model.WebsiteSEO,
    pages []dbpkg.PageRow,
    headByName map[string]*model.PageHead,
    log BuildLog,
) ([]byte, error)
```

The `headByName` map is already constructed in the builder when iterating pages for OG image generation — reuse the same map. `BuildLog` is the existing builder log type.

### 11.10 Integration Point in `builder.go`

Insert sitemap materialization after `copyOriginalAssets` and before OG image materialization (mirroring robots.txt placement):

```go
// 1. Generate sitemap
sitemapBytes, err := sitemap.GenerateSitemap(site.Website.Spec.SEO, snapshot.Pages, pageHeadMap, log)
if sitemapBytes != nil {
    if err := writeFile(filepath.Join(tmpReleaseDir, "sitemap.xml"), sitemapBytes); err != nil { ... }
}

// 2. Generate robots.txt (passing sitemap URL when enabled)
sitemapURL := ""
if site.Website.Spec.SEO != nil && site.Website.Spec.SEO.Sitemap != nil && site.Website.Spec.SEO.Sitemap.Enabled {
    sitemapURL = site.Website.Spec.SEO.PublicBaseURL + "/sitemap.xml"
}
if site.Website.Spec.SEO != nil && site.Website.Spec.SEO.Robots != nil && site.Website.Spec.SEO.Robots.Enabled {
    robotsText := robots.GenerateRobotsText(site.Website.Spec.SEO.Robots, sitemapURL)
    if err := writeFile(filepath.Join(tmpReleaseDir, "robots.txt"), []byte(robotsText)); err != nil { ... }
}
```

### 11.11 File Touch List Correction

The story's file touch list omits:
- `internal/db/models.go` — **no change needed** (E8-S5 already covers it; remove from this story's list)
- `internal/db/queries.go` — **no change needed** (same)
- `internal/state/merge.go` — **no change needed** (sitemap config is part of the SEO JSON blob already persisted by E8-S5's merge path; no new merge logic required)
- `internal/state/merge_test.go` — **no change needed** (same)

The `pkg/model/types.go` and `pkg/model/types_test.go` changes (adding `WebsiteSitemap` to `WebsiteSEO`) should be coordinated with E8-S5 or done here as an additive extension.

Remove from **Files to Modify** in section 6: `internal/db/models.go`, `internal/db/queries.go`, `internal/db/queries_test.go`, `internal/state/merge.go`, `internal/state/merge_test.go`.

---
