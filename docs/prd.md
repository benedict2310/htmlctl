# htmlctl / htmlservd — Product Requirements Document

## 0. One-page summary

Build a **kubectl-style control plane for plain HTML/CSS/JS websites**, designed for **AI agents** (CLI-only). A server daemon (**htmlservd**) manages websites as **declarative resources**, renders **publish-time stitched** static output, and serves via a front proxy (recommended: **Caddy** for TLS + custom domains). A CLI (**htmlctl**) applies changes, diffs state, promotes releases across environments (staging -> prod), and supports atomic rollouts/rollback.

Key decisions:

- **No Git in v1** (optional integration post-MVP).
- **Component-first editing**: agents edit components/styles/assets; pages are composition glue.
- **Publish-time composition** (stitch includes at render time, output static HTML).
- **Environments**: local preview -> staging -> production with **promotion** (no rebuild).
- **Production-first**: private control plane by default (SSH tunnel / localhost), immutable releases, atomic activation, rollback, audit log.
- Readers/visitors are not uploading content; admin JS is trusted code shipped by CLI.

---

## 1. Problem

Plain websites are easy to serve but hard to manage safely at scale when an **agent** needs to:

- update individual **sections** (pricing/features/etc.) without touching whole pages,
- deploy reliably to **staging** then **promote** to **production** without drift,
- ensure changes are **atomic**, reversible, auditable, and safe for production,
- manage **custom domains** and **TLS** with minimal operational overhead.

## 2. Goals

1. **kubectl-like UX** for websites (apply/diff/get/status/rollout/promote).
2. **Production-safe deployments**: immutable releases, atomic switch, instant rollback.
3. **Token-efficient agent workflow**: edit a single component (subsection) and deploy.
4. **Deterministic rendering**: local preview matches server output.
5. **Support custom domains + TLS** for real sites (e.g., `futurelab.studio`).

## 3. Non-goals (v1)

- Visitor authentication / user-generated content (comments/uploads).
- Admin UI in browser (no web UI; CLI-only).
- Dynamic server-side personalization.
- GitOps / CI pipeline integration.

## 4. Target users

- AI agents and power users operating via CLI.
- Self-hosted operators wanting a simple production website control plane.

## 5. Core user journeys

1. **Local preview**: render + serve locally.
2. **Deploy to staging**: diff -> apply -> verify.
3. **Promote to prod**: promote exact release from staging to prod.
4. **Rollback prod**: revert to previous release.
5. **Domain management**: bind domain to environment, verify, serve with TLS.

## 6. Success metrics

- Deployment is atomic; rollback < 1 second.
- `diff` shows small component-level changes for typical edits.
- Local render output is byte-stable with server render for same inputs.
- `promote` produces identical artifact in prod as staging (hash match).
- No public admin plane by default.

## 7. Risks

- Serving trusted JS means admin is a trust boundary; exposure of admin endpoints is dangerous.
- Rendering engine must be deterministic; otherwise diffs and promotions become unreliable.
- Domain/TLS automation introduces operational edge cases (DNS misconfig, cert issuance limits).

## 8. Product scope

### Components of the system

- **htmlctl**: CLI (kubectl-like)
- **htmlservd**: server daemon (resource API + renderer + release manager)
- **Front proxy**: Caddy (recommended) for TLS + custom domains + static serving

### Environments

- **local**: local-only render + optional local serve
- **staging**: server-managed environment
- **prod**: server-managed environment

Promotion should be **artifact promotion** (no rebuild): staging release bytes copied/linked into prod.

## 9. Acceptance criteria

### Functional

- Local render produces a working site with stitched components.
- Staging apply creates a new immutable release and activates atomically.
- Promote staging->prod activates identical artifact (hash match).
- Rollback switches prod to prior release instantly.
- Domain binding maps hostnames to correct environment outputs.
- Caddy serves HTTPS for bound domains.
- Audit log records all applies/promotions/rollbacks.

### Non-functional

- Control plane not publicly exposed by default.
- Renderer deterministic (byte-stable output given same inputs).
- Apply is safe under concurrent operations (locks per environment).

## 10. Post-MVP roadmap

1. Git as optional input mode (`htmlctl apply --from-git repo --ref sha`)
2. Visitor auth (site gating) — `AuthPolicy` resource, per-route protection
3. Component CSS/JS fragments — scoped injection, CSP improvements
4. Preview URLs for draft releases — ephemeral hostnames
5. Advanced templates — multi-template base layouts

## 11. Open questions

- Whether to support page-scoped components in v1 (default: no; global only).
- Whether to enforce a strict HTML sanitizer (not needed for trusted admin, but could catch mistakes).

## 12. Deliverables

- `htmlctl` binary
- `htmlservd` binary
- sample `site/` repo for `futurelab` demo
- service unit files (systemd) for `htmlservd` and `caddy`
