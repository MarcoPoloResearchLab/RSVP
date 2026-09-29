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

if [[ "$#" -ne 0 ]]; then
  fail "make up accepts no arguments"
fi
command -v docker >/dev/null 2>&1 || fail "docker is required"
docker compose version >/dev/null 2>&1 || fail "docker compose is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v openssl >/dev/null 2>&1 || fail "openssl is required"
[[ -f "${source_environment}" && ! -L "${source_environment}" ]] || fail "${source_environment} must be a regular file"

unset GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET
set -a
source "${source_environment}"
set +a
[[ -n "${GOOGLE_CLIENT_ID:-}" ]] || fail "${source_environment} must contain GOOGLE_CLIENT_ID"
[[ -n "${GOOGLE_CLIENT_SECRET:-}" ]] || fail "${source_environment} must contain GOOGLE_CLIENT_SECRET"

mkdir -p "${runtime_directory}"
if [[ ! -f "${calendar_key_file}" ]]; then
  "${calendar_key_tool}" generate "${calendar_key_file}"
fi
"${calendar_key_tool}" validate "${calendar_key_file}"

rm -f "${runtime_directory}/app.env"
for private_name in tauth-jwt-signing-key llm-proxy-secret; do
  private_path="${runtime_directory}/${private_name}"
  if [[ ! -e "${private_path}" ]]; then
    openssl rand -base64 32 >"${private_path}"
  fi
  [[ -f "${private_path}" && ! -L "${private_path}" ]] || fail "${private_name} must be a regular file"
done
export RSVP_TAUTH_JWT_SIGNING_KEY="$(<"${runtime_directory}/tauth-jwt-signing-key")"
export RSVP_LLM_PROXY_SECRET="$(<"${runtime_directory}/llm-proxy-secret")"
export RSVP_RUNTIME_ENV_FILE="${source_environment}"
export RSVP_PUBLIC_ORIGIN="${public_origin}"
export RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY="$(<"${calendar_key_file}")"
export RSVP_HOST_PORT=8080
docker compose --project-name "${compose_project}" --env-file "${source_environment}" up --build --detach --remove-orphans

ready=0
for _ in {1..60}; do
  if curl --fail --silent --show-error --max-time 2 "${public_origin}/healthz" >/dev/null 2>&1 &&
     curl --fail --silent --show-error --max-time 2 --request POST "${public_origin}/auth/nonce" --header "Origin: ${public_origin}" --header "X-TAuth-Tenant: rsvp-development" >/dev/null 2>&1; then
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
printf 'Google sign-in uses the TAuth popup exchange at %s/auth/google.\n' "${public_origin}"
printf 'Stop the stack with make down.\n'
