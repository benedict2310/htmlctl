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

if grep -q 'chmod 0644 "\$STATE_DIR/known_hosts"' "${workflow}"; then
  echo "${workflow}: do not chmod known_hosts on the host; the file is created by a root container and GitHub's runner cannot change it" >&2
  echo "run chmod inside the ssh-keyscan container instead" >&2
  exit 1
fi

if ! grep -q 'chmod 0644 /work/known_hosts' "${workflow}"; then
  echo "${workflow}: generated known_hosts must be made readable inside the container that creates it" >&2
  echo "add: chmod 0644 /work/known_hosts after ssh-keyscan" >&2
  exit 1
fi
