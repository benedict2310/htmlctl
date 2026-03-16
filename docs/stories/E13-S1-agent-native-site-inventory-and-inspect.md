# E13-S1 — Agent-Native Site Inventory and Inspect Commands

**Epic:** Epic 13 — Agent-Native Site Discoverability  
**Status:** Proposed  
**Priority:** P1  
**Estimated Effort:** 4-5 days  
**Dependencies:** E3-S3, E3-S4, E10-S5, E11-S4  
**Target:** `internal/server/`, `internal/client/`, `internal/cli/`, `docs/`, `.agent/skills/htmlctl-publish/`  
**Design Reference:** zero-context operator review on 2026-03-15

---

## 1. Summary

Add first-class remote inventory and inspect workflows so a zero-context operator can answer, from `htmlctl` alone: what routes exist, which components compose each page, which style bundle and branding metadata are active, and which assets or component sidecars are published for the selected environment.

## 2. Architecture Context and Reuse Guidance

- Reuse the existing resource aggregation work in `internal/server/resources.go`. Do not scrape rendered release artifacts from `current/`; inspect must operate on desired state, not rendered HTML output.
- Reuse the existing DB query layer:
  - `ListPagesByWebsite`
  - `ListComponentsByWebsite`
  - `ListStyleBundlesByWebsite`
  - `ListAssetsByWebsite`
  - `ListWebsiteIconsByWebsite`
- Reuse the current `manifest` endpoint semantics where possible. The current manifest already exposes file-level presence and hashes; the new inventory endpoint should extend that with resource metadata rather than invent a separate representation per command.
- Keep `get` as the inventory family and add a dedicated `inspect` command family for deep dives. Do not overload `status` into a catch-all inspector.
- Preserve machine-readable output. Any new inspect command must support `table|json|yaml` output and remain deterministic.
- Keep read-only behavior. This story must not mutate desired state or introduce hidden server-side rebuilds.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Remote desired-state inventory endpoint

Add a new authenticated read-only endpoint under the existing website/environment API surface:

- `GET /api/v1/websites/{website}/environments/{env}/resources`

Response should include:

- website summary:
  - `name`
  - `defaultStyleBundle`
  - `baseTemplate`
  - parsed website `head` / `seo` metadata summary
- pages:
  - `name`
  - `route`
  - `title`
  - `description`
  - ordered `layout` include names
  - page-head summary
  - content hash
- components:
  - `name`
  - `scope`
  - `hasCSS`
  - `hasJS`
  - html/css/js hashes
- style bundles:
  - `name`
  - ordered file list from `files_json`
- assets:
  - `filename`
  - `contentType`
  - `sizeBytes`
  - `contentHash`
- branding:
  - favicon/icon slot
  - source path
  - content type
  - size
  - content hash

Do not return full blob payloads in this story. This is inventory/inspect metadata only; full source export is handled in E13-S3.

### 3.2 `htmlctl get` support for content resources

Extend `get` to support:

- `pages`
- `components`
- `styles`
- `assets`
- `branding`
- `website`

Table output should optimize for fast operator scanning:

- `get pages`: `NAME`, `ROUTE`, `LAYOUT_ITEMS`, `UPDATED_AT`
- `get components`: `NAME`, `SCOPE`, `CSS`, `JS`, `UPDATED_AT`
- `get styles`: `NAME`, `FILES`, `UPDATED_AT`
- `get assets`: `PATH`, `TYPE`, `SIZE_BYTES`, `HASH`
- `get branding`: `SLOT`, `SOURCE_PATH`, `TYPE`, `SIZE_BYTES`
- `get website`: compact table of site-level metadata fields

`get --help` must list all supported resource types and explicitly tell users when to switch to `inspect`.

### 3.3 New `inspect` command family

Add a dedicated deep-inspection family:

- `htmlctl inspect website --context staging`
- `htmlctl inspect page <name> --context staging`
- `htmlctl inspect component <name> --context staging`

Expected behavior:

- `inspect website`:
  - show default style bundle, base template, icon config, SEO/publicBaseURL settings, generated-artifact flags
  - show aggregate counts and key warnings (for example `structuredData` enabled without visible `publicBaseURL`)
- `inspect page`:
  - show route, title, description, ordered component layout, canonical URL, OG/Twitter summary
  - list referenced components and flag any missing component names if server data is inconsistent
- `inspect component`:
  - show scope, sidecar presence, content hashes, and which pages include the component

Use the same active-context fallback model introduced in Epic 11.

### 3.4 Small status polish

Keep `status` lightweight, but add a small amount of discoverability:

- include `default_style_bundle`
- include `base_template`

Do not add page/component listings to `status`; those belong in `get`/`inspect`.

## 4. File Touch List

### Files to Modify

- `internal/server/resources.go` — add the new `resources` endpoint and response types; reuse existing query helpers
- `internal/server/routes.go` — route `/resources`
- `internal/server/resources_test.go` — add response coverage for inventory and inspect-related resource data
- `internal/server/path_parse_test.go` — cover any new path parsers
- `internal/client/client.go` — add `GetResources` helper
- `internal/client/types.go` — add typed responses for website/page/component/style/asset/branding inventory
- `internal/client/client_test.go` — cover the new client request/response handling
- `internal/cli/get_cmd.go` — extend supported resource types and table output
- `internal/cli/remote_helpers.go` — expand resource normalization and shared ref handling
- `internal/cli/status_cmd.go` — add style/template summary rows only
- `internal/cli/root.go` — register new `inspect` command group
- `internal/cli/inspect_cmd.go` — new inspect subcommands and table formatters
- `internal/cli/get_cmd_test.go`
- `internal/cli/status_cmd_test.go`
- `internal/cli/inspect_cmd_test.go`
- `docs/technical-spec.md` — document new API/CLI surface
- `docs/README.md`
- `docs/operations-manual-agent.md`
- `.agent/skills/htmlctl-publish/SKILL.md`
- `.agent/skills/htmlctl-publish/references/commands.md`

## 5. Implementation Steps

1. Add typed server/client inventory models for website, pages, components, styles, assets, and branding metadata.
2. Implement a single read-only server inventory endpoint that assembles those models from existing DB query helpers.
3. Extend `get` to support the new resource types with deterministic table/json/yaml output.
4. Add `inspect website`, `inspect page`, and `inspect component`.
5. Keep `inspect` output high-signal:
   - page layout order preserved
   - sidecar presence explicit
   - referencing pages for components computed client-side from the inventory payload
6. Add a small `status` enhancement for `default_style_bundle` and `base_template`.
7. Update command docs and the publish skill so agents are taught to use `get` for inventory and `inspect` for deep dives.

## 6. Tests and Validation

### Automated

- server `GET /resources` success path returns website, pages, components, styles, assets, and branding in deterministic order
- malformed website/environment path returns actionable `4xx`
- `htmlctl get pages|components|styles|assets|branding|website` supports `table|json|yaml`
- `htmlctl inspect website|page|component` supports `table|json|yaml`
- `inspect component` lists referencing pages correctly
- `status` remains backward-compatible aside from the added summary rows
- unsupported `get` types still return actionable errors naming the supported inventory types

### Manual

- on a real site, use only `doctor`, `get`, and `inspect` to answer:
  - which routes exist
  - which component powers a given route
  - whether the site uses component sidecars
  - which style bundle/template is active
  - whether `robots`, `sitemap`, `llmsTxt`, and `structuredData` are enabled

## 7. Acceptance Criteria

- [ ] AC-1: `htmlctl get` supports content-level inventory for `pages`, `components`, `styles`, `assets`, `branding`, and `website`.
- [ ] AC-2: `htmlctl inspect website`, `htmlctl inspect page <name>`, and `htmlctl inspect component <name>` are available with `table|json|yaml` output.
- [ ] AC-3: A zero-context operator can discover page routes, page layout composition, component sidecars, and active style/template configuration without reading repository docs.
- [ ] AC-4: Inventory and inspect commands are read-only and operate on desired state, not rendered release artifacts.
- [ ] AC-5: `.agent/skills/htmlctl-publish/` guidance explicitly points agents to the new inventory/inspect commands.

## 8. Risks and Open Questions

### Risks

- **Risk:** inspect grows into a second, inconsistent command language beside `get`.  
  **Mitigation:** keep `get` for tabular inventory and `inspect` for deep inspection; document that split clearly.

- **Risk:** server inventory payload becomes unstable or over-detailed.  
  **Mitigation:** keep fields focused on authoring and operator questions; defer raw source export to E13-S3.

- **Risk:** developers are tempted to read rendered release files instead of desired state because it feels easier.  
  **Mitigation:** state explicitly in code and docs that rendered artifacts are not a source-model substitute.

### Open Questions

- Should `inspect page` accept a route as an alternate key in v1, or only page name? Recommendation: page name only in v1 to avoid route/name ambiguity; route lookup can come later if needed.
