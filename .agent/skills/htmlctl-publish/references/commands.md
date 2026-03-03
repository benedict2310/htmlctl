# htmlctl Command Reference

## Context Management

```bash
# Create a context without editing YAML by hand
htmlctl context create <name> --server ssh://user@host --website <website> --environment <env>

# Show all configured contexts
htmlctl context list

# Show the active context name
htmlctl config current-context

# Show full config (all contexts, current) with tokens redacted
htmlctl config view

# Show full config including secrets
htmlctl config view --show-secrets

# Switch active context
htmlctl context use <name>

# Generate a random 32-byte hex API token (use for HTMLSERVD_API_TOKEN)
htmlctl context token generate

# Set token on an existing context
htmlctl context set <name> --token <token>
```

Config file: `~/.htmlctl/config.yaml` (override with `HTMLCTL_CONFIG` env var).

Context fields:

| Field | Description |
|-------|-------------|
| `name` | Context identifier used in `--context` |
| `server` | `ssh://user@host` or `ssh://user@host:port` |
| `website` | Website name (matches `metadata.name` in `website.yaml`) |
| `environment` | Environment name (`staging`, `prod`, etc.) |
| `port` | Port `htmlservd` listens on (default `9400`) |
| `token` | Shared bearer token for `/api/v1/*` auth |

---

## Local Operations

```bash
# Render site to a local output directory (deterministic, no server required)
htmlctl render -f ./site -o ./dist

# Serve a rendered dist directory locally
htmlctl serve ./dist --port 8080
```

Rendered output: `dist/<route>/index.html` for each page.

---

## Diff

Show what would change between local files and the current server desired state:

```bash
htmlctl diff -f site/ --context staging
```

---

## Apply

Push desired state to the server. The server validates, renders, and creates an immutable release. **`apply` always creates a new release**, even when the content is unchanged — useful for triggering a server-side feature (e.g. OG image generation) without editing any files.

```bash
# Apply full site directory
htmlctl apply -f site/ --context staging

# Apply a single changed file (server merges into current desired state)
htmlctl apply -f components/hero.html --context staging
htmlctl apply -f styles/default.css --context staging

# Dry run — validate and show what would change, no release created
htmlctl apply -f site/ --context staging --dry-run
```

On first deploy, `apply` bootstraps the environment. The output includes a hint pointing to the next domain-binding command.

---

## Status

```bash
# Use the active context website
htmlctl status --context staging

# Override the website explicitly
htmlctl status website/<name> --context staging
htmlctl status website/<name> --context prod
```

---

## Get

Inspect server state for specific resources.

**Supported resource types:** `websites`, `environments`, `releases` only.
`pages`, `components`, `styles`, and `assets` are not queryable via `get` — use `htmlctl diff` to compare local vs server state, or `htmlctl status` for summary counts.

```bash
# List all releases for a website/environment
htmlctl get releases --context staging

# List all websites on the server
htmlctl get websites --context staging

# List all environments
htmlctl get environments --context staging
```

---

## Logs

```bash
# View recent deploy log for the active context website/environment
htmlctl logs --context prod

# Override the website explicitly
htmlctl logs website/<name> --context prod

# Limit output lines
htmlctl logs website/<name> --context prod --limit 50
```

---

## Promote

Copy the exact staging release artifact to prod — no rebuild, byte-for-byte identical:

```bash
htmlctl promote website/<name> --from staging --to prod
```

> **Prerequisite:** The target environment must already exist (bootstrapped via a prior `apply`). If the prod environment does not exist yet, run `htmlctl apply -f site/ --context prod` once to bootstrap it, then use `promote` for all subsequent deploys.

---

## Rollback

```bash
# View release history for the active context website/environment
htmlctl rollout history --context prod

# View release history for an explicit website
htmlctl rollout history website/<name> --context prod

# Undo last deploy for the active context website/environment
htmlctl rollout undo --context prod

# Undo last deploy for an explicit website (activates previous release — symlink switch, < 1 second)
htmlctl rollout undo website/<name> --context prod
```

---

## Domains

```bash
# Add a domain binding (triggers Caddy config regeneration + reload + ACME cert)
htmlctl domain add example.com --context prod
htmlctl domain add staging.example.com --context staging

# List domain bindings for the active context's environment
htmlctl domain list --context prod

# Verify DNS resolution and TLS certificate validity
htmlctl domain verify example.com --context prod

# Remove a domain binding
htmlctl domain remove example.com --context prod
```

`domain verify` checks:
- DNS: A/AAAA record resolves to server IP
- TLS: valid certificate served on port 443

Both must pass for `verify` to succeed. In local/no-TLS environments, TLS failure is expected.

---

## Backends

Environment backends are runtime routing config, not bundle content. `apply`, `diff`, and `promote` do not create, copy, or remove them.

```bash
# Add a reverse-proxy backend using the active context website/environment
htmlctl backend add \
  --path /api/* \
  --upstream https://staging-api.example.com \
  --context staging

# Add a reverse-proxy backend for an explicit website/environment
htmlctl backend add website/<name> \
  --env staging \
  --path /api/* \
  --upstream https://staging-api.example.com \
  --context staging

# List configured backends for the active context website/environment
htmlctl backend list --context staging

# List configured backends for an explicit website/environment
htmlctl backend list website/<name> --env staging --context staging

# Remove a backend mapping by path using the active context website/environment
htmlctl backend remove --path /api/* --context staging

# Remove a backend mapping by path for an explicit website/environment
htmlctl backend remove website/<name> --env staging --path /api/* --context staging
```

Backend rules:
- `--path` must use canonical prefix form such as `/api/*`
- upstreams must be absolute `http://` or `https://` URLs
- matching backend prefixes are routed before static file serving

---

## Diagnostics

```bash
# Show the local CLI version
htmlctl version

# Show both the local CLI version and the selected remote htmlservd version
htmlctl version --remote --context staging

# Run read-only diagnostics for config, SSH transport, authenticated API access, health, readiness, and version
htmlctl doctor --context staging

# Structured diagnostics output for automation
htmlctl doctor --context staging --output json
```

Common failure hints:
- `ssh host key verification failed`: refresh `known_hosts` with `ssh-keyscan`, then rerun `htmlctl doctor`.
- `ssh authentication failed`: load the correct key into your agent or set `HTMLCTL_SSH_KEY_PATH`.
- version mismatch: update either `htmlctl` or `htmlservd` so both sides run compatible builds before using newer CLI features.
