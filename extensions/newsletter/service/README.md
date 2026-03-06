# htmlctl-newsletter service

Reference newsletter foundation service for the official `newsletter` extension.

This service is intentionally separate from `htmlctl` and `htmlservd` runtime. Operators deploy it independently and route public newsletter paths through Epic 9 backends (for example `/newsletter/*`).

## Commands

```bash
# Apply migrations using environment variables
NEWSLETTER_ENV=staging NEWSLETTER_DATABASE_URL='postgres://...' \
  go run ./cmd/htmlctl-newsletter migrate

# Run service (loopback only)
NEWSLETTER_ENV=staging \
NEWSLETTER_HTTP_ADDR=127.0.0.1:9501 \
NEWSLETTER_DATABASE_URL='postgres://...' \
NEWSLETTER_PUBLIC_BASE_URL='https://staging.example.com' \
NEWSLETTER_RESEND_API_KEY='re_xxx' \
  go run ./cmd/htmlctl-newsletter serve
```

## Required Environment Variables

- `NEWSLETTER_ENV` (`staging` or `prod`)
- `NEWSLETTER_DATABASE_URL`
- `NEWSLETTER_PUBLIC_BASE_URL` (must be an `https://` origin with no path/query/fragment)
- `NEWSLETTER_RESEND_API_KEY`

## Optional Environment Variables

- `NEWSLETTER_HTTP_ADDR` (defaults by env: `127.0.0.1:9501` for staging, `127.0.0.1:9502` for prod)

## Security Defaults

- Listener must be loopback-only (`localhost`, `127.0.0.1`, or `::1`).
- Migration schema stores only `token_hash` for verification flows.
- Service startup logs never print secret env values.
- Staging and prod must use separate DB URLs and credentials.

## Build Artifact

```bash
cd extensions/newsletter/service
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
  -o dist/htmlctl-newsletter-linux-amd64 ./cmd/htmlctl-newsletter
```
