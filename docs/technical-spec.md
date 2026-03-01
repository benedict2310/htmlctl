# htmlctl / htmlservd — Technical Specification

## 1. Repository layout (agent-facing)

```
site/
  website.yaml
  branding/
    favicon.svg
    favicon.ico
    apple-touch-icon.png
  pages/
    index.page.yaml
    product.page.yaml
  components/
    header.html
    hero.html
    features.html
    pricing.html
    faq.html
    footer.html
  styles/
    tokens.css
    default.css
  scripts/
    site.js            # optional global JS
  assets/
    hero.jpg
    logo.svg
```

Agent rule: edit `components/*`, `styles/*`, `assets/*` 99% of the time.

## 2. Resource model

All resources are stored in sqlite (metadata/manifests/audit), with blobs/releases on filesystem.

### 2.1 Website

- `metadata.name`: string
- `spec.defaultStyleBundle`: string
- `spec.baseTemplate`: string (built-in template name)
- `spec.head` (optional): website-scoped head metadata
  - `icons` (optional):
    - `svg`: path under `branding/` for the SVG favicon source
    - `ico`: path under `branding/` for the ICO favicon source
    - `appleTouch`: path under `branding/` for the Apple touch icon source
- `spec.seo` (optional): website-scoped crawl metadata
  - `publicBaseURL`: canonical absolute `http(s)` site URL used by generated crawl artifacts
  - `robots` (optional):
    - `enabled`: generate `/robots.txt` when `true`
    - `groups`: ordered crawler policy groups
      - `userAgents`: ordered user-agent values
      - `allow`: ordered path-prefix rules
      - `disallow`: ordered path-prefix rules
  - `sitemap` (optional):
    - `enabled`: generate `/sitemap.xml` when `true`

Example:

```yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
      ico: branding/favicon.ico
      appleTouch: branding/apple-touch-icon.png
  seo:
    publicBaseURL: https://example.com/
    robots:
      enabled: true
      groups:
        - userAgents:
            - "*"
          allow:
            - /
          disallow:
            - /preview/
            - /drafts/
    sitemap:
      enabled: true
```

### 2.2 Environment

Logical instance of a website:

- `website`: reference
- `name`: `staging|prod`
- `activeReleaseId`
- domain bindings

### 2.3 Page

- `spec.route`: string (`/`, `/product`)
- `spec.title`, `spec.description`
- `spec.layout`: ordered list of `include` references
- `spec.head` (optional): server-rendered SEO/share metadata
  - `canonicalURL`: canonical URL value for `<link rel="canonical">`
  - `meta`: map of `<meta name="...">` values (for example `keywords`, `robots`, `author`, `application-name`)
  - `openGraph`: typed `og:*` values (`type`, `url`, `siteName`, `locale`, `title`, `description`, `image`)
  - `twitter`: typed `twitter:*` values (`card`, `url`, `title`, `description`, `image`)
  - `jsonLD`: ordered list of JSON-LD blocks (`id`, `payload`)

Example:

```yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: product
spec:
  route: /product
  title: "Sample — Product"
  description: "..."
  layout:
    - include: header
    - include: hero
    - include: features
    - include: pricing
    - include: faq
    - include: footer
  head:
    canonicalURL: https://example.com/product
    meta:
      robots: index,follow
      keywords: Sample product
    openGraph:
      type: website
      url: https://example.com/product
      title: Sample Product
      description: Product details
      image: https://example.com/assets/product/og-image.jpg
    twitter:
      card: summary_large_image
      title: Sample Product
      description: Product details
      image: https://example.com/assets/product/og-image.jpg
    jsonLD:
      - id: product
        payload:
          "@context": https://schema.org
          "@type": Product
          name: Sample Product
```

### 2.4 Component

Stored as HTML fragment, strongly validated.

- `name`: string (e.g., `pricing`)
- `scope`: default global; optional page-scoped later
- `html`: string (file content)
- optional: `cssFragment` (post-v1)
- optional: `jsFragment` (post-v1) — in v1, JS is via `scripts/site.js`

Component file rule:

- **Exactly one root element**.
- For anchor-nav components, root must include `id="<componentName>"`.

Example `components/pricing.html`:

```html
<section id="pricing">
  <h2>Pricing</h2>
</section>
```

### 2.5 StyleBundle

In v1, a single bundle `default`:

- `styles/tokens.css` (CSS vars)
- `styles/default.css` (base styles)

### 2.6 Asset

- content-addressed storage by sha256
- metadata: original filename, content-type, size

### 2.7 Release (immutable)

- ID: timestamp + random suffix or ULID
- Contains:
  - snapshot of manifests
  - rendered output directory
  - build log
  - hashes of outputs

### 2.8 Telemetry event (optional platform capability)

- Public ingest endpoint: `POST /collect/v1/events`
- Authenticated read endpoint:
  - `GET /api/v1/websites/{website}/environments/{env}/telemetry/events`
- Stored fields:
  - `environment_id` (resolved from request host via domain binding)
  - `event_name`
  - `path` (normalized absolute path)
  - `occurred_at` (optional client timestamp, validated RFC3339)
  - `received_at` (server timestamp)
  - `session_id` (optional, validated identifier)
  - `attrs_json` (validated key/value metadata map)
- Ingest validation defaults:
  - request body <= 64 KiB (`telemetry.maxBodyBytes`)
  - max 50 events/request (`telemetry.maxEvents`)
  - event names match `[a-zA-Z0-9][a-zA-Z0-9_-]*`, max 64 chars
  - max 16 attrs/event; attr key <= 64 bytes; attr value <= 256 bytes
  - accepted request body content types: `application/json` and `text/plain` (sendBeacon-compatible)
  - `telemetry.maxBodyBytes: 0` and `telemetry.maxEvents: 0` mean "use server defaults" (not unlimited)
- Query filters:
  - `event`, `since`, `until`, `limit`, `offset`
- Retention:
  - background cleanup deletes rows older than `telemetry.retentionDays` (default 90)
  - set `retentionDays: 0` to disable automatic deletion

## 3. Rendering & composition

### 3.1 Publish-time stitching

Pages are not full HTML docs; they are layouts that reference components. Renderer produces full HTML:

- Base template provides `<html><head>...</head><body><main>{{content}}</main>...</body></html>`
- Layout includes inserted in order into `<main>`
- `scripts/site.js` injected at end of body if present
- Stylesheets injected into `<head>`
- `spec.head` metadata is rendered directly into `<head>` (no runtime JS injection path)
- Website icon files from `branding/` are copied verbatim into conventional root paths during release materialization:
  - `/favicon.svg`
  - `/favicon.ico`
  - `/apple-touch-icon.png`
- `robots.txt` is generated during release materialization from `website.yaml spec.seo.robots` and written to `/robots.txt` when enabled
- `sitemap.xml` is generated during release materialization from `website.yaml spec.seo.sitemap` and declared pages when enabled
- sitemap URL selection is deterministic:
  - pages default to `publicBaseURL + route`
  - relative `canonicalURL` values override the derived route URL when they resolve within the configured public-base scope
  - absolute `canonicalURL` values are used only when they match the configured canonical public scheme+host and stay within the configured public-base path scope
  - pages with `head.meta.robots` containing `noindex` or `none` are excluded
- `robots.txt` generation is deterministic:
  - default allow-all policy when `enabled: true` and no groups are defined
  - groups emitted in input order
  - within each group: `User-agent`, then `Allow`, then `Disallow`, all in input order
  - appends a single `Sitemap: .../sitemap.xml` line when both robots and sitemap are enabled
  - LF-only output, no request-time generation
- Favicon support introduces no generation/transcoding step: htmlctl/htmlservd never resize, convert, or synthesize icon variants
- Head metadata render order is deterministic:
  1. website icon links
  2. canonical link
  3. `meta[name]` tags sorted by `name`
  4. Open Graph tags in fixed field order
  5. Twitter tags in fixed field order
  6. JSON-LD blocks in manifest order

### 3.2 Determinism requirements

- Stable ordering for injections (styles, scripts)
- Stable ordering for head metadata tags and JSON-LD block emission
- Normalized line endings (LF)
- No time-dependent output
- Asset names are content-addressed

## 4. Release pipeline (atomic)

For any environment apply:

1. Receive bundle (manifests + files)
2. Validate (see section 6)
3. Render to `releases/<id>/tmp` (not served)
4. Finalize: rename tmp -> final directory
5. Atomically switch active pointer: `current -> releases/<id>` symlink
6. Append audit entry

**Rollback:** switch `current` back to previous release.

**Promotion (staging -> prod):** copy/link the **exact** release artifact bytes from staging into prod release store, then activate it in prod.

## 5. Storage layout (server)

```
/var/lib/htmlservd/
  db.sqlite
  blobs/
    sha256/<hash>
  websites/sample/
    envs/staging/
      releases/<releaseId>/
      current -> releases/<releaseId>/
    envs/prod/
      releases/<releaseId>/
      current -> releases/<releaseId>/
```

## 6. Validation rules (production safety)

### 6.1 Component validation

- Exactly one root element.
- Root tag allowlist: `section|header|footer|main|nav|article|div` (configurable).
- If component is anchor-navigable: root must include `id="<componentName>"`.
- In v1, disallow `<script>` tags and inline event handler attributes matching `(?i)^on\w+$` in components (JS only from `scripts/site.js`).

### 6.2 Page validation

- routes normalized
- all includes exist
- prevent cycles (component cannot include other components in v1)
- metadata URL fields in `spec.head` allow only relative URLs or `http(s)` schemes

### 6.3 Asset validation

- sanitize filenames
- size limits
- content-type allowlist
- prevent path traversal

### 6.4 Bundle validation

- verify hashes from client bundle manifest

## 7. Control plane security model

- `htmlservd` binds to **127.0.0.1 only** by default.
- Remote access is via **SSH tunnel** (recommended) or reverse-proxy private network.
- SSH host keys are verified against `known_hosts`; insecure host-key bypass is not supported in production transport APIs.
- API authentication in v1:
  - all `/api/v1/*` routes require `Authorization: Bearer <token>` when `api.token` (or `HTMLSERVD_API_TOKEN`) is configured.
  - health routes (`/healthz`, `/readyz`) remain unauthenticated.
  - telemetry ingest `POST /collect/v1/events` is intentionally unauthenticated and outside `/api/v1/*`.
  - if no API token is configured, the server starts with a prominent warning for rollout safety.
  - operators can enforce token configuration at startup via `htmlservd --require-auth`.
- token comparison uses constant-time checks (`crypto/subtle`).
- telemetry host attribution trust model:
  - ingest host is accepted only when it matches an existing domain binding.
  - unbound hosts return `400`.
  - ingest is same-origin only in v1; cross-origin CORS preflight is intentionally not supported.
  - recommended browser usage: `navigator.sendBeacon('/collect/v1/events', JSON.stringify(payload))`.
  - when telemetry is enabled, run htmlservd behind Caddy and keep htmlservd bound to loopback for trustworthy host attribution.

Audit log records:

- actor identity (SSH principal or user id)
- timestamp
- environment
- resource change summary (hashes)
- release id activated

## 8. Domains + TLS

- Recommended front proxy: **Caddy**
- `htmlservd` writes Caddy snippets and triggers reload safely.
- Domain binding per environment:
  - `example.com` -> prod current directory
  - `staging.example.com` -> staging current directory
- When telemetry is enabled, generated site blocks include:
  - `handle /collect/v1/events* { reverse_proxy 127.0.0.1:<htmlservd-port> }`
  - static file serving behavior is unchanged for all non-telemetry paths.
  - if the server listen port is dynamic (`port: 0`), Caddy telemetry proxy generation cannot resolve a stable backend port; use an explicit port in telemetry-enabled environments.

## 9. CLI design (htmlctl)

### 9.1 Config / contexts

`~/.htmlctl/config.yaml`:

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@yourserver
    website: sample
    environment: staging
    token: "<shared-api-token>"
  - name: prod
    server: ssh://root@yourserver
    website: sample
    environment: prod
    token: "<shared-api-token>"
```

Token utilities:

- `htmlctl context token generate` (prints a random 32-byte hex token)
- `htmlctl context set <name> --token <token>`

### 9.2 Core commands

Local:

- `htmlctl render -f ./site -o ./dist`
- `htmlctl serve ./dist --port 8080`

Remote ops:

- `htmlctl diff -f ./site --context staging`
- `htmlctl apply -f ./site --context staging [--dry-run]`
- `htmlctl status website/sample --context staging`
- `htmlctl promote website/sample --from staging --to prod`
- `htmlctl rollout history website/sample --context prod`
- `htmlctl rollout undo website/sample --context prod`
- `htmlctl logs website/sample --context prod`

Domains:

- `htmlctl domain add example.com --context prod`
- `htmlctl domain add staging.example.com --context staging`
- `htmlctl domain verify example.com --context prod`

### 9.3 Agent-friendly partial apply (v1 UX)

Although the server always creates a full release, the CLI supports applying only changed files:

- `htmlctl apply -f components/pricing.html --context staging`
- `htmlctl apply -f styles/default.css --context staging`

Server merges into last known desired state for that environment, validates, renders, releases.

## 10. Implementation notes (Go v1)

- Prefer Go for both CLI + daemon.
- Use standard `net/http` for API.
- Use SQLite driver (`modernc.org/sqlite` or `mattn/go-sqlite3`) depending on CGO preference.
- Use robust file ops: `os.Rename` for atomic finalize, symlink switch for activation.
- Use `ulid` for release ids.
