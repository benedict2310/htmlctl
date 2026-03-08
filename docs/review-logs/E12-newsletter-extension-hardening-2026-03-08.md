# E12 Newsletter Extension Hardening Review (2026-03-08)

## Scope

Reviewed `extensions/newsletter` as a reusable extension package rather than a Futurelab-specific deployment:

- service runtime in `extensions/newsletter/service`
- installer and env assets in `extensions/newsletter/ops`
- operator docs in `docs/guides/*` and `docs/reference/extensions.md`

## Findings Resolved

1. Site-specific runtime leak
- Issue: public verify/unsubscribe HTML still contained a hardcoded CTA back to `futurelab.studio`.
- Risk: broke the extension abstraction and shipped an adopter-visible host-specific reference.
- Fix: verify/unsubscribe pages now derive the CTA target from `NEWSLETTER_PUBLIC_BASE_URL` and render a generic `Back to site` action.

2. Weak unsubscribe secret acceptance
- Issue: `NEWSLETTER_LINK_SECRET` only required a non-empty value.
- Risk: low-entropy secrets reduce confidence in the HMAC-based unsubscribe token scheme.
- Fix: service startup now rejects secrets shorter than 32 characters; installer docs and env examples were updated accordingly.

3. Late sender-address validation
- Issue: `NEWSLETTER_RESEND_FROM` was accepted without syntax validation and would only fail later at delivery time.
- Risk: operator misconfiguration would surface as campaign or verification send failures instead of a deterministic startup error.
- Fix: service config now validates `NEWSLETTER_RESEND_FROM` with `net/mail.ParseAddress`; installer guidance and examples were updated to match.

4. Installer contract drift
- Issue: the setup script documented generated link secrets but did not enforce minimum secret length or sender-address shape.
- Risk: docs and runtime guarantees could diverge during adoption.
- Fix: installer now validates link-secret minimum length and sender-address format before writing env files.

5. Stale runbook language
- Issue: the Hetzner runbook still referenced the old placeholder `/newsletter/*` foundation behavior.
- Risk: operators following the runbook would test against outdated expectations.
- Fix: runbook now consistently documents the real signup, verification, unsubscribe, import, preview, and paced-send workflow.

## Verification

Executed after the fixes:

```bash
cd extensions/newsletter/service && go test ./...
cd extensions/newsletter/service && go test -race ./...
cd extensions/newsletter/service && go vet ./...
bash -n extensions/newsletter/ops/setup-newsletter-extension.sh
codex review --uncommitted
```

Additional store-layer coverage added after the initial hardening pass:

```bash
cd extensions/newsletter/service && go test ./internal/server ./...
cd extensions/newsletter/service && go test -race ./internal/server ./...
```

## Residual Notes

- Campaign content remains operator-controlled HTML/text by design; the service treats it as trusted extension content, not end-user input.
- `campaign preview` intentionally omits real unsubscribe tokens and instead renders a preview-only note.
- Production bulk sends still rely on explicit pacing (`--interval 30s`) appropriate for lower-tier Resend rate limits.
