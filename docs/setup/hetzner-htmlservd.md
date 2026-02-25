# VPS Phase 1 Setup: htmlservd + staging.example.com

This runbook installs `htmlservd` on an existing Ubuntu 24.04 ARM64 VPS host and adds a staging static site vhost in Nginx without touching the running production stack.

## Prerequisites

- Server: VPS host (`203.0.113.10`), Ubuntu 24.04 ARM64.
- SSH alias exists locally:

```sshconfig
Host staging-vps
  HostName 203.0.113.10
  User deploy
```

- Existing services remain in place and must stay untouched:
- Nginx serving `example.com`.
- Next.js/PM2 on `:3000`.
- PostgreSQL local.
- Local tooling installed: Go toolchain, `scp`, `ssh`, and `htmlctl`.

## 1. Build htmlservd for linux/arm64

From your local `htmlctl` repo root (run this after every `git pull` to avoid using a stale binary):

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o htmlservd-linux-arm64 ./cmd/htmlservd
```

Also rebuild the local `htmlctl` client if you use it from `bin/`:

```bash
go build -o bin/htmlctl ./cmd/htmlctl
```

## 2. Upload/install binary on VPS

```bash
scp htmlservd-linux-arm64 staging-vps:/tmp/htmlservd && ssh staging-vps "sudo mv /tmp/htmlservd /usr/local/bin/htmlservd && sudo chmod 755 /usr/local/bin/htmlservd"
```

## 3. Run htmlservd host setup script

Copy scripts to the server (if needed):

```bash
scp scripts/setup-htmlservd-hetzner.sh scripts/setup-staging-nginx.sh staging-vps:/tmp/
```

Run `htmlservd` setup with your local SSH public key (required for tunnel login as `htmlservd`):

```bash
ssh staging-vps "export HTMLSERVD_SSH_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' && bash /tmp/setup-htmlservd-hetzner.sh"
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
ssh staging-vps "sudoedit /etc/htmlservd/env"
```

Expected line:

```bash
HTMLSERVD_API_TOKEN=REPLACE_WITH_LONG_RANDOM_TOKEN
```

Restart service after setting token:

```bash
ssh staging-vps "sudo systemctl restart htmlservd"
```

## 5. Add DNS for staging domain

Create DNS record:

- Type: `A`
- Name: `staging.example.com`
- Value: `203.0.113.10`
- TTL: default (or 300 during rollout)

Wait for propagation before cert issuance.

## 6. Configure Nginx staging vhost + TLS

Run the Nginx setup script:

```bash
ssh staging-vps "bash /tmp/setup-staging-nginx.sh"
```

What it does:

- Creates `/etc/nginx/sites-available/staging.example.com`.
- Serves static files from `/var/lib/htmlservd/websites/sample/envs/staging/current`.
- Ensures `general_limit` rate-limit zone exists at `50r/s` (or reuses existing definition).
- Obtains certificate with `certbot --nginx -d staging.example.com` if missing.
- Enables site and reloads Nginx.

## 7. Configure local htmlctl context

Trust host key locally first:

```bash
ssh-keyscan -H 203.0.113.10 >> ~/.ssh/known_hosts
```

Write `~/.htmlctl/config.yaml`:

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://htmlservd@203.0.113.10:22
    website: sample
    environment: staging
    port: 9400
    token: <HTMLSERVD_API_TOKEN value from /etc/htmlservd/env>
```

## 8. Verification

Run these commands in order:

```bash
# 1. Check service is running
ssh staging-vps "systemctl status htmlservd"

# 2. Check health endpoint via tunnel
ssh staging-vps "curl -sf http://127.0.0.1:9400/healthz"

# 3. Apply a hello-world site
htmlctl apply -f site/ --context staging

# 4. Check staging URL (after DNS + cert)
curl -I https://staging.example.com/
```

Expected outcomes:

- `htmlservd` status is `active (running)`.
- Health endpoint returns HTTP 200.
- `htmlctl apply` succeeds over SSH tunnel to `127.0.0.1:9400`.
- `https://staging.example.com/` returns 200/304 with TLS valid.

## 9. Operations: status and logs

```bash
ssh staging-vps "sudo systemctl status htmlservd"
ssh staging-vps "sudo journalctl -u htmlservd -n 200 --no-pager"
ssh staging-vps "sudo nginx -t && sudo systemctl status nginx"
```

Notes:

- No UFW change is needed for `9400` (stays loopback only).
- `htmlctl` uses SSH port forwarding over `22`.
- This Phase 1 keeps Nginx terminating TLS for staging; Caddy takeover happens in a later phase.
