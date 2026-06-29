variable "aws_profile" {
  description = "Local AWS CLI profile used by Terraform."
  type        = string
  default     = "default"
}

variable "aws_region" {
  description = "AWS region for the Terraform state bucket."
  type        = string
  default     = "eu-central-1"
}

variable "project" {
  description = "Project name used for resource naming and tags."
  type        = string
  default     = "mandelbrot"
}

variable "environment" {
  description = "Deployment environment name."
  type        = string
  default     = "prod"
}

variable "state_bucket_name" {
  description = "Optional explicit S3 bucket name for Terraform state."
  type        = string
  default     = ""
}
