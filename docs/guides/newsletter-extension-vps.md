# Newsletter Extension on Ubuntu VPS

This runbook installs the official `htmlctl-newsletter` extension as two loopback-only systemd services (`staging`, `prod`) with isolated PostgreSQL credentials and databases.

## Scope

- Host model: one Linux host running `htmlservd`, `caddy`, PostgreSQL, and newsletter extension services.
- Extension assets source: `extensions/newsletter/ops/`.
- Public routing: configured through `htmlctl backend add` after service verification.

## 1. Build and Upload Extension Binary

From local workstation:

```bash
cd /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/newsletter/service
go test ./...
TARGET_HOST="user@host.example.com"
TARGET_ARCH_RAW="$(ssh "${TARGET_HOST}" 'uname -m')"
case "${TARGET_ARCH_RAW}" in
  x86_64) TARGET_GOARCH=amd64 ;;
  aarch64|arm64) TARGET_GOARCH=arm64 ;;
  *) echo "unsupported target architecture: ${TARGET_ARCH_RAW}" >&2; exit 1 ;;
esac
GOOS=linux GOARCH="${TARGET_GOARCH}" CGO_ENABLED=0 go build -o htmlctl-newsletter ./cmd/htmlctl-newsletter
```

Upload binary + ops assets:

```bash
scp htmlctl-newsletter "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/newsletter/ops/setup-newsletter-extension.sh "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/newsletter/ops/systemd/htmlctl-newsletter-staging.service "${TARGET_HOST}":/tmp/
scp /Users/bene/Dev-Source-NoBackup/htmlctl/extensions/newsletter/ops/systemd/htmlctl-newsletter-prod.service "${TARGET_HOST}":/tmp/
```

## 2. Run Installer Script

```bash
ssh "${TARGET_HOST}" '
  export NEWSLETTER_BINARY_PATH=/tmp/htmlctl-newsletter
  export NEWSLETTER_STAGING_DB_PASSWORD=<staging-db-password>
  export NEWSLETTER_PROD_DB_PASSWORD=<prod-db-password>
  export NEWSLETTER_STAGING_RESEND_API_KEY=<staging-resend-key>
  export NEWSLETTER_PROD_RESEND_API_KEY=<prod-resend-key>
  export NEWSLETTER_STAGING_RESEND_FROM="Team <newsletter@staging.example.com>"
  export NEWSLETTER_PROD_RESEND_FROM="Team <newsletter@example.com>"
  export NEWSLETTER_STAGING_PUBLIC_BASE_URL=https://staging.example.com
  export NEWSLETTER_PROD_PUBLIC_BASE_URL=https://example.com
  export NEWSLETTER_STAGING_UNIT_PATH=/tmp/htmlctl-newsletter-staging.service
  export NEWSLETTER_PROD_UNIT_PATH=/tmp/htmlctl-newsletter-prod.service
  bash /tmp/setup-newsletter-extension.sh
'
```

Input constraints enforced by installer:
- `NEWSLETTER_STAGING_HTTP_ADDR` / `NEWSLETTER_PROD_HTTP_ADDR` must be loopback addresses (`localhost`, `127.x.x.x`, or `[::1]`).
- Public base URLs must be `https://` origins with no path, query, fragment, or userinfo.
- `NEWSLETTER_STAGING_RESEND_FROM` / `NEWSLETTER_PROD_RESEND_FROM` must be valid sender addresses.
- `NEWSLETTER_STAGING_LINK_SECRET` / `NEWSLETTER_PROD_LINK_SECRET` must be high-entropy secrets at least 32 characters long.
- DB passwords are URL-encoded before `NEWSLETTER_DATABASE_URL` is written.
- DB passwords and API keys must not contain whitespace or single quotes.

Installer results:
- Creates system user/group `htmlctl-newsletter`.
- Creates databases and roles (default names):
- `htmlctl_newsletter_staging`
- `htmlctl_newsletter_prod`
- Writes env files:
- `/etc/htmlctl-newsletter/staging.env`
- `/etc/htmlctl-newsletter/prod.env`
- Adds a generated `NEWSLETTER_LINK_SECRET` per environment when not supplied explicitly.
- Installs/enables/restarts:
- `htmlctl-newsletter-staging.service`
- `htmlctl-newsletter-prod.service`

## 3. Required Security and Isolation Verification

Service/process checks:

```bash
ssh "${TARGET_HOST}" "sudo systemctl status htmlctl-newsletter-staging --no-pager"
ssh "${TARGET_HOST}" "sudo systemctl status htmlctl-newsletter-prod --no-pager"
ssh "${TARGET_HOST}" "curl -sf http://127.0.0.1:9501/healthz"
ssh "${TARGET_HOST}" "curl -sf http://127.0.0.1:9502/healthz"
ssh "${TARGET_HOST}" "sudo ss -tlnp | grep ':9501\\|:9502'"
```

Expected:
- Both units are `active (running)`.
- Both listeners bind to loopback only (`127.0.0.1` or `::1`), never `0.0.0.0`.

Env permission checks:

```bash
ssh "${TARGET_HOST}" "stat -c '%a %U %G %n' /etc/htmlctl-newsletter/staging.env /etc/htmlctl-newsletter/prod.env"
```

Expected: `640 root htmlctl-newsletter`.

DB isolation checks:

```bash
ssh "${TARGET_HOST}" "sudo -u postgres psql -Atqc \"SELECT has_database_privilege('htmlctl_newsletter_staging','htmlctl_newsletter_prod','CONNECT');\""
ssh "${TARGET_HOST}" "sudo -u postgres psql -Atqc \"SELECT has_database_privilege('htmlctl_newsletter_prod','htmlctl_newsletter_staging','CONNECT');\""
```

Expected: both return `f`.

## 4. Pilot Backend Validation (Staging First)

The current runtime serves real signup, verification, and unsubscribe flows. Probe `/newsletter/verify` and `/newsletter/unsubscribe` with expected safe failures when no token is provided.
Preflight compatibility gate:

```bash
htmlctl extension validate extensions/newsletter --remote --context staging
```

Add staging backend:

```bash
htmlctl backend add website/<site> --env staging --path /newsletter/* --upstream http://127.0.0.1:9501
htmlctl backend list website/<site> --env staging
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/unsubscribe
```

Expected status code without tokens: `400`.

Failure-mode checks (required):

```bash
ssh "${TARGET_HOST}" "sudo systemctl stop htmlctl-newsletter-staging"
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify
ssh "${TARGET_HOST}" "sudo systemctl start htmlctl-newsletter-staging"

htmlctl backend remove website/<site> --env staging --path /newsletter/*
htmlctl backend add website/<site> --env staging --path /newsletter/* --upstream http://127.0.0.1:9599
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify

htmlctl backend remove website/<site> --env staging --path /newsletter/*
htmlctl backend add website/<site> --env staging --path /newsletter/* --upstream http://127.0.0.1:9501
```

Expected during outage/wrong upstream checks: `502` (or equivalent upstream-unreachable gateway failure).
Expected during backend mutation failure: `htmlctl backend add/remove` should fail and roll back instead of silently leaving stale backend state behind.

## 5. Production Cutover Checklist

Only add prod routing after all staging checks are green.

Checklist:
- `htmlctl extension validate extensions/newsletter --remote --context prod` passes
- prod unit healthy on loopback and no public listener
- prod env file permissions and DB isolation checks pass
- staging backend validation and failure drills completed
- rollback command prepared and verified

Cutover:

```bash
htmlctl backend add website/<site> --env prod --path /newsletter/* --upstream http://127.0.0.1:9502
htmlctl backend list website/<site> --env prod
curl -s -o /dev/null -w '%{http_code}\n' https://example.com/newsletter/verify
```

Rollback:

```bash
htmlctl backend remove website/<site> --env prod --path /newsletter/*
htmlctl backend list website/<site> --env prod
```

## 6. Upgrade and Binary Rollback

Upgrade sequence:

1. Build/install new binary to `/usr/local/bin/htmlctl-newsletter`.
2. Run migration per environment (`NEWSLETTER_ENV=<env> ... htmlctl-newsletter migrate`).
3. Restart staging and verify health + public route behavior.
4. Validate campaign preview and unsubscribe flow on staging.
5. Restart prod and verify the same checks.

Commands:

```bash
ssh "${TARGET_HOST}" "sudo systemctl restart htmlctl-newsletter-staging"
ssh "${TARGET_HOST}" "sudo systemctl restart htmlctl-newsletter-prod"
```

Binary rollback:

1. Restore previous binary in `/usr/local/bin/htmlctl-newsletter`.
2. Restart affected unit(s).
3. Re-check `/healthz` and `/newsletter` route behavior.

Logs:

```bash
ssh "${TARGET_HOST}" "sudo journalctl -u htmlctl-newsletter-staging -n 200 --no-pager"
ssh "${TARGET_HOST}" "sudo journalctl -u htmlctl-newsletter-prod -n 200 --no-pager"
```

## 7. Campaign Operator Workflow

Store campaign content:

```bash
ssh "${TARGET_HOST}" 'sudo python3 - <<\"PY\"\nimport os, subprocess\nfor env_path in (\"/etc/htmlctl-newsletter/staging.env\",):\n    env = os.environ.copy()\n    with open(env_path, \"r\", encoding=\"utf-8\") as f:\n        for line in f:\n            line = line.strip()\n            if not line or line.startswith(\"#\") or \"=\" not in line:\n                continue\n            k, v = line.split(\"=\", 1)\n            env[k] = v\n    subprocess.run([\n        \"/usr/local/bin/htmlctl-newsletter\", \"campaign\", \"upsert\",\n        \"--slug\", \"launch\",\n        \"--subject\", \"Launch update\",\n        \"--html-file\", \"/srv/newsletter/launch.html\",\n        \"--text-file\", \"/srv/newsletter/launch.txt\",\n    ], check=True, env=env)\nPY'
```

Preview send from staging:

```bash
ssh "${TARGET_HOST}" 'sudo python3 - <<\"PY\"\nimport os, subprocess\nenv = os.environ.copy()\nwith open(\"/etc/htmlctl-newsletter/staging.env\", \"r\", encoding=\"utf-8\") as f:\n    for line in f:\n        line = line.strip()\n        if not line or line.startswith(\"#\") or \"=\" not in line:\n            continue\n        k, v = line.split(\"=\", 1)\n        env[k] = v\nsubprocess.run([\n    \"/usr/local/bin/htmlctl-newsletter\", \"campaign\", \"preview\",\n    \"--slug\", \"launch\",\n    \"--to\", \"you@example.com\",\n], check=True, env=env)\nPY'
```

Full send with low-tier Resend pacing:

```bash
ssh "${TARGET_HOST}" 'sudo python3 - <<\"PY\"\nimport os, subprocess\nenv = os.environ.copy()\nwith open(\"/etc/htmlctl-newsletter/prod.env\", \"r\", encoding=\"utf-8\") as f:\n    for line in f:\n        line = line.strip()\n        if not line or line.startswith(\"#\") or \"=\" not in line:\n            continue\n        k, v = line.split(\"=\", 1)\n        env[k] = v\nsubprocess.run([\n    \"/usr/local/bin/htmlctl-newsletter\", \"campaign\", \"send\",\n    \"--slug\", \"launch\",\n    \"--mode\", \"all\",\n    \"--interval\", \"30s\",\n    \"--confirm\",\n], check=True, env=env)\nPY'
```

## 8. Troubleshooting

`502` on `/newsletter/*`:
- check backend mappings (`htmlctl backend list ...`)
- check local listener and health endpoint on expected port
- check unit status/logs for migration/config failures

Unexpected `404` on `/newsletter/*`:
- verify Caddy is routing `/newsletter/*` to newsletter upstream
- verify upstream service build contains expected handlers

DB auth failures:
- confirm env `NEWSLETTER_DATABASE_URL` role/db pairing per environment
- re-run installer with corrected DB passwords
- re-check `has_database_privilege(...)` isolation queries
