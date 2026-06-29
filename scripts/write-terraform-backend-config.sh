#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_DIR="${ROOT_DIR}/infra/terraform-state"
APP_TERRAFORM_DIR="${ROOT_DIR}/infra/terraform"
BACKEND_CONFIG="${APP_TERRAFORM_DIR}/backend.hcl"

AWS_REGION="${AWS_REGION:-eu-central-1}"

if [[ ! -d "${STATE_DIR}/.terraform" ]]; then
  echo "${STATE_DIR} is not initialized. Run make tf-state-init first."
  exit 1
fi

state_bucket="$(terraform -chdir="${STATE_DIR}" output -raw state_bucket_name)"
state_key="$(terraform -chdir="${STATE_DIR}" output -raw state_key)"

cat >"${BACKEND_CONFIG}" <<EOF
bucket         = "${state_bucket}"
key            = "${state_key}"
region         = "${AWS_REGION}"
use_lockfile   = true
encrypt        = true
EOF

if [[ ! -s "${BACKEND_CONFIG}" ]]; then
  echo "Terraform returned an empty backend config."
  echo "Run make tf-state-output and confirm backend_config_hcl is populated."
  exit 1
fi

if ! grep -q '^bucket[[:space:]]*=' "${BACKEND_CONFIG}"; then
  echo "${BACKEND_CONFIG} does not contain a bucket setting."
  echo "Run make tf-state-output and confirm backend_config_hcl is populated."
  exit 1
fi

echo "Wrote ${BACKEND_CONFIG}"
