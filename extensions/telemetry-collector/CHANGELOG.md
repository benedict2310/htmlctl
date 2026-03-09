# telemetry-collector changelog

## 0.1.0
- Initial official extension release.
- Adds same-origin `/site-telemetry/v1/events` ingest with narrow event allowlist.
- Forwards accepted browser events to htmlservd `POST /collect/v1/events` using bearer auth without exposing the token to browsers.
