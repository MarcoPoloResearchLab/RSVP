#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
runtime_directory="${repository_root}/.cache/rsvp-local"
source_environment="${repository_root}/.env.docker"
calendar_key_file="${runtime_directory}/calendar-credential-encryption-key"

if [[ "$#" -ne 0 ]]; then
  printf 'error: make down accepts no arguments\n' >&2
  exit 2
fi
command -v docker >/dev/null 2>&1 || { printf 'error: docker is required\n' >&2; exit 1; }
docker compose version >/dev/null 2>&1 || { printf 'error: docker compose is required\n' >&2; exit 1; }

umask 077
mkdir -p "${runtime_directory}"
if [[ -f "${calendar_key_file}" ]]; then
  RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY="$(<"${calendar_key_file}")"
else
  RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY="rsvp-local-down-placeholder"
fi
export RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY
export RSVP_RUNTIME_ENV_FILE="${source_environment}"
export RSVP_PUBLIC_ORIGIN="http://localhost:8080"
export RSVP_HOST_PORT=8080
docker compose --project-name rsvp-local --env-file "${source_environment}" down --remove-orphans
rm -f "${runtime_directory}/app.env"
printf 'RSVP localhost services stopped. Local data remains available for the next start.\n'
