# htmlservd HTTP API Reference

All `/api/v1/*` routes require `Authorization: Bearer <token>` when the server is configured with an API token.
Health and version endpoints are unauthenticated.

Base URL: `http://127.0.0.1:9400` (via SSH tunnel) or direct if on the server.

---

## Health & Version

```http
GET /healthz
GET /readyz
GET /version
```

No authentication required. Used for startup checks and load-balancer probes.

---

## Websites & Environments

```http
GET /api/v1/websites
GET /api/v1/websites/{website}/environments
```

---

## Apply (deploy desired state)

```http
POST /api/v1/websites/{website}/environments/{env}/apply
```

Uploads a bundle (manifests + files). The server validates, renders, creates an immutable release, and atomically switches the `current` pointer.

---

## Releases

```http
# List releases for an environment
GET /api/v1/websites/{website}/environments/{env}/releases

# Create a release (triggered by apply — prefer using htmlctl apply)
POST /api/v1/websites/{website}/environments/{env}/releases
```

---

## Rollback

```http
POST /api/v1/websites/{website}/environments/{env}/rollback
```

Activates the previous release (symlink switch, < 1 second).

---

## Promote

```http
POST /api/v1/websites/{website}/promote
```

Copies the exact staging release artifact bytes to prod and activates them. No rebuild.

---

## Status & Manifest

```http
GET /api/v1/websites/{website}/environments/{env}/status
GET /api/v1/websites/{website}/environments/{env}/manifest
```

---

## Logs

```http
GET /api/v1/websites/{website}/environments/{env}/logs
```

---

## Domains

```http
# List all domain bindings
GET /api/v1/domains

# Add a domain binding (triggers Caddy config regeneration + reload)
POST /api/v1/domains

# Get a specific binding
GET /api/v1/domains/{domain}

# Remove a domain binding
DELETE /api/v1/domains/{domain}
```

Domain add/remove operations update SQLite, regenerate the Caddyfile, and reload Caddy safely. If Caddy reload fails, the database change is rolled back (metadata preserved).

---

## Telemetry

### Ingest (unauthenticated, same-origin only)

```http
POST /collect/v1/events
Content-Type: application/json   # or text/plain (sendBeacon-compatible)
```

Request body:

```json
{
  "events": [
    {
      "name": "page_view",
      "path": "/about",
      "occurredAt": "2026-02-25T10:00:00Z",
      "sessionId": "sess_abc123",
      "attrs": {
        "source": "browser",
        "referrer": "https://google.com"
      }
    }
  ]
}
```

Limits (defaults, configurable):
- Max body: 64 KiB
- Max events per request: 50
- Event name: `[a-zA-Z0-9][a-zA-Z0-9_-]*`, max 64 chars
- Max 16 attrs/event; key ≤ 64 bytes; value ≤ 256 bytes

Trust model: the `Host` header is used to resolve the environment. Requests from unbound hosts return `400`. Route telemetry through Caddy (not directly to `htmlservd`) for accurate host attribution.

### Query (authenticated)

```http
GET /api/v1/websites/{website}/environments/{env}/telemetry/events
Authorization: Bearer <token>
```

Query parameters:

| Parameter | Description |
|-----------|-------------|
| `event` | Filter by event name |
| `since` | RFC3339 timestamp lower bound |
| `until` | RFC3339 timestamp upper bound |
| `limit` | Max rows to return |
| `offset` | Pagination offset |

Example:

```bash
curl -sS \
  -H "Authorization: Bearer $API_TOKEN" \
  "http://127.0.0.1:9400/api/v1/websites/mysite/environments/prod/telemetry/events?event=page_view&limit=100"
```

Telemetry rows older than `retentionDays` (default 90) are automatically deleted by a background cleanup job. Set `retentionDays: 0` to disable.

---

## Audit Log

All apply, rollback, promote, and domain operations are recorded with:
- Actor identity (SSH principal or user ID)
- Timestamp
- Environment
- Resource change summary (hashes)
- Release ID activated
