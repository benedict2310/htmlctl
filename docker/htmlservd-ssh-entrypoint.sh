#!/usr/bin/env bash
set -euo pipefail

# Security constraints for environment inputs:
# - SSH_PUBLIC_KEY must be a single bare OpenSSH public key line (no options).
# - HTMLSERVD_CADDY_BOOTSTRAP_LISTEN must be :PORT, HOST:PORT, or [IPv6]:PORT.
# - HTMLSERVD_PREVIEW_WEBSITE and HTMLSERVD_PREVIEW_ENV must be safe path
#   components (alphanumeric, dot, underscore, hyphen; no traversal).
# - HTMLSERVD_CADDY_BOOTSTRAP_LISTEN and HTMLSERVD_PREVIEW_ROOT must not contain
#   newlines, '{', or '}' (to prevent Caddyfile directive injection).

contains_forbidden_caddy_chars() {
	local value="$1"
	[[ "${value}" == *$'\n'* || "${value}" == *$'\r'* || "${value}" == *'{'* || "${value}" == *'}'* ]]
}

validate_caddy_plain_value() {
	local name="$1"
	local value="$2"

	if contains_forbidden_caddy_chars "${value}"; then
		echo "invalid ${name}: contains forbidden characters (newline, '{', '}')" >&2
		return 1
	fi
}

validate_caddy_bootstrap_listen() {
	local listen="$1"
	local port=""

	validate_caddy_plain_value "HTMLSERVD_CADDY_BOOTSTRAP_LISTEN" "${listen}" || return 1

	if [[ "${listen}" =~ ^:([0-9]{1,5})$ ]]; then
		port="${BASH_REMATCH[1]}"
	elif [[ "${listen}" =~ ^[A-Za-z0-9._-]+:([0-9]{1,5})$ ]]; then
		port="${BASH_REMATCH[1]}"
	elif [[ "${listen}" =~ ^\[[0-9A-Fa-f:]+\]:([0-9]{1,5})$ ]]; then
		port="${BASH_REMATCH[1]}"
	else
		echo "invalid HTMLSERVD_CADDY_BOOTSTRAP_LISTEN: ${listen} (expected :PORT, HOST:PORT, or [IPv6]:PORT)" >&2
		return 1
	fi

	if ((10#${port} < 1 || 10#${port} > 65535)); then
		echo "invalid HTMLSERVD_CADDY_BOOTSTRAP_LISTEN: port out of range (${port})" >&2
		return 1
	fi
}

validate_preview_root() {
	local root="$1"
	validate_caddy_plain_value "HTMLSERVD_PREVIEW_ROOT" "${root}"
}

validate_preview_path_component() {
	local name="$1"
	local component="$2"

	if [[ -z "${component}" ]]; then
		echo "invalid ${name}: value must not be empty" >&2
		return 1
	fi

	if [[ "${component}" == *".."* ]]; then
		echo "invalid ${name}: path traversal patterns are not allowed" >&2
		return 1
	fi

	if [[ ! "${component}" =~ ^[A-Za-z0-9._-]+$ ]]; then
		echo "invalid ${name}: allowed characters are [A-Za-z0-9._-]" >&2
		return 1
	fi
}

is_supported_ssh_key_type() {
	local key_type="$1"
	case "${key_type}" in
	ssh-ed25519 | ssh-rsa | ecdsa-sha2-nistp256 | ecdsa-sha2-nistp384 | ecdsa-sha2-nistp521 | sk-ssh-ed25519@openssh.com | sk-ecdsa-sha2-nistp256@openssh.com)
		return 0
		;;
	*)
		return 1
		;;
	esac
}

validate_ssh_public_key() {
	local public_key="$1"
	local key_type=""
	local key_data=""
	local key_comment=""
	local normalized_key=""

	if [[ -z "${public_key}" ]]; then
		echo "SSH_PUBLIC_KEY is required for htmlservd-ssh image" >&2
		return 1
	fi

	if [[ "${public_key}" == *$'\n'* || "${public_key}" == *$'\r'* ]]; then
		echo "invalid SSH_PUBLIC_KEY: must be a single line public key" >&2
		return 1
	fi

	read -r key_type key_data key_comment <<< "${public_key}"
	if [[ -z "${key_type}" || -z "${key_data}" ]]; then
		echo "invalid SSH_PUBLIC_KEY: expected '<type> <base64> [comment]'" >&2
		return 1
	fi

	if ! is_supported_ssh_key_type "${key_type}"; then
		echo "invalid SSH_PUBLIC_KEY: unsupported key type or key options are not allowed" >&2
		return 1
	fi

	if [[ ! "${key_data}" =~ ^[A-Za-z0-9+/=]+$ ]]; then
		echo "invalid SSH_PUBLIC_KEY: invalid key payload encoding" >&2
		return 1
	fi

	normalized_key="${key_type} ${key_data}"
	if [[ -n "${key_comment}" ]]; then
		normalized_key="${normalized_key} ${key_comment}"
	fi

	if ! printf '%s\n' "${normalized_key}" | ssh-keygen -l -f /dev/stdin >/dev/null 2>&1; then
		echo "invalid SSH_PUBLIC_KEY: ssh-keygen could not parse key" >&2
		return 1
	fi

	printf '%s\n' "${normalized_key}"
}

write_restricted_authorized_key() {
	local public_key="$1"
	local destination="$2"
	printf 'restrict,port-forwarding %s\n' "${public_key}" > "${destination}"
}

run_as_htmlservd() {
	if command -v runuser >/dev/null 2>&1; then
		runuser -u htmlservd -- "$@"
		return $?
	fi
	su -m -s /bin/sh -c 'exec "$@"' htmlservd -- "$@"
}

main() {
	local ssh_public_key="${SSH_PUBLIC_KEY:-}"
	local caddyfile_path="${HTMLSERVD_CADDYFILE_PATH:-/etc/caddy/Caddyfile}"
	local caddy_bootstrap_mode="${HTMLSERVD_CADDY_BOOTSTRAP_MODE:-preview}"
	local caddy_bootstrap_listen="${HTMLSERVD_CADDY_BOOTSTRAP_LISTEN:-:80}"
	local preview_website="${HTMLSERVD_PREVIEW_WEBSITE:-futurelab}"
	local preview_env="${HTMLSERVD_PREVIEW_ENV:-staging}"
	local data_dir="${HTMLSERVD_DATA_DIR:-/var/lib/htmlservd}"
	local preview_root_default="${data_dir}/websites/${preview_website}/envs/${preview_env}/current"
	local preview_root="${HTMLSERVD_PREVIEW_ROOT:-${preview_root_default}}"
	local normalized_ssh_public_key=""
	local caddy_dir
	local caddy_pid=0
	local htmlservd_pid=0
	local sshd_pid=0
	local status=0

	normalized_ssh_public_key="$(validate_ssh_public_key "${ssh_public_key}")"
	validate_caddy_bootstrap_listen "${caddy_bootstrap_listen}"
	validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "${preview_website}"
	validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "${preview_env}"
	validate_preview_root "${preview_root}"

	mkdir -p /run/sshd /home/htmlservd/.ssh
	chmod 700 /home/htmlservd/.ssh
	write_restricted_authorized_key "${normalized_ssh_public_key}" /home/htmlservd/.ssh/authorized_keys
	chmod 600 /home/htmlservd/.ssh/authorized_keys
	chown -R htmlservd:htmlservd /home/htmlservd/.ssh

	if [[ ! -f /etc/ssh/ssh_host_ed25519_key ]]; then
		ssh-keygen -A >/dev/null 2>&1
	fi

	caddy_dir="$(dirname "${caddyfile_path}")"
	mkdir -p "${data_dir}" "${caddy_dir}"
	# Normalize mounted directory permissions so the non-root daemon can write runtime state.
	chown -R htmlservd:htmlservd "${data_dir}" "${caddy_dir}"
	chmod u+rwx "${data_dir}" "${caddy_dir}"
	if [[ ! -f "${caddyfile_path}" ]]; then
		case "${caddy_bootstrap_mode,,}" in
		preview)
			cat > "${caddyfile_path}" <<EOF
# bootstrap caddy config for first startup (preview mode)
${caddy_bootstrap_listen} {
	root * ${preview_root}
	file_server
}
EOF
			;;
		bootstrap)
			cat > "${caddyfile_path}" <<EOF
# bootstrap caddy config for first startup (bootstrap mode)
${caddy_bootstrap_listen} {
	respond "htmlservd caddy bootstrap"
}
EOF
			;;
		*)
			echo "unsupported HTMLSERVD_CADDY_BOOTSTRAP_MODE: ${caddy_bootstrap_mode} (expected preview|bootstrap)" >&2
			exit 1
			;;
		esac
		# Fix ownership so htmlservd can overwrite it
		chown htmlservd:htmlservd "${caddyfile_path}"
	fi

	run_as_htmlservd /usr/local/bin/caddy run --config "${caddyfile_path}" --adapter caddyfile &
	caddy_pid=$!

	run_as_htmlservd /usr/local/bin/htmlservd &
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
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
