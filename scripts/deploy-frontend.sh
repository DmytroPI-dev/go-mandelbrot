#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${ROOT_DIR}/frontend/fractal-app"
DIST_DIR="${FRONTEND_DIR}/dist"

AWS_PROFILE="${AWS_PROFILE:-default}"
AWS_REGION="${AWS_REGION:-eu-central-1}"
FRONTEND_BUCKET="${FRONTEND_BUCKET:-}"
DISTRIBUTION_ID="${DISTRIBUTION_ID:-}"

if [[ -z "${FRONTEND_BUCKET}" ]]; then
  echo "FRONTEND_BUCKET is required."
  exit 1
fi

if [[ ! -d "${DIST_DIR}" ]]; then
  echo "${DIST_DIR} does not exist. Run make frontend-build first."
  exit 1
fi

echo "Syncing frontend assets to s3://${FRONTEND_BUCKET}..."
aws s3 sync "${DIST_DIR}/" "s3://${FRONTEND_BUCKET}/" \
  --profile "${AWS_PROFILE}" \
  --region "${AWS_REGION}" \
  --delete

if [[ -n "${DISTRIBUTION_ID}" ]]; then
  echo "Creating CloudFront invalidation for ${DISTRIBUTION_ID}..."
  aws cloudfront create-invalidation \
    --profile "${AWS_PROFILE}" \
    --distribution-id "${DISTRIBUTION_ID}" \
    --paths "/*"
else
  echo "DISTRIBUTION_ID is not set; skipped CloudFront invalidation."
fi

