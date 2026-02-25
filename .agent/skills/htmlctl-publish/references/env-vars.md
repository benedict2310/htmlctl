# htmlctl Environment Variables

## htmlctl (CLI)

| Variable | Default | Description |
|----------|---------|-------------|
| `HTMLCTL_CONFIG` | `~/.htmlctl/config.yaml` | Path to contexts config file |
| `HTMLCTL_SSH_KNOWN_HOSTS_PATH` | `~/.ssh/known_hosts` | Path to known_hosts for SSH host-key verification |
| `HTMLCTL_SSH_KEY_PATH` | `~/.ssh/id_ed25519` etc. | Private key file fallback when SSH agent is unavailable |

SSH auth order:
1. SSH agent socket (`SSH_AUTH_SOCK`)
2. `HTMLCTL_SSH_KEY_PATH` if set
3. Default key files: `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`

---

## htmlservd (Daemon)

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `HTMLSERVD_BIND` | `127.0.0.1` | Interface to bind the API server |
| `HTMLSERVD_PORT` | `9400` | TCP port for the API server (must be `1..65535`) |
| `HTMLSERVD_DATA_DIR` | `/var/lib/htmlservd` | Root directory for SQLite DB, blobs, and releases |
| `HTMLSERVD_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `HTMLSERVD_DB_PATH` | `<dataDir>/db.sqlite` | Explicit SQLite DB path override |
| `HTMLSERVD_DB_WAL` | `false` | Enable WAL mode for better SQLite concurrency |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `HTMLSERVD_API_TOKEN` | _(none)_ | Shared bearer token — required for all `/api/v1/*` routes when set |

When `HTMLSERVD_API_TOKEN` is set (or `api.token` in config), all `/api/v1/*` requests must include `Authorization: Bearer <token>`. Health and version endpoints remain unauthenticated. Use `htmlservd --require-auth` to fail at startup if no token is configured.

### Caddy Integration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTMLSERVD_CADDY_BINARY` | `/usr/local/bin/caddy` | Path to the caddy binary |
| `HTMLSERVD_CADDYFILE_PATH` | `/etc/caddy/Caddyfile` | Path to the Caddyfile managed by htmlservd |
| `HTMLSERVD_CADDY_CONFIG_BACKUP` | `<caddyfilePath>.bak` | Backup path for Caddyfile before each reload |
| `HTMLSERVD_CADDY_AUTO_HTTPS` | `true` | Enable automatic HTTPS via ACME; set `false` for local HTTP |

When `HTMLSERVD_CADDY_AUTO_HTTPS=false`, domain blocks are written as `http://<domain>` to prevent local ACME failures.

### Telemetry

| Variable | Default | Description |
|----------|---------|-------------|
| `HTMLSERVD_TELEMETRY_ENABLED` | `false` | Enable the `POST /collect/v1/events` ingest endpoint |
| `HTMLSERVD_TELEMETRY_MAX_BODY_BYTES` | `65536` | Max request body size in bytes (0 = use default) |
| `HTMLSERVD_TELEMETRY_MAX_EVENTS` | `50` | Max events per request (0 = use default, not unlimited) |
| `HTMLSERVD_TELEMETRY_RETENTION_DAYS` | `90` | Days to retain telemetry rows (0 = no auto-deletion) |

---

## Docker Entrypoint Controls (`htmlservd-ssh`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_PUBLIC_KEY` | _(required)_ | Single bare public key line injected into `authorized_keys` |
| `HTMLSERVD_CADDY_BOOTSTRAP_MODE` | `preview` | Bootstrap mode: `preview` (auto-creates website/env) or `bootstrap` |
| `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` | `:80` | Listen address for the bootstrap Caddyfile |
| `HTMLSERVD_PREVIEW_WEBSITE` | `sample` | Website name to auto-create in preview mode |
| `HTMLSERVD_PREVIEW_ENV` | `staging` | Environment name to auto-create in preview mode |
| `HTMLSERVD_PREVIEW_ROOT` | _(auto)_ | Explicit preview root directory override |

### Entrypoint validation constraints

- `SSH_PUBLIC_KEY` must be a single bare public-key line (no `authorized_keys` options prefix).
- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` must match `:PORT`, `HOST:PORT`, or `[IPv6]:PORT`.
- `HTMLSERVD_PREVIEW_WEBSITE` and `HTMLSERVD_PREVIEW_ENV` must be safe path components (`[A-Za-z0-9._-]+`, no `..`).
- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` and `HTMLSERVD_PREVIEW_ROOT` are rejected if they contain newlines, `{`, or `}`.
- `HTMLSERVD_PORT` must be a numeric TCP port in range `1..65535`.
- `HTMLSERVD_TELEMETRY_ENABLED` must be a boolean (`true` or `false`).

### Security defaults (Docker image)

- `htmlservd` and `htmlctl` run as non-root UID `10001`.
- `authorized_keys` entries are written as `restrict,port-forwarding <key>`.
- `PermitTunnel no` is set in the SSH server config to prevent TUN/TAP pivoting.
- Root SSH login is disabled.
- Caddy receives `CAP_NET_BIND_SERVICE` to bind ports 80/443 as non-root.

---

## config.yaml Reference

Full `htmlservd` config file (all fields, with environment variable equivalents):

```yaml
bind: 127.0.0.1              # HTMLSERVD_BIND
port: 9400                   # HTMLSERVD_PORT
dataDir: /var/lib/htmlservd  # HTMLSERVD_DATA_DIR
logLevel: info               # HTMLSERVD_LOG_LEVEL
dbWAL: true                  # HTMLSERVD_DB_WAL

caddyBinaryPath: /usr/local/bin/caddy   # HTMLSERVD_CADDY_BINARY
caddyfilePath: /etc/caddy/Caddyfile     # HTMLSERVD_CADDYFILE_PATH
caddyConfigBackupPath: /etc/caddy/Caddyfile.bak  # HTMLSERVD_CADDY_CONFIG_BACKUP
caddyAutoHTTPS: true                    # HTMLSERVD_CADDY_AUTO_HTTPS

api:
  token: "YOUR_SHARED_API_TOKEN"        # HTMLSERVD_API_TOKEN

telemetry:
  enabled: false                        # HTMLSERVD_TELEMETRY_ENABLED
  maxBodyBytes: 65536                   # HTMLSERVD_TELEMETRY_MAX_BODY_BYTES
  maxEvents: 50                         # HTMLSERVD_TELEMETRY_MAX_EVENTS
  retentionDays: 90                     # HTMLSERVD_TELEMETRY_RETENTION_DAYS
```
