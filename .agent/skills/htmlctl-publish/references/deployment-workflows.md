# htmlctl Deployment Workflows

## Workflow 1 — Local Render & Validate (no server)

Goal: validate site structure and preview rendered output locally.

```bash
htmlctl render -f ./site -o ./dist
htmlctl serve ./dist --port 8080
```

Open `http://localhost:8080/`. Check:
- `dist/index.html` exists
- Each page has `dist/<route>/index.html`
- No validator errors (single root tag, no script tags in components, anchor id rules)
- If `website.yaml` enables website-level features, verify generated artifacts as applicable:
  - favicon: `dist/favicon.svg`, `dist/favicon.ico`
  - robots: `dist/robots.txt`
  - sitemap: `dist/sitemap.xml`
  - llms: `dist/llms.txt`
  - structured data: page HTML includes website-level `Organization` + `WebSite` JSON-LD before page-level JSON-LD

---

## Workflow 2 — Local Docker (full e2e with SSH tunnel)

Goal: full local end-to-end with SSH tunnel, release activation, and Caddy serving.

### Build images

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
docker build --target htmlctl -t htmlctl:local .          # optional
```

### Start daemon container

```bash
API_TOKEN="$(htmlctl context token generate)"
mkdir -p .tmp/first-deploy/{data,caddy}
docker rm -f htmlservd-first-deploy >/dev/null 2>&1 || true

docker run -d \
  --name htmlservd-first-deploy \
  -p 23222:22 \
  -p 19420:9400 \
  -p 18080:80 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -e HTMLSERVD_CADDY_BOOTSTRAP_MODE=preview \
  -e HTMLSERVD_PREVIEW_WEBSITE=sample \
  -e HTMLSERVD_PREVIEW_ENV=staging \
  -e HTMLSERVD_API_TOKEN="$API_TOKEN" \
  -e HTMLSERVD_CADDY_AUTO_HTTPS=false \
  -e HTMLSERVD_TELEMETRY_ENABLED=true \
  -v "$PWD/.tmp/first-deploy/data:/var/lib/htmlservd" \
  -v "$PWD/.tmp/first-deploy/caddy:/etc/caddy" \
  htmlservd-ssh:local
```

### Health check + trust host key

```bash
curl -sf http://127.0.0.1:19420/healthz
curl -sf http://127.0.0.1:19420/readyz
ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/first-deploy/known_hosts
```

### Create context config

```bash
cat > .tmp/first-deploy/htmlctl-config.yaml <<YAML
apiVersion: htmlctl.dev/v1
current-context: local-staging
contexts:
  - name: local-staging
    server: ssh://htmlservd@127.0.0.1:23222
    website: sample
    environment: staging
    port: 9400
    token: ${API_TOKEN}
YAML
```

### Apply + verify

```bash
ssh-add ~/.ssh/id_ed25519   # ensure key is in agent

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl apply -f ./site --context local-staging

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl apply --from-git /path/to/repo --ref <commit-sha> --context local-staging

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl apply --from-git /path/to/repo --ref <commit-sha> --subdir site --context local-staging --dry-run

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl domain add 127.0.0.1.nip.io --context local-staging
```

Git-input notes:
- `htmlctl` resolves the repository locally with the system `git` binary, then uploads the normal tar bundle. `htmlservd` never clones repositories.
- Use a pinned commit SHA for `--ref`; branch and symbolic refs are rejected.
- `htmlctl logs ... -o json` includes Git provenance (`source.repo`, `source.ref`, optional `source.subdir`) for Git-backed applies.

Open `http://127.0.0.1.nip.io:18080/`.

> Caddy uses virtual hosting. `curl http://127.0.0.1:18080/` returns empty because `Host: 127.0.0.1` matches no vhost. Always use the bound hostname.

If you are testing a newly added website-level server capability, rebuild the local image first:

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
```

### Verify telemetry

```bash
curl -sS \
  -H "Authorization: Bearer ${API_TOKEN}" \
  "http://127.0.0.1:19420/api/v1/websites/sample/environments/staging/telemetry/events?limit=20"
```

### Verify generated website artifacts

```bash
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/robots.txt
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/sitemap.xml
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/favicon.svg
```


### Optional: verify browser telemetry collector routing

Use this when site JavaScript should post browser telemetry through the official telemetry collector extension.

```bash
(
  export TELEMETRY_COLLECTOR_ENV=staging
  export TELEMETRY_COLLECTOR_HTTP_ADDR=127.0.0.1:19601
  export TELEMETRY_COLLECTOR_PUBLIC_BASE_URL=http://127.0.0.1.nip.io:18080
  export TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL=http://host.docker.internal:19420
  export TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN="${API_TOKEN}"
  cd extensions/telemetry-collector/service
  go run ./cmd/htmlctl-telemetry-collector serve
) &
COLLECTOR_PID=$!
trap 'kill ${COLLECTOR_PID}' EXIT

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl backend add website/sample \
  --env staging \
  --path /site-telemetry/* \
  --upstream http://host.docker.internal:19601 \
  --context local-staging

curl -i \
  -H 'Host: 127.0.0.1.nip.io:18080' \
  -H 'Origin: http://127.0.0.1.nip.io:18080' \
  -H 'Content-Type: application/json' \
  --data '{"events":[{"name":"page_view","path":"/"}]}' \
  http://127.0.0.1:18080/site-telemetry/v1/events

curl -sS \
  -H "Authorization: Bearer ${API_TOKEN}" \
  "http://127.0.0.1:19420/api/v1/websites/sample/environments/staging/telemetry/events?limit=20"

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl backend remove website/sample --env staging --path /site-telemetry/* --context local-staging
```

A valid event should return `202` and then appear in the telemetry API for the staging environment.

### Optional: verify environment backend routing

Backends are environment-scoped runtime config. They are not part of `site/` and are not copied by `promote`.

```bash
mkdir -p .tmp/first-deploy/backend/api
printf 'backend-ok\n' > .tmp/first-deploy/backend/api/ping
python3 -m http.server 18081 --bind 127.0.0.1 --directory .tmp/first-deploy/backend

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl backend add website/sample \
  --env staging \
  --path /api/* \
  --upstream http://host.docker.internal:18081 \
  --context local-staging

curl -sf http://127.0.0.1.nip.io:18080/api/ping

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl backend list website/sample --env staging --context local-staging

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl backend remove website/sample --env staging --path /api/* --context local-staging
```

### Optional: verify environment auth policy routing

Auth policies are environment-scoped runtime config. They are not part of `site/` and are not copied by `promote`.

```bash
printf 'docs-secret\n' | \
HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl authpolicy add website/sample \
  --env staging \
  --path /docs/* \
  --username reviewer \
  --password-stdin \
  --context local-staging

curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1.nip.io:18080/docs/
curl -sf -u reviewer:docs-secret http://127.0.0.1.nip.io:18080/docs/

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl authpolicy list website/sample --env staging --context local-staging

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl authpolicy remove website/sample --env staging --path /docs/* --context local-staging
```

### Run htmlctl inside Docker (key-file auth, no agent required)

```bash
# Config for Docker-internal networking
cat > .tmp/first-deploy/htmlctl-config-container.yaml <<YAML
apiVersion: htmlctl.dev/v1
current-context: local-staging
contexts:
  - name: local-staging
    server: ssh://htmlservd@host.docker.internal:23222
    website: sample
    environment: staging
    port: 9400
    token: ${API_TOKEN}
YAML

docker run --rm \
  -e HTMLCTL_CONFIG=/work/.tmp/first-deploy/htmlctl-config-container.yaml \
  -e HTMLCTL_SSH_KNOWN_HOSTS_PATH=/home/htmlctl/.ssh/known_hosts \
  -v "$PWD:/work" \
  -v "$HOME/.ssh/id_ed25519:/home/htmlctl/.ssh/id_ed25519:ro" \
  -v "$PWD/.tmp/first-deploy/known_hosts:/home/htmlctl/.ssh/known_hosts:ro" \
  -w /work \
  htmlctl:local status website/sample --context local-staging
```

### Cleanup

```bash
docker rm -f htmlservd-first-deploy
```

---

## Workflow 3 — VPS Native (systemd)

Goal: production-grade VPS deployment with native binaries, systemd, and SSH-tunneled control.

### 1. Create service user and directories

```bash
sudo useradd --system --home /var/lib/htmlservd --shell /usr/sbin/nologin htmlservd || true
sudo mkdir -p /var/lib/htmlservd /etc/htmlservd /etc/caddy
sudo chown -R htmlservd:htmlservd /var/lib/htmlservd /etc/caddy
```

### 2. Install binaries

```bash
# Build from source
git clone https://github.com/benedict2310/htmlctl && cd htmlctl
go build -o bin/htmlservd ./cmd/htmlservd
go build -o bin/htmlctl  ./cmd/htmlctl

sudo install -m 0755 ./bin/htmlservd /usr/local/bin/htmlservd
sudo install -m 0755 ./bin/htmlctl   /usr/local/bin/htmlctl
sudo install -m 0755 /path/to/caddy  /usr/local/bin/caddy
```

When server-side release materialization features change, upgrade `htmlservd` before applying a site that depends on them. A new client alone is not enough.

### 3. Create config

```yaml
# /etc/htmlservd/config.yaml
bind: 127.0.0.1
port: 9400
dataDir: /var/lib/htmlservd
logLevel: info
dbWAL: true
caddyBinaryPath: /usr/local/bin/caddy
caddyfilePath: /etc/caddy/Caddyfile
caddyConfigBackupPath: /etc/caddy/Caddyfile.bak
caddyAutoHTTPS: true
api:
  token: "YOUR_SHARED_API_TOKEN"
```

### 4. Create systemd unit

```ini
# /etc/systemd/system/htmlservd.service
[Unit]
Description=htmlservd
After=network.target

[Service]
User=htmlservd
Group=htmlservd
ExecStart=/usr/local/bin/htmlservd --config /etc/htmlservd/config.yaml --require-auth
Restart=always
RestartSec=2
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

### 5. Start and verify

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now htmlservd
sudo systemctl status htmlservd --no-pager
curl -sf http://127.0.0.1:9400/healthz
curl -sf http://127.0.0.1:9400/readyz
```

### 6. Prepare workstation config

```yaml
# ~/.htmlctl/config.yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://deploy@YOUR_HOST
    website: sample
    environment: staging
    port: 9400
    token: YOUR_SHARED_API_TOKEN
  - name: prod
    server: ssh://deploy@YOUR_HOST
    website: sample
    environment: prod
    port: 9400
    token: YOUR_SHARED_API_TOKEN
```

The SSH user must be able to open a tunnel to `127.0.0.1:9400` on the server.

### 7. Run remote workflow

```bash
htmlctl diff -f ./site --context staging
htmlctl apply -f ./site --context staging
htmlctl status website/sample --context staging
curl -sf https://staging.example.com/robots.txt
curl -sf https://staging.example.com/sitemap.xml
htmlctl backend add website/sample --env staging --path /api/* --upstream https://staging-api.example.com --context staging
htmlctl domain add example.com --context prod
htmlctl promote website/sample --from staging --to prod
htmlctl backend add website/sample --env prod --path /api/* --upstream https://api.example.com --context prod
```

If prod needs the same backend prefix, declare it explicitly for prod. Promotion does not carry backend state across environments.

---

## Workflow 3A — Extension Adoption (Newsletter Service)

Goal: validate and cut over a dynamic extension path (`/newsletter/*`) safely.

### 1. Verify extension runtime invariants on host

```bash
ssh <host> "sudo systemctl status htmlctl-newsletter-staging --no-pager"
ssh <host> "curl -sf http://127.0.0.1:9501/healthz"
ssh <host> "sudo ss -tlnp | grep ':9501'"
htmlctl extension validate extensions/newsletter --remote --context staging
```

Checks:
- unit active
- loopback-only listener
- health endpoint responds
- manifest compatibility validated against the target `htmlservd`

### 2. Add staging backend and validate route

```bash
htmlctl backend add website/sample --env staging --path /newsletter/* --upstream http://127.0.0.1:9501 --context staging
htmlctl backend list website/sample --env staging --context staging
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/unsubscribe
```

Expected safe probe result: HTTP `400` from `/newsletter/verify` and `/newsletter/unsubscribe` when the route is correct but no token is supplied.
Note: backend path `/newsletter/*` routes subpaths, not the bare `/newsletter` path.

### 3. Run controlled failure drills

```bash
ssh <host> "sudo systemctl stop htmlctl-newsletter-staging"
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify
ssh <host> "sudo systemctl start htmlctl-newsletter-staging"

htmlctl backend remove website/sample --env staging --path /newsletter/* --context staging
htmlctl backend add website/sample --env staging --path /newsletter/* --upstream http://127.0.0.1:9599 --context staging
curl -s -o /dev/null -w '%{http_code}\n' https://staging.example.com/newsletter/verify
```

Expected during outage/wrong upstream: gateway failure (typically `502`).

### 4. Restore staging mapping and cut over prod

```bash
htmlctl backend remove website/sample --env staging --path /newsletter/* --context staging
htmlctl backend add website/sample --env staging --path /newsletter/* --upstream http://127.0.0.1:9501 --context staging

htmlctl extension validate extensions/newsletter --remote --context prod
htmlctl backend add website/sample --env prod --path /newsletter/* --upstream http://127.0.0.1:9502 --context prod
htmlctl backend list website/sample --env prod --context prod
curl -s -o /dev/null -w '%{http_code}\n' https://example.com/newsletter/verify
```

Rollback command:

```bash
htmlctl backend remove website/sample --env prod --path /newsletter/* --context prod
```

---

## Workflow 4 — VPS via Docker

```bash
docker run -d \
  --name htmlservd \
  --restart unless-stopped \
  -p 22:22 \
  -p 9400:9400 \
  -p 80:80 \
  -p 443:443 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -e HTMLSERVD_API_TOKEN="YOUR_SHARED_API_TOKEN" \
  -v /srv/htmlservd/data:/var/lib/htmlservd \
  -v /srv/htmlservd/caddy:/etc/caddy \
  htmlservd-ssh:local
```

For no-TLS local/dev environment, add `-e HTMLSERVD_CADDY_AUTO_HTTPS=false`.

---

## Release Lifecycle Reference

```bash
# Daily staging cycle
htmlctl config current-context
htmlctl diff -f ./site --context staging
htmlctl apply -f ./site --context staging
htmlctl status website/sample --context staging
htmlctl logs website/sample --context staging --limit 50

# Release history
htmlctl rollout history website/sample --context staging
htmlctl get releases --context staging

# Rollback
htmlctl rollout undo website/sample --context staging

# Promote staging → prod (exact artifact bytes, no rebuild)
htmlctl promote website/sample --from staging --to prod --context staging
htmlctl rollout history website/sample --context prod
htmlctl status website/sample --context prod
htmlctl logs website/sample --context prod --limit 20
```

---

## Data Paths and Backup

Server state root: `/var/lib/htmlservd`

Critical data:
- `db.sqlite` — desired state, release metadata, domains, audit log
- `blobs/sha256/*` — content-addressed file blobs
- `websites/*/envs/*/releases/*` — immutable rendered release directories
- `websites/*/envs/*/current` — active release symlinks

Backup strategy: snapshot the full data directory before high-risk operations. Preserve DB and release blobs together.
