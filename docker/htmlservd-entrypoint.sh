#!/usr/bin/env bash
set -euo pipefail

CADDYFILE_PATH="${HTMLSERVD_CADDYFILE_PATH:-/etc/caddy/Caddyfile}"
mkdir -p "$(dirname "${CADDYFILE_PATH}")"
if [[ ! -f "${CADDYFILE_PATH}" ]]; then
	cat > "${CADDYFILE_PATH}" <<'EOF'
# bootstrap caddy config for first startup
:18080 {
	respond "htmlservd caddy bootstrap"
}
EOF
fi

/usr/local/bin/caddy run --config "${CADDYFILE_PATH}" --adapter caddyfile &
caddy_pid=$!

/usr/local/bin/htmlservd &
htmlservd_pid=$!

cleanup() {
	kill "${htmlservd_pid}" "${caddy_pid}" 2>/dev/null || true
	wait "${htmlservd_pid}" "${caddy_pid}" 2>/dev/null || true
}
trap cleanup INT TERM

wait -n "${htmlservd_pid}" "${caddy_pid}"
status=$?
cleanup
exit "${status}"
