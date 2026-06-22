#!/usr/bin/env bash
set -euo pipefail

workflow=".github/workflows/docker-e2e.yml"

if [[ ! -f "${workflow}" ]]; then
  echo "missing ${workflow}" >&2
  exit 1
fi

if ! grep -q 'chmod 0644 "\$STATE_DIR/id_ed25519"' "${workflow}"; then
  echo "${workflow}: generated CI private key must be readable by the non-root htmlctl container user" >&2
  echo "add: chmod 0644 \"\$STATE_DIR/id_ed25519\" after ssh-keygen" >&2
  exit 1
fi

if ! grep -q 'chmod 0644 "\$STATE_DIR/known_hosts"' "${workflow}"; then
  echo "${workflow}: generated known_hosts must be readable by the non-root htmlctl container user" >&2
  echo "add: chmod 0644 \"\$STATE_DIR/known_hosts\" after ssh-keyscan" >&2
  exit 1
fi
