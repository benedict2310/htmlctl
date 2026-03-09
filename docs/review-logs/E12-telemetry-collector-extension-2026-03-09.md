# E12 Telemetry Collector Extension Review Log

Date: 2026-03-09
Scope: `extensions/telemetry-collector/`, extension docs/skills, Futurelab staging/prod adoption

## Review Summary

The implementation keeps the original telemetry trust boundary intact:

- browser traffic terminates at the collector extension, not directly at htmlservd
- htmlservd `POST /collect/v1/events` remains bearer-authenticated
- the collector preserves the validated public host when forwarding so htmlservd attributes events to the correct environment
- public ingest is narrowed to a specific event set and rate-limited
- upstream 5xx responses are sanitized before returning to browsers

## Review Inputs

- implementation diff inspection across service, ops, docs, and skill updates
- targeted automated checks:
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `bash -n extensions/telemetry-collector/ops/setup-telemetry-collector-extension.sh`
  - `git diff --check`
- local Docker e2e:
  - sample site published through htmlservd/Caddy
  - `/site-telemetry/*` backend added
  - collector run locally on loopback
  - public `202` ingest verified
  - stored-event query verified through htmlservd telemetry API
- Futurelab rollout checks:
  - staging collector unit healthy on `127.0.0.1:9601`
  - prod collector unit healthy on `127.0.0.1:9602`
  - staging `/site-telemetry/v1/events` returned `202` and stored event under `futurelab/staging`
  - prod `/site-telemetry/v1/events` returned `202` and stored event under `futurelab/prod`

## Review Findings Fixed During Implementation

1. Local Docker workflow needed `Host` with the non-default port when simulating same-origin requests against `127.0.0.1.nip.io`.
2. Futurelab host telemetry sink was still disabled in `/etc/htmlservd/config.yaml`, so the collector initially forwarded into a `404` sink. Enabled htmlservd telemetry before final staging/prod verification.
3. Futurelab site JS still posted to `/collect/v1/events`; updated it to `/site-telemetry/v1/events` before production rollout.

## codex review

`codex review --uncommitted` was run twice during the change. The local CLI inspected the worktree and entered review execution, but did not emit a final findings block before stalling. No concrete findings were produced by the tool in those runs.

Because of that tooling behavior, the final sign-off used:

- manual diff review
- full extension test/race/vet validation
- local Docker e2e verification
- Futurelab staging verification
- Futurelab production verification

## Result

No unresolved implementation or security issues remain from this review pass.
