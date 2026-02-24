# Hetzner Phase 1 Setup: htmlservd + staging.futurelab.studio

This runbook installs `htmlservd` on an existing Ubuntu 24.04 ARM64 Hetzner host and adds a staging static site vhost in Nginx without touching the running Next.js production stack.

## Prerequisites

- Server: Hetzner CAX11 (`65.21.0.203`), Ubuntu 24.04 ARM64.
- SSH alias exists locally:

```sshconfig
Host hetzner
  HostName 65.21.0.203
  User bene
```

- Existing services remain in place and must stay untouched:
- Nginx serving `futurelab.studio`.
- Next.js/PM2 on `:3000`.
- PostgreSQL local.
- Local tooling installed: Go toolchain, `scp`, `ssh`, and `htmlctl`.

## 1. Build htmlservd for linux/arm64

From your local `htmlctl` repo root:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o htmlservd-linux-arm64 ./cmd/htmlservd
```

## 2. Upload/install binary on Hetzner

```bash
scp htmlservd-linux-arm64 hetzner:/tmp/htmlservd && ssh hetzner "sudo mv /tmp/htmlservd /usr/local/bin/htmlservd && sudo chmod 755 /usr/local/bin/htmlservd"
```

## 3. Run htmlservd host setup script

Copy scripts to the server (if needed):

```bash
scp scripts/setup-htmlservd-hetzner.sh scripts/setup-staging-nginx.sh hetzner:/tmp/
```

Run `htmlservd` setup with your local SSH public key (required for tunnel login as `htmlservd`):

```bash
ssh hetzner "export HTMLSERVD_SSH_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' && bash /tmp/setup-htmlservd-hetzner.sh"
```

The script will:

- Create system user `htmlservd` (nologin) and SSH auth setup.
- Write `authorized_keys` with `restrict,port-forwarding` prefix.
- Create `/var/lib/htmlservd` and `/etc/htmlservd`.
- Write `/etc/htmlservd/config.yaml` and `/etc/htmlservd/env`.
- Install no-op Caddy shim (`/usr/local/bin/htmlservd-caddy-noop`).
- Install/enable/start systemd service `htmlservd`.
- Install SSH drop-in `/etc/ssh/sshd_config.d/htmlservd.conf` and reload sshd.

## 4. Set API token on server

Edit `/etc/htmlservd/env` and set a strong shared token:

```bash
ssh hetzner "sudoedit /etc/htmlservd/env"
```

Expected line:

```bash
HTMLSERVD_API_TOKEN=REPLACE_WITH_LONG_RANDOM_TOKEN
```

Restart service after setting token:

```bash
ssh hetzner "sudo systemctl restart htmlservd"
```

## 5. Add DNS for staging domain

Create DNS record:

- Type: `A`
- Name: `staging.futurelab.studio`
- Value: `65.21.0.203`
- TTL: default (or 300 during rollout)

Wait for propagation before cert issuance.

## 6. Configure Nginx staging vhost + TLS

Run the Nginx setup script:

```bash
ssh hetzner "bash /tmp/setup-staging-nginx.sh"
```

What it does:

- Creates `/etc/nginx/sites-available/staging.futurelab.studio`.
- Serves static files from `/var/lib/htmlservd/websites/futurelab/envs/staging/current`.
- Ensures `general_limit` rate-limit zone exists at `50r/s` (or reuses existing definition).
- Obtains certificate with `certbot --nginx -d staging.futurelab.studio` if missing.
- Enables site and reloads Nginx.

## 7. Configure local htmlctl context

Trust host key locally first:

```bash
ssh-keyscan -H 65.21.0.203 >> ~/.ssh/known_hosts
```

Write `~/.htmlctl/config.yaml`:

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://htmlservd@65.21.0.203:22
    website: futurelab
    environment: staging
    port: 9400
    token: <HTMLSERVD_API_TOKEN value from /etc/htmlservd/env>
```

## 8. Verification

Run these commands in order:

```bash
# 1. Check service is running
ssh hetzner "systemctl status htmlservd"

# 2. Check health endpoint via tunnel
ssh hetzner "curl -sf http://127.0.0.1:9400/healthz"

# 3. Apply a hello-world site
htmlctl apply -f site/ --context staging

# 4. Check staging URL (after DNS + cert)
curl -I https://staging.futurelab.studio/
```

Expected outcomes:

- `htmlservd` status is `active (running)`.
- Health endpoint returns HTTP 200.
- `htmlctl apply` succeeds over SSH tunnel to `127.0.0.1:9400`.
- `https://staging.futurelab.studio/` returns 200/304 with TLS valid.

## 9. Operations: status and logs

```bash
ssh hetzner "sudo systemctl status htmlservd"
ssh hetzner "sudo journalctl -u htmlservd -n 200 --no-pager"
ssh hetzner "sudo nginx -t && sudo systemctl status nginx"
```

Notes:

- No UFW change is needed for `9400` (stays loopback only).
- `htmlctl` uses SSH port forwarding over `22`.
- This Phase 1 keeps Nginx terminating TLS for staging; Caddy takeover happens in a later phase.
