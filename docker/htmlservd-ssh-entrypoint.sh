#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${SSH_PUBLIC_KEY:-}" ]]; then
	echo "SSH_PUBLIC_KEY is required for htmlservd-ssh image" >&2
	exit 1
fi

mkdir -p /run/sshd /home/htmlservd/.ssh
chmod 700 /home/htmlservd/.ssh
printf '%s\n' "${SSH_PUBLIC_KEY}" > /home/htmlservd/.ssh/authorized_keys
chmod 600 /home/htmlservd/.ssh/authorized_keys
chown -R htmlservd:htmlservd /home/htmlservd/.ssh

if [[ ! -f /etc/ssh/ssh_host_ed25519_key ]]; then
	ssh-keygen -A >/dev/null 2>&1
fi

CADDYFILE_PATH="${HTMLSERVD_CADDYFILE_PATH:-/etc/caddy/Caddyfile}"
CADDY_BOOTSTRAP_MODE="${HTMLSERVD_CADDY_BOOTSTRAP_MODE:-preview}"
CADDY_BOOTSTRAP_LISTEN="${HTMLSERVD_CADDY_BOOTSTRAP_LISTEN:-:80}"
PREVIEW_WEBSITE="${HTMLSERVD_PREVIEW_WEBSITE:-futurelab}"
PREVIEW_ENV="${HTMLSERVD_PREVIEW_ENV:-staging}"
DATA_DIR="${HTMLSERVD_DATA_DIR:-/var/lib/htmlservd}"
PREVIEW_ROOT_DEFAULT="${DATA_DIR}/websites/${PREVIEW_WEBSITE}/envs/${PREVIEW_ENV}/current"
PREVIEW_ROOT="${HTMLSERVD_PREVIEW_ROOT:-${PREVIEW_ROOT_DEFAULT}}"
CADDY_DIR="$(dirname "${CADDYFILE_PATH}")"
mkdir -p "${DATA_DIR}" "${CADDY_DIR}"
# Normalize mounted directory permissions so the non-root daemon can write runtime state.
chown -R htmlservd:htmlservd "${DATA_DIR}" "${CADDY_DIR}"
chmod u+rwx "${DATA_DIR}" "${CADDY_DIR}"
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
	# Fix ownership so htmlservd can overwrite it
	chown htmlservd:htmlservd "${CADDYFILE_PATH}"
fi

su -m -s /bin/sh -c "/usr/local/bin/caddy run --config ${CADDYFILE_PATH} --adapter caddyfile" htmlservd &
caddy_pid=$!

su -m -s /bin/sh -c "/usr/local/bin/htmlservd" htmlservd &
htmlservd_pid=$!

/usr/sbin/sshd -D -e &
sshd_pid=$!

cleanup() {
	kill "${htmlservd_pid}" "${sshd_pid}" "${caddy_pid}" 2>/dev/null || true
	wait "${htmlservd_pid}" "${sshd_pid}" "${caddy_pid}" 2>/dev/null || true
}
trap cleanup INT TERM

wait -n "${htmlservd_pid}" "${sshd_pid}" "${caddy_pid}"
status=$?
cleanup
exit "${status}"
