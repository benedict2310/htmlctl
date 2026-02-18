# First Deploy With Docker

This guide deploys `htmlservd` in Docker, tunnels through SSH, and publishes a first release with `htmlctl`.

## 1. Build Images

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
docker build --target htmlctl -t htmlctl:local .
```

## 2. Start Server Container

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

Health check:

```bash
curl -sf http://127.0.0.1:19420/healthz
```

Trust host key for SSH transport:

```bash
ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/first-deploy/known_hosts
```

## 3. Prepare `htmlctl` Config

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

## 4. Create Sample Site

```bash
mkdir -p .tmp/first-deploy/site/{pages,components,styles,scripts,assets}
```

```bash
cat > .tmp/first-deploy/site/website.yaml <<'YAML'
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: futurelab
spec:
  defaultStyleBundle: default
  baseTemplate: default
YAML
```

```bash
cat > .tmp/first-deploy/site/pages/index.page.yaml <<'YAML'
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Futurelab
  description: Demo landing page
  layout:
    - include: hero
YAML
```

```bash
cat > .tmp/first-deploy/site/components/hero.html <<'HTML'
<section id="hero">
  <h1>Futurelab</h1>
  <p>htmlctl first deploy</p>
</section>
HTML
```

```bash
cat > .tmp/first-deploy/site/styles/tokens.css <<'CSS'
:root {
  --bg: #f5f5f5;
}
CSS
cat > .tmp/first-deploy/site/styles/default.css <<'CSS'
body { font-family: sans-serif; margin: 2rem; background: var(--bg); }
h1 { margin-bottom: 0.5rem; }
CSS
```

## 5. Apply + Verify

If your SSH key is not already loaded:

```bash
ssh-add ~/.ssh/id_ed25519
```

Apply:

```bash
HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl apply -f .tmp/first-deploy/site --context local-staging
```

Status:

```bash
HTMLCTL_CONFIG="$PWD/.tmp/first-deploy/htmlctl-config.yaml" \
HTMLCTL_SSH_KNOWN_HOSTS_PATH="$PWD/.tmp/first-deploy/known_hosts" \
htmlctl status website/futurelab --context local-staging
```

Open the site:

```bash
open http://127.0.0.1:18080/
```

## 6. Optional: Run `htmlctl` in Docker

`htmlctl:local` can use mounted key files (agent not required):

```bash
docker run --rm \
  --network htmlctl-net \
  -e HTMLCTL_CONFIG=/work/.tmp/first-deploy/htmlctl-config-container.yaml \
  -e HTMLCTL_SSH_KNOWN_HOSTS_PATH=/home/htmlctl/.ssh/known_hosts \
  -v "$PWD:/work" \
  -v "$HOME/.ssh/id_ed25519:/home/htmlctl/.ssh/id_ed25519:ro" \
  -v "$PWD/.tmp/first-deploy/known_hosts:/home/htmlctl/.ssh/known_hosts:ro" \
  -w /work \
  htmlctl:local status website/futurelab --context local-staging
```

Use this config for containerized `htmlctl` (Docker-to-host networking):

```bash
cat > .tmp/first-deploy/htmlctl-config-container.yaml <<'YAML'
apiVersion: htmlctl.dev/v1
current-context: local-staging
contexts:
  - name: local-staging
    server: ssh://htmlservd@host.docker.internal:23222
    website: futurelab
    environment: staging
    port: 9400
YAML
```

## 7. Cleanup

```bash
docker rm -f htmlservd-first-deploy
docker network rm htmlctl-net
```

## 8. Troubleshooting

- `ssh host key verification failed`: regenerate `.tmp/first-deploy/known_hosts` with `ssh-keyscan`.
- `ssh agent unavailable`: `htmlctl` now supports key-file fallback; mount/provide `~/.ssh/id_ed25519` or set `HTMLCTL_SSH_KEY_PATH`.
- Permission errors under `.tmp/first-deploy`: avoid overriding `HOME` into a bind-mounted path; use `HTMLCTL_SSH_KNOWN_HOSTS_PATH` instead (as shown above).
