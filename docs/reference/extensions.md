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

Validation and compatibility gate:
- `htmlctl extension validate <extension-dir-or-manifest>` validates manifest structure and checks `minHTMLCTL` against the local CLI version.
- `htmlctl extension validate <extension-dir-or-manifest> --remote --context <ctx>` also checks `minHTMLSERVD` against the selected remote `htmlservd`.
- Compatibility metadata is not enforced implicitly during `backend add`; operators should run the validation command before adopting or upgrading an extension.

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
  - Runtime binary command: `htmlctl-newsletter <serve|migrate|import-legacy|campaign>`
  - Installer assets: `extensions/newsletter/ops/`
  - VPS runbook: `docs/guides/newsletter-extension-vps.md`
- `telemetry-collector` (reference implementation available)
  - Contract: `extensions/telemetry-collector/extension.yaml`
  - Service module: `extensions/telemetry-collector/service`
  - Runtime binary command: `htmlctl-telemetry-collector <serve>`
  - Installer assets: `extensions/telemetry-collector/ops/`
  - VPS runbook: `docs/guides/telemetry-collector-extension-vps.md`

## Newsletter Install and Verify

Install artifacts:
- `extensions/newsletter/ops/setup-newsletter-extension.sh`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-staging.service`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-prod.service`
- `extensions/newsletter/ops/env/staging.env.example`
- `extensions/newsletter/ops/env/prod.env.example`

Post-install checks:
- `htmlctl extension validate extensions/newsletter --remote --context <ctx>`
- `systemctl status` for staging and prod units
- loopback-only listener verification via `ss -tlnp`
- `/healthz` probes on staging/prod loopback ports
- env file mode verification (`640 root htmlctl-newsletter`)
- env contract verification:
  - `NEWSLETTER_RESEND_FROM` is a valid sender address
  - `NEWSLETTER_LINK_SECRET` is unique per environment and at least 32 characters
- staging/prod DB isolation query checks (`has_database_privilege`)
- campaign workflow verification:
  - `htmlctl-newsletter campaign upsert --slug <slug> --subject ... --html-file ... --text-file ...`
  - `htmlctl-newsletter campaign preview --slug <slug> --to you@example.com`
  - `htmlctl-newsletter campaign send --slug <slug> --mode all --interval 30s --confirm`

## Telemetry Collector Install and Verify

Install artifacts:
- `extensions/telemetry-collector/ops/setup-telemetry-collector-extension.sh`
- `extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-staging.service`
- `extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-prod.service`
- `extensions/telemetry-collector/ops/env/staging.env.example`
- `extensions/telemetry-collector/ops/env/prod.env.example`

Post-install checks:
- `htmlctl extension validate extensions/telemetry-collector --remote --context <ctx>`
- `systemctl status` for staging and prod units
- loopback-only listener verification via `ss -tlnp`
- `/healthz` probes on staging/prod loopback ports
- env file mode verification (`640 root htmlctl-telemetry`)
- env contract verification:
  - `TELEMETRY_COLLECTOR_PUBLIC_BASE_URL` is the exact public `https://` origin for that environment
  - `TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL` stays loopback-only
  - `TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN` is stored server-side only
- telemetry route verification:
  - browser/site JS posts to `/site-telemetry/v1/events`
  - a valid same-origin event returns `202`
  - `GET /api/v1/websites/<site>/environments/<env>/telemetry/events` shows the stored event
