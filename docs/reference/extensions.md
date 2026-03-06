# Extensions Reference

`htmlctl` extensions are optional companion services that integrate with static sites through environment backends.

Core boundary:
- `htmlctl` and `htmlservd` do not load extension code at runtime.
- Extensions are separately deployed services, usually loopback-bound on the same host.
- Public path routing is configured with Epic 9 backend commands.

## Manifest Contract

Each official extension must provide `extension.yaml` with:
- identity (`apiVersion`, `kind`, `metadata.name`, `metadata.version`)
- compatibility (`spec.compatibility.minHTMLCTL`, `spec.compatibility.minHTMLSERVD`)
- runtime dependencies and health endpoints
- integration backend path prefixes
- required environment variables (including secret classification)
- baseline security expectations

Schema location:
- `extensions/schema/extension.schema.yaml`

Catalog location:
- `extensions/`

## Security Baseline

Official extensions must document and satisfy:
- loopback-only listener policy by default
- staging/prod credential and datastore isolation
- secrets stored only in server-side env/config (never committed)
- endpoint abuse controls for public unauthenticated routes
- sanitized 5xx behavior (no internal stack paths/IDs in client responses)

## Current Official Extensions

- `newsletter` (reference implementation available)
  - Contract: `extensions/newsletter/extension.yaml`
  - Service module: `extensions/newsletter/service`
  - Runtime binary command: `htmlctl-newsletter <serve|migrate>`
  - Installer assets: `extensions/newsletter/ops/`
  - Hetzner runbook: `docs/guides/newsletter-extension-hetzner.md`

## Newsletter Install and Verify

Install artifacts:
- `extensions/newsletter/ops/setup-newsletter-extension.sh`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-staging.service`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-prod.service`
- `extensions/newsletter/ops/env/staging.env.example`
- `extensions/newsletter/ops/env/prod.env.example`

Post-install checks:
- `systemctl status` for staging and prod units
- loopback-only listener verification via `ss -tlnp`
- `/healthz` probes on staging/prod loopback ports
- env file mode verification (`640 root htmlctl-newsletter`)
- staging/prod DB isolation query checks (`has_database_privilege`)
