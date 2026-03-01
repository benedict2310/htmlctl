# htmlctl Troubleshooting & Operational Safety

## Common Failure Modes

### SSH / Transport

| Error | Cause | Fix |
|-------|-------|-----|
| `ssh host key verification failed` | Container was recreated, host key changed | `ssh-keyscan -p <port> -H <host> > known_hosts` |
| `unable to authenticate, attempted methods [none publickey]` | Stale binary (pre-E8) or no key available | Rebuild `htmlctl`: `go build -o bin/htmlctl ./cmd/htmlctl`; or `ssh-add ~/.ssh/id_ed25519` |
| `ssh agent unavailable` | `SSH_AUTH_SOCK` not set | Set `HTMLCTL_SSH_KEY_PATH` or add key with `ssh-add` |
| `connection refused` | Wrong port, container not running | Check `HTMLSERVD_PORT` and container status |

### Authentication

| Error | Cause | Fix |
|-------|-------|-----|
| `request failed (401)` / `unauthorized` | Token mismatch | Verify context `token` matches `HTMLSERVD_API_TOKEN` on server |
| Server starts with auth warning | No API token configured | Set `HTMLSERVD_API_TOKEN` or `api.token` in config |

### Apply / Release

| Error | Cause | Fix |
|-------|-------|-----|
| `environment not found` on promote | Target env doesn't exist yet | Run `htmlctl apply -f site/ --context prod` once to bootstrap, then use `promote` |
| Validation error: multiple root elements | Component has more than one root tag | Wrap in a single `<div>` or `<section>` |
| Validation error: script tag in component | `<script>` inside a component file | Move JS to `scripts/site.js` |
| Validation error: event handler attribute | `onclick`, `onload`, etc. in component | Remove inline handlers; use `site.js` instead |
| `apply -f` fails on individual file | Wrong path or file outside site dir | Provide correct relative path; `apply -f` accepts both files and directories |
| `robots.txt` or `sitemap.xml` is 404 after successful apply | Server binary predates website SEO feature | Upgrade `htmlservd`, restart it, then re-apply the site |
| favicon files missing after successful apply | `branding/` files not configured or server binary predates website icon feature | Add `spec.head.icons` plus `branding/*`, or upgrade `htmlservd` and re-apply |

### Telemetry

| Error | Cause | Fix |
|-------|-------|-----|
| No telemetry rows after page load | Browsing raw IP, not hostname | Use bound hostname: `http://127.0.0.1.nip.io:18080/` not `http://127.0.0.1:18080/` |
| `400` on telemetry ingest | Host not in domain bindings | `htmlctl domain add <hostname> --context ...` first |
| Telemetry endpoint missing | `HTMLSERVD_TELEMETRY_ENABLED` not set | Set `HTMLSERVD_TELEMETRY_ENABLED=true` and restart |

### Docker / Local

| Error | Cause | Fix |
|-------|-------|-----|
| Site not updating in preview | Preview env vars don't match context | Confirm `HTMLSERVD_PREVIEW_WEBSITE` and `HTMLSERVD_PREVIEW_ENV` match context `website`/`environment` |
| Empty body from `curl http://127.0.0.1:18080/` | Caddy virtual hosting; no domain bound | Bind a domain: `htmlctl domain add 127.0.0.1.nip.io --context ...` |
| Permission errors under `.tmp` | `HOME` overridden to bind-mounted path | Use `HTMLCTL_SSH_KNOWN_HOSTS_PATH` instead of relying on `~/.ssh` |

---

## Pre-Apply Safety Checklist

```bash
# 1. Confirm you're on the right context
htmlctl config current-context

# 2. Preview changes before applying
htmlctl diff -f site/ --context staging

# 3. For risky changes, dry-run first (validates + shows diff, no release created)
htmlctl apply -f site/ --context staging --dry-run

# 4. Apply
htmlctl apply -f site/ --context staging

# 5. Verify
htmlctl status website/mysite --context staging
htmlctl logs website/mysite --context staging --limit 50
curl -sf https://staging.example.com/robots.txt
curl -sf https://staging.example.com/sitemap.xml
```

---

## Pre-Promote Safety Checklist

```bash
# 1. Validate staging end-to-end (open URL, check functionality)

# 2. Review release history
htmlctl rollout history website/mysite --context staging

# 3. Promote
htmlctl promote website/mysite --from staging --to prod

# 4. Verify prod
htmlctl status website/mysite --context prod
htmlctl logs website/mysite --context prod --limit 20

# 5. Keep rollback ready
htmlctl rollout undo website/mysite --context prod
```

---

## Domain Operations Notes

- `domain add` triggers Caddy config regeneration and reload. If `caddyAutoHTTPS=true`, Caddy will attempt ACME certificate issuance for the domain.
- ACME cert issuance requires public DNS pointing to the server and ports 80/443 open.
- In local/dev environments with `HTMLSERVD_CADDY_AUTO_HTTPS=false`, TLS is not attempted; domain blocks use `http://` addresses.
- `domain verify` checks DNS resolution and TLS certificate validity. Both must pass.
- Concurrent add/delete operations on the same domain are serialized server-side.
- If Caddy reload fails during `domain remove`, the binding is automatically restored in DB (metadata preserved).

---

## Operational Safety Rules

1. **Never skip diff before apply** on environments with live traffic.
2. **Always bootstrap before promote**: `promote` requires the target environment to already exist (created by a prior `apply`).
3. **Rollback is always available**: `rollout undo` switches the active pointer to the previous release in < 1 second.
4. **Token security**: never log or expose `HTMLSERVD_API_TOKEN`. Use constant-time comparison (enforced server-side).
5. **SSH host key**: never disable host-key verification. If the key changes, regenerate `known_hosts` with `ssh-keyscan`.
6. **Data backup**: snapshot `/var/lib/htmlservd` (DB + blobs) before high-risk operations.
7. **Telemetry host attribution**: keep `htmlservd` bound to loopback and route telemetry through Caddy for accurate `Host`-based environment resolution.
8. **Server/client feature parity**: when a feature affects release materialization, upgrade `htmlservd` before relying on it in a site apply.
