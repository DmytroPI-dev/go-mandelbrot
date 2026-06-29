# Terraform Infrastructure

This stack is the clean replacement for the old manually created AWS resources.
It does not import the old Lambda, API Gateway, S3 bucket, or CloudFront
distribution.

## Managed Resources

- Lambda baseline renderer
- API Gateway HTTP API
- private S3 frontend bucket
- CloudFront distribution with Origin Access Control
- IAM role and policy attachment
- CloudWatch log groups

The existing `*.i-dmytro.org` ACM certificate is discovered as a data source in
`us-east-1` because CloudFront viewer certificates must be in that region.

## First Run

Build the Lambda package first:

```sh
make backend-package
```

Then initialize and plan Terraform:

```sh
make tf-init AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-plan AWS_PROFILE=default AWS_REGION=eu-central-1 FRONTEND_DOMAIN=mandelbrot.i-dmytro.org
```

Copy `terraform.tfvars.example` to `terraform.tfvars` only for local overrides.
`terraform.tfvars` is ignored by Git.

## Frontend Deployment

After `terraform apply`, use the outputs to build and upload the frontend:

```sh
terraform -chdir=infra/terraform output -raw api_url
terraform -chdir=infra/terraform output -raw frontend_bucket_name
terraform -chdir=infra/terraform output -raw cloudfront_distribution_id
```

Set `VITE_API_URL` to the `api_url` output before building the frontend.

## Destroy

To take down the Terraform-managed demo stack:

```sh
make tf-destroy AWS_PROFILE=default AWS_REGION=eu-central-1 FRONTEND_DOMAIN=mandelbrot.i-dmytro.org
```

This does not delete the reusable ACM wildcard certificate.
