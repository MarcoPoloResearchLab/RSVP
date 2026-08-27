#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source_environment="${repository_root}/.env.docker"
calendar_key_file="${repository_root}/.cache/rsvp-local/calendar-credential-encryption-key"

if [[ ! -f "${calendar_key_file}" ]]; then
  printf 'error: run make up before make %s\n' "${RSVP_LOCAL_COMMAND:?RSVP_LOCAL_COMMAND is required}" >&2
  exit 1
fi
export RSVP_RUNTIME_ENV_FILE="${source_environment}"
export RSVP_PUBLIC_ORIGIN="http://localhost:8080"
export RSVP_CALENDAR_CREDENTIAL_ENCRYPTION_KEY="$(<"${calendar_key_file}")"
export RSVP_HOST_PORT=8080
docker compose --project-name rsvp-local --env-file "${source_environment}" "${RSVP_LOCAL_COMMAND}" "$@"
