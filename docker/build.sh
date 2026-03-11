#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

IMAGE_NAME="${IMAGE_NAME:-chirpstack-simulator}"
IMAGE_TAG="${IMAGE_TAG:-$(
  git -C "${ROOT_DIR}" describe --always --dirty 2>/dev/null | sed -e "s/^v//" || echo dev
)}"

docker build \
  --file "${ROOT_DIR}/docker/Dockerfile" \
  --build-arg VERSION="${IMAGE_TAG}" \
  --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
  "${ROOT_DIR}"
