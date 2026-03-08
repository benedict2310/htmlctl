# Extensions Overview

Use extensions when you need dynamic behavior alongside a static `htmlctl`-managed site.

Examples:
- newsletter signup/verification
- form processing APIs
- authenticated application paths

## Integration Model

1. Deploy extension services separately from `htmlservd`.
2. Keep extension listeners loopback-only on the host.
3. Validate extension compatibility before cutover.
4. Route public paths with environment backends.
5. Validate behavior on staging first.
6. Promote static releases independently; maintain extension runtime config per environment.

Backends are environment-scoped runtime config, not bundle content:

```bash
htmlctl extension validate extensions/newsletter --remote --context staging
htmlctl backend add website/<site> --env staging --path /service/* --upstream http://127.0.0.1:<port>
htmlctl backend list website/<site> --env staging
```

Backend constraints and failure semantics:
- upstreams must be origin-only `http://` or `https://` targets; do not include path segments such as `/base`
- if `backend add` or `backend remove` cannot be applied because Caddy reload fails, the mutation is rolled back and the command fails instead of leaving stored state ahead of live routing

## Newsletter Quickstart (New Operator)

Reference implementation:
- Service module: `extensions/newsletter/service`
- Installer and ops assets: `extensions/newsletter/ops`
- Host runbook: `docs/guides/newsletter-extension-hetzner.md`
- Adoption validation log: `docs/review-logs/E12-newsletter-extension-adoption-validation-2026-03-06.md`

1. Install staging/prod service units and env files with `extensions/newsletter/ops/setup-newsletter-extension.sh`.
2. Validate manifest compatibility before routing:
- `htmlctl extension validate extensions/newsletter --remote --context staging`
3. Verify service invariants before routing:
- `systemctl status htmlctl-newsletter-{staging,prod}`
- `curl -sf http://127.0.0.1:9501/healthz` and `:9502/healthz`
- `ss -tlnp` confirms loopback bind only
- env files mode `640 root htmlctl-newsletter`
- env files include both `NEWSLETTER_RESEND_FROM` and `NEWSLETTER_LINK_SECRET`
- `NEWSLETTER_RESEND_FROM` is a valid sender address and `NEWSLETTER_LINK_SECRET` is a distinct high-entropy secret per environment (minimum 32 characters)
4. Add staging backend and validate route behavior.
5. Validate failure mode handling (upstream down, wrong mapping).
6. Add prod backend only after staging checks are clean.

Operator workflow after install:

```bash
# import from a legacy database with a compatible `subscribers` table
NEWSLETTER_ENV=prod NEWSLETTER_DATABASE_URL='postgres://target...' \
htmlctl-newsletter import-legacy --source-database-url 'postgres://source...'

# store campaign content
NEWSLETTER_ENV=staging NEWSLETTER_DATABASE_URL='postgres://...' \
htmlctl-newsletter campaign upsert --slug launch --subject 'Launch update' \
  --html-file ./campaigns/launch.html --text-file ./campaigns/launch.txt

# preview and then send with pacing
NEWSLETTER_ENV=staging ... htmlctl-newsletter campaign preview --slug launch --to you@example.com
NEWSLETTER_ENV=prod ... htmlctl-newsletter campaign send --slug launch --mode all --interval 30s --confirm
```

## Production Cutover Checklist

- Staging and prod use separate databases, DB roles, and API keys.
- Health probes succeed on both local loopback ports.
- No public listener on extension ports.
- Backend mappings are explicit per environment (`staging` and `prod`).
- Route tests pass:
- expected app response on `/newsletter/*`
- expected upstream failure response during a controlled outage test
- rollback command tested (`htmlctl backend remove ...`) and documented.
- campaign tests pass:
- preview delivery succeeds from staging
- unsubscribe link in delivered message resolves cleanly
- full send uses explicit interval pacing (`--interval 30s` on low-tier Resend plans)

## Rollback and Failure Drills

If cutover introduces issues:

```bash
htmlctl backend remove website/<site> --env prod --path /newsletter/*
htmlctl backend list website/<site> --env prod
```

Recommended drills before public launch:
- stop the newsletter unit and confirm proxied path fails as expected (`502` from reverse proxy)
- restore service and confirm route recovery
- test incorrect upstream mapping in staging, then restore correct mapping
- confirm failed backend mutations are rejected instead of silently persisting

## Upgrade Path

1. Build and install new `htmlctl-newsletter` binary.
2. Run migrations with the target environment config (`NEWSLETTER_ENV=<env> ... migrate`).
3. Restart staging first and verify health + `/newsletter/*` behavior.
4. Restart prod and verify the same checks.
5. Keep previous binary available for fast rollback.

## Boundaries

- Extensions are optional and independently deployable.
- `htmlctl promote` does not copy extension runtime config; extension routing remains environment-scoped operational state.
- Extension onboarding should be possible from docs/runbooks alone without requiring source-code inspection.
