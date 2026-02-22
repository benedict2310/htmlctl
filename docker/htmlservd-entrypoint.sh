#!/usr/bin/env bash
set -euo pipefail

# Security constraints for environment inputs:
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

main() {
	local caddyfile_path="${HTMLSERVD_CADDYFILE_PATH:-/etc/caddy/Caddyfile}"
	local caddy_bootstrap_mode="${HTMLSERVD_CADDY_BOOTSTRAP_MODE:-preview}"
	local caddy_bootstrap_listen="${HTMLSERVD_CADDY_BOOTSTRAP_LISTEN:-:80}"
	local preview_website="${HTMLSERVD_PREVIEW_WEBSITE:-futurelab}"
	local preview_env="${HTMLSERVD_PREVIEW_ENV:-staging}"
	local preview_root_default="${HTMLSERVD_DATA_DIR:-/var/lib/htmlservd}/websites/${preview_website}/envs/${preview_env}/current"
	local preview_root="${HTMLSERVD_PREVIEW_ROOT:-${preview_root_default}}"
	local caddy_pid=0
	local htmlservd_pid=0
	local status=0

	validate_caddy_bootstrap_listen "${caddy_bootstrap_listen}"
	validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "${preview_website}"
	validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "${preview_env}"
	validate_preview_root "${preview_root}"

	mkdir -p "$(dirname "${caddyfile_path}")"
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
	fi

	/usr/local/bin/caddy run --config "${caddyfile_path}" --adapter caddyfile &
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
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
