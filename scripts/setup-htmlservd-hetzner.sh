#!/usr/bin/env bash
set -euo pipefail

HTMLSERVD_USER="htmlservd"
HTMLSERVD_HOME="/home/htmlservd"
HTMLSERVD_SSH_DIR="${HTMLSERVD_HOME}/.ssh"
HTMLSERVD_AUTH_KEYS="${HTMLSERVD_SSH_DIR}/authorized_keys"
HTMLSERVD_DATA_DIR="/var/lib/htmlservd"
HTMLSERVD_ETC_DIR="/etc/htmlservd"
HTMLSERVD_CONFIG_PATH="${HTMLSERVD_ETC_DIR}/config.yaml"
HTMLSERVD_ENV_PATH="${HTMLSERVD_ETC_DIR}/env"
HTMLSERVD_SERVICE_PATH="/etc/systemd/system/htmlservd.service"
HTMLSERVD_NOOP_CADDY="/usr/local/bin/htmlservd-caddy-noop"
SSHD_DROPIN="/etc/ssh/sshd_config.d/htmlservd.conf"

run_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "error: ${key} must be set" >&2
    exit 1
  fi
}

normalize_ssh_key() {
  printf '%s' "$1" | tr -d '\r' | awk '{$1=$1; print}'
}

require_env "HTMLSERVD_SSH_PUBKEY"
SSH_PUBKEY="$(normalize_ssh_key "${HTMLSERVD_SSH_PUBKEY}")"

if [[ -z "${SSH_PUBKEY}" || "${SSH_PUBKEY}" == *$'\n'* ]]; then
  echo "error: HTMLSERVD_SSH_PUBKEY must contain exactly one public key" >&2
  exit 1
fi
if [[ "${SSH_PUBKEY}" == restrict,* ]]; then
  echo "error: HTMLSERVD_SSH_PUBKEY must not include key options; script adds restrict,port-forwarding" >&2
  exit 1
fi
if ! [[ "${SSH_PUBKEY}" =~ ^(ssh-(ed25519|rsa)|ecdsa-sha2-nistp(256|384|521))[[:space:]] ]]; then
  echo "error: HTMLSERVD_SSH_PUBKEY does not look like a supported SSH public key" >&2
  exit 1
fi

if ! id -u "${HTMLSERVD_USER}" >/dev/null 2>&1; then
  run_root useradd \
    --system \
    --create-home \
    --home-dir "${HTMLSERVD_HOME}" \
    --shell /usr/sbin/nologin \
    "${HTMLSERVD_USER}"
else
  run_root usermod --home "${HTMLSERVD_HOME}" --shell /usr/sbin/nologin "${HTMLSERVD_USER}"
  run_root mkdir -p "${HTMLSERVD_HOME}"
fi

run_root chown "${HTMLSERVD_USER}:${HTMLSERVD_USER}" "${HTMLSERVD_HOME}"
run_root chmod 750 "${HTMLSERVD_HOME}"

run_root install -d -m 700 -o "${HTMLSERVD_USER}" -g "${HTMLSERVD_USER}" "${HTMLSERVD_SSH_DIR}"

auth_keys_tmp="$(mktemp)"
trap 'rm -f "${auth_keys_tmp}"' EXIT
printf 'restrict,port-forwarding %s\n' "${SSH_PUBKEY}" > "${auth_keys_tmp}"
run_root install -m 600 -o "${HTMLSERVD_USER}" -g "${HTMLSERVD_USER}" "${auth_keys_tmp}" "${HTMLSERVD_AUTH_KEYS}"

run_root install -d -m 755 -o "${HTMLSERVD_USER}" -g "${HTMLSERVD_USER}" "${HTMLSERVD_DATA_DIR}"
run_root install -d -m 775 -o root -g "${HTMLSERVD_USER}" "${HTMLSERVD_ETC_DIR}"

config_tmp="$(mktemp)"
cat > "${config_tmp}" <<'YAML'
bind: 127.0.0.1
port: 9400
dataDir: /var/lib/htmlservd
logLevel: info
dbWAL: true
# Caddy is not yet active in Phase 1 (Nginx handles staging).
# Point to a no-op script so domain reload commands don't crash.
caddyBinaryPath: /usr/local/bin/htmlservd-caddy-noop
caddyfilePath: /etc/htmlservd/Caddyfile
caddyAutoHTTPS: false
api:
  token: ""  # filled via HTMLSERVD_API_TOKEN env var in service unit
telemetry:
  enabled: false
YAML
run_root install -m 640 -o root -g "${HTMLSERVD_USER}" "${config_tmp}" "${HTMLSERVD_CONFIG_PATH}"
rm -f "${config_tmp}"

env_tmp="$(mktemp)"
cat > "${env_tmp}" <<'ENV'
# Set this before first htmlctl apply.
HTMLSERVD_API_TOKEN=
ENV
run_root install -m 640 -o root -g "${HTMLSERVD_USER}" "${env_tmp}" "${HTMLSERVD_ENV_PATH}"
rm -f "${env_tmp}"

noop_tmp="$(mktemp)"
cat > "${noop_tmp}" <<'NOOP'
#!/bin/bash
# Phase 1 no-op: Caddy not yet active. Remove when Caddy replaces Nginx.
exit 0
NOOP
run_root install -m 755 -o root -g root "${noop_tmp}" "${HTMLSERVD_NOOP_CADDY}"
rm -f "${noop_tmp}"

service_tmp="$(mktemp)"
cat > "${service_tmp}" <<'UNIT'
[Unit]
Description=htmlservd static site daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=htmlservd
Group=htmlservd
EnvironmentFile=/etc/htmlservd/env
ExecStart=/usr/local/bin/htmlservd -config /etc/htmlservd/config.yaml
WorkingDirectory=/var/lib/htmlservd
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/var/lib/htmlservd /etc/htmlservd

[Install]
WantedBy=multi-user.target
UNIT
run_root install -m 644 -o root -g root "${service_tmp}" "${HTMLSERVD_SERVICE_PATH}"
rm -f "${service_tmp}"

run_root install -d -m 755 -o root -g root /etc/ssh/sshd_config.d

sshd_tmp="$(mktemp)"
cat > "${sshd_tmp}" <<'SSHD'
# htmlservd tunnel user — TCP forwarding only, no shell
Match User htmlservd
    AllowTcpForwarding yes
    X11Forwarding no
    PermitTTY no
    ForceCommand /usr/sbin/nologin
    GatewayPorts no
    PermitTunnel no
SSHD
run_root install -m 644 -o root -g root "${sshd_tmp}" "${SSHD_DROPIN}"
rm -f "${sshd_tmp}"

run_root systemctl daemon-reload
run_root systemctl enable htmlservd
run_root systemctl start htmlservd
# Ubuntu 24.04 uses ssh.service (socket-activated); older distros use sshd.service.
run_root systemctl reload ssh 2>/dev/null || run_root systemctl reload sshd

echo "htmlservd Phase 1 setup complete"
