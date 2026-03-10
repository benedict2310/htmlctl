# Documentation Index

## Product + Architecture

- `docs/prd.md`
- `docs/technical-spec.md`
- `docs/epics.md`

## Implementation Stories

- `docs/stories/` contains epic/story implementation plans and acceptance criteria.

## Deployment Guides

- `docs/guides/first-deploy-docker.md` - first deployment path using Docker images and SSH transport, including telemetry-ready local host binding (`127.0.0.1.nip.io`).
- `docs/guides/extensions-overview.md` - extension model overview and operator integration checks.
- `docs/guides/newsletter-extension-vps.md` - newsletter extension install/verify/runbook for Ubuntu VPS hosts.
- `docs/guides/telemetry-collector-extension-vps.md` - browser telemetry collector install/verify/runbook for Ubuntu VPS hosts.
- `docs/reference/docker-images.md` - Docker build targets, runtime defaults, and hardening notes.
- `docs/operations-manual-agent.md` - end-to-end operations runbook for agents (local, Docker, remote SSH, release lifecycle, domains, VPS).

## Extensions

- `docs/reference/extensions.md` - extension contract, compatibility expectations, and security baseline.
- `extensions/README.md` - catalog layout and extension boundaries.
- `htmlctl extension validate <extension-dir-or-manifest> [--remote --context <ctx>]` - validate manifest structure and compatibility against local `htmlctl` and optional remote `htmlservd`.

## Operations Notes

- `docs/operations/domain-hardening.md` - domain rollback metadata preservation and same-domain concurrency locking behavior.
- Environment backends are environment-scoped runtime config managed with `htmlctl backend add/list/remove`; origin-only upstreams are required, and failed reloads now roll backend mutations back instead of silently persisting stale intent.
- Environment auth policies are environment-scoped runtime config managed with `htmlctl authpolicy add/list/remove`; they are enforced by Caddy and are not copied by `promote`.
- `scripts/clean-dev-state.sh` - clean `.tmp` dev/runtime state safely when mixed ownership appears.

## Review Logs

- `docs/review-logs/` stores PI/code review artifacts.
- `docs/review-logs/E12-newsletter-extension-adoption-validation-2026-03-06.md` - pilot validation evidence, security observations, and follow-up backlog for newsletter extension adoption.
- `docs/review-logs/E12-extension-core-hardening-2026-03-08.md` - core extension-system hardening for backend rollback safety, upstream validation, and compatibility enforcement.
- `docs/review-logs/E12-newsletter-extension-hardening-2026-03-08.md` - newsletter extension hardening notes covering import, unsubscribe, campaign send tracking, and installer/config validation.
- `docs/review-logs/E12-telemetry-collector-extension-2026-03-09.md` - browser telemetry collector review findings, fixes, and staging/prod validation evidence.
