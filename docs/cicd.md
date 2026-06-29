# CI/CD

## Current State

The repository has two GitHub Actions workflows:

- `.github/workflows/ci.yml`
- `.github/workflows/deploy-frontend.yml`

CI validates the repository without AWS credentials. Frontend CD is manual and
uses GitHub OIDC to deploy already-built static assets to the Terraform-managed
S3 bucket and invalidate CloudFront.

The main Terraform stack uses an S3 remote backend with native S3 lockfile
support. Full infrastructure/backend CD is still intentionally deferred until
the GitHub OIDC role, IAM permissions, and environment approval workflow are
designed.

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

## Why Full Terraform CD Is Still Deferred

Remote state is now in place, so GitHub Actions can safely read the same
Terraform state as local development. The remaining risk is permission and
release-control design: a GitHub runner that can apply Terraform can change or
destroy real AWS resources.

Before adding infrastructure/backend CD, still do the following:

1. add a GitHub OIDC deployment role with the required Terraform permissions;
2. decide whether the workflow should plan only, apply only after approval, or
   stay manual for this portfolio project;
3. add a plan/apply workflow with environment approval if automation is worth
   the operational risk;
4. only then allow Terraform apply from GitHub Actions.

The bootstrap stack lives in `infra/terraform-state`. It creates the S3 bucket
used for remote state. The main stack in `infra/terraform` then uses that
bucket through a generated, ignored `backend.hcl` file with `use_lockfile =
true`.

## Current State Backend Commands

The state backend has already been bootstrapped and the main stack has been
migrated. These commands are kept here for future rebuilds or new machines:

Commands:

```sh
make tf-state-init AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-plan AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-apply AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-write-backend-config
make tf-migrate-state
```

If the backend is already migrated and only the backend configuration changed:

```sh
make tf-write-backend-config
make tf-reconfigure-backend
make tf-state-plan AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-apply AWS_PROFILE=default AWS_REGION=eu-central-1
```

## Next CD Step

Keep backend and Terraform deployment manual until the distributed renderer
settles. The next useful automation step is a protected GitHub Actions workflow
that packages the backend, runs `terraform plan`, stores the plan as an
artifact, and requires a manual approval before `terraform apply`.
