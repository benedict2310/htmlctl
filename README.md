# htmlctl

<p align="center">
  <img src="assets/logo.png" alt="htmlctl logo" width="180">
</p>

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Cobra CLI](https://img.shields.io/badge/Cobra-CLI-3EAAAF?logo=go&logoColor=white)](https://github.com/spf13/cobra)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](https://sqlite.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![Caddy](https://img.shields.io/badge/Caddy-domain%20routing-1F88C0?logo=caddy&logoColor=white)](https://caddyserver.com/)
[![Docker E2E](https://github.com/benedict2310/htmlctl/actions/workflows/docker-e2e.yml/badge.svg)](https://github.com/benedict2310/htmlctl/actions/workflows/docker-e2e.yml)

`htmlctl` is a CLI-first control plane for static HTML/CSS/JS sites.  
It pairs with `htmlservd` to deliver deterministic builds, immutable releases, atomic activation, and domain routing on VPS-class infrastructure.

## Problem Statement

Static websites are simple to build but often painful to operate at scale:

- Deploys are ad-hoc, manual, and hard to reproduce.
- Rollbacks are slow or unsafe.
- Promotion across environments is inconsistent.
- Domain routing and TLS setup are usually bolted on separately.
- Agent-driven workflows need deterministic commands and machine-parseable output.

## How htmlctl Solves It

`htmlctl` and `htmlservd` provide a lightweight release platform for static content:

- Declarative website resources and deterministic rendering.
- Immutable release history with atomic `current` pointer switches.
- Fast rollback (`rollout undo`) and promotion (`promote`) using artifact hash parity.
- SSH-tunneled remote control plane with context-aware CLI commands.
- Bearer-token API authentication for all `/api/v1/*` operations.
- Built-in domain lifecycle commands with Caddy config/reload integration.
- Optional first-party telemetry ingest (`POST /collect/v1/events`) with authenticated query APIs.

## Key Features

- Local workflow: `render`, `serve`, validation, and preview.
- Remote workflow: `diff`, `apply`, `status`, `logs`, `get`.
- Release lifecycle: `rollout history`, `rollout undo`, `promote`.
- Domain ops: `domain add`, `domain list`, `domain verify`, `domain remove`.
- Docker-first path for reproducible deployment and end-to-end testing.
- Same-origin browser telemetry path using `navigator.sendBeacon` (no external analytics service required).

## Quick Start

### 1. Local preview

```bash
htmlctl render -f ./site -o ./dist
htmlctl serve ./dist --port 8080
```

### 2. Remote deploy (SSH context)

Create a config (`~/.htmlctl/config.yaml` by default):

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://YOUR_USER@YOUR_HOST
    website: futurelab
    environment: staging
    port: 9400
    token: YOUR_API_TOKEN
```

Generate a token and set it on the context:

```bash
htmlctl context token generate
htmlctl context set staging --token YOUR_API_TOKEN
```

Deploy and inspect:

```bash
htmlctl diff -f ./site --context staging
htmlctl apply -f ./site --context staging
htmlctl status website/futurelab --context staging
htmlctl rollout history website/futurelab --context staging
```

## Install

Build from source:

```bash
make build
```

Run tests:

```bash
make test
```

Or directly:

```bash
go test ./...
```

Build Docker images:

```bash
docker build --target htmlctl -t htmlctl:local .
docker build --target htmlservd -t htmlservd:local .
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
```

## Commands

`htmlctl` currently ships with:

- `apply`
- `config`
- `context`
- `diff`
- `domain`
- `get`
- `logs`
- `promote`
- `render`
- `rollout`
- `serve`
- `status`
- `version`

## Documentation

- `docs/README.md` for the full docs index
- `docs/guides/first-deploy-docker.md` for first website deployment, including a telemetry-ready local flow (`127.0.0.1.nip.io` binding + verification)
- `docs/operations-manual-agent.md` for end-to-end agent runbook
- `docs/reference/docker-images.md` for image/runtime details
- `docs/stories/` for epic/story implementation specs

## Contributing

Use story-driven workflows in `docs/stories/` and validate with repository scripts:

- `.claude/skills/implement-story/scripts/preflight.sh <story-file> --quiet --no-color`

## License

License is currently not defined.
