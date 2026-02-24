# htmlctl Resource Schemas

All resources use `apiVersion: htmlctl.dev/v1`. Files live in a `site/` directory tree and are applied via `htmlctl apply`.

## Directory Layout

```
site/
  website.yaml
  pages/
    index.page.yaml
    about.page.yaml
  components/
    nav.html
    hero.html
    footer.html
  styles/
    tokens.css
    default.css
  scripts/
    site.js          # optional global JS
  assets/
    logo.svg
    og-image.png
```

---

## Website

Declares the site. One per deployment.

```yaml
apiVersion: htmlctl.dev/v1
kind: Website
metadata:
  name: mysite
spec:
  defaultStyleBundle: default
  baseTemplate: default
```

- `metadata.name`: used in all CLI commands (`website/<name>`)
- `spec.defaultStyleBundle`: must match a StyleBundle name (`default` in v1)
- `spec.baseTemplate`: base HTML template (`default` in v1)

---

## Page

Defines a rendered page: its route, title, component layout, and optional SEO metadata.

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
    - include: projects
    - include: contact
    - include: footer
  head:                                    # optional — server-rendered SEO metadata
    canonicalURL: https://mysite.com/
    meta:
      robots: index,follow
      keywords: mysite, product
    openGraph:
      type: website
      url: https://mysite.com/
      siteName: My Site
      title: My Site
      description: Short description
      image: https://mysite.com/assets/og-image.png
    twitter:
      card: summary_large_image
      title: My Site
      description: Short description
      image: https://mysite.com/assets/og-image.png
    jsonLD:
      - id: website
        payload:
          "@context": https://schema.org
          "@type": WebSite
          name: My Site
          url: https://mysite.com/
```

### Rules

- `spec.route`: normalized absolute path (`/`, `/about`, `/projects/ora`)
- `spec.layout`: ordered list of component names; all referenced components must exist
- `spec.head.meta`: key/value map of `<meta name="...">` tags, sorted alphabetically on render
- Open Graph and Twitter fields render in fixed field order (deterministic)
- JSON-LD blocks render in manifest order
- URL fields in `spec.head` accept only `http(s)` or relative paths

---

## Component

An HTML fragment inserted into a page layout. Stored as a plain `.html` file in `components/`.

```html
<section id="hero">
  <h1>Welcome</h1>
  <p>Short tagline.</p>
</section>
```

### Rules

- Exactly one root element.
- Root tag must be one of: `section`, `header`, `footer`, `main`, `nav`, `article`, `div`.
- If the component is anchor-navigable (linked from nav), root must have `id="<componentName>"`.
- No `<script>` tags inside components. JS belongs in `scripts/site.js`.
- No inline event handler attributes (`onclick`, `onload`, etc.) — rejected at validation time.

---

## StyleBundle (v1: always `default`)

Two CSS files compose the bundle:

```
styles/tokens.css    # CSS custom properties (design tokens)
styles/default.css   # base layout and component styles
```

Both are injected into `<head>` in deterministic order on every rendered page.

---

## Asset

Binary files (images, fonts, SVGs) placed in `assets/`. Stored content-addressed (SHA-256) by the server.

- Accepted content types: images, SVG, fonts, common web assets
- Filenames are sanitized; path traversal is rejected
- Reference from HTML: `<img src="/assets/logo.svg">`

---

## Telemetry (optional)

To collect page view events from the browser without external infrastructure, add to `scripts/site.js`:

```js
var payload = JSON.stringify({
  events: [
    {
      name: 'page_view',
      path: window.location.pathname || '/',
      occurredAt: new Date().toISOString(),
      sessionId: 'sess_' + Date.now().toString(36),
      attrs: { source: 'browser' }
    }
  ]
});

navigator.sendBeacon(
  '/collect/v1/events',
  new Blob([payload], { type: 'text/plain;charset=UTF-8' })
);
```

The ingest endpoint (`POST /collect/v1/events`) is unauthenticated and same-origin only.
Events are queryable via the authenticated API: `GET /api/v1/websites/<name>/environments/<env>/telemetry/events`.
