output "state_bucket_name" {
  description = "S3 bucket name for Terraform state."
  value       = aws_s3_bucket.terraform_state.bucket
}

output "state_key" {
  description = "Recommended object key for the application Terraform state."
  value       = "${var.project}/${var.environment}/terraform.tfstate"
}

output "backend_config_hcl" {
  description = "Backend config to write into infra/terraform/backend.hcl."
  value       = <<EOT
bucket         = "${aws_s3_bucket.terraform_state.bucket}"
key            = "${var.project}/${var.environment}/terraform.tfstate"
region         = "${var.aws_region}"
use_lockfile   = true
encrypt        = true
EOT
}
