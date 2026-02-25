---
name: htmlctl-publish
description: Publish content to an htmlctl-managed website. Use when an agent needs to create or update pages, components, styles, or assets on a site managed by htmlctl/htmlservd. Handles both agent-driven content updates (apply directly to staging, then promote to prod) and structural changes (test locally with Docker first). The server holds all desired state; no git repository is required for content management.
---

# htmlctl Publish

Publish pages, components, and styles to a site managed by `htmlctl` / `htmlservd`.

## Deployment Model

The server is the **source of truth**. `htmlservd` stores all desired state in SQLite and maintains a full, immutable release history. Rollback is a symlink switch — under a second. No git repository is required for day-to-day content management.

```
  htmlctl (CLI)
       │
       │  SSH tunnel · Bearer auth
       ▼
  htmlservd (daemon)
  ├── SQLite (desired state, release history, domains, audit log)
  ├── Filesystem (immutable release artifacts, content-addressed blobs)
  └── Caddy (Caddyfile managed by htmlservd · automatic TLS via ACME)
```

## Workflow Decision

| Change type | Workflow |
|-------------|----------|
| Content update (copy, cards, links, small edits to existing components) | Apply directly to staging → verify → promote to prod |
| Structural change (new page, layout redesign, style overhaul, new component) | Test locally with Docker → apply to staging → promote to prod |

## Prerequisites

A context must exist in `~/.htmlctl/config.yaml` (or `HTMLCTL_CONFIG`):

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://user@yourserver
    website: mysite
    environment: staging
    port: 9400
    token: "<api-token>"
  - name: prod
    server: ssh://user@yourserver
    website: mysite
    environment: prod
    port: 9400
    token: "<api-token>"
```

SSH auth order: SSH agent socket (`SSH_AUTH_SOCK`) → key file fallback (`HTMLCTL_SSH_KEY_PATH`, then `~/.ssh/id_ed25519|id_rsa|id_ecdsa`).

## Workflow A — Content Update (direct to staging)

Use for targeted changes: updating copy, adding a project card, editing a single component.

```bash
# 1. Preview what will change
htmlctl diff -f site/ --context staging

# 2. Apply changed file(s)
htmlctl apply -f components/projects.html --context staging
#   or the full site:
htmlctl apply -f site/ --context staging

# 3. Verify on staging URL
htmlctl status website/mysite --context staging
htmlctl logs website/mysite --context staging

# 4. Promote exact artifact to prod (no rebuild, byte-for-byte identical)
htmlctl promote website/mysite --from staging --to prod

# 5. Verify prod
htmlctl status website/mysite --context prod
```

> **Note:** The first `apply` bootstraps the environment. Subsequent deploys can use `promote` to copy the staging artifact to prod without rebuilding.

## Workflow B — Structural Change (Docker local first)

Use for new pages, layout changes, style overhauls, or any change touching many files at once.

### Start local Docker server

```bash
API_TOKEN="$(htmlctl context token generate)"
mkdir -p .tmp/htmlctl-publish/{data,caddy}
docker rm -f htmlservd-local >/dev/null 2>&1 || true

docker run -d \
  --name htmlservd-local \
  -p 23222:22 -p 19420:9400 -p 18080:80 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -e HTMLSERVD_API_TOKEN="$API_TOKEN" \
  -e HTMLSERVD_CADDY_BOOTSTRAP_MODE=preview \
  -e HTMLSERVD_PREVIEW_WEBSITE=mysite \
  -e HTMLSERVD_PREVIEW_ENV=staging \
  -e HTMLSERVD_CADDY_AUTO_HTTPS=false \
  -e HTMLSERVD_TELEMETRY_ENABLED=true \
  -v "$PWD/.tmp/htmlctl-publish/data:/var/lib/htmlservd" \
  -v "$PWD/.tmp/htmlctl-publish/caddy:/etc/caddy" \
  htmlservd-ssh:local

# Trust the container's host key
ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/htmlctl-publish/known_hosts

# Health check
curl -sf http://127.0.0.1:19420/healthz
```

### Add a local-docker context entry

```yaml
# Add to ~/.htmlctl/config.yaml
- name: local-docker
  server: ssh://htmlservd@127.0.0.1:23222
  website: mysite
  environment: staging
  port: 9400
  token: "<API_TOKEN>"
```

Set `HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/htmlctl-publish/known_hosts"` when running htmlctl.

### Iterate locally

```bash
htmlctl apply -f site/ --context local-docker
htmlctl domain add 127.0.0.1.nip.io --context local-docker
```

Open `http://127.0.0.1.nip.io:18080/` — use the hostname, not the raw IP (Caddy uses virtual hosting; raw `127.0.0.1` matches no vhost and returns empty body).

```bash
# Verify
htmlctl status website/mysite --context local-docker
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/ | grep "<title>"
```

### Ship to staging and prod

```bash
htmlctl apply -f site/ --context staging
htmlctl status website/mysite --context staging
htmlctl promote website/mysite --from staging --to prod
```

### Cleanup

```bash
docker rm -f htmlservd-local
```

## Safety Checklist

Before any apply:
- `htmlctl diff -f site/ --context staging` — review changes
- `htmlctl apply -f site/ --context staging --dry-run` — for risky changes
- `htmlctl config current-context` — confirm you're on the right context

After apply:
- `htmlctl status website/mysite --context staging`
- `htmlctl logs website/mysite --context staging`
- Check the site URL

Before promote:
- Verify staging behavior end-to-end
- `htmlctl rollout history website/mysite --context staging` — confirm active release

Rollback:
```bash
htmlctl rollout undo website/mysite --context prod
```

## Reference Files

| File | Contents |
|------|----------|
| `references/commands.md` | Full command reference with all flags |
| `references/resource-schemas.md` | YAML schemas for all resource kinds |
| `references/deployment-workflows.md` | Docker local, VPS native, VPS Docker runbooks |
| `references/env-vars.md` | All htmlctl and htmlservd environment variables |
| `references/api.md` | Direct HTTP API reference and telemetry |
| `references/troubleshooting.md` | Failure modes, fixes, and operational safety |
