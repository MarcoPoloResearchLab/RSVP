#!/bin/bash

set -e

# Configuration
DOCKER_IMAGE="ghcr.io/temirov/rsvp:latest"
CONTAINER_NAME="rsvp-app"
ENV_FILE=".env.docker"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT_MAPPING="8081:8080"
CERTS_FOLDER="${PROJECT_DIR}/certs"
ENV_FILE="${PROJECT_DIR}/${ENV_FILE}"

echo "Pulling the latest image..."
docker pull "${DOCKER_IMAGE}"

echo "Stopping existing container (if running)..."
if docker ps -q --filter "name=^/${CONTAINER_NAME}$" | grep -q .; then
    docker stop "${CONTAINER_NAME}"
fi

echo "Removing existing container (if exists)..."
if docker ps -aq --filter "name=^/${CONTAINER_NAME}$" | grep -q .; then
    docker rm "${CONTAINER_NAME}"
fi

echo "Starting new container..."
docker run -d \
  --env-file "${ENV_FILE}" \
  -p "${PORT_MAPPING}" \
  -v "${CERTS_FOLDER}:/app/certs" \
  --name "${CONTAINER_NAME}" \
  "${DOCKER_IMAGE}"

echo "Deployment complete."
