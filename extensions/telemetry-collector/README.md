# telemetry-collector extension

Official browser-facing telemetry collector extension package for `htmlctl` ecosystems.

Current contents:
- `extension.yaml`: compatibility and runtime contract metadata.
- `service/`: Go reference implementation (`htmlctl-telemetry-collector`) with public ingest and forwarding.
- `ops/`: installer script, systemd templates, env examples.
- `CHANGELOG.md`: extension release notes.
- adopter docs:
  - `docs/guides/extensions-overview.md`
  - `docs/guides/telemetry-collector-extension-vps.md`
