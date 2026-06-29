#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
BUILD_DIR="${ROOT_DIR}/build"
BOOTSTRAP="${BUILD_DIR}/bootstrap"
PACKAGE="${BUILD_DIR}/lambda.zip"
GO_BIN="${GO_BIN:-go}"
BUILD_ONLY=false

if [[ "${1:-}" == "--build-only" ]]; then
  BUILD_ONLY=true
fi

mkdir -p "${BUILD_DIR}"

echo "Building Go Lambda bootstrap..."
(
  cd "${BACKEND_DIR}"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "${GO_BIN}" build -tags lambda.norpc -trimpath -ldflags="-s -w" -o "${BOOTSTRAP}" .
)

if [[ "${BUILD_ONLY}" == "true" ]]; then
  echo "Built ${BOOTSTRAP}"
  exit 0
fi

echo "Packaging ${PACKAGE}..."
(
  cd "${BUILD_DIR}"
  rm -f lambda.zip
  zip -q lambda.zip bootstrap
)

echo "Created ${PACKAGE}"
