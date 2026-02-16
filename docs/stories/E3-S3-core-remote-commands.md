# E3-S3 - Core Remote Commands

**Epic:** Epic 3 — Remote transport + kubectl UX
**Status:** Done
**Priority:** P1 (Critical Path)
**Estimated Effort:** 4 days
**Dependencies:** E3-S1 (context config), E3-S2 (SSH tunnel transport), E2-S3 (server apply API), E2-S5 (server audit log API)
**Target:** htmlctl v1
**Design Reference:** Technical Spec Section 9.2 (Core commands), PRD Section 5 (Core user journeys)

---

## 1. Objective

Implement the primary remote CLI commands that operators and AI agents use to interact with htmlservd: `get`, `status`, `apply`, and `logs`. These commands form the kubectl-style UX that makes htmlctl a practical control plane for websites. Each command resolves a context, opens an SSH tunnel, makes HTTP requests to the server API, and formats the response for the terminal.

## 2. User Story

As an operator or AI agent, I want to run `htmlctl get`, `status`, `apply`, and `logs` commands against a remote server so that I can list resources, check environment health, deploy site changes, and review the audit trail — all through a familiar kubectl-style CLI.

## 3. Scope

### In Scope

- `htmlctl get <resource-type>` — list resources from the server (websites, environments, releases)
- `htmlctl status website/<name> --context <ctx>` — show environment status including active release, release timestamp, and resource counts
- `htmlctl apply -f ./site --context <ctx>` — bundle local site directory, upload to server, trigger release pipeline
- `htmlctl logs website/<name> --context <ctx>` — fetch and display the audit log for a website
- All commands use the `--context` flag (or `current-context`) to resolve connection details
- All commands use the SSH tunnel transport layer to reach htmlservd
- Output formatting: default human-readable table output, with `--output json` and `--output yaml` flags
- Bundle creation for `apply`: tar/gzip the site directory, compute manifest hashes, send as multipart or streaming upload
- Progress feedback during apply (uploading, validating, rendering, activating)
- Error handling: server errors mapped to user-friendly CLI messages

### Out of Scope

- `htmlctl diff` (separate story: E3-S4)
- `htmlctl promote`, `htmlctl rollout` (Epic 4)
- `htmlctl domain` commands (Epic 5)
- Partial apply of individual files (v1 sends the full site directory; server-side merge is in E2-S3)
- Watch/streaming mode for logs
- Pagination for large resource lists
- Custom column selection for `get` output

## 4. Architecture Alignment

- **Technical Spec Section 9.2**: Defines the command signatures: `apply -f`, `status website/<name>`, `logs website/<name>`. This story implements the remote subset.
- **Technical Spec Section 9.3**: Agent-friendly partial apply — the CLI always bundles and sends the full `-f` path contents. Server-side merge with existing desired state is handled by E2-S3 (bundle ingestion).
- **PRD Section 2, Goal 1**: "kubectl-like UX for websites (apply/diff/get/status/rollout/promote)" — this story delivers `get`, `status`, `apply`, and `logs`.
- **PRD Section 5**: Core user journeys 2 (deploy to staging) and 3 (promote to prod) both depend on `apply` and `status`.
- **Component boundary**: Commands live in `cmd/htmlctl/` and use `internal/transport.Transport` for HTTP communication. Commands should not contain transport logic directly — they construct HTTP requests and pass them to the transport layer.
- **Server API contract**: Commands depend on server API endpoints defined in E2-S3 (apply) and E2-S5 (audit log). The `get` and `status` endpoints are implicit requirements of the server's resource API.

## 5. Implementation Plan (Draft)

### 5.1 Files to Create

- `internal/client/client.go` — `APIClient` struct wrapping `transport.Transport`, provides typed methods: `ListResources()`, `GetStatus()`, `Apply()`, `GetLogs()`
- `internal/client/types.go` — Response types: `ResourceList`, `WebsiteStatus`, `ApplyResult`, `AuditLogEntry`
- `internal/bundle/bundle.go` — Bundle creation: walk site directory, compute SHA256 hashes, create tar.gz archive with manifest
- `internal/bundle/manifest.go` — Bundle manifest type: file paths, hashes, metadata
- `internal/output/formatter.go` — Output formatter: table (default), JSON, YAML modes
- `internal/output/table.go` — Table renderer using `text/tabwriter`
- `cmd/htmlctl/get_cmd.go` — `htmlctl get` command implementation
- `cmd/htmlctl/status_cmd.go` — `htmlctl status` command implementation
- `cmd/htmlctl/apply_cmd.go` — `htmlctl apply` command implementation
- `cmd/htmlctl/logs_cmd.go` — `htmlctl logs` command implementation

### 5.2 Files to Modify

- `cmd/htmlctl/root.go` — Register `get`, `status`, `apply`, `logs` subcommands
- `go.mod` — Any additional dependencies for output formatting

### 5.3 Tests to Add

- `internal/client/client_test.go` — Test API client with mock transport (verify correct HTTP methods, paths, headers, and response parsing)
- `internal/bundle/bundle_test.go` — Test bundle creation: correct tar structure, manifest hash accuracy, handling of symlinks and empty dirs
- `internal/bundle/manifest_test.go` — Test manifest generation and hash computation
- `internal/output/formatter_test.go` — Test table, JSON, YAML output formatting
- `cmd/htmlctl/get_cmd_test.go` — Integration test for get command with mock server
- `cmd/htmlctl/apply_cmd_test.go` — Integration test for apply command with mock server
- `cmd/htmlctl/status_cmd_test.go` — Integration test for status command with mock server
- `cmd/htmlctl/logs_cmd_test.go` — Integration test for logs command with mock server

### 5.4 Dependencies/Config

- `go.mod` — `encoding/json` and `gopkg.in/yaml.v3` (likely already present from E3-S1)
- `go.mod` — Standard library `archive/tar`, `compress/gzip`, `crypto/sha256` (no external deps needed for bundling)

## 6. Acceptance Criteria

- [x] AC-1: `htmlctl get websites --context <ctx>` lists all websites on the server in a formatted table
- [x] AC-2: `htmlctl get environments --context <ctx>` lists all environments with their active release IDs
- [x] AC-3: `htmlctl status website/<name> --context <ctx>` displays environment name, active release ID, release timestamp, and resource counts (pages, components, styles)
- [x] AC-4: `htmlctl apply -f ./site --context <ctx>` bundles the local site directory, uploads it to the server, and reports the new release ID upon success
- [x] AC-5: `apply` displays progress feedback: "Bundling... Uploading... Validating... Rendering... Activating... Done. Release <id> active."
- [x] AC-6: `apply` with an invalid site directory (missing `website.yaml`) fails with a clear local validation error before uploading
- [x] AC-7: `htmlctl logs website/<name> --context <ctx>` displays audit log entries with timestamp, action, actor, environment, and release ID
- [x] AC-8: All commands accept `--output json` to produce machine-parseable JSON output
- [x] AC-9: All commands accept `--output yaml` to produce YAML output
- [x] AC-10: All commands use the SSH tunnel transport — no direct network connections to the server
- [x] AC-11: Server-side errors (4xx, 5xx) are translated to user-friendly CLI error messages with actionable suggestions
- [x] AC-12: When the `--context` flag is omitted, commands use `current-context` from config; when provided, it overrides
- [x] AC-13: Bundle manifest includes SHA256 hashes for all files; server can verify integrity

## 7. Verification Plan

### Automated Tests

- [x] Unit tests for bundle creation (correct tar structure, manifest hashes match file content)
- [x] Unit tests for API client methods with mock transport (correct HTTP requests constructed, responses parsed)
- [x] Unit tests for output formatters (table alignment, JSON validity, YAML validity)
- [x] Integration tests for each command using a mock HTTP server behind the transport interface
- [x] Test error paths: server returns 404, 409, 500; verify CLI error messages

### Manual Tests

- [ ] Run `htmlctl apply -f ./site --context staging` against a running htmlservd and verify a release is created
- [ ] Run `htmlctl status website/futurelab --context staging` and verify output matches server state
- [ ] Run `htmlctl get websites --context staging` and verify table output is readable
- [ ] Run `htmlctl logs website/futurelab --context staging` and verify audit entries appear
- [ ] Run `htmlctl get websites --context staging --output json` and verify valid JSON output
- [ ] Test with a large site directory (100+ files) to verify bundle performance

## 8. Performance / Reliability Considerations

- Bundle creation should stream the tar.gz to avoid holding the entire archive in memory. For v1, in-memory is acceptable for typical site sizes (< 50MB).
- Upload should use a streaming HTTP body (not buffer entire bundle in memory before sending) for larger sites.
- The `apply` command is the most latency-sensitive operation. Expected breakdown: bundle (< 1s), upload (depends on size/bandwidth), server processing (< 5s for typical sites). Total should be under 10s for a typical site.
- Output formatting should be efficient; `text/tabwriter` is suitable for v1 table sizes.

## 9. Risks & Mitigations

- **Risk:** Server API contract not yet finalized (E2-S3, E2-S5 may evolve). **Mitigation:** Define API client against an interface; use integration tests with mock server. Update client when server API is finalized.
- **Risk:** Large site bundles may cause upload timeouts or memory issues. **Mitigation:** Set reasonable limits (e.g., 100MB bundle max); implement streaming upload. Document size limits.
- **Risk:** Output table formatting may not handle long values well (e.g., long release IDs, long file names). **Mitigation:** Truncate long fields with ellipsis; provide `--output json` for full data.
- **Risk:** Different Go YAML/JSON serialization behavior between client and server. **Mitigation:** Use the same YAML library (`gopkg.in/yaml.v3`) on both sides; test round-trip serialization.

## 10. Open Questions

- What are the exact server API endpoint paths? **Implemented answer:** `GET /api/v1/websites`, `GET /api/v1/websites/{website}/environments`, `GET /api/v1/websites/{website}/environments/{env}/status`, `POST /api/v1/websites/{website}/environments/{env}/apply`, `POST|GET /api/v1/websites/{website}/environments/{env}/releases`, `GET /api/v1/websites/{website}/environments/{env}/logs`.
- Should `htmlctl get` support a `releases` resource type? **Implemented answer:** Yes, `htmlctl get releases` lists releases for the context website/environment.
- Should `apply` support `--wait` to block until the release is fully active? **Implemented answer:** Apply remains synchronous in v1 (upload + release build in-sequence).
- Should logs support `--since` or `--limit` filtering? **Implemented answer:** `--limit` is implemented (default 50); `--since` remains out of scope.

---

## Implementation Summary

Implemented core remote command stack and supporting APIs:
- Added `internal/client` typed API client (`ListWebsites`, `ListEnvironments`, `ListReleases`, `GetStatus`, `ApplyBundle`, `CreateRelease`, `GetLogs`) with SSH/transport error mapping and user-friendly HTTP error handling.
- Added `internal/bundle.BuildTarFromDir` to validate a site directory, generate a full manifest with SHA256 hashes, and build a deterministic tar archive for apply uploads.
  - Added a 100MB bundle-size guardrail and context/`website.yaml` name consistency validation.
- Added `internal/output` formatting package for table/json/yaml output modes.
- Added CLI commands in `internal/cli`: `get`, `status`, `apply`, `logs`, including:
  - `--output table|json|yaml` support on all commands
  - `--limit` for logs
  - apply progress messages in table mode
  - local validation before upload (`website.yaml` missing fails fast)
- Extended server APIs in `internal/server`:
  - `GET /api/v1/websites`
  - `GET /api/v1/websites/{website}/environments`
  - `GET /api/v1/websites/{website}/environments/{env}/status`
  - `GET /api/v1/websites/{website}/environments/{env}/releases`
  - Optimized status counts to use `COUNT(*)` queries rather than full-row scans.
- Registered remote commands in CLI root and kept lifecycle on SSH transport via existing runtime annotations.
- Added comprehensive tests:
  - bundle generation/validation tests
  - client request/response + error-path tests
  - formatter tests
  - CLI command tests using mocked transport
  - server endpoint tests for website/environment/status/release listing
- Verification evidence:
  - `go test ./...`

## Code Review Findings

`pi` review iterations:
- `docs/review-logs/E3-S3-review-pi-2026-02-16-114102.log`
  - Initial pass: no P0/P1, recommended scalability improvements.
- `docs/review-logs/E3-S3-review-pi-2026-02-16-114423.log`
  - Follow-up pass: identified P2 robustness gaps (script counting + MIME fallback).
- `docs/review-logs/E3-S3-review-pi-2026-02-16-114704.log`
  - Final status: **CLEAN** (no critical findings).

## Completion Status

Implemented, tested, and review-clean. Ready for merge.
