# Telemetry Collector Extension on Ubuntu VPS

This runbook installs the official `htmlctl-telemetry-collector` extension as two loopback-only systemd services (`staging`, `prod`) and routes public browser telemetry through `/site-telemetry/*` without exposing the htmlservd bearer token to browsers.

## Scope

- Host model: one Linux host running `htmlservd`, `caddy`, and telemetry collector extension services.
- Extension assets source: `extensions/telemetry-collector/ops/`.
- Public routing: configured through `htmlctl backend add` after service verification.
- Core telemetry sink remains `POST /collect/v1/events` inside `htmlservd` and stays bearer-authenticated.

## 1. Build and Upload Extension Binary

From local workstation:

```bash
cd /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/telemetry-collector/service
go test ./...
TARGET_HOST="user@host.example.com"
TARGET_ARCH_RAW="$(ssh "${TARGET_HOST}" 'uname -m')"
case "${TARGET_ARCH_RAW}" in
  x86_64) TARGET_GOARCH=amd64 ;;
  aarch64|arm64) TARGET_GOARCH=arm64 ;;
  *) echo "unsupported target architecture: ${TARGET_ARCH_RAW}" >&2; exit 1 ;;
esac
GOOS=linux GOARCH="${TARGET_GOARCH}" CGO_ENABLED=0 go build -o htmlctl-telemetry-collector ./cmd/htmlctl-telemetry-collector
```

Upload binary + ops assets:

```bash
scp htmlctl-telemetry-collector "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/telemetry-collector/ops/setup-telemetry-collector-extension.sh "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-staging.service "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/telemetry-collector/ops/systemd/htmlctl-telemetry-collector-prod.service "${TARGET_HOST}":/tmp/
```

## 2. Run Installer Script

```bash
ssh "${TARGET_HOST}" '
  export TELEMETRY_COLLECTOR_BINARY_PATH=/tmp/htmlctl-telemetry-collector
  export TELEMETRY_COLLECTOR_STAGING_PUBLIC_BASE_URL=https://staging.example.com
  export TELEMETRY_COLLECTOR_PROD_PUBLIC_BASE_URL=https://example.com
  export TELEMETRY_COLLECTOR_STAGING_HTMLSERVD_TOKEN=<staging-htmlservd-token>
  export TELEMETRY_COLLECTOR_PROD_HTMLSERVD_TOKEN=<prod-htmlservd-token>
  export TELEMETRY_COLLECTOR_STAGING_UNIT_PATH=/tmp/htmlctl-telemetry-collector-staging.service
  export TELEMETRY_COLLECTOR_PROD_UNIT_PATH=/tmp/htmlctl-telemetry-collector-prod.service
  bash /tmp/setup-telemetry-collector-extension.sh
'
```

Input constraints enforced by installer:
- `TELEMETRY_COLLECTOR_STAGING_HTTP_ADDR` / `TELEMETRY_COLLECTOR_PROD_HTTP_ADDR` must be loopback addresses.
- Public base URLs must be `https://` origins with no path, query, fragment, or userinfo.
- htmlservd base URLs must be `http://` loopback origins with no path, query, fragment, or userinfo.
- htmlservd tokens must not contain whitespace.
- body/event limits must be positive integers.

Installer results:
- Creates system user/group `htmlctl-telemetry`.
- Writes env files:
  - `/etc/htmlctl-telemetry-collector/staging.env`
  - `/etc/htmlctl-telemetry-collector/prod.env`
- Installs/enables/restarts:
  - `htmlctl-telemetry-collector-staging.service`
  - `htmlctl-telemetry-collector-prod.service`

## 3. Required Security and Isolation Verification

Service/process checks:

```bash
ssh "${TARGET_HOST}" "sudo systemctl status htmlctl-telemetry-collector-staging --no-pager"
ssh "${TARGET_HOST}" "sudo systemctl status htmlctl-telemetry-collector-prod --no-pager"
ssh "${TARGET_HOST}" "curl -sf http://127.0.0.1:9601/healthz"
ssh "${TARGET_HOST}" "curl -sf http://127.0.0.1:9602/healthz"
ssh "${TARGET_HOST}" "sudo ss -tlnp | grep ':9601\|:9602'"
```

Expected:
- Both units are `active (running)`.
- Both listeners bind to loopback only, never `0.0.0.0`.

Env permission checks:

```bash
ssh "${TARGET_HOST}" "stat -c '%a %U %G %n' /etc/htmlctl-telemetry-collector/staging.env /etc/htmlctl-telemetry-collector/prod.env"
```

Expected: `640 root htmlctl-telemetry`.

## 4. Staging Backend Validation

Preflight compatibility gate:

```bash
htmlctl extension validate extensions/telemetry-collector --remote --context staging
```

Add staging backend:

```bash
htmlctl backend add website/<site> --env staging --path /site-telemetry/* --upstream http://127.0.0.1:9601
htmlctl backend list website/<site> --env staging
```

Functional checks:

```bash
curl -i -X POST \
  -H 'Content-Type: application/json' \
  -H 'Origin: https://staging.example.com' \
  --data '{"events":[{"name":"page_view","path":"/"}]}' \
  https://staging.example.com/site-telemetry/v1/events
```

Expected: `202` after site JS is updated and the public host is bound in htmlservd.

Then query stored events:

```bash
curl -sS \
  -H 'Authorization: Bearer <htmlservd-token>' \
  'http://127.0.0.1:9400/api/v1/websites/<site>/environments/staging/telemetry/events?limit=20'
```

Expected: the posted event appears under the target website/environment.

Failure-mode checks (required):

```bash
ssh "${TARGET_HOST}" "sudo systemctl stop htmlctl-telemetry-collector-staging"
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/site-telemetry/v1/events
ssh "${TARGET_HOST}" "sudo systemctl start htmlctl-telemetry-collector-staging"

htmlctl backend remove website/<site> --env staging --path /site-telemetry/*
htmlctl backend add website/<site> --env staging --path /site-telemetry/* --upstream http://127.0.0.1:9699
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/site-telemetry/v1/events

htmlctl backend remove website/<site> --env staging --path /site-telemetry/*
htmlctl backend add website/<site> --env staging --path /site-telemetry/* --upstream http://127.0.0.1:9601
```

Expected during outage/wrong-upstream drills: `502` (or equivalent upstream-unreachable gateway failure).
Expected during backend mutation failure: `htmlctl backend add/remove` should fail and roll back instead of persisting stale backend state.

## 5. Production Cutover Checklist

Only add prod routing after all staging checks are green.

Checklist:
- `htmlctl extension validate extensions/telemetry-collector --remote --context prod` passes.
- prod unit healthy on loopback and no public listener.
- env file permissions are correct.
- staging backend validation and failure drills completed.
- site JavaScript posts to `/site-telemetry/v1/events`.
- telemetry events are queryable through htmlservd before broader analysis begins.

Cutover:

```bash
htmlctl backend add website/<site> --env prod --path /site-telemetry/* --upstream http://127.0.0.1:9602
htmlctl backend list website/<site> --env prod
```

Rollback:

```bash
htmlctl backend remove website/<site> --env prod --path /site-telemetry/*
htmlctl backend list website/<site> --env prod
```

## 6. Upgrade and Binary Rollback

Upgrade sequence:

1. Build/install new binary to `/usr/local/bin/htmlctl-telemetry-collector`.
2. Restart staging and verify health + `/site-telemetry/*` behavior.
3. Restart prod and verify the same checks.
4. Keep previous binary available for fast rollback.

Commands:

```bash
ssh "${TARGET_HOST}" "sudo systemctl restart htmlctl-telemetry-collector-staging"
ssh "${TARGET_HOST}" "sudo systemctl restart htmlctl-telemetry-collector-prod"
```

Logs:

```bash
ssh "${TARGET_HOST}" "sudo journalctl -u htmlctl-telemetry-collector-staging -n 200 --no-pager"
ssh "${TARGET_HOST}" "sudo journalctl -u htmlctl-telemetry-collector-prod -n 200 --no-pager"
```
