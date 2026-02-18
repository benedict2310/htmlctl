#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TARGET="${1:-.tmp}"

case "${TARGET}" in
  .tmp|.tmp/*)
    ;;
  *)
    echo "refusing to clean outside .tmp: ${TARGET}" >&2
    exit 2
    ;;
esac

TARGET_PATH="${ROOT_DIR}/${TARGET}"

if [[ ! -e "${TARGET_PATH}" ]]; then
  echo "nothing to clean: ${TARGET}"
  exit 0
fi

rm_path() {
  local path="$1"
  rm -rf "${path}" || true
}

docker_force_clean() {
  local path="$1"
  if ! command -v docker >/dev/null 2>&1; then
    return 1
  fi

  docker run --rm -v "${path}:/target" alpine:3.20 sh -lc '
    rm -rf /target/* /target/.[!.]* /target/..?* 2>/dev/null || true
  ' >/dev/null
}

rm_path "${TARGET_PATH}"
if [[ -e "${TARGET_PATH}" ]]; then
  echo "normal cleanup did not fully remove ${TARGET}; trying docker fallback"
  docker_force_clean "${TARGET_PATH}" || true
  rm_path "${TARGET_PATH}"
fi

if [[ -e "${TARGET_PATH}" ]]; then
  echo "failed to clean ${TARGET}. Check ownership/permissions." >&2
  exit 1
fi

echo "cleaned ${TARGET}"
