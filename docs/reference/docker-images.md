# Docker Images

`Dockerfile` defines three build targets:

- `htmlctl`: CLI image with `htmlctl` + `openssh-client`
- `htmlservd`: daemon image with `htmlservd` + `caddy` (both started by entrypoint)
- `htmlservd-ssh`: daemon + SSH entrypoint for full tunnel-based e2e flows

## Build

```bash
docker build --target htmlctl -t htmlctl:local .
docker build --target htmlservd -t htmlservd:local .
docker build --target htmlservd-ssh -t htmlservd-ssh:local .
```

## Runtime Defaults (`htmlservd`)

- `HTMLSERVD_BIND=0.0.0.0`
- `HTMLSERVD_PORT=9400`
- `HTMLSERVD_DATA_DIR=/var/lib/htmlservd`
- `HTMLSERVD_CADDY_BINARY=/usr/local/bin/caddy`
- `HTMLSERVD_CADDYFILE_PATH=/etc/caddy/Caddyfile`
- `HTMLSERVD_CADDY_BOOTSTRAP_MODE=preview` (`preview|bootstrap`)
- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN=:80`
- `HTMLSERVD_PREVIEW_WEBSITE=futurelab`
- `HTMLSERVD_PREVIEW_ENV=staging`
- `HTMLSERVD_PREVIEW_ROOT` (optional explicit override)
- `HTMLSERVD_CADDY_AUTO_HTTPS=true` (set `false` for local plain-HTTP workflows)
- `HTMLSERVD_API_TOKEN` (recommended; protects `/api/v1/*` with bearer auth)

When `HTMLSERVD_CADDY_AUTO_HTTPS=false`, generated domain config uses explicit `http://<domain>` site addresses to avoid local ACME/TLS failures.

Entrypoint validation constraints:

- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` must be `:PORT`, `HOST:PORT`, or `[IPv6]:PORT`.
- `HTMLSERVD_PREVIEW_WEBSITE` and `HTMLSERVD_PREVIEW_ENV` must be safe path components (`[A-Za-z0-9._-]+`, no `..`).
- `HTMLSERVD_CADDY_BOOTSTRAP_LISTEN` and `HTMLSERVD_PREVIEW_ROOT` reject values containing newlines, `{`, or `}`.

Mount these paths for persistence:

- `/var/lib/htmlservd`
- `/etc/caddy`

Exposed ports:

- `9400` (`htmlservd` API)
- `80/443` (Caddy)

Auth behavior:

- `/api/v1/*` requires `Authorization: Bearer <token>` when `HTMLSERVD_API_TOKEN` (or config `api.token`) is set.
- `/healthz`, `/readyz`, `/version` remain unauthenticated.
- `htmlservd --require-auth` forces startup failure if no API token is configured.

## Runtime Defaults (`htmlctl`)

- SSH auth order:
1. SSH agent (`SSH_AUTH_SOCK`)
2. Private key file fallback (`HTMLCTL_SSH_KEY_PATH`, then `~/.ssh/id_ed25519|id_rsa|id_ecdsa`)
- Known hosts path:
1. `HTMLCTL_SSH_KNOWN_HOSTS_PATH`
2. `~/.ssh/known_hosts`

## Runtime Defaults (`htmlservd-ssh`)

- `SSH_PUBLIC_KEY` is required.
- `SSH_PUBLIC_KEY` must be a single bare public-key line (no `authorized_keys` options prefix).

## Security/Hardening Notes

- `htmlservd` and `htmlctl` run as non-root UID `10001`.
- `htmlservd-ssh` requires `SSH_PUBLIC_KEY`, disables password login, and allows SSH only for `htmlservd` user (root SSH disabled).
- `htmlservd-ssh` writes authorized keys as `restrict,port-forwarding <key>` to enforce least privilege for tunnel usage.
- SSH `PermitTunnel` is disabled (`PermitTunnel no`) to prevent TUN/TAP pivoting through the container.
- Caddy receives `CAP_NET_BIND_SERVICE` so non-root runtime can bind `80/443`.
- Caddy reload uses explicit `--adapter caddyfile` for validate/reload consistency.
- Use `.dockerignore` to keep build context small and avoid shipping local artifacts.
