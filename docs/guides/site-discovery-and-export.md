# Site Discovery And Export

This guide is the fastest way for an operator or AI agent with no prior site context to understand, inspect, bootstrap, and reconstruct a site managed by `htmlctl`.

## 1. Learn The Authoring Model

Start with the built-in explanation before touching remote state:

```bash
htmlctl site explain
htmlctl site explain -o yaml
```

This command describes:

- the canonical site skeleton
- how `website.yaml`, pages, components, styles, scripts, assets, and branding fit together
- component sidecar rules
- metadata and SEO expectations

## 2. Bootstrap A New Site

Create a minimal renderable site locally:

```bash
htmlctl site init --dir ./site
```

Force replacement is only allowed when you explicitly opt in:

```bash
htmlctl site init --dir ./site --force
```

Typical next steps:

```bash
htmlctl render -f ./site
htmlctl apply -f ./site --context staging
```

## 3. Discover A Remote Site

Use `get` for inventory and `inspect` for deeper detail.

Start with the site-level summary:

```bash
htmlctl status --context staging
htmlctl get website --context staging
htmlctl inspect website --context staging
```

Then enumerate the content model:

```bash
htmlctl get pages --context staging
htmlctl get components --context staging
htmlctl get styles --context staging
htmlctl get assets --context staging
htmlctl get branding --context staging
```

Inspect specific content when you need page or component detail:

```bash
htmlctl inspect page index --context staging
htmlctl inspect component hero --context staging
```

What these commands answer quickly:

- `status`: which site/environment is selected and whether desired state differs from active release
- `get website`: site-level metadata, counts, default style bundle, base template, public base URL
- `get pages/components/styles/assets/branding`: inventory lists suitable for scripting and triage
- `inspect website`: richer site summary including routes, asset counts, and branding presence
- `inspect page <name>`: route, layout includes, head metadata, canonical URL, social metadata
- `inspect component <name>`: scope, sidecars, and which pages reference the component

## 4. Apply Small Changes Safely

`apply` supports full-site and file-level updates. Valid single-file targets include:

- `website.yaml`
- `pages/*.page.yaml`
- `components/<name>.html`
- `components/<name>.css`
- `components/<name>.js`
- `styles/tokens.css`
- `styles/default.css`
- `scripts/site.js`
- `assets/**`
- `branding/**`

Examples:

```bash
htmlctl apply -f pages/index.page.yaml --context staging
htmlctl apply -f components/hero.css --context staging
htmlctl diff -f ./site --context staging
```

For page files, referenced components are bundled automatically. For component sidecars, `htmlctl` sends the component HTML plus present CSS/JS companions together.

## 5. Reconstruct Remote Source Locally

If the server is the source of truth, export the stored source bundle back to disk:

```bash
htmlctl site export --context staging --output ./exported-site
```

You can also keep the source archive:

```bash
htmlctl site export --context staging --output ./exported-site --archive ./exported-site/site.tgz
```

Export behavior:

- preserves the originally stored `website.yaml` and page YAML bytes when available
- writes an `.htmlctl-export` marker into extracted export directories
- allows `--force` only when replacing an existing prior export tree
- rejects unsupported cases that cannot round-trip cleanly, such as page-less sites or sites using multiple style bundles

Safe replacement of an existing export tree:

```bash
htmlctl site export --context staging --output ./exported-site --force
```

## 6. Recommended Zero-Context Workflow

For a fresh operator or agent, use this order:

```bash
htmlctl site explain
htmlctl status --context staging
htmlctl get website --context staging
htmlctl get pages --context staging
htmlctl inspect page index --context staging
htmlctl inspect component hero --context staging
htmlctl site export --context staging --output ./site
```

That sequence gives you:

- the expected local source layout
- the selected remote site/environment
- the current remote inventory
- page/component structure for editing
- a local source tree you can inspect, diff, render, and re-apply
