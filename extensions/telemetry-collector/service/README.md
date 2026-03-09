# htmlctl-telemetry-collector service

Reference browser telemetry collector for the official `telemetry-collector` extension.

This service is intentionally separate from `htmlctl` and `htmlservd`. Operators deploy it independently and route public telemetry paths through Epic 9 backends (for example `/site-telemetry/*`).

## Commands

```bash
# Run service (loopback only)
TELEMETRY_COLLECTOR_ENV=staging \
TELEMETRY_COLLECTOR_HTTP_ADDR=127.0.0.1:9601 \
TELEMETRY_COLLECTOR_PUBLIC_BASE_URL='https://staging.example.com' \
TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL='http://127.0.0.1:9400' \
TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN='replace-with-htmlservd-token' \
  go run ./cmd/htmlctl-telemetry-collector serve
```

## Required Environment Variables

- `TELEMETRY_COLLECTOR_ENV` (`staging` or `prod`)
- `TELEMETRY_COLLECTOR_PUBLIC_BASE_URL` (must be an `http://` or `https://` origin with no path/query/fragment; real staging/prod deployments should use `https://`)
- `TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN`

## Optional Environment Variables

- `TELEMETRY_COLLECTOR_HTTP_ADDR` (defaults by env: `127.0.0.1:9601` for staging, `127.0.0.1:9602` for prod)
- `TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL` (defaults to `http://127.0.0.1:9400`; must remain loopback-only)
- `TELEMETRY_COLLECTOR_ALLOWED_EVENTS`
- `TELEMETRY_COLLECTOR_MAX_BODY_BYTES`
- `TELEMETRY_COLLECTOR_MAX_EVENTS`

## Security Defaults

- Listener must be loopback-only (`localhost`, `127.0.0.1`, or `::1`).
- Public requests must match the configured `TELEMETRY_COLLECTOR_PUBLIC_BASE_URL` host and scheme.
- Accepted event names default to `page_view`, `link_click`, `cta_click`, and `newsletter_signup` only.
- Public ingest is rate-limited per client IP.
- The htmlservd bearer token stays server-side only.
- Upstream htmlservd 5xx responses are sanitized before being returned to browsers.
- Forwarded requests preserve the original public host so htmlservd can attribute events to the right environment.

## Build Artifact

```bash
cd extensions/telemetry-collector/service
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
  -o dist/htmlctl-telemetry-collector-linux-amd64 ./cmd/htmlctl-telemetry-collector
```
