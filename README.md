# htmlctl

<p align="center">
  <img src="assets/logo.png" alt="htmlctl logo" width="180">
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white" alt="Go 1.24+"></a>
  <a href="https://sqlite.org/"><img src="https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white" alt="SQLite"></a>
  <a href="https://caddyserver.com/"><img src="https://img.shields.io/badge/Caddy-TLS-1F88C0?logo=caddy&logoColor=white" alt="Caddy"></a>
  <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white" alt="Docker"></a>
  <a href="https://github.com/benedict2310/htmlctl/actions/workflows/docker-e2e.yml"><img src="https://github.com/benedict2310/htmlctl/actions/workflows/docker-e2e.yml/badge.svg" alt="Docker E2E"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="MIT License"></a>
</p>

Deploy static HTML/CSS/JS sites like infrastructure.
`htmlctl` (CLI) pairs with `htmlservd` (daemon) to give you immutable releases, atomic rollback, environment promotion, and automatic TLS — on any VPS.

---

## How it works

Write your site as declarative YAML resources. `htmlctl` renders them deterministically, bundles them into a content-addressed release, and activates it atomically on the server over an SSH tunnel. Rollback is a symlink switch that takes under a second.

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

**Environments** (`staging`, `prod`) each have their own active release pointer. Promotion copies the exact artifact bytes — no rebuild, guaranteed hash parity.

---

## Quickstart

### Local preview

```bash
make build
htmlctl render -f ./site -o ./dist
htmlctl serve ./dist --port 8080
```

### Docker — full stack locally

The fastest way to try the whole system. No VPS required.

**1. Build images**

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
```

**2. Start the server**

```bash
API_TOKEN="$(htmlctl context token generate)"
mkdir -p .tmp/demo/{data,caddy}

docker run -d --name htmlservd-demo \
  -p 23222:22 -p 19420:9400 -p 18080:80 \
  -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
  -e HTMLSERVD_API_TOKEN="$API_TOKEN" \
  -e HTMLSERVD_CADDY_AUTO_HTTPS=false \
  -v "$PWD/.tmp/demo/data:/var/lib/htmlservd" \
  -v "$PWD/.tmp/demo/caddy:/etc/caddy" \
  htmlservd-ssh:local

curl -sf http://127.0.0.1:19420/healthz   # → {"status":"ok"}
ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/demo/known_hosts
```

**3. Configure a context**

```bash
cat > .tmp/demo/config.yaml << YAML
apiVersion: htmlctl.dev/v1
current-context: demo
contexts:
  - name: demo
    server: ssh://htmlservd@127.0.0.1:23222
    website: mysite
    environment: staging
    port: 9400
    token: $API_TOKEN
    knownHostsPath: $PWD/.tmp/demo/known_hosts
YAML
export HTMLCTL_CONFIG="$PWD/.tmp/demo/config.yaml"
```

**4. Create a minimal site**

```bash
mkdir -p .tmp/demo/site/{pages,components,styles}

cat > .tmp/demo/site/website.yaml << 'YAML'
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: mysite
spec:
  defaultStyleBundle: default
  baseTemplate: default
YAML

cat > .tmp/demo/site/pages/index.page.yaml << 'YAML'
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: My Site
  description: Built with htmlctl
  layout:
    - include: hero
YAML

cat > .tmp/demo/site/components/hero.html << 'HTML'
<section id="hero">
  <h1>Hello from htmlctl</h1>
</section>
HTML

printf ':root { --bg: #f8f8f8; }\n' > .tmp/demo/site/styles/tokens.css
printf 'body { font-family: sans-serif; background: var(--bg); }\n' > .tmp/demo/site/styles/default.css
```

**5. Deploy and verify**

```bash
htmlctl apply -f .tmp/demo/site --context demo

# Bind a domain so Caddy serves it
htmlctl domain add 127.0.0.1.nip.io --context demo

open http://127.0.0.1.nip.io:18080/
```

**Cleanup**

```bash
docker rm -f htmlservd-demo
```

---

## Site directory structure

```
site/
├── website.yaml            # Website resource (required)
├── pages/
│   ├── index.page.yaml     # One file per page
│   └── about.page.yaml
├── components/
│   ├── nav.html            # HTML fragment — one root element
│   ├── hero.html
│   └── footer.html
├── styles/
│   ├── tokens.css          # CSS custom properties
│   └── default.css         # Base styles
├── scripts/
│   └── site.js             # Optional global JS (single file)
└── assets/
    └── logo.svg            # Images, fonts — content-addressed by SHA-256
```

---

## Resources

### Website

```yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: mysite                   # [a-zA-Z0-9][a-zA-Z0-9_-]*, max 128 chars
spec:
  defaultStyleBundle: default
  baseTemplate: default
```

### Page

```yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: "My Site"
  description: "A short description for search engines"
  layout:
    - include: nav
    - include: hero
    - include: footer
  head:                          # optional — server-rendered into <head>
    canonicalURL: https://example.com/
    openGraph:
      type: website
      title: My Site
      description: A short description
      image: https://example.com/og.png
    twitter:
      card: summary_large_image
      title: My Site
      image: https://example.com/og.png
    jsonLD:
      - id: org
        payload:
          "@context": https://schema.org
          "@type": Organization
          name: My Org
          url: https://example.com
```

### Component

Components are plain HTML fragments (`components/*.html`):

```html
<!-- components/hero.html -->
<section id="hero">
  <h1>Hello</h1>
  <p>No &lt;script&gt; tags or on* event handlers — validated at apply time.</p>
</section>
```

Rules:
- Exactly **one** root element (`section`, `header`, `footer`, `main`, `nav`, `article`, or `div`)
- No `<script>` tags; JS goes in `scripts/site.js`
- No inline event handlers (`onclick`, `onload`, etc.)

---

## Commands

### Local

| Command | Description |
|---------|-------------|
| `htmlctl render -f ./site -o ./dist` | Render site to static HTML |
| `htmlctl serve ./dist --port 8080` | Serve rendered output locally |

### Remote (require `--context`)

| Command | Description |
|---------|-------------|
| `htmlctl apply -f ./site` | Upload and activate a new release |
| `htmlctl apply -f ./site --dry-run` | Show diff without deploying |
| `htmlctl diff -f ./site` | Show file-level diff against current desired state |
| `htmlctl status website/<name>` | Show active release and environment status |
| `htmlctl get website <name>` | Fetch a specific resource |
| `htmlctl logs website/<name>` | Show audit log |

### Release lifecycle

| Command | Description |
|---------|-------------|
| `htmlctl rollout history website/<name>` | List release history |
| `htmlctl rollout undo website/<name>` | Rollback to the previous release (< 1 second) |
| `htmlctl promote website/<name> --from staging --to prod` | Copy release to another environment without rebuild |

### Domains

| Command | Description |
|---------|-------------|
| `htmlctl domain add <domain>` | Bind a domain; triggers Caddyfile regen + TLS cert |
| `htmlctl domain list` | List bound domains |
| `htmlctl domain verify <domain>` | Check DNS propagation and TLS readiness |
| `htmlctl domain remove <domain>` | Remove a domain binding |

### Context

| Command | Description |
|---------|-------------|
| `htmlctl context token generate` | Generate a 32-byte hex API token |
| `htmlctl config view` | Print current config |
| `htmlctl config use-context <name>` | Switch active context |

All commands accept `--output json` or `--output yaml` for machine-parseable output.

---

## Running on a VPS

The full operator guide is in [`docs/setup/hetzner-htmlservd.md`](docs/setup/hetzner-htmlservd.md).
The short version:

```bash
# 1. Build for your server architecture
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o htmlservd-linux-arm64 ./cmd/htmlservd

# 2. Upload and install on the server
scp htmlservd-linux-arm64 user@host:/tmp/htmlservd
ssh user@host "sudo mv /tmp/htmlservd /usr/local/bin/htmlservd && sudo chmod 755 /usr/local/bin/htmlservd"

# 3. Run the setup script (creates htmlservd OS user, SSH config, systemd service)
scp scripts/setup-htmlservd-hetzner.sh user@host:/tmp/
ssh user@host "HTMLSERVD_SSH_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' bash /tmp/setup-htmlservd-hetzner.sh"

# 4. Set the API token
ssh user@host "sudoedit /etc/htmlservd/env"   # set HTMLSERVD_API_TOKEN=<strong-token>
ssh user@host "sudo systemctl restart htmlservd"

# 5. Configure your local context (see ~/.htmlctl/config.yaml)
# 6. Apply your site
htmlctl apply -f ./site --context staging

# 7. Add your domain (Caddy issues TLS automatically)
htmlctl domain add example.com --context prod
```

`htmlservd` is designed to run behind nothing — Caddy handles TLS termination directly. Port 9400 (the API) stays loopback-only; `htmlctl` reaches it via SSH port-forward.

---

## Install

Requires Go 1.24+. Always rebuild after pulling new code.

```bash
make build          # → bin/htmlctl, bin/htmlservd
make test           # unit + integration tests
go test -race ./... # required before merging server changes
```

Docker images:

```bash
docker build --target htmlservd-ssh -t htmlservd-ssh:local .   # server with SSH
docker build --target htmlctl -t htmlctl:local .               # CLI-only image
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `HTMLCTL_CONFIG` | Config file path (default: `~/.htmlctl/config.yaml`) |
| `HTMLCTL_SSH_KNOWN_HOSTS_PATH` | known_hosts override |
| `HTMLCTL_SSH_KEY_PATH` | Private key path (fallback if agent key is rejected) |

---

## Documentation

| Document | Description |
|----------|-------------|
| [`docs/guides/first-deploy-docker.md`](docs/guides/first-deploy-docker.md) | Full Docker quickstart with telemetry |
| [`docs/setup/hetzner-htmlservd.md`](docs/setup/hetzner-htmlservd.md) | VPS setup runbook |
| [`docs/technical-spec.md`](docs/technical-spec.md) | Architecture, API, and resource model |
| [`docs/reference/docker-images.md`](docs/reference/docker-images.md) | Docker image reference |

---

## License

MIT — see [LICENSE](LICENSE).
