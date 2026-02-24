---
name: htmlctl-publish
description: Publish content to an htmlctl-managed website. Use when an agent needs to create or update pages, components, styles, or assets on a site managed by htmlctl/htmlservd. Handles both agent-driven content updates (apply directly to staging, then promote to prod) and structural changes (test locally with Docker first). The server holds all desired state; no git repository is required for content management.
---

# htmlctl Publish

Publish pages, components, and styles to a site managed by `htmlctl` / `htmlservd`.

## Deployment Model

The server is the source of truth. `htmlservd` stores all desired state in SQLite and maintains a full release history with rollback support. No git repository is required for day-to-day content management.

Two workflows cover all scenarios:

| Change type | Workflow |
|-------------|----------|
| Agent content update (copy, cards, small edits to existing components) | Apply directly to staging → verify → promote to prod |
| Structural change (new page, layout redesign, style overhaul, new component) | Test locally with Docker → apply to staging → promote to prod |

## Prerequisites

A context must be configured in `~/.htmlctl/config.yaml` (or `HTMLCTL_CONFIG`):

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://user@yourserver
    website: mysite
    environment: staging
    token: "<api-token>"
  - name: prod
    server: ssh://user@yourserver
    website: mysite
    environment: prod
    token: "<api-token>"
```

The SSH key must be loaded (`ssh-add`) or available at `HTMLCTL_SSH_KEY_PATH`.
If the Docker test container is recreated, regenerate `known_hosts` to avoid host-key mismatch errors.

## Workflow A — Agent Content Update

Use for targeted changes: updating copy, adding a project card, editing a single component.

1. Write or edit the file(s) locally.
2. Apply the changed file(s) directly to staging:
   ```bash
   htmlctl apply -f components/projects.html --context staging
   ```
   The server merges the change into the current desired state, re-renders, and creates a new release — without touching unchanged resources.
3. Verify the change on the staging URL.
4. Promote to prod (copies the exact release artifact, no rebuild):
   ```bash
   htmlctl promote website/<name> --from staging --to prod
   ```

## Workflow B — Structural Change

Use for new pages, layout changes, style overhauls, or any change touching many files at once.

1. Start a local Docker server for fast, safe iteration:
   ```bash
   API_TOKEN="$(htmlctl context token generate)"
   mkdir -p .tmp/htmlctl-publish/{data,caddy}

   docker rm -f htmlservd-local >/dev/null 2>&1 || true
   docker run -d \
     --name htmlservd-local \
     -p 23222:22 -p 19420:9400 -p 18080:80 \
     -e SSH_PUBLIC_KEY="$(cat ~/.ssh/id_ed25519.pub)" \
     -e HTMLSERVD_API_TOKEN="$API_TOKEN" \
     -e HTMLSERVD_CADDY_BOOTSTRAP_MODE=preview \
     -e HTMLSERVD_PREVIEW_WEBSITE=<website-name> \
     -e HTMLSERVD_PREVIEW_ENV=staging \
     -e HTMLSERVD_CADDY_AUTO_HTTPS=false \
     -e HTMLSERVD_TELEMETRY_ENABLED=true \
     -v "$PWD/.tmp/htmlctl-publish/data:/var/lib/htmlservd" \
     -v "$PWD/.tmp/htmlctl-publish/caddy:/etc/caddy" \
     htmlservd-ssh:local

   ssh-keyscan -p 23222 -H 127.0.0.1 > .tmp/known_hosts
   ```
   Health check: `curl -sf http://127.0.0.1:19420/healthz`

2. Create a local context entry pointing at the Docker instance.

3. Apply the full site and verify on a bound hostname:
   ```bash
   htmlctl apply -f site/ --context local-docker
   htmlctl domain add 127.0.0.1.nip.io --context local-docker
   ```
   Open `http://127.0.0.1.nip.io:18080` (prefer hostname over raw `127.0.0.1` when validating telemetry attribution).

   > **Note:** Caddy uses virtual hosting — `curl http://127.0.0.1:18080/` returns an empty body because `Host: 127.0.0.1` matches no vhost. Always use the bound hostname for verification:
   > ```bash
   > curl -sf -H "Host: 127.0.0.1.nip.io" http://127.0.0.1:18080/ | grep "<title>"
   > # or open the nip.io URL directly in a browser
   > ```

4. Iterate locally until satisfied, then apply to the real staging environment:
   ```bash
   htmlctl apply -f site/ --context staging
   ```

5. Verify on the staging URL, then promote to prod:
   ```bash
   htmlctl promote website/<name> --from staging --to prod
   ```

6. Stop the local Docker instance:
   ```bash
   docker rm -f htmlservd-local
   ```

## Resource Schemas

See `references/resource-schemas.md` for YAML schemas and full examples for all resource kinds: Website, Page (including SEO metadata), Component, and StyleBundle.

## Command Reference

See `references/commands.md` for all htmlctl commands: apply, diff, status, promote, rollback, domain, and context management.

## Rollback

To undo a production deploy:
```bash
htmlctl rollout undo website/<name> --context prod
```

To view release history before rolling back:
```bash
htmlctl rollout history website/<name> --context prod
```
