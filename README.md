# Serverless Mandelbrot Renderer on AWS

Portfolio-quality Mandelbrot explorer built with a Go renderer, React/Vite
frontend, and Terraform-managed AWS infrastructure.

The current live architecture is a baseline serverless renderer:

- Go Lambda renders raw RGBA Mandelbrot pixels on request.
- API Gateway HTTP API exposes the render endpoint.
- React/Vite with Chakra UI 3 draws the returned pixel buffer into a canvas.
- S3 stores the static frontend privately.
- CloudFront serves the frontend with the `*.i-dmytro.org` ACM certificate.
- Terraform manages the clean AWS stack.
- Makefile targets wrap the build, deploy, inventory, and cleanup workflow.

The next portfolio phase is distributed rendering: split each image into tiles
or bands, fan the work out across Lambda workers, assemble the result, and add
operational metrics around duration, tile count, failures, and cost.

## Demo

Current demo domain:

```text
https://mandelbrot.i-dmytro.org
```

The custom domain is expected to be managed through Cloudflare DNS/proxying.
The AWS stack itself uses CloudFront, S3, API Gateway, Lambda, IAM, CloudWatch,
and an existing ACM wildcard certificate in `us-east-1`.

## Repository Layout

```text
backend/               Go Lambda renderer
frontend/fractal-app/  React/Vite and Chakra UI 3 Mandelbrot explorer
infra/terraform/       AWS infrastructure
scripts/               Build, inventory, deploy, and cleanup helpers
plan.md                Project restoration and roadmap notes
AGENTS.md              Local agent/project guidelines
```

Generated AWS inventory reports under `docs/aws-*.md`, Terraform state,
frontend `dist`, `node_modules`, Lambda packages, and real `.env` files are
ignored and should not be committed.

## Requirements

- Go 1.26.4
- Node.js and npm
- Chakra UI 3
- Terraform
- AWS CLI with an authenticated profile
- Existing ACM certificate for `*.i-dmytro.org` in `us-east-1`

Copy examples when local overrides are needed:

```sh
cp .env.example .env
cp frontend/fractal-app/.env.example frontend/fractal-app/.env
```

## Common Commands

Show available targets:

```sh
make help
```

Run backend tests:

```sh
make backend-test
```

Build and package the Lambda artifact:

```sh
make backend-package
```

Build the frontend:

```sh
make frontend-build
```

Check the active AWS identity:

```sh
make aws-whoami AWS_PROFILE=default AWS_REGION=eu-central-1
```

## CI/CD

GitHub Actions CI is configured in `.github/workflows/ci.yml` for backend tests,
frontend lint/build, and Terraform formatting/validation.

Frontend deployment is configured as a manual workflow in
`.github/workflows/deploy-frontend.yml`. It builds the Vite app, syncs assets to
the Terraform-managed S3 bucket, and invalidates CloudFront using GitHub OIDC.

Full Terraform/backend CD is intentionally deferred until Terraform state is
migrated from local state to an S3 remote backend with S3 lockfile support. See
`docs/cicd.md`.

## Deploy

Build the Lambda package before planning Terraform:

```sh
make backend-package
```

Initialize and validate Terraform:

```sh
make tf-state-init AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-plan AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-apply AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-write-backend-config
make tf-migrate-state
make tf-init AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-validate AWS_PROFILE=default AWS_REGION=eu-central-1
```

Plan and apply the AWS stack:

```sh
make tf-plan AWS_PROFILE=default AWS_REGION=eu-central-1 ACM_REGION=us-east-1 FRONTEND_DOMAIN=mandelbrot.i-dmytro.org
make tf-apply AWS_PROFILE=default AWS_REGION=eu-central-1 ACM_REGION=us-east-1 FRONTEND_DOMAIN=mandelbrot.i-dmytro.org
```

Configure the frontend API URL from Terraform output:

```sh
terraform -chdir=infra/terraform output -raw api_url
```

Set that value in `frontend/fractal-app/.env`:

```text
VITE_API_URL=https://example.execute-api.eu-central-1.amazonaws.com/render
```

Build and deploy frontend assets:

```sh
make frontend-build
make deploy-frontend \
  AWS_PROFILE=default \
  AWS_REGION=eu-central-1 \
  FRONTEND_BUCKET=<terraform frontend_bucket_name output> \
  DISTRIBUTION_ID=<terraform cloudfront_distribution_id output>
```

## Verify

Test the render API directly:

```sh
curl -o /tmp/fractal.bin \
  "$(terraform -chdir=infra/terraform output -raw api_url)?width=32&height_px=32&samples=1&maxIter=50"

ls -lh /tmp/fractal.bin
```

For `32x32` RGBA output, the file should be `4096` bytes.

Test CORS from the frontend domain:

```sh
curl -I \
  -H "Origin: https://mandelbrot.i-dmytro.org" \
  "$(terraform -chdir=infra/terraform output -raw api_url)?width=32&height_px=32&samples=1&maxIter=50"
```

## Observability

The Go Lambda emits structured JSON logs and low-cardinality CloudWatch metrics
through AWS Embedded Metric Format. See `docs/observability.md`.

## Cleanup

The old manual AWS deployment has been removed. The reusable ACM wildcard
certificate was kept.

To take down the current Terraform-managed demo stack:

```sh
make tf-destroy AWS_PROFILE=default AWS_REGION=eu-central-1 ACM_REGION=us-east-1 FRONTEND_DOMAIN=mandelbrot.i-dmytro.org
```

This is useful when the demo no longer needs to stay live and you want to avoid
ongoing AWS usage.

## Roadmap

- Add S3 remote Terraform state with S3 lockfile support so backend and
  infrastructure deployment can move safely into GitHub Actions.
- Implement distributed tile rendering with an orchestrator and worker Lambda.
- Add architecture diagram, screenshots, and portfolio write-up.

## Credits

The original Go Mandelbrot idea was adapted from
[parallel-mandelbrot-go](https://github.com/daniellferreira/parallel-mandelbrot-go)
by Daniela Ferreira, then refactored into a serverless AWS portfolio project.
