# E8-S4 — Website-Scoped Favicon Support

**Epic:** Epic 8 — DX & Reliability  
**Status:** Implemented (2026-03-01)  
**Priority:** P2 (Medium — common browser/site identity requirement, currently awkward to manage)  
**Estimated Effort:** 2–3 days  
**Dependencies:** E2-S3 (bundle ingestion), E2-S4 (release builder), E3-S4 (diff engine), E7-S1 (server-rendered head metadata)  
**Target:** `pkg/model`, `pkg/loader`, `internal/bundle`, `internal/state`, `internal/db`, `internal/release`, `pkg/renderer`  
**Design Reference:** PRD declarative-resource model + publish-time render pipeline

---

## 1. Objective

Add first-class favicon support for websites managed by `htmlctl`/`htmlservd` without introducing a favicon-specific build process, runtime mutation, or promote-time rebuild.

This story must make website-scoped head configuration a real part of desired state so favicon configuration is handled with the same apply/diff/release semantics as existing pages, components, styles, and assets.

## 2. User Story

As an operator publishing a site with `htmlctl`, I want favicon files and their `<head>` tags to be managed declaratively at the website level, so I can update site identity cleanly without abusing `assets/`, hand-editing templates, or relying on undocumented file-placement conventions.

## 3. Scope

### In Scope

- Add website-scoped favicon configuration to `website.yaml`.
- Add a dedicated source namespace for favicon inputs: `branding/`.
- Persist website desired state remotely, including website head configuration.
- Persist favicon source files as website-scoped binary resources.
- Render website favicon tags into every page head deterministically.
- Materialize favicon files into conventional public output paths:
  - `/favicon.svg`
  - `/favicon.ico`
  - `/apple-touch-icon.png`
- Include website config and favicon source files in desired-state diff.
- Preserve artifact-promotion invariant: promote copies the exact built release bytes, with no rebuild and no favicon regeneration.

### Out of Scope

- Auto-generating favicon variants from a logo.
- Image transcoding or format conversion (`.svg` -> `.ico`, `.png` resizing, etc.).
- Automatic `manifest.webmanifest` generation.
- Page-specific favicon overrides.
- Generic website-scoped arbitrary file support.
- Runtime JS-based favicon injection.

## 4. Architecture Context and Reuse Guidance

- The PRD and technical spec already define `Website` as a first-class resource, but the current implementation only parses `website.yaml` locally and does not carry it through remote desired state. This story should correct that gap instead of adding a favicon-only shortcut.
- Reuse the existing page-head architecture from E7-S1:
  - typed model structs in `pkg/model`
  - validation in `pkg/loader/validate.go`
  - JSON persistence in SQLite
  - reconstruction during release materialization
  - deterministic `html/template` rendering in `pkg/renderer`
- Keep the product’s existing publish/release model:
  - favicon bytes are user-provided source artifacts,
  - bytes are stored and copied verbatim,
  - release creation materializes files and renders tags,
  - promote reuses the exact release artifact and does not rebuild.
- Do not introduce a broad `website_files` abstraction. Keep the new binary surface area specific to favicon support.

## 5. Proposed Design

### 5.1 Source Layout

Add a dedicated source directory:

```text
site/
  website.yaml
  branding/
    favicon.svg
    favicon.ico
    apple-touch-icon.png
```

This avoids further cluttering `assets/` while keeping the operator-facing layout simple and explicit.

### 5.2 Website Schema

Extend `WebsiteSpec` with a website-level head block:

```yaml
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
      ico: branding/favicon.ico
      appleTouch: branding/apple-touch-icon.png
```

Use a narrow typed schema for v1:

- `spec.head.icons.svg`
- `spec.head.icons.ico`
- `spec.head.icons.appleTouch`

These values are source paths, not public URLs.

### 5.3 Remote Desired-State Model

This story must make `website.yaml` part of remote desired state.

Required outcomes:

- `website.yaml` is bundled and hash-tracked.
- apply persists website metadata changes instead of defaulting forever to `"default"`.
- release builder reconstructs `website.yaml` from DB state.
- desired-state manifest includes `website.yaml`.

Recommended persistence:

- add `websites.head_json TEXT NOT NULL DEFAULT '{}'`
- add `websites.content_hash TEXT NOT NULL DEFAULT ''`

`content_hash` is required so remote diff can compare the real uploaded `website.yaml` hash, not a re-marshaled approximation.

### 5.4 Website Icon Binary State

Add a dedicated website-scoped table for favicon source files, not a generic website-file table.

Recommended table: `website_icons`

Suggested columns:

- `id`
- `website_id`
- `slot` (`svg`, `ico`, `appleTouch`)
- `source_path`
- `content_type`
- `size_bytes`
- `content_hash`
- `created_at`
- `updated_at`

Uniqueness:

- `UNIQUE(website_id, slot)`

This keeps the DB model aligned with the typed schema and prevents the feature from expanding into an unbounded generic file subsystem.

### 5.5 Bundle Transport

Add two transport-level resource kinds:

- `Website`
- `WebsiteIcon`

Transport notes:

- `Website` references `website.yaml` exactly once.
- `WebsiteIcon` references one `branding/...` file and includes the icon slot in resource metadata/name.
- `branding/...` files are regular tar members and participate in hash verification like other files.

### 5.6 Rendering and Output

Release creation should materialize website icons into conventional public filenames regardless of source filename:

- `svg` slot -> `favicon.svg`
- `ico` slot -> `favicon.ico`
- `appleTouch` slot -> `apple-touch-icon.png`

Then render deterministic head tags:

- `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`
- `<link rel="icon" href="/favicon.ico">`
- `<link rel="apple-touch-icon" href="/apple-touch-icon.png">`

Only emit tags for configured/present slots.

Render order in `<head>` should become:

1. built-in charset/viewport/title/description tags
2. website icon tags
3. page `spec.head` metadata
4. stylesheet links

## 6. Compatibility Rules

This story changes desired-state scope, so mixed-version safety must be explicit.

Server compatibility requirement:

- If an apply manifest does not contain a `Website` resource, preserve the existing website row values.
- If an apply manifest does not contain any `WebsiteIcon` resources, preserve existing website icon rows.

This avoids destructive behavior when a newer server receives a bundle from an older client that does not yet know about website-scoped desired state.

New-client full apply requirement:

- When a manifest does contain `Website`/`WebsiteIcon` resources, full apply must reconcile them authoritatively, including deletion of removed icon slots.

## 7. File Touch List

### Files to Modify

- `pkg/model/types.go`
  - Add `WebsiteHead` and `WebsiteIcons` typed structs under `WebsiteSpec`.
- `pkg/model/types_test.go`
  - Add YAML/JSON roundtrip coverage for website head icons.
- `pkg/loader/loader.go`
  - Discover `branding/` files and attach them to the `Site` aggregate.
- `pkg/loader/validate.go`
  - Validate icon path presence, slot/path consistency, allowed extensions, and size/path rules.
- `pkg/loader/loader_test.go`
  - Add fixture coverage for website head icon parsing and branding discovery.
- `internal/bundle/manifest.go`
  - Add `Website` and `WebsiteIcon` transport kinds and validation rules.
- `internal/bundle/build.go`
  - Include `website.yaml` and configured `branding/...` files in bundle creation.
- `internal/bundle/build_test.go`
  - Verify new manifest resources and tar contents.
- `internal/db/models.go`
  - Extend `WebsiteRow`; add `WebsiteIconRow`.
- `internal/db/queries.go`
  - Add website-head persistence, website icon CRUD/list helpers, and website row update helpers.
- `internal/db/queries_test.go`
  - Cover website row persistence and website icon persistence.
- `internal/state/merge.go`
  - Merge website desired state and website icon rows during apply.
- `internal/state/merge_test.go`
  - Cover full/partial apply behavior, update detection, and compatibility preservation.
- `internal/release/builder.go`
  - Load website head/icon state, reconstruct source tree, and materialize final favicon output files.
- `internal/release/builder_test.go`
  - Verify reconstructed source state and final release output files/tags.
- `pkg/renderer/template.go`
  - Add a website-head injection slot distinct from page head metadata.
- `pkg/renderer/renderer.go`
  - Pass website icon metadata into the template pipeline.
- `pkg/renderer/head.go`
  - Add deterministic website icon tag rendering helpers.
- `pkg/renderer/head_test.go`
  - Verify tag ordering and escaping.
- `pkg/renderer/template_test.go`
  - Verify icon tags appear before page metadata/styles as specified.
- `pkg/renderer/renderer_test.go`
  - Verify final HTML contains configured favicon tags.
- `internal/server/resources.go`
  - Include `website.yaml` and `branding/...` entries in desired-state manifest response.
- `internal/cli/diff_helpers.go`
  - No logic change expected beyond consuming the expanded server manifest, but confirm tests cover the new entries.
- `docs/technical-spec.md`
  - Document `Website.spec.head.icons`, `branding/`, and the no-generation constraint.

### Files to Create

- `internal/db/migrations/005_website_head_and_icons.go`
  - Add `websites.head_json`, `websites.content_hash`, and the `website_icons` table.
- `internal/db/migrations/005_website_head_and_icons_test.go`
  - Verify schema and defaults.

## 8. Implementation Steps

1. Add typed website-head/icon schema to `pkg/model`.
2. Add `branding/` discovery and validation in the loader.
3. Extend bundle transport to include `Website` and `WebsiteIcon` resources.
4. Add DB migration and query/model support for website head state and icon rows.
5. Update apply/state merge:
   - persist website config,
   - persist website icons,
   - preserve old values when new resource kinds are absent,
   - reconcile authoritatively when present in full apply.
6. Update release builder to:
   - reconstruct `website.yaml`,
   - reconstruct source branding files,
   - materialize root favicon output files,
   - keep promote artifact behavior unchanged.
7. Extend renderer with website-scoped icon tag rendering.
8. Expose website/icon entries through desired-state manifest for diff.
9. Update docs and fixtures.

## 9. Tests and Validation

### Automated

- Unit test: `website.yaml` parses `spec.head.icons` correctly.
- Unit test: invalid icon slot/path combinations fail validation.
- Unit test: `branding/` discovery is deterministic.
- Unit test: bundle manifest includes `Website` and `WebsiteIcon` resources.
- Unit test: apply persists `websites.head_json`, `websites.content_hash`, and icon rows.
- Unit test: full apply removes stale icon slots when website icon resources are present.
- Unit test: server preserves existing website/icon state when receiving an old-style manifest with no website resources.
- Unit test: release builder reconstructs `website.yaml` with icon config.
- Unit test: release output contains configured public favicon files with byte-identical content.
- Unit test: renderer emits deterministic favicon tags in the required order.
- Unit test: desired-state manifest includes `website.yaml` and `branding/...` file hashes.
- `go test -race ./...`

### Manual

- Apply a site with `branding/favicon.svg` and `branding/favicon.ico`.
- Confirm `htmlctl diff` reports changes to `website.yaml` and branding files.
- Create a release and verify:
  - `/favicon.svg` and `/favicon.ico` exist in the release output,
  - page HTML contains the corresponding `<link>` tags,
  - file bytes match the source uploads exactly.
- Promote staging to prod and verify no rebuild/generation occurs and output hashes remain artifact-derived.

## 10. Acceptance Criteria

- [ ] AC-1: `Website` resource supports optional `spec.head.icons` metadata and parses successfully from `website.yaml`.
- [ ] AC-2: A dedicated `branding/` source directory is supported for favicon input files.
- [ ] AC-3: `website.yaml` becomes part of remote desired state and is included in desired-state diff/manifests.
- [ ] AC-4: Website favicon files are persisted as website-scoped binary state, not as page metadata and not as a renderer-only convention.
- [ ] AC-5: Release output materializes configured favicon files at conventional root paths without altering bytes.
- [ ] AC-6: Rendered page HTML contains deterministic website-level favicon tags for configured slots.
- [ ] AC-7: The feature introduces no favicon-specific build/generation process: no transcoding, no resizing, no automatic variant generation.
- [ ] AC-8: Promotion continues to copy exact release artifacts without rebuild and without mutating favicon files.
- [ ] AC-9: Full apply authored by a new client reconciles removed favicon slots correctly.
- [ ] AC-10: A newer server receiving an older bundle that lacks website/favicon resources preserves existing website/favicon state rather than deleting it.
- [ ] AC-11: Technical spec documentation is updated to describe the website-head icon model and no-generation constraint.

## 11. Risks and Mitigations

- **Risk:** Scope expands into generic website-file management.  
  **Mitigation:** Limit binary support to typed favicon slots only.

- **Risk:** Mixed-version apply could delete website icon state.  
  **Mitigation:** Preserve website/icon state when the manifest lacks the new resource kinds.

- **Risk:** Website hash/diff behavior becomes nondeterministic if the server re-marshals YAML differently from the uploaded file.  
  **Mitigation:** persist the uploaded `website.yaml` content hash directly.

- **Risk:** Browser expectations differ by platform.  
  **Mitigation:** support the common conventional filenames only in v1 (`favicon.svg`, `favicon.ico`, `apple-touch-icon.png`) and keep the schema narrow.

## 12. Open Questions

- Should `appleTouch` be included in the first implementation or deferred to a follow-up story if the initial scope must stay tighter? Recommendation: include it now; it is small and fits the same model cleanly.

## 13. Architectural Review Notes

> Added after codebase review. These notes resolve ambiguities and provide implementation-ready guidance.

### 13.1 Current State Baseline

- `WebsiteSpec` currently only has `DefaultStyleBundle` and `BaseTemplate`. There are no `Head`, `Icons`, or `SEO` fields.
- `WebsiteRow` has no JSON metadata columns — only `id`, `name`, `default_style_bundle`, `base_template`, `created_at`, `updated_at`.
- The highest existing DB migration is **v4** (`telemetry_events`). This story's migration is **005**.
- E7-S1 is the correct pattern to follow: typed structs with dual `yaml`/`json` tags, `HeadJSON` column on the row, `HeadJSONOrDefault()` helper, and deterministic HTML rendering in `pkg/renderer/head.go`.

### 13.2 DB Schema Clarifications

**Slot naming in `website_icons.slot`:** Use lowercase snake_case to match Go/SQL conventions:
- `svg` → SVG favicon slot
- `ico` → ICO favicon slot
- `apple_touch` → Apple touch icon slot (not `appleTouch`)

The Go struct field remains `AppleTouch string` with `yaml:"appleTouch"` tag; the DB slot value is `"apple_touch"`.

**`head_json` scope:** Persist the full marshaled `WebsiteSpec.Head` (the `*WebsiteHead` pointer) as JSON, mirroring how `pages.head_json` stores `PageSpec.Head`. This accommodates future website-level head additions. E8-S5 will add a separate `seo_json` column for SEO/robots metadata; do **not** mix them into `head_json`.

**Migration 005 SQL template:**
```sql
ALTER TABLE websites ADD COLUMN head_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE websites ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS website_icons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE RESTRICT,
    slot TEXT NOT NULL CHECK(slot IN ('svg', 'ico', 'apple_touch')),
    source_path TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(website_id, slot)
);
CREATE INDEX IF NOT EXISTS idx_website_icons_website_id ON website_icons(website_id);
```

Note the `CHECK` constraint on `slot` — it prevents the table from becoming a generic file store.

### 13.3 Resource Kind & Name Conventions

**Bundle resource kinds** (add to allowed list in `internal/bundle/manifest.go`):
- `"website"` — references exactly `website.yaml`, must appear at most once per bundle
- `"websiteicon"` — references one branding file per slot

**WebsiteIcon resource name pattern:** `"website-icon-{slot}"` with slot as snake_case:
- `"website-icon-svg"`, `"website-icon-ico"`, `"website-icon-apple-touch"`

This keeps names stable, distinct from file paths, and matching DB slot naming.

**Bundle manifest validation additions in `manifest.go`:**
```go
case "website":
    if ref.File != "website.yaml" {
        return fmt.Errorf("website resource file must be website.yaml")
    }
case "websiteicon":
    validIconNames := map[string]bool{
        "website-icon-svg": true,
        "website-icon-ico": true,
        "website-icon-apple-touch": true,
    }
    if !validIconNames[r.Name] {
        return fmt.Errorf("invalid websiteicon name %q", r.Name)
    }
```

### 13.4 Model Additions to `pkg/model/types.go`

```go
type WebsiteIcons struct {
    SVG        string `yaml:"svg,omitempty"        json:"svg,omitempty"`
    ICO        string `yaml:"ico,omitempty"        json:"ico,omitempty"`
    AppleTouch string `yaml:"appleTouch,omitempty" json:"appleTouch,omitempty"`
}

type WebsiteHead struct {
    Icons *WebsiteIcons `yaml:"icons,omitempty" json:"icons,omitempty"`
}

// Extend WebsiteSpec:
type WebsiteSpec struct {
    DefaultStyleBundle string       `yaml:"defaultStyleBundle"`
    BaseTemplate       string       `yaml:"baseTemplate"`
    Head               *WebsiteHead `yaml:"head,omitempty" json:"head,omitempty"`
}
```

**Add `Branding` field to `Site`:**
```go
type BrandingAsset struct {
    Slot       string // db/bundle slot: "svg", "ico", "apple_touch"
    SourcePath string // relative to site root, e.g. "branding/favicon.svg"
}

type Site struct {
    // existing fields ...
    Branding map[string]BrandingAsset // keyed by slot, e.g. "svg" -> {...}
}
```

### 13.5 Loader Validation Additions

Add to `pkg/loader/validate.go`:
- `spec.head.icons.svg` must end with `.svg` (case-insensitive)
- `spec.head.icons.ico` must end with `.ico`
- `spec.head.icons.appleTouch` must end with `.png`
- Referenced file must exist under `branding/` (checked at load time, not just at validation time)
- Reject paths with `..` components

### 13.6 DB Query Additions

Required new helpers in `internal/db/queries.go`:
- `UpsertWebsiteHead(ctx, websiteID, headJSON, contentHash string) error`
- `UpsertWebsiteIcon(ctx, WebsiteIconRow) error` — uses `ON CONFLICT(website_id, slot) DO UPDATE`
- `ListWebsiteIconsByWebsite(ctx, websiteID) ([]WebsiteIconRow, error)` — ordered by `slot`
- `DeleteWebsiteIconsNotIn(ctx, websiteID, slots []string) (int64, error)` — used for full-apply cleanup
- Update `GetWebsiteByName` to `SELECT` new columns `head_json`, `content_hash`

Add `WebsiteIconRow` to `internal/db/models.go`:
```go
type WebsiteIconRow struct {
    ID          int64
    WebsiteID   int64
    Slot        string
    SourcePath  string
    ContentType string
    SizeBytes   int64
    ContentHash string
    CreatedAt   string
    UpdatedAt   string
}
```

### 13.7 Renderer Changes

**`pageTemplateData` struct** (in `pkg/renderer/template.go`) needs a new field:
```go
type pageTemplateData struct {
    Title            string
    Description      string
    WebsiteIconsHTML template.HTML // NEW — injected between description and HeadMetaHTML
    HeadMetaHTML     template.HTML
    StyleHrefs       []string
    ContentHTML      template.HTML
    ScriptSrc        string
}
```

**Template injection order** (between `<meta name="description">` and HeadMetaHTML):
```html
{{- if .WebsiteIconsHTML }}
{{.WebsiteIconsHTML}}{{- end }}
```

**New rendering function** (add to `pkg/renderer/head.go` or new `pkg/renderer/website_icons.go`):
```go
func renderWebsiteIcons(icons *model.WebsiteIcons) (template.HTML, error)
```

This generates `<link rel="icon" ...>` tags; only emits tags for configured slots; output is deterministic.

Add `pkg/renderer/website_icons.go` to the **Files to Create** list.

### 13.8 State Merge Full-Apply Cleanup

In `internal/state/merge.go`, full-apply cleanup for icons requires checking whether any `WebsiteIcon` resources were in the manifest before running `DeleteWebsiteIconsNotIn`. If no `WebsiteIcon` resources are present (old-style bundle), do **not** call the delete helper — this preserves existing icon state.

```go
// Pseudo-code pattern:
hasIconResources := /* any res.Kind == "websiteicon" in manifest */
if b.Manifest.Mode == bundle.ApplyModeFull && hasIconResources {
    deleted, err := q.DeleteWebsiteIconsNotIn(ctx, website.ID, keepSlots)
    // track deletions
}
```

Apply the same pattern for the `Website` resource: only update `head_json`/`content_hash` if a `Website` resource is present in the manifest.

### 13.9 Release Builder Materialization Sequence

The current materialization sequence in `builder.go` is:
1. `materializeSource` → reconstructs YAML source tree
2. `loader.LoadSite` → parses into model
3. `ensureOGBlobs` → per-page OG image blobs
4. `renderer.Render` → renders HTML
5. `copyOriginalStyles` → copies CSS
6. `copyOriginalAssets` → copies assets from blob store

**Insert after step 6:**
```
7. materializeFaviconFiles → writes /favicon.svg, /favicon.ico, /apple-touch-icon.png from blobs
```

`materializeSource` must also write back branding source files from blobs so `loader.LoadSite` can parse the fully reconstructed site (including `website.yaml` with head/icons and branding files).

### 13.10 Coordination with E8-S5

E8-S5 (robots.txt) will add a separate `seo_json TEXT NOT NULL DEFAULT '{}'` column to the `websites` table in a separate **migration 006**. Do not pre-create this column in migration 005. The two metadata stores (`head_json` for favicon/head, `seo_json` for SEO/robots) remain isolated by design.

---
