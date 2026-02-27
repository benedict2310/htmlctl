# E8-S2 — Automatic OG Image Generation

**Epic:** Epic 8 — DX & Reliability  
**Status:** Implemented (2026-02-27)  
**Priority:** P2 (Medium — removes manual friction for every new page)  
**Estimated Effort:** 2–3 days  
**Dependencies:** E7-S1 (server-rendered SEO metadata), E2-S4 (release builder)  
**Target:** `internal/release/builder.go` + new `internal/ogimage/` package  
**Design Reference:** Architecture review decisions — 2026-02-26

---

## 1. Objective

Generate deterministic OG card PNGs at release build time and auto-wire image metadata where safe, without introducing rebuild-on-promote behavior, dynamic endpoints, or Caddy changes.

This story keeps the existing promotion invariant from E4-S3: promoted artifacts are byte-identical and are not rebuilt during promotion.

## 2. Architecture Decisions (Resolved)

1. **No-rebuild promotion invariant remains primary.**
2. **Auto-injection requires absolute canonical URL (`http`/`https`).**  
   Relative or empty canonical URLs are valid in schema but are not eligible for auto-injection.
3. **Injection is field-by-field, not coupled.**  
   `openGraph.image` and `twitter.image` are populated independently only when each specific field is empty.
4. **OG generation failures do not fail the release build.**  
   Builder logs warnings, continues, and skips auto-injection for pages whose OG image failed.
5. **Cache key must include an explicit template version prefix** (for example `og-v1:`).

## 3. Scope

### In Scope

- New `internal/ogimage/` package that renders deterministic 1200×630 PNG cards from:
  - `title`
  - `description`
  - `siteName`
- Embedded subset fonts via `//go:embed`.
- Blob-store-backed OG cache keyed by `sha256("og-v1:" + title + "\x00" + description + "\x00" + siteName)`.
- Builder integration that:
  - ensures OG blobs exist (cache hit or render+put),
  - injects metadata before render only for eligible pages,
  - links/copies OG blobs into `releases/<id>/og/<pagename>.png` after render.
- Warning-based failure isolation: per-page OG generation errors do not abort release.
- Unit tests for `internal/ogimage` and release builder integration.

### Out of Scope

- Dynamic OG HTTP endpoint.
- Custom per-site/per-page OG templates.
- Promotion-time artifact rewriting.
- Domain-binding-based OG URL derivation.

## 4. Architecture Context and Reuse Guidance

- `pkg/renderer.Render()` clears and recreates output dir at start.  
  Therefore, metadata decisions must happen before render, while final OG files are materialized after render.
- Reuse `internal/blob.Store` (`Path`, `Put`) directly; there is no blob `Get` API.
- Keep renderer unchanged; head-tag rendering already supports `og:image` and `twitter:image`.
- Respect loader URL semantics:
  - relative URLs are valid in `spec.head`,
  - OG auto-injection only runs when canonical URL is absolute `http(s)`.

## 5. Implementation Plan

### 5.1 New Package

**`internal/ogimage/ogimage.go`**

```go
type Card struct {
    Title       string
    Description string
    SiteName    string
}

func Generate(c Card) ([]byte, error)
func CacheKey(c Card) [32]byte // sha256("og-v1:" + ...)
```

Implementation requirements:

- Deterministic output for identical `Card`.
- Canvas size exactly `1200x630`.
- Pure Go rendering (`image`, `image/draw`, `image/png`, `golang.org/x/image/font/...`).
- `templateVersion` constant included in key prefix.

**`SiteName` source (builder responsibility, not `ogimage` package):**
Use `page.Spec.Head.OpenGraph.SiteName` when non-empty; fall back to `site.Website.Metadata.Name` (the technical website name stored in the model). `Website.Spec` has no display-name field.

### 5.2 Builder Integration (`internal/release/builder.go`)

Add a dedicated OG preparation/materialization phase with this order:

1. Load site from materialized source (existing flow).
2. For each page:
   - Build `Card` (see `SiteName` source rule in §5.1).
   - Compute `key := ogimage.CacheKey(card)` then convert to hex: `hashHex := hex.EncodeToString(key[:])` — this 64-char lowercase hex string is used for `blobs.Path` and `blobs.Put`.
   - Check `os.Stat(blobs.Path(hashHex))` for cache hit; skip `Generate` on hit.
   - On miss, call `b.generateFn(card)` to get `pngBytes`, then call `blobs.Put(ctx, hashHex, pngBytes)`.
   - If either `Generate` or `Put` returns an error, treat as generation failure for this page (log warning, do not record in successful set).
   - On success, record page name → `hashHex` in a local map.
3. For each page, if all conditions hold:
   - Page name is present in the successful map from step 2,
   - `spec.head.canonicalURL` is absolute `http(s)`,
   - `openGraph.image` is empty or `openGraph` is nil → set `openGraph.image`,
   - `twitter.image` is empty or `twitter` is nil → set `twitter.image`.
   - **Important:** `site.Pages` is `map[string]Page` where `Page` is a value type. Direct field assignment through a map index does not compile. Use copy-modify-reassign:
     ```go
     page := site.Pages[name]
     // modify page.Spec.Head...
     site.Pages[name] = page
     ```
4. Render HTML (`renderer.Render`). (`renderer.Render` calls `os.RemoveAll(outputDir)` at start, so OG PNG materialization must happen after this step.)
5. Materialize OG PNGs into the release tree:
   - `pagename` is the page's resource name (the map key in `site.Pages`, e.g. `"htmlctl"`).
   - Destination path: `filepath.Join(tmpReleaseDir, "og", pagename+".png")`.
   - Create the directory first: `os.MkdirAll(filepath.Join(tmpReleaseDir, "og"), 0o755)`.
   - Attempt `os.Link(blobPath, dst)` first; fall back to byte copy when hardlink fails (cross-device or unsupported FS).

Failure policy:

- If OG generation or `Put` fails for one page, log warning with page name and continue.
- Do not inject OG/Twitter image URLs for failed pages.
- OG materialization failures (link/copy in step 5) should also log warning and continue; they do not abort the release.

### 5.3 Testability Seam

Add a `generateFn func(ogimage.Card) ([]byte, error)` field to `Builder`, alongside the existing `nowFn`/`idFn` fields. In `NewBuilder`, default it to `ogimage.Generate`:

```go
// in Builder struct
generateFn func(ogimage.Card) ([]byte, error)

// in NewBuilder
generateFn: ogimage.Generate,
```

Tests inject a custom function to simulate cache hits (return same bytes deterministically) or generation failures (return `nil, errors.New("forced failure")`). No additional interface or package-private type is needed.

### 5.4 File Touch List

**Font prerequisite (must be done before writing `internal/ogimage` code):**
Prepare Latin-subset TTF files using `pyftsubset` (or equivalent) restricting to `U+0020-U+007E,U+00A0-U+00FF`. Committing full font files is prohibited by AC-10. Subset files must be small enough to pass the AC-10 size guard — target < 50 KB each.

**Create**

- `internal/ogimage/ogimage.go`
- `internal/ogimage/ogimage_test.go`
- `internal/ogimage/fonts/JetBrainsMono-Bold.ttf` (Latin subset only)
- `internal/ogimage/fonts/Inter-Regular.ttf` (Latin subset only)

**Modify**

- `internal/release/builder.go`
- `internal/release/builder_test.go`
- `go.mod`
- `go.sum`

## 6. Acceptance Criteria

- [x] AC-1: Release output contains `og/<pagename>.png` for every page whose OG pipeline completed successfully (generate/cache + materialize).
- [x] AC-2: Generated PNGs are exactly 1200×630 and valid PNG files.
- [x] AC-3: Same card input yields identical PNG bytes and same cache key.
- [x] AC-4: Cache hit path avoids re-rendering on subsequent builds with unchanged metadata.
- [x] AC-5: Auto-injection only occurs when canonical URL is absolute `http(s)` and OG generation succeeded.
- [x] AC-6: `openGraph.image` and `twitter.image` are injected independently and never overwrite explicit values.
- [x] AC-7: Pages with missing/relative canonical URL are left unchanged by auto-injection.
- [x] AC-8: Any per-page OG failure (`Generate`, blob `Put`, or OG materialization link/copy) does not fail the release; warning is logged and that page is skipped for auto-injection.
- [x] AC-9: `go test -race ./internal/ogimage/... ./internal/release/...` passes.
- [x] AC-10: Font embed size guard exists (test or CI check) to prevent accidentally committing full font files.

## 7. Tests and Validation

### Automated

- `TestGenerateDimensions`
- `TestGenerateDeterministic`
- `TestGenerateCacheKey`
- `TestGenerateRendersText`
- `TestBuildOGImageGenerated`
- `TestBuildOGImageCacheHit`
- `TestBuildOGImageNoInjectionWithoutAbsoluteCanonical`
- `TestBuildOGImagePreservesExplicitOpenGraphImage`
- `TestBuildOGImagePreservesExplicitTwitterImage`
- `TestBuildOGImageWarnsAndContinuesOnGenerationError`
- `TestBuildOGImageWarnsAndContinuesOnPutError`
- `TestBuildOGImageFallsBackToCopyWhenHardlinkFails`
- `TestBuildOGImageWarnsAndContinuesOnMaterializeFailure`
- Validation run on 2026-02-27:
  - `go test -race ./internal/ogimage/... ./internal/release/...`
  - `go test ./...`

### Manual

- Apply a site with absolute canonical and no explicit image fields:
  - verify `og/<pagename>.png` is served.
  - verify rendered `<head>` includes auto-generated OG/Twitter image URLs.
- Apply a site with relative canonical:
  - verify no auto-injected OG/Twitter image meta tags.
- Force one-page generation failure (temporary test hook):
  - verify release succeeds and warning appears in build log.

## 8. Risks and Mitigations

- **Risk:** stale OG cache when template/font/colors change.  
  **Mitigation:** required `templateVersion` in key prefix; bump version whenever visual output changes.

- **Risk:** silent quality regressions with warn-and-continue.  
  **Mitigation:** clear structured warnings in build log and release metadata; add alerting in ops docs later.

- **Risk:** broken absolute URL derivation from canonical values.  
  **Mitigation:** parse via `net/url`; require absolute `http(s)` for injection eligibility.

## 9. Follow-Up Recommendation (Separate Story)

Add **non-blocking promote warnings** in `promote` flow when likely staging hosts are detected in canonical/OG/Twitter URLs while promoting to prod.

Rationale:

- useful operator signal,
- no artifact rewriting,
- does not violate no-rebuild promotion invariant.

---

## Implementation Summary

**Date:** 2026-02-27  
**Branch:** current workspace  
**Implemented by:** Codex (GPT-5)

### Files Changed

- `internal/ogimage/ogimage.go` (new deterministic OG renderer + cache key)
- `internal/ogimage/ogimage_test.go` (new unit tests + font size guard)
- `internal/ogimage/fonts/Inter-Regular.ttf` (Latin subset)
- `internal/ogimage/fonts/JetBrainsMono-Bold.ttf` (Latin subset)
- `internal/release/builder.go` (OG cache/render/materialize pipeline + metadata injection + warning isolation)
- `internal/release/builder_test.go` (OG integration tests)
- `go.mod` / `go.sum` (added `golang.org/x/image` dependency and transitive sums)
- `docs/epics.md` (Epic 8.2 marked Implemented)

### Notes

- Implemented `templateVersion` cache prefix (`og-v1:`) in `ogimage.CacheKey`.
- Materialization uses hardlink-first with byte-copy fallback.
- Auto-injection is gated by absolute canonical `http(s)` URL and preserves explicit `openGraph.image` / `twitter.image` values.
- Per-page OG generation/materialization failures are warning-only and do not fail release build.

### Verification Evidence

- `go test -race ./internal/ogimage/... ./internal/release/...` (pass)
- `go test ./...` (pass)

### Completion Status

- [x] Implementation complete
- [x] Acceptance criteria marked complete
- [x] Automated tests passing
- [ ] Manual staging/prod verification performed

## Post-Review Fixes (2026-02-27)

Follow-up fixes from code review after initial implementation:

- Replaced package-level mutable materialization seams with per-`Builder` fields (`linkFileFn`, `copyFileFn`) to avoid global test state races.
- Replaced `runtime.Caller`-based font size guard with embedded-byte-size assertions for `-trimpath` portability.
- Reduced `descriptionMaxLines` from `4` to `3` to avoid worst-case text overlap/clipping near the footer region.
- Added explicit `templateVersion` maintenance comment describing when cache-key version must be bumped.
- Removed unreachable nil-head branch in OG metadata injection.
- Added build-log phase marker (`ensuring og image blobs`) before OG generation/materialization pipeline.
- Consolidated redundant no-injection tests into the warning/failure-path tests that already assert both warning and no injection.
- Simplified test seed data by removing unused page blob writes and using deterministic page content-hash metadata.
