# Terraform State Bootstrap

This stack creates the remote state backend used by the main application
Terraform stack:

- S3 bucket for `terraform.tfstate`
- S3 versioning and server-side encryption
- public access blocking and bucket-owner-enforced ownership
- S3 native lockfile support for Terraform state locking

This stack intentionally keeps its own local state. It is the small bootstrap
layer that lets the main stack move away from local state.

## Create Remote State Resources

```sh
make tf-state-init AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-plan AWS_PROFILE=default AWS_REGION=eu-central-1
make tf-state-apply AWS_PROFILE=default AWS_REGION=eu-central-1
```

## Generate Main Backend Config

After the bootstrap stack is applied:

```sh
make tf-write-backend-config
```

This writes `infra/terraform/backend.hcl`, which is ignored by Git.

## Migrate Main Terraform State

After `backend.hcl` exists:

```sh
make tf-migrate-state
```

Terraform will copy the existing local state for `infra/terraform` into the S3
backend and use S3 lockfiles for locking.

## Reconfigure After Backend Changes

If `backend.hcl` changes after migration:

```sh
make tf-write-backend-config
make tf-reconfigure-backend
```
