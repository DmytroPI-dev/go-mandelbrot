# CI/CD

## Current State

The repository has two GitHub Actions workflows:

- `.github/workflows/ci.yml`
- `.github/workflows/deploy-frontend.yml`

CI validates the repository without AWS credentials. Frontend CD is manual and
uses GitHub OIDC to deploy already-built static assets to the Terraform-managed
S3 bucket and invalidate CloudFront.

Full infrastructure/backend CD is intentionally deferred until Terraform state
is moved from local state into a remote backend.

## CI Workflow

The CI workflow runs on pushes to `main` and on pull requests.

Jobs:

- Backend: `go test ./...`
- Frontend: `npm ci`, `npm run lint`, `npm run build`
- Terraform: `terraform fmt -check -recursive`, `terraform init -backend=false`,
  `terraform validate`

The Terraform job does not plan or apply. It only checks formatting and
configuration validity without contacting the live AWS account.

## Frontend Deployment Workflow

The frontend deployment workflow is manual through `workflow_dispatch`.

Required repository or environment variables:

- `AWS_REGION`: AWS region for the frontend bucket, default expectation is
  `eu-central-1`
- `FRONTEND_BUCKET`: Terraform output `frontend_bucket_name`
- `CLOUDFRONT_DISTRIBUTION_ID`: Terraform output `cloudfront_distribution_id`
- `VITE_API_URL`: Terraform output `api_url`

Required secret:

- `AWS_DEPLOY_ROLE_ARN`: IAM role ARN assumed by GitHub Actions through OIDC

The workflow:

1. installs frontend dependencies;
2. builds the Vite app with `VITE_API_URL`;
3. syncs `frontend/fractal-app/dist` to S3;
4. creates a CloudFront invalidation.

## AWS OIDC Role

Create an IAM role trusted by the GitHub repository OIDC provider and restrict
it to this repository and the production environment.

Minimum permissions for frontend deployment:

- `s3:ListBucket` on the frontend bucket;
- `s3:GetObject`, `s3:PutObject`, and `s3:DeleteObject` on bucket objects;
- `cloudfront:CreateInvalidation` on the CloudFront distribution.

This role does not need permissions for Lambda, API Gateway, ACM, or Terraform
state while the workflow only deploys frontend assets.

## Why Full Terraform CD Is Deferred

The current Terraform stack uses local state under `infra/terraform`. A GitHub
Actions runner would not have that state file. Running `terraform apply` from
GitHub without remote state could make Terraform believe the live resources do
not exist and attempt to create duplicates.

Before adding infrastructure/backend CD:

1. create a remote Terraform backend, likely S3 with DynamoDB locking;
2. migrate the existing local state into that backend;
3. add a GitHub OIDC deployment role with the required Terraform permissions;
4. add a plan/apply workflow with environment approval;
5. only then allow Terraform apply from GitHub Actions.

## Next CD Step

Add Terraform-managed GitHub OIDC IAM resources or document the manual IAM setup
in more detail, then migrate Terraform state to a remote backend. After that,
backend packaging and Terraform apply can be safely automated.
