# Changelog

## 0.3.1 (2026-03-06)

- Hardened service config validation: `NEWSLETTER_HTTP_ADDR` now requires explicit numeric port `1..65535`.
- Hardened SQL migration execution: statement splitting is now SQL-aware (handles comments, quoted strings, and dollar-quoted function bodies safely).
- Fixed installer/unit binary path consistency: systemd units now inherit `NEWSLETTER_BIN_PATH` replacement during install.
- Aligned extension manifest contract with runtime defaults by making `NEWSLETTER_HTTP_ADDR` optional.

## 0.3.0 (2026-03-06)

- Added `extensions/newsletter/ops/setup-newsletter-extension.sh` bootstrap installer.
- Added systemd unit templates for staging/prod service instances.
- Added non-secret env templates for staging/prod.
- Added Hetzner-focused operator runbook (`docs/guides/newsletter-extension-hetzner.md`).

## 0.2.0 (2026-03-06)

- Added `extensions/newsletter/service` reference implementation module.
- Added `htmlctl-newsletter` commands: `serve` and `migrate`.
- Added foundation PostgreSQL migration (`001_foundation.sql`) for subscribers, verification tokens, campaigns, and campaign sends.
- Added loopback bind/public URL/env validation tests and migration idempotency tests.

## 0.1.0 (2026-03-06)

- Initial extension metadata contract for newsletter extension.
