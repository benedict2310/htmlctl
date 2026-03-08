# htmlctl-newsletter service

Reference newsletter service for the official `newsletter` extension.

This service is intentionally separate from `htmlctl` and `htmlservd` runtime. Operators deploy it independently and route public newsletter paths through Epic 9 backends (for example `/newsletter/*`).

## Commands

```bash
# Apply migrations using environment variables
NEWSLETTER_ENV=staging NEWSLETTER_DATABASE_URL='postgres://...' \
  go run ./cmd/htmlctl-newsletter migrate

# Import subscribers from a legacy database schema with a `subscribers` table
NEWSLETTER_ENV=prod \
NEWSLETTER_DATABASE_URL='postgres://target...' \
  go run ./cmd/htmlctl-newsletter import-legacy --source-database-url 'postgres://source...'

# Store or update campaign content from files
NEWSLETTER_ENV=staging \
NEWSLETTER_DATABASE_URL='postgres://...' \
  go run ./cmd/htmlctl-newsletter campaign upsert \
    --slug launch \
    --subject 'Launch update' \
    --html-file ./campaigns/launch.html \
    --text-file ./campaigns/launch.txt

# Send a preview to a single inbox
NEWSLETTER_ENV=staging \
NEWSLETTER_DATABASE_URL='postgres://...' \
NEWSLETTER_PUBLIC_BASE_URL='https://staging.example.com' \
NEWSLETTER_RESEND_API_KEY='re_xxx' \
NEWSLETTER_RESEND_FROM='Team <newsletter@example.com>' \
NEWSLETTER_LINK_SECRET='replace-with-32-plus-random-chars' \
  go run ./cmd/htmlctl-newsletter campaign preview --slug launch --to you@example.com

# Send to the full confirmed audience with pacing
NEWSLETTER_ENV=prod \
NEWSLETTER_DATABASE_URL='postgres://...' \
NEWSLETTER_PUBLIC_BASE_URL='https://example.com' \
NEWSLETTER_RESEND_API_KEY='re_xxx' \
NEWSLETTER_RESEND_FROM='Team <newsletter@example.com>' \
NEWSLETTER_LINK_SECRET='replace-with-32-plus-random-chars' \
  go run ./cmd/htmlctl-newsletter campaign send --slug launch --mode all --interval 30s --confirm

# Run service (loopback only)
NEWSLETTER_ENV=staging \
NEWSLETTER_HTTP_ADDR=127.0.0.1:9501 \
NEWSLETTER_DATABASE_URL='postgres://...' \
NEWSLETTER_PUBLIC_BASE_URL='https://staging.example.com' \
NEWSLETTER_RESEND_API_KEY='re_xxx' \
NEWSLETTER_RESEND_FROM='Team <newsletter@example.com>' \
NEWSLETTER_LINK_SECRET='replace-with-32-plus-random-chars' \
  go run ./cmd/htmlctl-newsletter serve
```

## Required Environment Variables

- `NEWSLETTER_ENV` (`staging` or `prod`)
- `NEWSLETTER_DATABASE_URL`
- `NEWSLETTER_PUBLIC_BASE_URL` (must be an `https://` origin with no path/query/fragment)
- `NEWSLETTER_RESEND_FROM` (must parse as a valid RFC 5322 sender address, for example `Team <newsletter@example.com>`)
- `NEWSLETTER_LINK_SECRET` (minimum 32 characters; use a random URL-safe secret)

## Optional Environment Variables

- `NEWSLETTER_HTTP_ADDR` (defaults by env: `127.0.0.1:9501` for staging, `127.0.0.1:9502` for prod)
- `NEWSLETTER_RESEND_API_KEY` (required for real verification/campaign delivery; staging may omit it only when no outbound mail should be sent)

## Security Defaults

- Listener must be loopback-only (`localhost`, `127.0.0.1`, or `::1`).
- Migration schema stores only `token_hash` for verification flows.
- Campaign emails always append a signed unsubscribe link generated from `NEWSLETTER_LINK_SECRET`.
- Weak unsubscribe secrets are rejected at startup; provision a high-entropy secret per environment.
- Send tracking is idempotent at the campaign-recipient level; successful recipients are not resent on retries.
- Service startup logs never print secret env values.
- Staging and prod must use separate DB URLs and credentials.

## Build Artifact

```bash
cd extensions/newsletter/service
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
  -o dist/htmlctl-newsletter-linux-amd64 ./cmd/htmlctl-newsletter
```
