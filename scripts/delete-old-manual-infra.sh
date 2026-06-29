#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-default}"
AWS_REGION="${AWS_REGION:-eu-central-1}"

LAMBDA_FUNCTION="${LAMBDA_FUNCTION:-MandelbrotFRA}"
API_ID="${API_ID:-ro3n8xxh1e}"
BUCKET_NAME="${BUCKET_NAME:-mandelbro-bucket}"
DISTRIBUTION_ID="${DISTRIBUTION_ID:-E3VOOYYQE4GTM7}"
LOG_GROUP="${LOG_GROUP:-/aws/lambda/${LAMBDA_FUNCTION}}"
CONFIRM_DELETE="${CONFIRM_DELETE_OLD_MANUAL_INFRA:-false}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

aws_cmd() {
  aws --profile "${AWS_PROFILE}" --region "${AWS_REGION}" "$@"
}

aws_global_cmd() {
  aws --profile "${AWS_PROFILE}" "$@"
}

run() {
  if [[ "${CONFIRM_DELETE}" == "true" ]]; then
    echo "+ $*"
    "$@"
  else
    echo "[dry-run] $*"
  fi
}

retry() {
  local attempts="$1"
  local delay="$2"
  shift 2

  local attempt=1
  until "$@"; do
    if (( attempt >= attempts )); then
      return 1
    fi
    echo "Attempt ${attempt} failed; retrying in ${delay}s..."
    sleep "${delay}"
    attempt=$((attempt + 1))
  done
}

exists() {
  "$@" >/dev/null 2>&1
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required."
    exit 1
  fi
}

require_tool aws
require_tool jq

cat <<EOF
Old manual infra cleanup

AWS profile:              ${AWS_PROFILE}
AWS region:               ${AWS_REGION}
Lambda function:          ${LAMBDA_FUNCTION}
API Gateway HTTP API:     ${API_ID}
S3 bucket:                ${BUCKET_NAME}
CloudFront distribution:  ${DISTRIBUTION_ID}
CloudWatch log group:     ${LOG_GROUP}

This script intentionally does not delete ACM certificates.
EOF

if [[ "${CONFIRM_DELETE}" != "true" ]]; then
  cat <<'EOF'

Dry-run only. To delete these resources, run with:

  CONFIRM_DELETE_OLD_MANUAL_INFRA=true ./scripts/delete-old-manual-infra.sh
EOF
fi

echo
echo "Checking current AWS identity..."
aws_cmd sts get-caller-identity --query '{Account:Account,Arn:Arn}' --output table

echo
echo "Cleaning old CloudFront distribution..."
if exists aws_global_cmd cloudfront get-distribution --id "${DISTRIBUTION_ID}"; then
  DIST_JSON="${TMP_DIR}/distribution.json"
  DISABLED_CONFIG="${TMP_DIR}/distribution-disabled.json"
  aws_global_cmd cloudfront get-distribution-config --id "${DISTRIBUTION_ID}" --output json > "${DIST_JSON}"

  OAC_ID="$(jq -r '.DistributionConfig.Origins.Items[]? | select(.OriginAccessControlId != null and .OriginAccessControlId != "") | .OriginAccessControlId' "${DIST_JSON}" | head -n 1)"
  ENABLED="$(jq -r '.DistributionConfig.Enabled' "${DIST_JSON}")"

  if [[ "${ENABLED}" == "true" ]]; then
    jq '.DistributionConfig.Enabled = false | .DistributionConfig' "${DIST_JSON}" > "${DISABLED_CONFIG}"
    ETAG="$(jq -r '.ETag' "${DIST_JSON}")"
    run aws_global_cmd cloudfront update-distribution \
      --id "${DISTRIBUTION_ID}" \
      --if-match "${ETAG}" \
      --distribution-config "file://${DISABLED_CONFIG}" \
      --output table

    if [[ "${CONFIRM_DELETE}" == "true" ]]; then
      echo "Waiting for CloudFront distribution ${DISTRIBUTION_ID} to finish disabling..."
      aws_global_cmd cloudfront wait distribution-deployed --id "${DISTRIBUTION_ID}"
    fi
  else
    echo "CloudFront distribution ${DISTRIBUTION_ID} is already disabled."
  fi

  if [[ "${CONFIRM_DELETE}" == "true" ]]; then
    aws_global_cmd cloudfront get-distribution-config --id "${DISTRIBUTION_ID}" --output json > "${DIST_JSON}"
  fi

  DELETE_ETAG="$(jq -r '.ETag' "${DIST_JSON}")"
  run aws_global_cmd cloudfront delete-distribution \
    --id "${DISTRIBUTION_ID}" \
    --if-match "${DELETE_ETAG}"

  if [[ -n "${OAC_ID}" && "${OAC_ID}" != "null" ]]; then
    echo "Cleaning old CloudFront Origin Access Control ${OAC_ID}..."
    if exists aws_global_cmd cloudfront get-origin-access-control-config --id "${OAC_ID}"; then
      OAC_ETAG="$(aws_global_cmd cloudfront get-origin-access-control-config --id "${OAC_ID}" --query 'ETag' --output text)"
      if [[ "${CONFIRM_DELETE}" == "true" ]]; then
        retry 6 20 aws_global_cmd cloudfront delete-origin-access-control \
          --id "${OAC_ID}" \
          --if-match "${OAC_ETAG}"
      else
        run aws_global_cmd cloudfront delete-origin-access-control \
          --id "${OAC_ID}" \
          --if-match "${OAC_ETAG}"
      fi
    else
      echo "CloudFront OAC ${OAC_ID} not found."
    fi
  fi
else
  echo "CloudFront distribution ${DISTRIBUTION_ID} not found."
fi

echo
echo "Cleaning old API Gateway HTTP API..."
if exists aws_cmd apigatewayv2 get-api --api-id "${API_ID}"; then
  run aws_cmd apigatewayv2 delete-api --api-id "${API_ID}"
else
  echo "API Gateway HTTP API ${API_ID} not found."
fi

echo
echo "Cleaning old Lambda function and IAM role..."
ROLE_ARN=""
ROLE_NAME=""
if exists aws_cmd lambda get-function-configuration --function-name "${LAMBDA_FUNCTION}"; then
  ROLE_ARN="$(aws_cmd lambda get-function-configuration --function-name "${LAMBDA_FUNCTION}" --query 'Role' --output text)"
  ROLE_NAME="${ROLE_ARN##*/}"
  run aws_cmd lambda delete-function --function-name "${LAMBDA_FUNCTION}"
else
  echo "Lambda function ${LAMBDA_FUNCTION} not found."
fi

if [[ -n "${ROLE_NAME}" && "${ROLE_NAME}" != "None" ]]; then
  echo "Cleaning Lambda IAM role ${ROLE_NAME}..."
  while read -r POLICY_ARN; do
    [[ -z "${POLICY_ARN}" || "${POLICY_ARN}" == "None" ]] && continue
    run aws_global_cmd iam detach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${POLICY_ARN}"

    if [[ "${POLICY_ARN}" == arn:aws:iam::*:policy/service-role/AWSLambdaBasicExecutionRole-* ]]; then
      ENTITIES="$(aws_global_cmd iam list-entities-for-policy --policy-arn "${POLICY_ARN}" --query 'PolicyRoles | length(@)' --output text 2>/dev/null || echo "1")"
      if [[ "${ENTITIES}" == "0" ]]; then
        run aws_global_cmd iam delete-policy --policy-arn "${POLICY_ARN}"
      else
        echo "Policy ${POLICY_ARN} is still attached elsewhere; not deleting it."
      fi
    fi
  done < <(aws_global_cmd iam list-attached-role-policies --role-name "${ROLE_NAME}" --query 'AttachedPolicies[].PolicyArn' --output text 2>/dev/null | tr '\t' '\n')

  while read -r POLICY_NAME; do
    [[ -z "${POLICY_NAME}" || "${POLICY_NAME}" == "None" ]] && continue
    run aws_global_cmd iam delete-role-policy --role-name "${ROLE_NAME}" --policy-name "${POLICY_NAME}"
  done < <(aws_global_cmd iam list-role-policies --role-name "${ROLE_NAME}" --query 'PolicyNames[]' --output text 2>/dev/null | tr '\t' '\n')

  run aws_global_cmd iam delete-role --role-name "${ROLE_NAME}"
fi

echo
echo "Cleaning old CloudWatch log group..."
if exists aws_cmd logs describe-log-groups --log-group-name-prefix "${LOG_GROUP}"; then
  run aws_cmd logs delete-log-group --log-group-name "${LOG_GROUP}"
else
  echo "CloudWatch log group ${LOG_GROUP} not found."
fi

echo
echo "Cleaning old S3 bucket..."
if exists aws_global_cmd s3api head-bucket --bucket "${BUCKET_NAME}"; then
  run aws s3 rm "s3://${BUCKET_NAME}" --recursive --profile "${AWS_PROFILE}" --region "${AWS_REGION}"
  run aws_global_cmd s3api delete-bucket-policy --bucket "${BUCKET_NAME}"
  run aws_global_cmd s3api delete-bucket --bucket "${BUCKET_NAME}"
else
  echo "S3 bucket ${BUCKET_NAME} not found."
fi

echo
echo "Old manual infra cleanup complete."
