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
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -v "$PWD/.tmp/first-deploy/data:/var/lib/htmlservd" \
  -v "$PWD/.tmp/first-deploy/caddy:/etc/caddy" \
  htmlservd-ssh:local
```

Health check:

```bash
curl -sf http://127.0.0.1:19420/healthz
```

Capture the container IP and trust host key for SSH transport:

```bash
SERVER_IP="$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' htmlservd-first-deploy)"
ssh-keyscan -H "$SERVER_IP" > .tmp/first-deploy/known_hosts
```

## 3. Prepare htmlctl Config

```bash
cat > .tmp/first-deploy/htmlctl-config.yaml <<'YAML'
apiVersion: htmlctl.dev/v1
current-context: local-staging
contexts:
  - name: local-staging
    server: ssh://htmlservd@SERVER_IP:22
    website: futurelab
    environment: staging
    port: 9400
YAML
```

```bash
sed -i.bak "s/SERVER_IP/$SERVER_IP/" .tmp/first-deploy/htmlctl-config.yaml && rm .tmp/first-deploy/htmlctl-config.yaml.bak
```

## 4. Create Sample Site

```bash
mkdir -p .tmp/first-deploy/site/{pages,components,styles,assets}
cat > .tmp/first-deploy/site/website.yaml <<'YAML'
website: futurelab
defaultStyleBundle: main
baseTemplate: base
YAML
cat > .tmp/first-deploy/site/pages/index.yaml <<'YAML'
name: home
route: /
title: Futurelab
description: Demo landing page
layout:
  - component: hero
YAML
cat > .tmp/first-deploy/site/components/hero.html <<'HTML'
<main><h1>Futurelab</h1><p>htmlctl first deploy</p></main>
HTML
cat > .tmp/first-deploy/site/styles/main.css <<'CSS'
body { font-family: sans-serif; margin: 2rem; }
h1 { margin-bottom: 0.5rem; }
CSS
```

## 5. Apply + Release

```bash
docker run --rm \
  --network htmlctl-net \
  -e HOME=/home/htmlctl \
  -e HTMLCTL_CONFIG=/work/.tmp/first-deploy/htmlctl-config.yaml \
  -v "$PWD:/work" \
  -v "$HOME/.ssh/id_ed25519:/home/htmlctl/.ssh/id_ed25519:ro" \
  -v "$PWD/.tmp/first-deploy/known_hosts:/home/htmlctl/.ssh/known_hosts:ro" \
  -w /work \
  htmlctl:local apply -f .tmp/first-deploy/site --context local-staging
```

Verify:

```bash
docker run --rm \
  --network htmlctl-net \
  -e HOME=/home/htmlctl \
  -e HTMLCTL_CONFIG=/work/.tmp/first-deploy/htmlctl-config.yaml \
  -v "$PWD:/work" \
  -v "$HOME/.ssh/id_ed25519:/home/htmlctl/.ssh/id_ed25519:ro" \
  -v "$PWD/.tmp/first-deploy/known_hosts:/home/htmlctl/.ssh/known_hosts:ro" \
  -w /work \
  htmlctl:local status website/futurelab --context local-staging
```

## 6. Cleanup

```bash
docker rm -f htmlservd-first-deploy
docker network rm htmlctl-net
```
