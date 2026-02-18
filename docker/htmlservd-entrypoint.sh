#!/usr/bin/env bash
set -euo pipefail

CADDYFILE_PATH="${HTMLSERVD_CADDYFILE_PATH:-/etc/caddy/Caddyfile}"
CADDY_BOOTSTRAP_MODE="${HTMLSERVD_CADDY_BOOTSTRAP_MODE:-preview}"
CADDY_BOOTSTRAP_LISTEN="${HTMLSERVD_CADDY_BOOTSTRAP_LISTEN:-:80}"
PREVIEW_WEBSITE="${HTMLSERVD_PREVIEW_WEBSITE:-futurelab}"
PREVIEW_ENV="${HTMLSERVD_PREVIEW_ENV:-staging}"
PREVIEW_ROOT_DEFAULT="${HTMLSERVD_DATA_DIR:-/var/lib/htmlservd}/websites/${PREVIEW_WEBSITE}/envs/${PREVIEW_ENV}/current"
PREVIEW_ROOT="${HTMLSERVD_PREVIEW_ROOT:-${PREVIEW_ROOT_DEFAULT}}"
mkdir -p "$(dirname "${CADDYFILE_PATH}")"
if [[ ! -f "${CADDYFILE_PATH}" ]]; then
	case "${CADDY_BOOTSTRAP_MODE,,}" in
	preview)
		cat > "${CADDYFILE_PATH}" <<EOF
# bootstrap caddy config for first startup (preview mode)
${CADDY_BOOTSTRAP_LISTEN} {
	root * ${PREVIEW_ROOT}
	file_server
}
EOF
		;;
	bootstrap)
		cat > "${CADDYFILE_PATH}" <<EOF
# bootstrap caddy config for first startup (bootstrap mode)
${CADDY_BOOTSTRAP_LISTEN} {
	respond "htmlservd caddy bootstrap"
}
EOF
		;;
	*)
		echo "unsupported HTMLSERVD_CADDY_BOOTSTRAP_MODE: ${CADDY_BOOTSTRAP_MODE} (expected preview|bootstrap)" >&2
		exit 1
		;;
	esac
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
