#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

expect_success() {
	local name="$1"
	shift
	if ! "$@" >/dev/null 2>&1; then
		echo "FAIL: ${name}" >&2
		return 1
	fi
}

expect_failure() {
	local name="$1"
	shift
	if "$@" >/dev/null 2>&1; then
		echo "FAIL: ${name}" >&2
		return 1
	fi
}

(
	source "${REPO_ROOT}/docker/htmlservd-entrypoint.sh"

	expect_success "accept :PORT listen" validate_caddy_bootstrap_listen ":8080"
	expect_success "accept HOST:PORT listen" validate_caddy_bootstrap_listen "127.0.0.1:8080"
	expect_success "accept [IPv6]:PORT listen" validate_caddy_bootstrap_listen "[::1]:8080"
	expect_failure "reject listen with brace" validate_caddy_bootstrap_listen ":80{"
	expect_failure "reject listen with newline" validate_caddy_bootstrap_listen $':80\nbad'
	expect_failure "reject listen with invalid format" validate_caddy_bootstrap_listen "8080"
	expect_failure "reject listen with out-of-range port" validate_caddy_bootstrap_listen ":70000"
	expect_failure "reject listen with port zero" validate_caddy_bootstrap_listen ":0"
	expect_success "accept preview root path" validate_preview_root "/var/lib/htmlservd/websites/futurelab/envs/staging/current"
	expect_failure "reject preview root with brace" validate_preview_root "/var/lib/htmlservd/{bad}"
	expect_failure "reject preview root with newline" validate_preview_root $'/var/lib/htmlservd/current\nbad'
	expect_success "accept preview website component" validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "futurelab"
	expect_success "accept preview env component" validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "staging"
	expect_failure "reject preview website traversal" validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "../../etc"
	expect_failure "reject preview env separator" validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "prod/main"
)

(
	source "${REPO_ROOT}/docker/htmlservd-ssh-entrypoint.sh"

	ssh-keygen -t ed25519 -N '' -f "${TMP_DIR}/id_ed25519" >/dev/null
	PUBLIC_KEY="$(<"${TMP_DIR}/id_ed25519.pub")"

	expect_success "accept valid ssh public key" validate_ssh_public_key "${PUBLIC_KEY}"
	expect_failure "reject empty ssh public key" validate_ssh_public_key ""
	expect_failure "reject ssh key options prefix" validate_ssh_public_key "command=\"id\" ${PUBLIC_KEY}"
	expect_failure "reject multi-line ssh public key" validate_ssh_public_key "${PUBLIC_KEY}"$'\n'"${PUBLIC_KEY}"
	expect_success "accept preview website component in ssh entrypoint" validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "futurelab"
	expect_success "accept preview env component in ssh entrypoint" validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "staging"
	expect_failure "reject preview website traversal in ssh entrypoint" validate_preview_path_component "HTMLSERVD_PREVIEW_WEBSITE" "../../etc"
	expect_failure "reject preview env separator in ssh entrypoint" validate_preview_path_component "HTMLSERVD_PREVIEW_ENV" "prod/main"

	PUBLIC_KEY_PADDED="$(awk '{print $1 "   " $2 "   " $3}' <<< "${PUBLIC_KEY}")"
	NORMALIZED_KEY="$(validate_ssh_public_key "${PUBLIC_KEY_PADDED}")"
	write_restricted_authorized_key "${NORMALIZED_KEY}" "${TMP_DIR}/authorized_keys"
	AUTHORIZED_KEY_LINE="$(<"${TMP_DIR}/authorized_keys")"
	if [[ "${AUTHORIZED_KEY_LINE}" != "restrict,port-forwarding ${NORMALIZED_KEY}" ]]; then
		echo "FAIL: authorized_keys line does not enforce normalized restrict,port-forwarding prefix" >&2
		exit 1
	fi
)

echo "entrypoint security validation tests passed"
