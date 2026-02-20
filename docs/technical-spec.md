# htmlctl / htmlservd — Technical Specification

## 1. Repository layout (agent-facing)

```
site/
  website.yaml
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

Example:

```yaml
apiVersion: htmlctl.dev/v1
kind: Page
metadata:
  name: product
spec:
  route: /product
  title: "Futurelab — Product"
  description: "..."
  layout:
    - include: header
    - include: hero
    - include: features
    - include: pricing
    - include: faq
    - include: footer
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

## 3. Rendering & composition

### 3.1 Publish-time stitching

Pages are not full HTML docs; they are layouts that reference components. Renderer produces full HTML:

- Base template provides `<html><head>...</head><body><main>{{content}}</main>...</body></html>`
- Layout includes inserted in order into `<main>`
- `scripts/site.js` injected at end of body if present
- Stylesheets injected into `<head>`

### 3.2 Determinism requirements

- Stable ordering for injections (styles, scripts)
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
  websites/futurelab/
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
- In v1, disallow `<script>` tags in components (JS only from `scripts/site.js`).

### 6.2 Page validation

- routes normalized
- all includes exist
- prevent cycles (component cannot include other components in v1)

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
- API authentication in v1:
  - all `/api/v1/*` routes require `Authorization: Bearer <token>` when `api.token` (or `HTMLSERVD_API_TOKEN`) is configured.
  - health routes (`/healthz`, `/readyz`) remain unauthenticated.
  - if no API token is configured, the server starts with a prominent warning for rollout safety.
  - operators can enforce token configuration at startup via `htmlservd --require-auth`.
- token comparison uses constant-time checks (`crypto/subtle`).

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
  - `futurelab.studio` -> prod current directory
  - `staging.futurelab.studio` -> staging current directory

## 9. CLI design (htmlctl)

### 9.1 Config / contexts

`~/.htmlctl/config.yaml`:

```yaml
apiVersion: htmlctl.dev/v1
current-context: staging
contexts:
  - name: staging
    server: ssh://root@yourserver
    website: futurelab
    environment: staging
    token: "<shared-api-token>"
  - name: prod
    server: ssh://root@yourserver
    website: futurelab
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
- `htmlctl status website/futurelab --context staging`
- `htmlctl promote website/futurelab --from staging --to prod`
- `htmlctl rollout history website/futurelab --context prod`
- `htmlctl rollout undo website/futurelab --context prod`
- `htmlctl logs website/futurelab --context prod`

Domains:

- `htmlctl domain add futurelab.studio --context prod`
- `htmlctl domain add staging.futurelab.studio --context staging`
- `htmlctl domain verify futurelab.studio --context prod`

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
