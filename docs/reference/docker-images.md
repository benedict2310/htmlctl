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

Mount these paths for persistence:

- `/var/lib/htmlservd`
- `/etc/caddy`

Exposed ports:

- `9400` (`htmlservd` API)
- `80/443` (Caddy)

## Security/Hardening Notes

- `htmlservd` and `htmlctl` run as non-root UID `10001`.
- `htmlservd-ssh` requires `SSH_PUBLIC_KEY`, disables password login, and allows SSH only for `htmlservd` user (root SSH disabled).
- Caddy receives `CAP_NET_BIND_SERVICE` so non-root runtime can bind `80/443`.
- Caddy reload uses explicit `--adapter caddyfile` for validate/reload consistency.
- Use `.dockerignore` to keep build context small and avoid shipping local artifacts.
