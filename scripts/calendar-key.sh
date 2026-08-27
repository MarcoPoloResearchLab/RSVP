#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

file_mode() {
  if stat -f '%Lp' "$1" >/dev/null 2>&1; then
    stat -f '%Lp' "$1"
  else
    stat -c '%a' "$1"
  fi
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
    umask 077
    openssl rand -base64 32 >"${key_file}"
    chmod 600 "${key_file}"
    ;;
  validate)
    ;;
  *)
    fail "usage: calendar-key.sh <generate|validate> <key-file>"
    ;;
esac

[[ -f "${key_file}" && ! -L "${key_file}" ]] || fail "calendar credential encryption key must be a regular file"
[[ "$(file_mode "${key_file}")" == "600" ]] || fail "calendar credential encryption key must have mode 0600"

if ! decoded_bytes="$(openssl base64 -d -A -in "${key_file}" | wc -c | tr -d '[:space:]')"; then
  fail "calendar credential encryption key must be valid base64"
fi
[[ "${decoded_bytes}" == "32" ]] || fail "calendar credential encryption key must contain 32 base64-encoded bytes"
