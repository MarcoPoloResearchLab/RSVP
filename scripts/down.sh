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

RSVP_LOCAL_COMMAND=down "${repository_root}/scripts/local-compose.sh" --remove-orphans
printf 'RSVP localhost services stopped. Local data remains available for the next start.\n'
