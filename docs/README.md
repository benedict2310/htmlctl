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
- `docs/guides/newsletter-extension-hetzner.md` - newsletter extension install/verify/runbook for Ubuntu/Hetzner hosts.
- `docs/reference/docker-images.md` - Docker build targets, runtime defaults, and hardening notes.
- `docs/operations-manual-agent.md` - end-to-end operations runbook for agents (local, Docker, remote SSH, release lifecycle, domains, VPS).

## Extensions

- `docs/reference/extensions.md` - extension contract, compatibility expectations, and security baseline.
- `extensions/README.md` - catalog layout and extension boundaries.

## Operations Notes

- `docs/operations/domain-hardening.md` - domain rollback metadata preservation and same-domain concurrency locking behavior.
- Environment backends are environment-scoped runtime config managed with `htmlctl backend add/list/remove`; see `docs/technical-spec.md` and the optional backend-routing verification in `docs/guides/first-deploy-docker.md`.
- Environment auth policies are environment-scoped runtime config managed with `htmlctl authpolicy add/list/remove`; they are enforced by Caddy and are not copied by `promote`.
- `scripts/clean-dev-state.sh` - clean `.tmp` dev/runtime state safely when mixed ownership appears.

## Review Logs

- `docs/review-logs/` stores PI/code review artifacts.
- `docs/review-logs/E12-newsletter-extension-adoption-validation-2026-03-06.md` - pilot validation evidence, security observations, and follow-up backlog for newsletter extension adoption.
