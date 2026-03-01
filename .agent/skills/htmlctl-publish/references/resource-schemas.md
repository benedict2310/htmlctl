# htmlctl Resource Schemas

All resources use `apiVersion: htmlctl.dev/v1`. Files live in a `site/` directory tree and are applied via `htmlctl apply`.

## Directory Layout

```
site/
  website.yaml
  branding/
    favicon.svg
    favicon.ico
  pages/
    index.page.yaml
    about.page.yaml
  components/
    nav.html
    hero.html
    footer.html
  styles/
    tokens.css        # CSS custom properties (design tokens)
    default.css       # base layout and component styles
  scripts/
    site.js           # optional global JS (injected at end of <body>)
  assets/
    logo.svg
    og-image.png
```

**Agent rule:** edit `components/*`, `styles/*`, `scripts/*`, and `assets/*` for most content work. Touch `website.yaml` for website-level metadata and `branding/*` for favicon inputs. Only touch `pages/*.page.yaml` for routing or page-specific metadata changes.

---

## Website

Declares the site. One per deployment.

```yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: mysite          # used in all CLI commands: website/mysite
spec:
  defaultStyleBundle: default
  baseTemplate: default
  head:
    icons:
      svg: branding/favicon.svg
      ico: branding/favicon.ico
  seo:
    publicBaseURL: https://mysite.com
    robots:
      enabled: true
    sitemap:
      enabled: true
```

- `metadata.name`: alphanumeric + `_-`, max 128 chars; used in all CLI/API paths
- `spec.defaultStyleBundle`: must match a StyleBundle name (`default` in v1)
- `spec.baseTemplate`: base HTML template (`default` in v1)
- `spec.head.icons`: optional website-level favicon config; source files live under `branding/`
- `spec.seo.publicBaseURL`: canonical public site origin used for generated crawl artifacts
- `spec.seo.robots.enabled`: generates root `/robots.txt`
- `spec.seo.sitemap.enabled`: generates root `/sitemap.xml` and appends a `Sitemap:` line to `robots.txt`
- `publicBaseURL` should point at the real public production URL because promote does not rebuild artifacts for prod

---

## Page

Defines a rendered page: its route, title, component layout, and optional server-rendered SEO metadata.

```yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: "My Site"
  description: "Short description for <meta name=description>"
  layout:
    - include: nav
    - include: hero
    - include: features
    - include: footer
  head:                                    # optional — server-rendered SEO metadata
    canonicalURL: https://mysite.com/
    meta:
      robots: index,follow
      keywords: mysite, product
      author: Author Name
    openGraph:
      type: website
      url: https://mysite.com/
      siteName: My Site
      title: My Site
      description: Short description
      image: https://mysite.com/assets/og-image.png   # omit to get auto-generated OG card
    twitter:
      card: summary_large_image
      title: My Site
      description: Short description
      image: https://mysite.com/assets/og-image.png   # omit to get auto-generated OG card
    jsonLD:
      - id: website
        payload:
          "@context": https://schema.org
          "@type": WebSite
          name: My Site
          url: https://mysite.com/
```

### Page rules

- `spec.route`: normalized absolute path (`/`, `/about`, `/projects/ora`)
- `spec.layout`: ordered list of component names; all referenced components must exist
- All `spec.head` metadata is server-rendered into `<head>` at build time (deterministic, no JS injection)
- `spec.head.meta`: key/value map of `<meta name="...">` tags — rendered sorted alphabetically by name
- Open Graph and Twitter fields render in fixed field order
- JSON-LD blocks render in manifest order, wrapped in `<script type="application/ld+json">`
- URL fields in `spec.head` accept only `http(s)://` or relative paths

### OG image auto-generation

At build time the server generates a 1200×630 PNG card for every page and places it in the release at `og/<pagename>.png`. The `openGraph.image` and `twitter.image` fields are then auto-populated **only when each field is absent** and `canonicalURL` is an absolute `http(s)://` URL. Explicitly set fields are never overwritten.

- OG generation failures are per-page warnings — the build still succeeds.
- The release always contains `/og/<pagename>.png` regardless of whether injection occurred.
- To use a custom image instead of the generated card, set `openGraph.image` / `twitter.image` explicitly.

### Head metadata render order

1. `<link rel="canonical">` (if `canonicalURL` set)
2. `<meta name="...">` tags sorted alphabetically by name
3. Open Graph `og:*` tags in fixed field order
4. Twitter `twitter:*` tags in fixed field order
5. JSON-LD `<script>` blocks in manifest order

---

## Component

An HTML fragment inserted into a page layout. Stored as a plain `.html` file in `components/`.

```html
<section id="hero">
  <h1>Welcome</h1>
  <p>Short tagline.</p>
</section>
```

### Component rules

- **Exactly one root element.**
- Root tag must be one of: `section`, `header`, `footer`, `main`, `nav`, `article`, `div`.
- If the component is anchor-navigable (linked from nav), root **must** have `id="<componentName>"`.
- **No `<script>` tags** inside components. JS belongs in `scripts/site.js`.
- **No inline event handler attributes** (`onclick`, `onload`, `onmouseover`, etc.) — rejected at validation time.

---

## StyleBundle (v1: always `default`)

Two CSS files compose the bundle, both injected into `<head>` in deterministic order on every rendered page:

```
styles/tokens.css    # CSS custom properties (design tokens)
styles/default.css   # base layout and component styles
```

Example `tokens.css`:

```css
:root {
  --bg: #0b0e14;
  --text: #e0e5e9;
  --accent: #6d9ea3;
}
```

---

## Asset

Binary files (images, fonts, SVGs) in `assets/`. Stored content-addressed (SHA-256) by the server.

- Accepted content types: images (PNG, JPG, GIF, WebP, AVIF, SVG), fonts (WOFF, WOFF2), common web assets
- Filenames are sanitized; path traversal is rejected
- Reference from HTML: `<img src="/assets/logo.svg">` or `<img src="/assets/og-image.png">`

## Branding

Website-level favicon source files live in `branding/`.

- Supported v1 inputs are the typed favicon slots referenced from `website.yaml`
- public output paths are conventional root files such as `/favicon.svg` and `/favicon.ico`
- do not use `assets/` for favicon files when `branding/` is available

---

## Global JS (`scripts/site.js`)

Injected at the end of `<body>` on every rendered page. Use for interactive behavior, telemetry, and dynamic content.

```js
(function () {
  // your code here
})();
```

> In v1, all JavaScript belongs here. Components cannot contain `<script>` tags.

---

## Minimum Valid Site

The smallest possible site that will pass validation:

```yaml
# website.yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: sample
spec:
  defaultStyleBundle: default
  baseTemplate: default
```

```yaml
# pages/index.page.yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: index
spec:
  route: /
  title: Sample
  description: Landing page
  layout:
    - include: hero
```

```html
<!-- components/hero.html -->
<section id="hero">
  <h1>Hello</h1>
</section>
```

```css
/* styles/tokens.css */
:root { --bg: #fff; }
```

```css
/* styles/default.css */
body { font-family: sans-serif; background: var(--bg); }
```

---

## Telemetry (optional)

To collect page-view events from the browser without external infrastructure, add to `scripts/site.js`:

```bash
curl -sS \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -H "Origin: https://example.com" \
  --data '{"events":[{"name":"page_view","path":"/","attrs":{"source":"trusted-collector"}}]}' \
  https://example.com/collect/v1/events
```

- Ingest endpoint `POST /collect/v1/events` is authenticated with the server bearer token.
- Do not embed the bearer token in public browser JavaScript.
- If an `Origin` header is present, it must exactly match scheme, host, and port.
- Telemetry is attributed by the request's `Host` header — must match a domain binding.
- Raw IP hosts (`127.0.0.1`) are rejected; always use a bound hostname (`127.0.0.1.nip.io` locally).
- Events are queryable via the authenticated API — see `references/api.md`.
