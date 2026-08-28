#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source_environment="${repository_root}/.env.docker"
runtime_directory="${repository_root}/.cache/rsvp-local"
calendar_key_file="${runtime_directory}/calendar-credential-encryption-key"
calendar_key_tool="${repository_root}/scripts/calendar-key.sh"
public_origin="http://localhost:8080"
compose_project="rsvp-local"

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

read_environment_value() {
  awk -F= -v key="$2" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$1" | tr -d '\r'
}

require_environment_value() {
  local value
  value="$(read_environment_value "${source_environment}" "$1")"
  if [[ -z "${value}" ]]; then
    fail "${source_environment} must contain $1"
  fi
}

if [[ "$#" -ne 0 ]]; then
  fail "make up accepts no arguments"
fi
command -v docker >/dev/null 2>&1 || fail "docker is required"
docker compose version >/dev/null 2>&1 || fail "docker compose is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v openssl >/dev/null 2>&1 || fail "openssl is required"
[[ -f "${source_environment}" && ! -L "${source_environment}" ]] || fail "${source_environment} must be a regular file"
[[ "$(file_mode "${source_environment}")" == "600" ]] || fail "${source_environment} must have mode 0600"

awk -F= -v path="${source_environment}" '
  /^[[:space:]]*($|#)/ { next }
  $0 !~ /^[A-Za-z_][A-Za-z0-9_]*=/ {
    printf "error: invalid environment entry at %s:%d\n", path, NR > "/dev/stderr"
    exit 1
  }
  seen[$1]++ {
    printf "error: duplicate environment key %s at %s:%d\n", $1, path, NR > "/dev/stderr"
    exit 1
  }
' "${source_environment}"

require_environment_value GOOGLE_CLIENT_ID
require_environment_value GOOGLE_CLIENT_SECRET
require_environment_value SESSION_SECRET

umask 077
mkdir -p "${runtime_directory}"
chmod 700 "${runtime_directory}"
if [[ ! -f "${calendar_key_file}" ]]; then
  "${calendar_key_tool}" generate "${calendar_key_file}"
fi
"${calendar_key_tool}" validate "${calendar_key_file}"

rm -f "${runtime_directory}/app.env"
export RSVP_RUNTIME_ENV_FILE="${source_environment}"
export RSVP_PUBLIC_ORIGIN="${public_origin}"
export RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY="$(<"${calendar_key_file}")"
export RSVP_HOST_PORT=8080
docker compose --project-name "${compose_project}" --env-file "${source_environment}" up --build --detach --remove-orphans

ready=0
for _ in {1..60}; do
  if curl --fail --silent --show-error --max-time 2 "${public_origin}/" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "${ready}" != 1 ]]; then
  docker compose --project-name "${compose_project}" --env-file "${source_environment}" logs --tail=100 >&2 || true
  docker compose --project-name "${compose_project}" --env-file "${source_environment}" down --remove-orphans >/dev/null 2>&1 || true
  fail "RSVP did not become ready at ${public_origin}"
fi

printf 'RSVP is ready at %s/\n' "${public_origin}"
printf 'Google sign-in callback: %s/auth/google/callback\n' "${public_origin}"
printf 'Stop the stack with make down.\n'
