# htmlctl

`htmlctl` is a kubectl-style control plane for static HTML/CSS/JS websites, designed for AI-agent and CLI-first workflows.

It manages declarative website resources, builds deterministic static output, and deploys via immutable releases with atomic activation.

## Project Status

Runtime binaries are implemented and usable:

- `htmlctl` (CLI)
- `htmlservd` (server daemon)

Implemented scope:
- Epic 1: Local parser/validation/render/serve
- Epic 2: Server daemon, desired state storage, apply ingestion, release build/activation, audit logs
- Epic 3: Context config, SSH transport, remote `get/status/apply/logs/diff`

Not yet implemented:
- Epic 4: Rollback + promote commands
- Epic 5: Domain/TLS/Caddy commands

## Build & Test

```bash
make build
make test
```

or:

```bash
go test ./...
```

## Quick Start (Local)

Render and preview a site directory:

```bash
htmlctl render -f ./site -o ./dist
htmlctl serve ./dist --port 8080
```

## Quick Start (Remote)

1) Start `htmlservd`:

```bash
HTMLSERVD_DATA_DIR="$PWD/.tmp/htmlservd" htmlservd
```

2) Configure `htmlctl` context (`~/.htmlctl/config.yaml` by default):

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://YOUR_USER@YOUR_HOST
    website: futurelab
    environment: staging
    port: 9400
```

`server` must be an SSH URL. `htmlctl` tunnels HTTP API calls through SSH.

3) Run remote workflow:

```bash
htmlctl diff -f ./site --context staging
htmlctl apply -f ./site --context staging
htmlctl status website/futurelab --context staging
htmlctl logs website/futurelab --context staging
htmlctl get releases --context staging
```

Dry run (diff-only, no upload/release):

```bash
htmlctl apply -f ./site --context staging --dry-run
```

## Implemented Commands

- `htmlctl render`
- `htmlctl serve`
- `htmlctl config view|current-context|use-context`
- `htmlctl get websites|environments|releases`
- `htmlctl status website/<name>`
- `htmlctl logs website/<name>`
- `htmlctl diff -f <site-dir>`
- `htmlctl apply -f <site-dir>`
- `htmlctl version`

## License

License not yet defined.
