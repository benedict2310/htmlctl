# htmlctl Command Reference

## Context Management

```bash
# Show current context
htmlctl context list

# Switch active context
htmlctl context use <name>

# Generate a random API token (use for HTMLSERVD_API_TOKEN)
htmlctl context token generate

# Set a token on an existing context
htmlctl context set <name> --token <token>
```

Config file: `~/.htmlctl/config.yaml` (or `HTMLCTL_CONFIG` env var).

---

## Local Operations

```bash
# Render site to a local output directory
htmlctl render -f ./site -o ./dist

# Serve a rendered dist directory locally
htmlctl serve ./dist --port 8080
```

---

## Apply (push desired state to server)

```bash
# Apply full site directory
htmlctl apply -f site/ --context staging

# Apply a single changed file (server merges into current desired state)
htmlctl apply -f components/hero.html --context staging

# Dry run — show what would change without applying
htmlctl apply -f site/ --context staging --dry-run
```

---

## Diff

```bash
# Show difference between local files and current server state
htmlctl diff -f site/ --context staging
```

---

## Status

```bash
htmlctl status website/<name> --context staging
htmlctl status website/<name> --context prod
```

---

## Promote

Copy the exact staging release artifact to prod — no rebuild, byte-for-byte identical:

```bash
htmlctl promote website/<name> --from staging --to prod
```

---

## Rollback

```bash
# View release history
htmlctl rollout history website/<name> --context prod

# Undo last deploy (activates previous release)
htmlctl rollout undo website/<name> --context prod
```

---

## Logs

```bash
htmlctl logs website/<name> --context prod
```

---

## Domains

```bash
# Add a domain binding
htmlctl domain add example.com --context prod
htmlctl domain add staging.example.com --context staging

# Verify DNS is pointing at the server
htmlctl domain verify example.com --context prod
```

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `HTMLCTL_CONFIG` | Path to config file (default: `~/.htmlctl/config.yaml`) |
| `HTMLCTL_SSH_KNOWN_HOSTS_PATH` | Path to known_hosts for SSH host-key verification |
| `HTMLCTL_SSH_KEY_PATH` | Path to private key (fallback when SSH agent unavailable) |

---

## Server Environment Variables (htmlservd)

| Variable | Purpose |
|----------|---------|
| `HTMLSERVD_API_TOKEN` | Shared API token (required when `--require-auth` is set) |
| `HTMLSERVD_CADDY_BOOTSTRAP_MODE` | `preview` for local Docker testing |
| `HTMLSERVD_PREVIEW_WEBSITE` | Website name to auto-create in preview mode |
| `HTMLSERVD_PREVIEW_ENV` | Environment name to auto-create in preview mode |
| `HTMLSERVD_CADDY_AUTO_HTTPS` | Set `false` for local Docker (no TLS needed) |
| `HTMLSERVD_TELEMETRY_ENABLED` | Enable `/collect/v1/events` routing in preview bootstrap mode |
| `SSH_PUBLIC_KEY` | Public key injected into the container's `authorized_keys` |

When telemetry testing locally, bind a hostname and browse that host instead of raw IP:

```bash
htmlctl domain add 127.0.0.1.nip.io --context local-docker
open http://127.0.0.1.nip.io:18080/
```
