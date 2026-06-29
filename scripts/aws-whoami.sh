#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-default}"
AWS_REGION="${AWS_REGION:-eu-central-1}"

echo "AWS profile: ${AWS_PROFILE}"
echo "AWS region:  ${AWS_REGION}"
aws sts get-caller-identity --profile "${AWS_PROFILE}" --region "${AWS_REGION}" --output table

