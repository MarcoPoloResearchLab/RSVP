#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

if [[ "$#" -ne 2 ]]; then
  fail "usage: calendar-key.sh <generate|validate> <key-file>"
fi

action="$1"
key_file="$2"

case "${action}" in
  generate)
    if [[ -e "${key_file}" || -L "${key_file}" ]]; then
      fail "calendar credential encryption key already exists"
    fi
    openssl rand -base64 32 >"${key_file}"
    ;;
  validate)
    ;;
  *)
    fail "usage: calendar-key.sh <generate|validate> <key-file>"
    ;;
esac

[[ -f "${key_file}" && ! -L "${key_file}" ]] || fail "calendar credential encryption key must be a regular file"

if ! decoded_bytes="$(openssl base64 -d -A -in "${key_file}" | wc -c | tr -d '[:space:]')"; then
  fail "calendar credential encryption key must be valid base64"
fi
[[ "${decoded_bytes}" == "32" ]] || fail "calendar credential encryption key must contain 32 base64-encoded bytes"
