# htmlctl Extensions Catalog

Official extensions are optional companion services for `htmlctl`/`htmlservd` operators.

Core boundary:
- `htmlctl` and `htmlservd` remain static-site control-plane components.
- Extensions run as separate deployable services.
- Public traffic routing to extensions uses Epic 9 environment backends (`htmlctl backend add/list/remove`).

Each official extension directory must contain:
- `extension.yaml` - compatibility and runtime contract
- `README.md` - operator guide and architecture notes
- `CHANGELOG.md` - release notes

See:
- `extensions/schema/extension.schema.yaml`
- `docs/reference/extensions.md`
- `docs/guides/extensions-overview.md`
