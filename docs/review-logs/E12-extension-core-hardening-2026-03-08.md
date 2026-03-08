# E12 Extension Core Hardening (2026-03-08)

## Scope

Focused review and remediation of core extension-system behavior in `htmlctl` and `htmlservd` after real newsletter adoption on Futurelab.

Reviewed areas:
- backend upstream validation
- backend mutation / Caddy reload consistency
- extension manifest compatibility enforcement

## Findings

1. Backend upstream validation allowed path-bearing URLs.
- Example previously accepted: `https://api.example.com/base`
- Risk: invalid or ambiguous `reverse_proxy` upstream targets in generated Caddy config.

2. Backend mutations could persist even when live routing failed to reload.
- Previous behavior: API returned success and stored the backend row even if Caddy reload failed.
- Risk: control-plane intent drifted ahead of live routing, especially dangerous during extension cutovers and rollback operations.

3. Extension compatibility metadata was validated syntactically but not consumed operationally.
- `extension.yaml` had `minHTMLCTL` / `minHTMLSERVD`, but operators had no first-class command to enforce them.
- Risk: incompatible extension adoption remained a docs-only hazard.

## Implemented Fixes

### Backend upstream validation

- Tightened `internal/backend.ValidateUpstreamURL`:
  - reject non-root path segments
  - continue rejecting credentials, query strings, and fragments
  - normalize a trailing `/` to the bare origin

Updated tests:
- `internal/backend/validate_test.go`
- `internal/caddy/config_test.go`

### Backend mutation rollback

- Added rollback-safe behavior in `internal/server/backends.go`:
  - `backend add`: rollback create/update when Caddy reload fails
  - `backend remove`: restore deleted backend when Caddy reload fails
- Added `RestoreBackend` query helper in `internal/db/queries.go`
- Added targeted tests proving rollback behavior:
  - failed add reload returns `500` and leaves no backend row
  - failed remove reload returns `500` and restores the backend row

### Compatibility enforcement

- Added semver comparison helpers in `internal/extensionspec`
- Added CLI command:

```bash
htmlctl extension validate <extension-dir-or-manifest>
htmlctl extension validate <extension-dir-or-manifest> --remote --context <ctx>
```

Command behavior:
- validates manifest structure with existing `extensionspec` loader
- checks local `htmlctl` version against `spec.compatibility.minHTMLCTL`
- optionally checks remote `htmlservd` version against `spec.compatibility.minHTMLSERVD`

## Verification

Focused test suite:

```bash
go test ./internal/backend ./internal/caddy ./internal/db ./internal/server ./internal/extensionspec ./internal/cli
```

Result: pass

## Operational Impact

- Backend changes are now fail-safe instead of eventually-applied.
- Extension adoption has an explicit compatibility gate before backend wiring.
- Documentation now reflects:
  - origin-only upstream requirements
  - rollback-on-reload-failure semantics
  - `htmlctl extension validate` preflight workflow
