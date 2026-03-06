# E12 Newsletter Extension Adoption Validation (2026-03-06)

## Scope

Pilot-validate newsletter extension adoption workflow and document operator-ready procedures for extension install, backend routing, failure handling, and rollback.

Adopter track:
- FutureLab-style deployment model (staging/prod with environment backends)
- Local Docker validation environment used for reproducible command evidence

## Validation Environment

- Repo: `/Users/bene/Dev-Source-NoBackup/htmlctl`
- Date: 2026-03-06
- Images built: `htmlservd-ssh:e12s4`, `htmlctl:e12s4`
- Local server container: `htmlservd-e12s4`
- Mock newsletter upstream container: `newsletter-mock-e12s4`

## Executed Checklist and Evidence

1. Core health/readiness gates
- `curl -sf http://127.0.0.1:19524/healthz`
- `curl -sf http://127.0.0.1:19524/readyz`
- Result: pass

2. End-to-end deploy baseline
- `go run ./cmd/htmlctl apply -f testdata/valid-site --context local-staging`
- `go run ./cmd/htmlctl status website/sample --context local-staging`
- `go run ./cmd/htmlctl domain add 127.0.0.1.nip.io --context local-staging`
- `curl -sf -H 'Host: 127.0.0.1.nip.io' http://127.0.0.1:18184/ | grep 'Sample'`
- Result: pass

3. Newsletter backend routing smoke (add/list/remove)
- `go run ./cmd/htmlctl backend add website/sample --env staging --path '/newsletter/*' --upstream http://newsletter-mock-e12s4:8080 --context local-staging`
- `go run ./cmd/htmlctl backend list website/sample --env staging --context local-staging`
- `curl -sf http://127.0.0.1.nip.io:18184/newsletter/ping | grep 'newsletter-route-ok'`
- `go run ./cmd/htmlctl backend remove website/sample --env staging --path '/newsletter/*' --context local-staging`
- Result: pass

4. Failure mode validation (wrong upstream mapping)
- `go run ./cmd/htmlctl backend add website/sample --env staging --path '/newsletter/*' --upstream http://newsletter-mock-e12s4:8099 --context local-staging`
- `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1.nip.io:18184/newsletter/ping`
- Observed status: `502`
- `go run ./cmd/htmlctl backend remove website/sample --env staging --path '/newsletter/*' --context local-staging`
- Result: pass

## Security Observations

- Backend routes remain environment-scoped runtime state and are not copied by `promote`; this reduces accidental prod exposure from staging-only experiments.
- Controlled failure drills (`502` verification + explicit backend remove) are required and now documented in operator/skill workflows.
- Installer/unit consistency issue found during independent review: custom `NEWSLETTER_BIN_PATH` could diverge from hardcoded unit `ExecStart` path. Fixed by rendering unit files with the configured binary path at install time.
- Service config and migration hardening done during this story gate:
  - `NEWSLETTER_HTTP_ADDR` now enforces numeric port range `1..65535`.
  - SQL migration splitting is now SQL-aware for quoted/commented/dollar-quoted content.

## Follow-up Backlog Items

1. Add a dedicated `newsletter` extension smoke script under `extensions/newsletter/ops/` to automate the add/list/remove + failure drill sequence.
2. Add CI for extension docs/runbook command drift checks (link + command lint where feasible).
3. Consider a future `htmlctl extension doctor` command for extension compatibility and health checks.

## Conclusion

Pilot workflow is validated for extension adoption readiness:
- install/verify/routing/failure/rollback procedures are documented
- skill references are extension-aware
- reproducible command evidence confirms backend integration behavior
