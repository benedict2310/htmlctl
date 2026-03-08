#!/usr/bin/env bash
set -euo pipefail

NEWSLETTER_USER="${NEWSLETTER_USER:-htmlctl-newsletter}"
NEWSLETTER_GROUP="${NEWSLETTER_GROUP:-htmlctl-newsletter}"
NEWSLETTER_ETC_DIR="${NEWSLETTER_ETC_DIR:-/etc/htmlctl-newsletter}"
NEWSLETTER_DATA_DIR="${NEWSLETTER_DATA_DIR:-/var/lib/htmlctl-newsletter}"
NEWSLETTER_BIN_PATH="${NEWSLETTER_BIN_PATH:-/usr/local/bin/htmlctl-newsletter}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"

ROLE_STAGING="${NEWSLETTER_STAGING_DB_ROLE:-htmlctl_newsletter_staging}"
ROLE_PROD="${NEWSLETTER_PROD_DB_ROLE:-htmlctl_newsletter_prod}"
DB_STAGING="${NEWSLETTER_STAGING_DB_NAME:-htmlctl_newsletter_staging}"
DB_PROD="${NEWSLETTER_PROD_DB_NAME:-htmlctl_newsletter_prod}"
DB_HOST="${NEWSLETTER_DB_HOST:-127.0.0.1}"
DB_PORT="${NEWSLETTER_DB_PORT:-5432}"

STAGING_HTTP_ADDR="${NEWSLETTER_STAGING_HTTP_ADDR:-127.0.0.1:9501}"
PROD_HTTP_ADDR="${NEWSLETTER_PROD_HTTP_ADDR:-127.0.0.1:9502}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
default_staging_unit_path="${SCRIPT_DIR}/systemd/htmlctl-newsletter-staging.service"
default_prod_unit_path="${SCRIPT_DIR}/systemd/htmlctl-newsletter-prod.service"
if [[ ! -f "${default_staging_unit_path}" ]]; then
  default_staging_unit_path="${SCRIPT_DIR}/htmlctl-newsletter-staging.service"
fi
if [[ ! -f "${default_prod_unit_path}" ]]; then
  default_prod_unit_path="${SCRIPT_DIR}/htmlctl-newsletter-prod.service"
fi
STAGING_UNIT_SOURCE="${NEWSLETTER_STAGING_UNIT_PATH:-${default_staging_unit_path}}"
PROD_UNIT_SOURCE="${NEWSLETTER_PROD_UNIT_PATH:-${default_prod_unit_path}}"

run_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

run_psql_as_postgres() {
  if [[ "${EUID}" -eq 0 ]]; then
    runuser -u postgres -- psql "$@"
  else
    sudo -u postgres psql "$@"
  fi
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "error: ${key} must be set" >&2
    exit 1
  fi
}

validate_no_single_quote() {
  local key="$1"
  local value="$2"
  if [[ "${value}" == *"'"* ]]; then
    echo "error: ${key} must not contain single quotes" >&2
    exit 1
  fi
}

validate_no_whitespace() {
  local key="$1"
  local value="$2"
  if [[ "${value}" =~ [[:space:]] ]]; then
    echo "error: ${key} must not contain whitespace" >&2
    exit 1
  fi
}

validate_min_length() {
  local key="$1"
  local value="$2"
  local min_len="$3"
  if (( ${#value} < min_len )); then
    echo "error: ${key} must be at least ${min_len} characters" >&2
    exit 1
  fi
}

validate_sender_address() {
  local key="$1"
  local value="$2"
  local bare_pattern='^[^[:space:]<>]+@[^[:space:]<>]+$'
  local named_pattern='^.+[[:space:]]<[^[:space:]<>]+@[^[:space:]<>]+>$'
  if [[ "${value}" =~ ${bare_pattern} ]]; then
    return
  fi
  if [[ "${value}" =~ ${named_pattern} ]]; then
    return
  fi
  echo "error: ${key} must be a valid sender address (for example Team <newsletter@example.com>)" >&2
  exit 1
}

validate_identifier() {
  local key="$1"
  local value="$2"
  if [[ ! "${value}" =~ ^[a-zA-Z0-9_]+$ ]]; then
    echo "error: ${key} must match ^[a-zA-Z0-9_]+$" >&2
    exit 1
  fi
}

validate_absolute_binary_path() {
  local key="$1"
  local value="$2"
  if [[ "${value}" != /* ]]; then
    echo "error: ${key} must be an absolute path" >&2
    exit 1
  fi
  if [[ "${value}" =~ [[:space:]] ]]; then
    echo "error: ${key} must not contain whitespace" >&2
    exit 1
  fi
}

validate_loopback_host_port() {
  local key="$1"
  local value="$2"
  local port=""
  local octet_a octet_b octet_c

  if [[ "${value}" =~ ^localhost:([0-9]+)$ ]]; then
    port="${BASH_REMATCH[1]}"
  elif [[ "${value}" =~ ^\[::1\]:([0-9]+)$ ]]; then
    port="${BASH_REMATCH[1]}"
  elif [[ "${value}" =~ ^127\.([0-9]+)\.([0-9]+)\.([0-9]+):([0-9]+)$ ]]; then
    octet_a="${BASH_REMATCH[1]}"
    octet_b="${BASH_REMATCH[2]}"
    octet_c="${BASH_REMATCH[3]}"
    port="${BASH_REMATCH[4]}"
    for octet in "${octet_a}" "${octet_b}" "${octet_c}"; do
      if ((octet < 0 || octet > 255)); then
        echo "error: ${key} has invalid loopback IPv4 octet in ${value}" >&2
        exit 1
      fi
    done
  else
    echo "error: ${key} must be loopback host:port (localhost, 127.x.x.x, or [::1])" >&2
    exit 1
  fi

  if [[ ! "${port}" =~ ^[0-9]+$ ]] || ((port < 1 || port > 65535)); then
    echo "error: ${key} port must be between 1 and 65535" >&2
    exit 1
  fi
}

validate_https_origin() {
  local key="$1"
  local value="$2"
  local port=""
  if [[ ! "${value}" =~ ^https://(([^/:?#@]+)|(\[[0-9A-Fa-f:]+\]))(:([0-9]+))?/?$ ]]; then
    echo "error: ${key} must be an https origin with no path/query/fragment" >&2
    exit 1
  fi
  port="${BASH_REMATCH[5]:-}"
  if [[ -n "${port}" ]] && ((port < 1 || port > 65535)); then
    echo "error: ${key} port must be between 1 and 65535" >&2
    exit 1
  fi
}

url_encode() {
  local raw="$1"
  local out=""
  local i ch
  for ((i = 0; i < ${#raw}; i++)); do
    ch="${raw:i:1}"
    case "${ch}" in
      [a-zA-Z0-9.~_-]) out+="${ch}" ;;
      *)
        printf -v ch '%%%02X' "'${ch}"
        out+="${ch}"
        ;;
    esac
  done
  printf '%s' "${out}"
}

generate_urlsafe_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 48 | tr '+/' '-_' | tr -d '=\n'
    return
  fi
  dd if=/dev/urandom bs=48 count=1 2>/dev/null | base64 | tr '+/' '-_' | tr -d '=\n'
}

require_env "NEWSLETTER_BINARY_PATH"
require_env "NEWSLETTER_STAGING_DB_PASSWORD"
require_env "NEWSLETTER_PROD_DB_PASSWORD"
require_env "NEWSLETTER_STAGING_RESEND_API_KEY"
require_env "NEWSLETTER_PROD_RESEND_API_KEY"
require_env "NEWSLETTER_STAGING_RESEND_FROM"
require_env "NEWSLETTER_PROD_RESEND_FROM"
require_env "NEWSLETTER_STAGING_PUBLIC_BASE_URL"
require_env "NEWSLETTER_PROD_PUBLIC_BASE_URL"

NEWSLETTER_STAGING_LINK_SECRET="${NEWSLETTER_STAGING_LINK_SECRET:-$(generate_urlsafe_secret)}"
NEWSLETTER_PROD_LINK_SECRET="${NEWSLETTER_PROD_LINK_SECRET:-$(generate_urlsafe_secret)}"

validate_no_single_quote "NEWSLETTER_STAGING_DB_PASSWORD" "${NEWSLETTER_STAGING_DB_PASSWORD}"
validate_no_single_quote "NEWSLETTER_PROD_DB_PASSWORD" "${NEWSLETTER_PROD_DB_PASSWORD}"
validate_no_single_quote "NEWSLETTER_STAGING_RESEND_API_KEY" "${NEWSLETTER_STAGING_RESEND_API_KEY}"
validate_no_single_quote "NEWSLETTER_PROD_RESEND_API_KEY" "${NEWSLETTER_PROD_RESEND_API_KEY}"
validate_no_single_quote "NEWSLETTER_STAGING_LINK_SECRET" "${NEWSLETTER_STAGING_LINK_SECRET}"
validate_no_single_quote "NEWSLETTER_PROD_LINK_SECRET" "${NEWSLETTER_PROD_LINK_SECRET}"
validate_no_single_quote "NEWSLETTER_STAGING_PUBLIC_BASE_URL" "${NEWSLETTER_STAGING_PUBLIC_BASE_URL}"
validate_no_single_quote "NEWSLETTER_PROD_PUBLIC_BASE_URL" "${NEWSLETTER_PROD_PUBLIC_BASE_URL}"
validate_no_whitespace "NEWSLETTER_STAGING_DB_PASSWORD" "${NEWSLETTER_STAGING_DB_PASSWORD}"
validate_no_whitespace "NEWSLETTER_PROD_DB_PASSWORD" "${NEWSLETTER_PROD_DB_PASSWORD}"
validate_no_whitespace "NEWSLETTER_STAGING_RESEND_API_KEY" "${NEWSLETTER_STAGING_RESEND_API_KEY}"
validate_no_whitespace "NEWSLETTER_PROD_RESEND_API_KEY" "${NEWSLETTER_PROD_RESEND_API_KEY}"
validate_no_whitespace "NEWSLETTER_STAGING_LINK_SECRET" "${NEWSLETTER_STAGING_LINK_SECRET}"
validate_no_whitespace "NEWSLETTER_PROD_LINK_SECRET" "${NEWSLETTER_PROD_LINK_SECRET}"
validate_no_whitespace "NEWSLETTER_STAGING_PUBLIC_BASE_URL" "${NEWSLETTER_STAGING_PUBLIC_BASE_URL}"
validate_no_whitespace "NEWSLETTER_PROD_PUBLIC_BASE_URL" "${NEWSLETTER_PROD_PUBLIC_BASE_URL}"
validate_min_length "NEWSLETTER_STAGING_LINK_SECRET" "${NEWSLETTER_STAGING_LINK_SECRET}" 32
validate_min_length "NEWSLETTER_PROD_LINK_SECRET" "${NEWSLETTER_PROD_LINK_SECRET}" 32
validate_sender_address "NEWSLETTER_STAGING_RESEND_FROM" "${NEWSLETTER_STAGING_RESEND_FROM}"
validate_sender_address "NEWSLETTER_PROD_RESEND_FROM" "${NEWSLETTER_PROD_RESEND_FROM}"
validate_https_origin "NEWSLETTER_STAGING_PUBLIC_BASE_URL" "${NEWSLETTER_STAGING_PUBLIC_BASE_URL}"
validate_https_origin "NEWSLETTER_PROD_PUBLIC_BASE_URL" "${NEWSLETTER_PROD_PUBLIC_BASE_URL}"

validate_identifier "NEWSLETTER_STAGING_DB_ROLE" "${ROLE_STAGING}"
validate_identifier "NEWSLETTER_PROD_DB_ROLE" "${ROLE_PROD}"
validate_identifier "NEWSLETTER_STAGING_DB_NAME" "${DB_STAGING}"
validate_identifier "NEWSLETTER_PROD_DB_NAME" "${DB_PROD}"
validate_absolute_binary_path "NEWSLETTER_BIN_PATH" "${NEWSLETTER_BIN_PATH}"
validate_loopback_host_port "NEWSLETTER_STAGING_HTTP_ADDR" "${STAGING_HTTP_ADDR}"
validate_loopback_host_port "NEWSLETTER_PROD_HTTP_ADDR" "${PROD_HTTP_ADDR}"

STAGING_DB_PASSWORD_URL="$(url_encode "${NEWSLETTER_STAGING_DB_PASSWORD}")"
PROD_DB_PASSWORD_URL="$(url_encode "${NEWSLETTER_PROD_DB_PASSWORD}")"

if [[ ! -f "${NEWSLETTER_BINARY_PATH}" ]]; then
  echo "error: NEWSLETTER_BINARY_PATH does not exist: ${NEWSLETTER_BINARY_PATH}" >&2
  exit 1
fi
if [[ ! -f "${STAGING_UNIT_SOURCE}" ]]; then
  echo "error: staging systemd unit not found: ${STAGING_UNIT_SOURCE}" >&2
  exit 1
fi
if [[ ! -f "${PROD_UNIT_SOURCE}" ]]; then
  echo "error: prod systemd unit not found: ${PROD_UNIT_SOURCE}" >&2
  exit 1
fi

if ! getent group "${NEWSLETTER_GROUP}" >/dev/null 2>&1; then
  run_root groupadd --system "${NEWSLETTER_GROUP}"
fi

if ! id -u "${NEWSLETTER_USER}" >/dev/null 2>&1; then
  run_root useradd \
    --system \
    --gid "${NEWSLETTER_GROUP}" \
    --create-home \
    --home-dir "${NEWSLETTER_DATA_DIR}" \
    --shell /usr/sbin/nologin \
    "${NEWSLETTER_USER}"
fi

run_root install -d -m 750 -o "${NEWSLETTER_USER}" -g "${NEWSLETTER_GROUP}" "${NEWSLETTER_DATA_DIR}"
run_root install -d -m 750 -o root -g "${NEWSLETTER_GROUP}" "${NEWSLETTER_ETC_DIR}"
run_root install -m 755 -o root -g root "${NEWSLETTER_BINARY_PATH}" "${NEWSLETTER_BIN_PATH}"

run_psql_as_postgres -v ON_ERROR_STOP=1 <<SQL
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', '${ROLE_STAGING}', '${NEWSLETTER_STAGING_DB_PASSWORD}')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${ROLE_STAGING}')\gexec
SELECT format('ALTER ROLE %I WITH PASSWORD %L', '${ROLE_STAGING}', '${NEWSLETTER_STAGING_DB_PASSWORD}')\gexec

SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', '${ROLE_PROD}', '${NEWSLETTER_PROD_DB_PASSWORD}')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${ROLE_PROD}')\gexec
SELECT format('ALTER ROLE %I WITH PASSWORD %L', '${ROLE_PROD}', '${NEWSLETTER_PROD_DB_PASSWORD}')\gexec

SELECT format('CREATE DATABASE %I OWNER %I', '${DB_STAGING}', '${ROLE_STAGING}')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '${DB_STAGING}')\gexec
SELECT format('CREATE DATABASE %I OWNER %I', '${DB_PROD}', '${ROLE_PROD}')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '${DB_PROD}')\gexec

SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', '${DB_STAGING}')\gexec
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', '${DB_PROD}')\gexec
SELECT format('GRANT CONNECT, TEMPORARY ON DATABASE %I TO %I', '${DB_STAGING}', '${ROLE_STAGING}')\gexec
SELECT format('GRANT CONNECT, TEMPORARY ON DATABASE %I TO %I', '${DB_PROD}', '${ROLE_PROD}')\gexec
SELECT format('REVOKE CONNECT ON DATABASE %I FROM %I', '${DB_PROD}', '${ROLE_STAGING}')\gexec
SELECT format('REVOKE CONNECT ON DATABASE %I FROM %I', '${DB_STAGING}', '${ROLE_PROD}')\gexec
SQL

staging_env_tmp="$(mktemp)"
cat > "${staging_env_tmp}" <<ENV
NEWSLETTER_ENV=staging
NEWSLETTER_HTTP_ADDR=${STAGING_HTTP_ADDR}
NEWSLETTER_DATABASE_URL=postgres://${ROLE_STAGING}:${STAGING_DB_PASSWORD_URL}@${DB_HOST}:${DB_PORT}/${DB_STAGING}?sslmode=disable
NEWSLETTER_PUBLIC_BASE_URL=${NEWSLETTER_STAGING_PUBLIC_BASE_URL}
NEWSLETTER_RESEND_API_KEY=${NEWSLETTER_STAGING_RESEND_API_KEY}
NEWSLETTER_RESEND_FROM=${NEWSLETTER_STAGING_RESEND_FROM}
NEWSLETTER_LINK_SECRET=${NEWSLETTER_STAGING_LINK_SECRET}
ENV
run_root install -m 640 -o root -g "${NEWSLETTER_GROUP}" "${staging_env_tmp}" "${NEWSLETTER_ETC_DIR}/staging.env"
rm -f "${staging_env_tmp}"

prod_env_tmp="$(mktemp)"
cat > "${prod_env_tmp}" <<ENV
NEWSLETTER_ENV=prod
NEWSLETTER_HTTP_ADDR=${PROD_HTTP_ADDR}
NEWSLETTER_DATABASE_URL=postgres://${ROLE_PROD}:${PROD_DB_PASSWORD_URL}@${DB_HOST}:${DB_PORT}/${DB_PROD}?sslmode=disable
NEWSLETTER_PUBLIC_BASE_URL=${NEWSLETTER_PROD_PUBLIC_BASE_URL}
NEWSLETTER_RESEND_API_KEY=${NEWSLETTER_PROD_RESEND_API_KEY}
NEWSLETTER_RESEND_FROM=${NEWSLETTER_PROD_RESEND_FROM}
NEWSLETTER_LINK_SECRET=${NEWSLETTER_PROD_LINK_SECRET}
ENV
run_root install -m 640 -o root -g "${NEWSLETTER_GROUP}" "${prod_env_tmp}" "${NEWSLETTER_ETC_DIR}/prod.env"
rm -f "${prod_env_tmp}"

escaped_bin_path="$(printf '%s' "${NEWSLETTER_BIN_PATH}" | sed -e 's/[|&]/\\&/g')"

staging_unit_tmp="$(mktemp)"
sed "s|/usr/local/bin/htmlctl-newsletter|${escaped_bin_path}|g" "${STAGING_UNIT_SOURCE}" > "${staging_unit_tmp}"
run_root install -m 644 -o root -g root "${staging_unit_tmp}" "${SYSTEMD_DIR}/htmlctl-newsletter-staging.service"
rm -f "${staging_unit_tmp}"

prod_unit_tmp="$(mktemp)"
sed "s|/usr/local/bin/htmlctl-newsletter|${escaped_bin_path}|g" "${PROD_UNIT_SOURCE}" > "${prod_unit_tmp}"
run_root install -m 644 -o root -g root "${prod_unit_tmp}" "${SYSTEMD_DIR}/htmlctl-newsletter-prod.service"
rm -f "${prod_unit_tmp}"

run_root systemctl daemon-reload
run_root systemctl enable htmlctl-newsletter-staging htmlctl-newsletter-prod
run_root systemctl restart htmlctl-newsletter-staging htmlctl-newsletter-prod

run_root systemctl --no-pager --full status htmlctl-newsletter-staging || true
run_root systemctl --no-pager --full status htmlctl-newsletter-prod || true

echo "newsletter extension setup complete"
echo "verify listeners: sudo ss -tlnp | grep ':9501\\|:9502'"
echo "verify health: curl -sf http://127.0.0.1:9501/healthz && curl -sf http://127.0.0.1:9502/healthz"
echo "verify env perms: stat -c '%a %U %G %n' ${NEWSLETTER_ETC_DIR}/staging.env ${NEWSLETTER_ETC_DIR}/prod.env"
