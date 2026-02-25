# E1-S1 - Define Resource Schemas and Parser

**Epic:** Epic 1 - Foundations: Repo schema + local render
**Status:** Done
**Priority:** P0 (Critical Path)
**Estimated Effort:** 2 days
**Dependencies:** None
**Target:** Go CLI (htmlctl)
**Design Reference:** docs/technical-spec.md sections 1-2

---

## 1. Objective

Establish the foundational data model for htmlctl by implementing Go structs and YAML parsing for the core resource types: Website, Page, and Component. This is the first story in the project and unblocks all subsequent rendering, validation, and CLI work. Without a well-defined resource schema, nothing else can be built.

## 2. User Story

As an AI agent operating via CLI, I want a site directory parsed into strongly-typed resource structs so that downstream systems (renderer, validator, CLI) can operate on a reliable, validated data model.

## 3. Scope

### In Scope

- Go structs for `Website` resource with fields: `apiVersion`, `kind`, `metadata.name`, `spec.defaultStyleBundle`, `spec.baseTemplate`
- Go structs for `Page` resource with fields: `apiVersion`, `kind`, `metadata.name`, `spec.route`, `spec.title`, `spec.description`, `spec.layout` (ordered list of objects shaped as `[{include: string}]`)
- Go struct for `Component` with fields: `name`, `scope` (hardcoded to `global` in v1), `html` (file content as string)
- Go struct for `StyleBundle` representing `styles/tokens.css` and `styles/default.css`
- YAML parsing of `website.yaml` and `pages/*.page.yaml` using `gopkg.in/yaml.v3`
- Component file discovery: read all `.html` files from `components/` directory, map by filename (without extension) to Component structs
- Style file discovery: read `styles/tokens.css` and `styles/default.css`
- Script file discovery: check for optional `scripts/site.js`
- Asset file discovery: recursively list files in `assets/` directory
- A `Site` aggregate struct that holds the parsed Website, all Pages, all Components, StyleBundle, optional script path, and asset list
- A `LoadSite(dirPath string) (*Site, error)` function that orchestrates full parsing of a site directory
- Validation that mandatory files exist: `website.yaml`, at least one page file, referenced components exist
- Validation that all `include` references in page layouts resolve to known components
- Validation that page routes are normalized deterministically (leading slash required, no trailing slash except `/`)

### Out of Scope

- Component HTML content validation (covered in E1-S3)
- Rendering/stitching of HTML output (covered in E1-S2)
- CLI command wiring (covered in E1-S4)
- Server-side storage, SQLite, releases, environments (Epic 2)
- Asset content-addressing or hash computation (later stories)
- Page-scoped components (post-v1 per PRD open questions)

## 4. Architecture Alignment

- **Resource model**: Follows the declarative resource model defined in technical-spec.md section 2 (Website, Page, Component, StyleBundle, Asset)
- **YAML structure**: Uses Kubernetes-inspired `apiVersion`/`kind`/`metadata`/`spec` pattern as shown in the technical spec Page example
- **Repository layout**: Parses the `site/` directory structure defined in technical-spec.md section 1
- **Go implementation**: Per technical-spec.md section 10, the project uses Go for both CLI and daemon
- **Component boundaries**: This story produces the `pkg/model/` and `pkg/loader/` packages; downstream stories consume these types
- **No concurrency concerns**: File parsing is sequential and single-threaded at this stage

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `pkg/model/types.go` - Core resource type definitions (Website, Page, Component, StyleBundle, Asset, Site)
- `pkg/model/constants.go` - API version string, kind constants, default values
- `pkg/loader/loader.go` - Site directory loader: reads and parses all resource files into a Site struct
- `pkg/loader/validate.go` - Post-parse validation: mandatory files, include resolution, route normalization
- `go.mod` - Go module definition (`github.com/bene/htmlctl` or appropriate module path)
- `go.sum` - Dependency checksums (auto-generated)

### 5.2 Files to Modify

- None (first story, greenfield)

### 5.3 Tests to Add

- `pkg/model/types_test.go` - Unit tests for struct construction and field defaults
- `pkg/loader/loader_test.go` - Tests for LoadSite with valid site directories, missing files, malformed YAML
- `pkg/loader/validate_test.go` - Tests for include resolution, missing component references, duplicate routes
- `testdata/valid-site/` - Minimal valid site fixture (website.yaml, one page, one component, styles)
- `testdata/missing-component/` - Site fixture with page referencing non-existent component
- `testdata/malformed-yaml/` - Site fixture with invalid YAML syntax
- `testdata/missing-website-yaml/` - Site fixture missing the required website.yaml

### 5.4 Dependencies/Config

- `gopkg.in/yaml.v3` - YAML parsing library (well-established, supports struct tags)
- Go 1.22+ (use current stable Go version)

## 6. Acceptance Criteria

- [ ] AC-1: `Website` struct correctly deserializes from `website.yaml` with fields `metadata.name`, `spec.defaultStyleBundle`, `spec.baseTemplate`
- [ ] AC-2: `Page` struct correctly deserializes from `pages/*.page.yaml` files with fields `spec.route`, `spec.title`, `spec.description`, `spec.layout` (list of objects with `include` key)
- [ ] AC-3: `Component` structs are populated by reading `.html` files from `components/` directory, with `name` derived from filename and `html` containing file content
- [ ] AC-4: `StyleBundle` is populated from `styles/tokens.css` and `styles/default.css`
- [ ] AC-5: `LoadSite()` returns error if `website.yaml` is missing
- [ ] AC-6: `LoadSite()` returns error if a page layout references a component name that does not exist in `components/`
- [ ] AC-7: `LoadSite()` returns error if YAML files contain syntax errors, with a clear error message including the filename
- [ ] AC-8: `LoadSite()` successfully parses the sample `sample`-style site directory into a complete `Site` struct
- [ ] AC-9: Scripts (`scripts/site.js`) are detected as optional; missing script does not cause an error
- [ ] AC-10: Assets in `assets/` directory are discovered and listed with original filenames
- [ ] AC-11: `LoadSite()` returns error if `pages/` contains no valid `*.page.yaml` files
- [ ] AC-12: `LoadSite()` normalizes routes (leading `/`, trailing slash removed except root `/`) for deterministic downstream matching

## 7. Verification Plan

### Automated Tests

- [ ] Unit test: parse valid `website.yaml` into Website struct, assert all fields
- [ ] Unit test: parse valid `*.page.yaml` into Page struct, assert layout includes are ordered correctly
- [ ] Unit test: parse `spec.layout` as object entries (`- include: header`) rather than plain string list
- [ ] Unit test: load components from directory, verify name mapping matches filenames
- [ ] Unit test: asset discovery includes nested files under `assets/` (recursive scan)
- [ ] Integration test: `LoadSite()` on valid testdata fixture returns fully populated Site with no error
- [ ] Integration test: `LoadSite()` on fixture with missing component reference returns descriptive error
- [ ] Integration test: `LoadSite()` on fixture with malformed YAML returns error mentioning the filename
- [ ] Integration test: `LoadSite()` on fixture missing `website.yaml` returns specific error
- [ ] Unit test: optional `scripts/site.js` is nil/empty when file does not exist
- [ ] Integration test: `LoadSite()` fails when `pages/` exists but has no valid `*.page.yaml` files
- [ ] Unit test: route normalization converts `about/` -> `/about` and preserves `/`

### Manual Tests

- [ ] Create a sample site directory matching the `sample` structure from the technical spec and verify `LoadSite()` parses it without errors
- [ ] Verify that Go struct tags produce clean YAML round-trip (marshal then unmarshal)

## 8. Performance / Reliability Considerations

- Site directories are expected to be small (tens of files); no performance concerns at this scale
- File I/O errors (permission denied, broken symlinks) should produce clear error messages with file paths
- YAML parsing errors should wrap the underlying error with the filename for debuggability

## 9. Risks & Mitigations

- **Risk**: YAML schema evolves as later stories reveal needs (e.g., new fields) - **Mitigation**: Keep structs simple with `yaml` struct tags; new optional fields can be added without breaking existing parsers
- **Risk**: Component naming collisions (e.g., `header.html` in nested dirs) - **Mitigation**: v1 supports flat `components/` only; no subdirectories. Document this constraint.
- **Risk**: Different line endings on different platforms could affect component HTML content - **Mitigation**: Normalize line endings to LF on read (preparation for deterministic rendering in E1-S2)

## 10. Open Questions

- Should the `apiVersion` field be validated strictly (e.g., must be `htmlctl.dev/v1`) or accepted loosely in v1?
- Should unknown YAML fields cause warnings or hard errors? (Recommendation: warn but accept, for forward compatibility)

## 11. Research Notes

### Go YAML Parsing (gopkg.in/yaml.v3)

- Standard Go YAML library with struct tag support (`yaml:"fieldName"`)
- Supports nested structs, slices, and maps
- Provides `yaml.Decoder` for streaming and `yaml.Unmarshal` for in-memory parsing
- Error messages include line/column information for syntax errors

### Resource Model Patterns (kubectl/k8s)

- Kubernetes uses `apiVersion`, `kind`, `metadata`, `spec` as top-level fields for all resources
- htmlctl adopts this pattern for familiarity: agents already understand this structure
- `metadata.name` serves as the unique identifier within a resource type
- `spec` contains the resource-specific configuration

---

## Implementation Summary

(TBD after implementation.)

## Code Review Findings

(TBD by review agent.)

## Completion Status

(TBD after merge.)
