#!/usr/bin/env bash
set -euo pipefail

SERVICE_USER="htmlctl-telemetry"
SERVICE_GROUP="htmlctl-telemetry"
INSTALL_ROOT="/etc/htmlctl-telemetry-collector"
STAGING_HTTP_ADDR="${TELEMETRY_COLLECTOR_STAGING_HTTP_ADDR:-127.0.0.1:9601}"
PROD_HTTP_ADDR="${TELEMETRY_COLLECTOR_PROD_HTTP_ADDR:-127.0.0.1:9602}"
STAGING_HTMLSERVD_BASE_URL="${TELEMETRY_COLLECTOR_STAGING_HTMLSERVD_BASE_URL:-http://127.0.0.1:9400}"
PROD_HTMLSERVD_BASE_URL="${TELEMETRY_COLLECTOR_PROD_HTMLSERVD_BASE_URL:-http://127.0.0.1:9400}"
STAGING_ALLOWED_EVENTS="${TELEMETRY_COLLECTOR_STAGING_ALLOWED_EVENTS:-page_view,link_click,cta_click,newsletter_signup}"
PROD_ALLOWED_EVENTS="${TELEMETRY_COLLECTOR_PROD_ALLOWED_EVENTS:-page_view,link_click,cta_click,newsletter_signup}"
STAGING_MAX_BODY_BYTES="${TELEMETRY_COLLECTOR_STAGING_MAX_BODY_BYTES:-32768}"
PROD_MAX_BODY_BYTES="${TELEMETRY_COLLECTOR_PROD_MAX_BODY_BYTES:-32768}"
STAGING_MAX_EVENTS="${TELEMETRY_COLLECTOR_STAGING_MAX_EVENTS:-10}"
PROD_MAX_EVENTS="${TELEMETRY_COLLECTOR_PROD_MAX_EVENTS:-10}"

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required env: $name" >&2
    exit 1
  fi
}

validate_loopback_addr() {
  local value="$1"
  if [[ ! "$value" =~ ^(127\.[0-9]+\.[0-9]+\.[0-9]+|localhost|\[::1\]):[0-9]{1,5}$ ]]; then
    echo "address must be loopback-only host:port: $value" >&2
    exit 1
  fi
}

validate_https_origin() {
  local value="$1"
  if [[ ! "$value" =~ ^https://[^/?#]+/?$ ]]; then
    echo "public base URL must be an https origin without path/query/fragment: $value" >&2
    exit 1
  fi
}

validate_http_loopback_origin() {
  local value="$1"
  if [[ ! "$value" =~ ^http://(127\.[0-9]+\.[0-9]+\.[0-9]+|localhost|\[::1\])(:[0-9]{1,5})?/?$ ]]; then
    echo "htmlservd base URL must be an http loopback origin without path/query/fragment: $value" >&2
    exit 1
  fi
}

validate_positive_int() {
  local value="$1"
  if [[ ! "$value" =~ ^[1-9][0-9]*$ ]]; then
    echo "expected positive integer, got: $value" >&2
    exit 1
  fi
}

validate_token() {
  local value="$1"
  if [[ "$value" =~ [[:space:]] ]]; then
    echo "tokens must not contain whitespace" >&2
    exit 1
  fi
}

require_env TELEMETRY_COLLECTOR_BINARY_PATH
require_env TELEMETRY_COLLECTOR_STAGING_PUBLIC_BASE_URL
require_env TELEMETRY_COLLECTOR_PROD_PUBLIC_BASE_URL
require_env TELEMETRY_COLLECTOR_STAGING_HTMLSERVD_TOKEN
require_env TELEMETRY_COLLECTOR_PROD_HTMLSERVD_TOKEN
require_env TELEMETRY_COLLECTOR_STAGING_UNIT_PATH
require_env TELEMETRY_COLLECTOR_PROD_UNIT_PATH

validate_loopback_addr "$STAGING_HTTP_ADDR"
validate_loopback_addr "$PROD_HTTP_ADDR"
validate_https_origin "$TELEMETRY_COLLECTOR_STAGING_PUBLIC_BASE_URL"
validate_https_origin "$TELEMETRY_COLLECTOR_PROD_PUBLIC_BASE_URL"
validate_http_loopback_origin "$STAGING_HTMLSERVD_BASE_URL"
validate_http_loopback_origin "$PROD_HTMLSERVD_BASE_URL"
validate_positive_int "$STAGING_MAX_BODY_BYTES"
validate_positive_int "$PROD_MAX_BODY_BYTES"
validate_positive_int "$STAGING_MAX_EVENTS"
validate_positive_int "$PROD_MAX_EVENTS"
validate_token "$TELEMETRY_COLLECTOR_STAGING_HTMLSERVD_TOKEN"
validate_token "$TELEMETRY_COLLECTOR_PROD_HTMLSERVD_TOKEN"

install -d -m 0750 -o root -g root "$INSTALL_ROOT"
install -d -m 0755 /var/lib/htmlctl-telemetry-collector
install -m 0755 "$TELEMETRY_COLLECTOR_BINARY_PATH" /usr/local/bin/htmlctl-telemetry-collector

if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
  groupadd --system "$SERVICE_GROUP"
fi
if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  useradd --system --home /var/lib/htmlctl-telemetry-collector --shell /usr/sbin/nologin --gid "$SERVICE_GROUP" "$SERVICE_USER"
fi
chown "$SERVICE_USER:$SERVICE_GROUP" /var/lib/htmlctl-telemetry-collector

cat > "$INSTALL_ROOT/staging.env" <<ENV
TELEMETRY_COLLECTOR_ENV=staging
TELEMETRY_COLLECTOR_HTTP_ADDR=${STAGING_HTTP_ADDR}
TELEMETRY_COLLECTOR_PUBLIC_BASE_URL=${TELEMETRY_COLLECTOR_STAGING_PUBLIC_BASE_URL}
TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL=${STAGING_HTMLSERVD_BASE_URL}
TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN=${TELEMETRY_COLLECTOR_STAGING_HTMLSERVD_TOKEN}
TELEMETRY_COLLECTOR_ALLOWED_EVENTS=${STAGING_ALLOWED_EVENTS}
TELEMETRY_COLLECTOR_MAX_BODY_BYTES=${STAGING_MAX_BODY_BYTES}
TELEMETRY_COLLECTOR_MAX_EVENTS=${STAGING_MAX_EVENTS}
ENV

cat > "$INSTALL_ROOT/prod.env" <<ENV
TELEMETRY_COLLECTOR_ENV=prod
TELEMETRY_COLLECTOR_HTTP_ADDR=${PROD_HTTP_ADDR}
TELEMETRY_COLLECTOR_PUBLIC_BASE_URL=${TELEMETRY_COLLECTOR_PROD_PUBLIC_BASE_URL}
TELEMETRY_COLLECTOR_HTMLSERVD_BASE_URL=${PROD_HTMLSERVD_BASE_URL}
TELEMETRY_COLLECTOR_HTMLSERVD_TOKEN=${TELEMETRY_COLLECTOR_PROD_HTMLSERVD_TOKEN}
TELEMETRY_COLLECTOR_ALLOWED_EVENTS=${PROD_ALLOWED_EVENTS}
TELEMETRY_COLLECTOR_MAX_BODY_BYTES=${PROD_MAX_BODY_BYTES}
TELEMETRY_COLLECTOR_MAX_EVENTS=${PROD_MAX_EVENTS}
ENV

chown root:"$SERVICE_GROUP" "$INSTALL_ROOT/staging.env" "$INSTALL_ROOT/prod.env"
chmod 0640 "$INSTALL_ROOT/staging.env" "$INSTALL_ROOT/prod.env"

install -m 0644 "$TELEMETRY_COLLECTOR_STAGING_UNIT_PATH" /etc/systemd/system/htmlctl-telemetry-collector-staging.service
install -m 0644 "$TELEMETRY_COLLECTOR_PROD_UNIT_PATH" /etc/systemd/system/htmlctl-telemetry-collector-prod.service

systemctl daemon-reload
systemctl enable --now htmlctl-telemetry-collector-staging.service
systemctl enable --now htmlctl-telemetry-collector-prod.service
