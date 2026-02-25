#!/usr/bin/env bash
set -euo pipefail

DOMAIN="staging.example.com"
SITE_ROOT="/var/lib/htmlservd/websites/sample/envs/staging/current"
SITE_FILE="/etc/nginx/sites-available/${DOMAIN}"
SITE_ENABLED="/etc/nginx/sites-enabled/${DOMAIN}"
GENERAL_LIMIT_FILE="/etc/nginx/conf.d/general_limit.conf"

run_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

write_file_as_root() {
  local target="$1"
  local mode="$2"
  local owner="$3"
  local group="$4"
  local content="$5"
  local tmp

  tmp="$(mktemp)"
  printf '%s\n' "${content}" > "${tmp}"
  run_root install -m "${mode}" -o "${owner}" -g "${group}" "${tmp}" "${target}"
  rm -f "${tmp}"
}

ensure_general_limit_zone() {
  local existing
  existing="$(run_root sh -c "grep -RhsE 'limit_req_zone[^;]*zone=general_limit[^;]*;' /etc/nginx/nginx.conf /etc/nginx/conf.d/*.conf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/* 2>/dev/null | head -n1" || true)"

  if [[ -z "${existing}" ]]; then
    write_file_as_root "${GENERAL_LIMIT_FILE}" 644 root root 'limit_req_zone $binary_remote_addr zone=general_limit:10m rate=50r/s;'
  elif [[ "${existing}" != *"rate=50r/s"* ]]; then
    echo "warning: existing general_limit zone does not advertise rate=50r/s: ${existing}" >&2
    echo "warning: reusing existing zone definition as requested" >&2
  fi
}

write_bootstrap_config() {
  local bootstrap_conf
  bootstrap_conf="server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};

    root ${SITE_ROOT};
    index index.html;

    location / {
        try_files \$uri \$uri/ \$uri.html =404;
    }
}"
  write_file_as_root "${SITE_FILE}" 644 root root "${bootstrap_conf}"
}

write_final_config() {
  local final_conf
  final_conf="server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};
    server_tokens off;

    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name ${DOMAIN};

    root ${SITE_ROOT};
    index index.html;
    server_tokens off;

    ssl_certificate /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    limit_req zone=general_limit burst=100 nodelay;

    add_header Strict-Transport-Security \"max-age=31536000; includeSubDomains; preload\" always;
    add_header X-Frame-Options \"SAMEORIGIN\" always;
    add_header X-Content-Type-Options \"nosniff\" always;
    add_header X-XSS-Protection \"1; mode=block\" always;
    add_header Referrer-Policy \"strict-origin-when-cross-origin\" always;
    add_header Permissions-Policy \"accelerometer=(), autoplay=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()\" always;

    location / {
        try_files \$uri \$uri/ \$uri.html =404;
    }
}"
  write_file_as_root "${SITE_FILE}" 644 root root "${final_conf}"
}

ensure_general_limit_zone
write_bootstrap_config
run_root ln -sfn "${SITE_FILE}" "${SITE_ENABLED}"
run_root nginx -t
run_root systemctl reload nginx

if [[ ! -f "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" ]]; then
  run_root certbot --nginx -d "${DOMAIN}"
fi

write_final_config
run_root nginx -t
run_root systemctl reload nginx

echo "staging nginx site configured: ${DOMAIN}"
