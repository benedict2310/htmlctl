---
name: htmlctl-publish
description: Publish content to an htmlctl-managed website. Use when an agent needs to create or update pages, components, styles, assets, website metadata, or branding on a site managed by htmlctl/htmlservd. Handles both agent-driven content updates (apply directly to staging, then promote to prod) and structural changes (test locally with Docker first). The server holds all desired state; no git repository is required for content management.
---

# htmlctl Publish

Publish pages, components, styles, website metadata, and branding to a site managed by `htmlctl` / `htmlservd`.

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

## Component Constraints

**Read this before writing any component HTML.** The validator enforces these at apply time:

- **No `<script>` tags** inside components — JS belongs in `scripts/site.js` (injected at end of `<body>` on every page).
- **Exactly one root element** per component file.
- **Root tag must be one of:** `section`, `header`, `footer`, `main`, `nav`, `article`, `div`.
- **No inline event handlers** (`onclick`, `onload`, etc.) — rejected at validation time.
- If the component is anchor-navigable, root element **must** have `id="<componentName>"`.

See `references/resource-schemas.md` for the full schema.

## OG Image Generation (automatic)

The server auto-generates a 1200×630 social preview PNG for every page at build time and serves it at `/og/<pagename>.png`.

**Auto-injection rules:**
- `openGraph.image` and `twitter.image` are populated independently — only when each field is **empty** in the page YAML.
- Auto-injection only runs when `spec.head.canonicalURL` is an absolute `http(s)://` URL. Relative or missing canonicals are left unchanged.
- Generated image URL format: `<scheme>://<host>/og/<pagename>.png` (derived from the canonical URL's scheme + host).

**Card content** (what's rendered on the PNG):
| Field | Source |
|-------|--------|
| Title | `openGraph.title` → `spec.title` |
| Description | `openGraph.description` → `spec.description` |
| Site name | `openGraph.siteName` → website `metadata.name` |

**To opt out / use a manual image:** Set an explicit `openGraph.image` or `twitter.image` in the page YAML. Auto-injection skips any field that already has a value.

**Cache:** Cards are cached by content hash. Re-applying with unchanged metadata is a cache hit — fast, no re-render. The cache is invalidated when title, description, or site name changes.

**Triggering OG generation:** `htmlctl apply` always creates a new release build, even when there are no content changes. To backfill OG images after a server upgrade, just re-apply and then promote.

**Staging URL caveat:** OG image URLs are derived from `canonicalURL`. If your staging pages use a staging canonical (`https://staging.example.com/`), promote will carry those URLs to prod. Ensure `canonicalURL` reflects the prod domain before promoting.

## Website Metadata

Website-scoped metadata lives in `website.yaml`, not in page files or ad hoc assets:

- `spec.head.icons` configures favicon links rendered into every page.
- favicon source files live under `branding/`, not `assets/`.
- `spec.seo.publicBaseURL` defines the canonical public crawl origin.
- `spec.seo.robots.enabled` generates `/robots.txt` during release materialization.
- `spec.seo.sitemap.enabled` generates `/sitemap.xml` during release materialization and appends a `Sitemap:` line to `robots.txt`.

Important constraints:

- `publicBaseURL` must be the real public production URL, not a staging host.
- `promote` copies the release artifact byte-for-byte. It does not rebuild `robots.txt`, `sitemap.xml`, favicon output, or OG metadata.
- if the client supports a website-level feature but the server binary is older, `apply` may still succeed while the expected generated artifact is missing. Upgrade `htmlservd`, then re-apply.

## Runtime Backends

Environment backends let a static site call dynamic services through relative paths such as `/api/*` or `/auth/*`.

- Manage them with `htmlctl backend add`, `htmlctl backend list`, and `htmlctl backend remove`.
- Backend paths must use the canonical prefix form `/<segment>/*`.
- Backends are environment-scoped runtime routing state, not site content. They are not stored in `site/`, are not affected by `htmlctl apply`, and are not copied by `htmlctl promote`.
- Use them when staging and prod should serve the same static release but proxy the same relative prefix to different upstreams.
- After changing a backend, verify the route on that environment directly. Do not assume a staging backend exists in prod.

## Workflow Decision

| Change type | Workflow |
|-------------|----------|
| Content update (copy, cards, links, small edits to existing components) | Apply directly to staging → verify → promote to prod |
| New standalone subpage (new component + new page, no changes to shared components) | Apply directly to staging → verify → promote to prod |
| Website-level metadata change (`website.yaml`, `branding/`, favicon, robots, sitemap) | Verify server version first → apply to staging → verify generated artifacts → promote to prod |
| Environment backend change (`htmlctl backend add/remove`) | Update the target environment directly → verify the proxied route on that environment |
| Structural change to shared components, layout redesign, style overhaul | Test locally with Docker → apply to staging → promote to prod |

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
curl -sf https://staging.example.com/ | grep "<title>"

# 4. Promote exact artifact to prod (no rebuild, byte-for-byte identical)
htmlctl promote website/mysite --from staging --to prod

# 5. Verify prod
htmlctl status website/mysite --context prod
```

> **Note:** The first `apply` bootstraps the environment. Subsequent deploys can use `promote` to copy the staging artifact to prod without rebuilding.

## Workflow A2 — Website Metadata Change

Use for changes to `website.yaml` or `branding/`: favicon, `publicBaseURL`, `robots`, `sitemap`, and similar website-scoped state.

```bash
# 0. Make sure both client and server binaries include the feature you are about to use
go build -o bin/htmlctl ./cmd/htmlctl

# 1. Preview declarative changes
htmlctl diff -f site/ --context staging

# 2. Apply to staging
htmlctl apply -f site/ --context staging

# 3. Verify generated outputs on staging
htmlctl status website/mysite --context staging
curl -sf https://staging.example.com/ | grep "<title>"
curl -sf https://staging.example.com/robots.txt
curl -sf https://staging.example.com/sitemap.xml

# 4. Promote exact artifact to prod
htmlctl promote website/mysite --from staging --to prod

# 5. Verify prod
htmlctl status website/mysite --context prod
curl -sf https://example.com/robots.txt
curl -sf https://example.com/sitemap.xml
```

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
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/robots.txt
curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/sitemap.xml
```

If the site depends on an environment backend, verify that separately after binding the local hostname:

```bash
mkdir -p .tmp/htmlctl-publish/backend/api
printf 'backend-ok\n' > .tmp/htmlctl-publish/backend/api/ping
python3 -m http.server 18081 --bind 127.0.0.1 --directory .tmp/htmlctl-publish/backend

htmlctl backend add website/mysite \
  --env staging \
  --path /api/* \
  --upstream http://host.docker.internal:18081 \
  --context local-docker

curl -sf http://127.0.0.1.nip.io:18080/api/ping
htmlctl backend remove website/mysite --env staging --path /api/* --context local-docker
```

Backends are evaluated before `file_server` for matching prefixes, so `/api/*` requests should hit the upstream rather than any static file under that path.

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
- `htmlctl diff -f site/ --context staging` — review changes (**exit code 1 = changes detected, not an error; exit code 0 = no changes**)
- `htmlctl apply -f site/ --context staging --dry-run` — for risky changes
- `htmlctl config current-context` — confirm you're on the right context
- if you are using newly added website-level features, verify the server binary is current enough to materialize them

After apply:
- `htmlctl status website/mysite --context staging`
- `htmlctl logs website/mysite --context staging` — check for `warning: og image generation failed` lines; these indicate pages whose OG PNG was skipped (build still succeeds)
- Check the site URL
- if `robots` is enabled, verify `/robots.txt`
- if `sitemap` is enabled, verify `/sitemap.xml`
- if favicon is configured, verify the root icon files and page `<head>` output
- if the environment uses backends, verify `htmlctl backend list website/mysite --env staging --context staging` and test the proxied URL directly

Before promote:
- Verify staging behavior end-to-end
- remember that backends do not promote with the release; configure or verify them separately per environment
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
