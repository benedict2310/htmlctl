# E12-S3 — Newsletter Extension Installer and Ops Runbook

**Epic:** Epic 12 — Optional Service Extensions
**Status:** Implemented (2026-03-06)
**Priority:** P1
**Estimated Effort:** 2-3 days
**Dependencies:** E12-S2
**Target:** `extensions/newsletter/ops/`, `docs/guides/`
**Design Reference:** FutureLab Hetzner rollout log (2026-03-05)

---

## 1. Summary

Ship reproducible installer tooling and operator runbooks for the newsletter extension so operators can deploy staging/prod service instances safely on a single host.

## 2. Architecture Context and Reuse Guidance

- Reuse hardened systemd patterns already used for `htmlservd` setup (least privilege user, no shell access assumptions, strict unit hardening).
- Reuse `htmlctl backend add/list/remove` as the documented public routing integration step.
- Keep secrets out of repo; provide only `.env.example` templates.

## 3. Proposed Changes and Architecture Improvements

### 3.1 Installer script

Provide a host bootstrap script that:

- installs service binary
- creates service user/group
- provisions staging/prod PostgreSQL roles + DBs
- writes env files with secure permissions
- installs/enables systemd units

### 3.2 Ops assets

Ship:

- staging/prod systemd unit templates
- env template examples
- rollback and restart commands
- verification command set (status, listeners, health, DB checks)

### 3.3 Security and isolation checks

Include explicit post-install checks for:

- loopback-only listeners
- env-file permission modes
- staging role cannot connect to prod DB and vice versa

## 4. File Touch List

### Files to Create

- `extensions/newsletter/ops/setup-newsletter-extension.sh`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-staging.service`
- `extensions/newsletter/ops/systemd/htmlctl-newsletter-prod.service`
- `extensions/newsletter/ops/env/staging.env.example`
- `extensions/newsletter/ops/env/prod.env.example`
- `docs/guides/newsletter-extension-hetzner.md`

### Files to Modify

- `docs/reference/extensions.md` — install/verify sections

## 5. Implementation Steps

1. Add installer script with idempotent user/DB/unit setup.
2. Add env/systemd templates and strict permission defaults.
3. Add Hetzner-focused runbook and generic Linux notes.
4. Add troubleshooting for common misconfigurations (`502`, wrong backend path, wrong DB role).

## 6. Tests and Validation

### Automated

- shellcheck + `bash -n` for installer script
- unit-file lint where available

### Manual

- deploy to test host and verify:
  - systemd services active
  - listeners on loopback only
  - `/healthz` returns `200`
  - staging/prod DB isolation

## 7. Acceptance Criteria

- [x] AC-1: Installer script can bootstrap staging/prod newsletter instances on a clean host.
- [x] AC-2: Systemd units and env templates are provided with security-focused defaults.
- [x] AC-3: Runbook includes explicit verification commands for all foundation invariants.
- [x] AC-4: Runbook includes rollback/restart/troubleshooting procedures.
- [x] AC-5: No credentials or secrets are committed in extension assets.

## 8. Risks and Open Questions

### Risks

- **Risk:** installer assumptions differ across distributions.  
  **Mitigation:** document supported baseline (Ubuntu 24.04 first), add distro caveats.
- **Risk:** operators skip verification and run with public listeners.  
  **Mitigation:** make listener verification a required checklist step before backend cutover.

### Open Questions

- Should a future release provide container-native installer assets in addition to systemd-first host setup?

## 9. Implementation Notes (2026-03-06)

- Added installer and ops assets under `extensions/newsletter/ops`:
  - `setup-newsletter-extension.sh`
  - `systemd/htmlctl-newsletter-staging.service`
  - `systemd/htmlctl-newsletter-prod.service`
  - `env/staging.env.example`
  - `env/prod.env.example`
- Installer behavior:
  - idempotent user/group creation (`htmlctl-newsletter`)
  - idempotent PostgreSQL role/database setup for staging/prod
  - secure env file creation (`/etc/htmlctl-newsletter/*.env`, mode `640`)
  - systemd unit install, enable, restart
  - printed post-install verification commands
- Added runbook:
  - `docs/guides/newsletter-extension-hetzner.md`
  - includes install, verification, backend wiring, restart/rollback, troubleshooting
- Updated extension reference docs:
  - `docs/reference/extensions.md`
  - `docs/guides/extensions-overview.md`
  - `extensions/newsletter/README.md`
  - `extensions/newsletter/CHANGELOG.md`
- Verification evidence:
  - `bash -n extensions/newsletter/ops/setup-newsletter-extension.sh`
  - independent `codex review` loops with all findings fixed
  - Docker E2E apply + domain routing check (`E2E_OK`)
  - Re-validated in containerized checks (2026-03-06):
    - `bash -n extensions/newsletter/ops/setup-newsletter-extension.sh` inside `golang:1.24-bookworm`
