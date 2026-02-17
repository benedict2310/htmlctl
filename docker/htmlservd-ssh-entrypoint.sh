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
mkdir -p "$(dirname "${CADDYFILE_PATH}")"
if [[ ! -f "${CADDYFILE_PATH}" ]]; then
	cat > "${CADDYFILE_PATH}" <<'EOF'
# bootstrap caddy config for first startup
:18080 {
	respond "htmlservd caddy bootstrap"
}
EOF
	# Fix ownership so htmlservd can overwrite it
	chown htmlservd:htmlservd "${CADDYFILE_PATH}"
fi

su -s /bin/sh -c "/usr/local/bin/caddy run --config ${CADDYFILE_PATH} --adapter caddyfile" htmlservd &
caddy_pid=$!

su -s /bin/sh -c "/usr/local/bin/htmlservd" htmlservd &
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
