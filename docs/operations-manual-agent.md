# htmlctl Operations Manual (Agent)

This document is the operational source of truth for `htmlctl` + `htmlservd` based on Epics E1-E5 (`docs/stories/`).

Status model:
- Parser/renderer/validator/local serve: implemented.
- Remote control plane over SSH tunnel: implemented.
- Immutable releases + history + rollback + promote: implemented.
- Domain bindings + Caddy config/reload + CLI domain operations: implemented.

Primary references:
- `docs/technical-spec.md`
- `docs/guides/first-deploy-docker.md`
- `docs/reference/docker-images.md`
- `docs/operations/domain-hardening.md`

## 1. Core Invariants

- All deploys are release-based and immutable.
- Active content is selected by atomic `current` pointer switch.
- `apply` updates desired state first; `releases` activate rendered artifacts.
- `promote` reuses exact artifact bytes from source environment.
- `rollout undo` moves active pointer to previous release.
- Domain operations update DB + regenerate Caddy + reload Caddy safely.

## 2. Required Inputs

- A valid site directory:
  - `website.yaml`
  - `pages/*.page.yaml`
  - `components/*.html`
  - `styles/tokens.css`
  - `styles/default.css`
  - optional `scripts/site.js`
  - optional `assets/**`
- SSH access to target server/container.
- `htmlctl` config (`HTMLCTL_CONFIG` or `~/.htmlctl/config.yaml`).

## 3. Environment Variables

`htmlctl`:
- `HTMLCTL_CONFIG`: config path.
- `HTMLCTL_SSH_KNOWN_HOSTS_PATH`: known_hosts override.
- `HTMLCTL_SSH_KEY_PATH`: private key fallback path when SSH agent is unavailable.

`htmlservd`:
- `HTMLSERVD_BIND`, `HTMLSERVD_PORT`, `HTMLSERVD_DATA_DIR`, `HTMLSERVD_LOG_LEVEL`
- `HTMLSERVD_DB_PATH`, `HTMLSERVD_DB_WAL`
- `HTMLSERVD_CADDY_BINARY`, `HTMLSERVD_CADDYFILE_PATH`, `HTMLSERVD_CADDY_CONFIG_BACKUP`
- `HTMLSERVD_CADDY_AUTO_HTTPS` (`true` default)

Docker entrypoint controls:
- `HTMLSERVD_CADDY_BOOTSTRAP_MODE` (`preview|bootstrap`, default `preview`)
- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` (default `:80`)
- `HTMLSERVD_PREVIEW_WEBSITE` (default `futurelab`)
- `HTMLSERVD_PREVIEW_ENV` (default `staging`)
- `HTMLSERVD_PREVIEW_ROOT` (explicit preview root override)

## 4. Canonical Site Skeleton

```text
site/
  website.yaml
  pages/
    index.page.yaml
  components/
    header.html
  styles/
    tokens.css
    default.css
  scripts/
    site.js
  assets/
    logo.svg
```

Minimum valid example:

```yaml
# website.yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: futurelab
spec:
  defaultStyleBundle: default
  baseTemplate: default
```

```yaml
# pages/index.page.yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Futurelab
  description: Landing page
  layout:
    - include: hero
```

```html
<!-- components/hero.html -->
<section id="hero">
  <h1>Futurelab</h1>
</section>
```

## 5. Runbook RB-LOCAL-01: Local Authoring Validation

Goal: validate local content and render deterministic output.

Commands:

```bash
htmlctl render -f ./site -o ./dist
htmlctl serve ./dist --port 8080
```

Checks:
- `/dist/index.html` exists.
- route pages exist at `<route>/index.html`.
- no validator errors (single root tag, script disallow, anchor id rules).

## 6. Runbook RB-DOCKER-01: First Website From Scratch (Local Docker)

Goal: full local e2e with SSH tunnel and release activation.

1. Build images.

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
docker build --target htmlctl -t htmlctl:local .
```

2. Start daemon container.

```bash
mkdir -p .tmp/first-deploy/{data,caddy,site}
docker network create htmlctl-net >/dev/null 2>&1 || true
docker rm -f htmlservd-first-deploy >/dev/null 2>&1 || true

docker run -d \
  --name htmlservd-first-deploy \
  --network htmlctl-net \
  -p 23222:22 \
  -p 19420:9400 \
  -p 18080:80 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -e HTMLSERVD_CADDY_BOOTSTRAP_MODE=preview \
  -e HTMLSERVD_PREVIEW_WEBSITE=futurelab \
  -e HTMLSERVD_PREVIEW_ENV=staging \
  -e HTMLSERVD_CADDY_AUTO_HTTPS=false \
  -v "$PWD/.tmp/first-deploy/data:/var/lib/htmlservd" \
  -v "$PWD/.tmp/first-deploy/caddy:/etc/caddy" \
  htmlservd-ssh:local
```

3. Health + host key.

```bash
curl -sf http://127.0.0.1:19420/healthz
ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/first-deploy/known_hosts
```

4. Configure context.

```bash
cat > .tmp/first-deploy/htmlctl-config.yaml <<'YAML'
apiVersion: htmlctl.dev/v1
current-context: local-staging
contexts:
  - name: local-staging
    server: ssh://htmlservd@127.0.0.1:23222
    website: futurelab
    environment: staging
    port: 9400
YAML
```

5. Apply.

```bash
ssh-add ~/.ssh/id_ed25519

HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl apply -f .tmp/first-deploy/site --context local-staging
```

6. Verify.

```bash
HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl status website/futurelab --context local-staging

open http://127.0.0.1:18080/
```

## 7. Runbook RB-REMOTE-01: Standard Remote Workflow (SSH Tunnel)

Goal: operate remote staging/prod from workstation.

1. Context file.

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://deploy@YOUR_HOST
    website: futurelab
    environment: staging
    port: 9400
  - name: prod
    server: ssh://deploy@YOUR_HOST
    website: futurelab
    environment: prod
    port: 9400
```

2. Daily cycle.

```bash
htmlctl config current-context
htmlctl diff -f ./site --context staging
htmlctl apply -f ./site --context staging
htmlctl status website/futurelab --context staging
htmlctl logs website/futurelab --context staging --limit 50
```

3. Dry-run-only.

```bash
htmlctl apply -f ./site --context staging --dry-run
```

## 8. Runbook RB-RELEASE-01: History, Rollback, Promote

Release history:

```bash
htmlctl rollout history website/futurelab --context staging
htmlctl get releases --context staging
```

Rollback previous active release:

```bash
htmlctl rollout undo website/futurelab --context staging
```

Promote active staging artifact to prod (no rebuild):

```bash
htmlctl promote website/futurelab --from staging --to prod --context staging
htmlctl rollout history website/futurelab --context prod
```

Post-promote verification:

```bash
htmlctl status website/futurelab --context prod
htmlctl logs website/futurelab --context prod --limit 20
```

## 9. Runbook RB-DOMAIN-01: Domain Binding + Verification

Add/list/remove:

```bash
htmlctl domain add futurelab.studio --context prod
htmlctl domain list --context prod
htmlctl domain remove futurelab.studio --context prod
```

Verify DNS + TLS:

```bash
htmlctl domain verify futurelab.studio --context prod
```

Expected behavior:
- DNS failure -> actionable output with A/AAAA guidance.
- TLS failure -> actionable output to check Caddy serving/cert issuance.
- success requires both DNS PASS and TLS PASS.

## 10. Runbook RB-VPS-01: Deploy htmlservd on VPS (Native Binaries)

Goal: production-grade VPS deployment with systemd and SSH-tunneled control.

1. Create service user + dirs.

```bash
sudo useradd --system --home /var/lib/htmlservd --shell /usr/sbin/nologin htmlservd || true
sudo mkdir -p /var/lib/htmlservd /etc/htmlservd /etc/caddy
sudo chown -R htmlservd:htmlservd /var/lib/htmlservd /etc/caddy
```

2. Install binaries (`htmlservd`, optional `htmlctl` for on-host diagnostics).

```bash
sudo install -m 0755 ./bin/htmlservd /usr/local/bin/htmlservd
sudo install -m 0755 ./bin/htmlctl /usr/local/bin/htmlctl
```

3. Install Caddy binary accessible by service.

```bash
sudo install -m 0755 /path/to/caddy /usr/local/bin/caddy
```

4. Create config `/etc/htmlservd/config.yaml`.

```yaml
bind: 127.0.0.1
port: 9400
dataDir: /var/lib/htmlservd
logLevel: info
dbWAL: true
caddyBinaryPath: /usr/local/bin/caddy
caddyfilePath: /etc/caddy/Caddyfile
caddyConfigBackupPath: /etc/caddy/Caddyfile.bak
caddyAutoHTTPS: true
```

5. Create systemd unit `/etc/systemd/system/htmlservd.service`.

```ini
[Unit]
Description=htmlservd
After=network.target

[Service]
User=htmlservd
Group=htmlservd
ExecStart=/usr/local/bin/htmlservd --config /etc/htmlservd/config.yaml
Restart=always
RestartSec=2
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

6. Start service.

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now htmlservd
sudo systemctl status htmlservd --no-pager
curl -sf http://127.0.0.1:9400/healthz
```

7. Prepare SSH access for deploy user from workstation:
- key-based auth only.
- user in `ssh://USER@HOST` must be able to open tunnel to `127.0.0.1:9400`.

8. Run remote workflow from workstation (RB-REMOTE-01).

## 11. Runbook RB-VPS-02: Deploy on VPS via Docker

Use `htmlservd-ssh` image with persistent volumes and mapped ports.

```bash
docker run -d \
  --name htmlservd \
  --restart unless-stopped \
  -p 22:22 \
  -p 9400:9400 \
  -p 80:80 \
  -p 443:443 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -v /srv/htmlservd/data:/var/lib/htmlservd \
  -v /srv/htmlservd/caddy:/etc/caddy \
  htmlservd-ssh:local
```

For local-like non-TLS dry environment, set `HTMLSERVD_CADDY_AUTO_HTTPS=false`.

## 12. API Reference (for Agent Direct Calls)

Health/version:
- `GET /healthz`
- `GET /version`

Website/environment operations:
- `GET /api/v1/websites`
- `GET /api/v1/websites/{website}/environments`
- `POST /api/v1/websites/{website}/environments/{env}/apply`
- `POST /api/v1/websites/{website}/environments/{env}/releases`
- `GET /api/v1/websites/{website}/environments/{env}/releases`
- `POST /api/v1/websites/{website}/environments/{env}/rollback`
- `POST /api/v1/websites/{website}/promote`
- `GET /api/v1/websites/{website}/environments/{env}/status`
- `GET /api/v1/websites/{website}/environments/{env}/manifest`
- `GET /api/v1/websites/{website}/environments/{env}/logs`

Domain operations:
- `GET /api/v1/domains`
- `POST /api/v1/domains`
- `GET /api/v1/domains/{domain}`
- `DELETE /api/v1/domains/{domain}`

## 13. Failure Modes and Deterministic Responses

- `ssh host key verification failed`:
  - regenerate known hosts with `ssh-keyscan`.
- `ssh agent unavailable`:
  - use `HTMLCTL_SSH_KEY_PATH` or mount key file (Docker).
- local Docker preview not updating:
  - confirm preview env vars (`HTMLSERVD_PREVIEW_WEBSITE`, `HTMLSERVD_PREVIEW_ENV`) match context.
- `domain verify` TLS fail in local/dev:
  - expected unless public DNS/TLS is valid; for local HTTP set `HTMLSERVD_CADDY_AUTO_HTTPS=false`.
- cleanup blocked by ownership in `.tmp`:
  - run `scripts/clean-dev-state.sh`.

## 14. Operational Safety Checklist

Before apply:
- run `htmlctl diff`.
- run `htmlctl apply --dry-run` for risky changes.
- confirm target context (`htmlctl config current-context`).

After apply:
- `htmlctl status`.
- `htmlctl logs`.
- external route check (HTTP/TLS).

Before promote:
- validate staging release behavior.
- verify release history and active release id.

After promote:
- verify prod status + external health.
- keep rollback command ready.

## 15. Data Paths and Backups

Default server state root:
- `/var/lib/htmlservd`

Critical data:
- `db.sqlite`
- `blobs/sha256/*`
- `websites/*/envs/*/releases/*`
- `websites/*/envs/*/current` (symlink)

Backup strategy:
- snapshot full data dir before high-risk operations.
- preserve both DB and release blobs together.

## 16. Validation/CI Commands

Local test suite:

```bash
go test ./...
```

Docker e2e workflow exists:
- `.github/workflows/docker-e2e.yml`

Manual equivalent:
- build `htmlservd-ssh` + `htmlctl` images
- start container in preview mode
- apply site via SSH tunnel
- verify `/blog`, `/about`, `/ora`
- add domain and verify host-header route

