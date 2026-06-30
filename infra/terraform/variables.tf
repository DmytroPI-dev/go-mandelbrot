variable "aws_profile" {
  description = "Local AWS CLI profile used by Terraform."
  type        = string
  default     = "default"
}

variable "aws_region" {
  description = "Primary AWS region for Lambda, API Gateway, S3, and logs."
  type        = string
  default     = "eu-central-1"
}

variable "acm_region" {
  description = "Region for the CloudFront viewer certificate. Must be us-east-1 for CloudFront."
  type        = string
  default     = "us-east-1"
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

variable "frontend_domain" {
  description = "Custom frontend domain served through CloudFront."
  type        = string
  default     = "fractal.i-dmytro.org"
}

variable "certificate_domain" {
  description = "ACM certificate domain to look up for CloudFront."
  type        = string
  default     = "*.i-dmytro.org"
}

variable "lambda_package_path" {
  description = "Path to the Lambda zip package produced by make backend-package."
  type        = string
  default     = "../../build/lambda.zip"
}

variable "lambda_memory_size" {
  description = "Memory size for the baseline renderer Lambda."
  type        = number
  default     = 2048
}

variable "lambda_timeout" {
  description = "Timeout in seconds for the baseline renderer Lambda."
  type        = number
  default     = 60
}

variable "distributed_lambda_memory_size" {
  description = "Memory size for distributed renderer orchestrator and worker Lambdas."
  type        = number
  default     = 2048
}

variable "distributed_lambda_timeout" {
  description = "Timeout in seconds for distributed renderer orchestrator and worker Lambdas."
  type        = number
  default     = 60
}

variable "log_retention_days" {
  description = "CloudWatch log retention for Lambda and API Gateway logs."
  type        = number
  default     = 14
}

variable "allowed_cors_origins" {
  description = "Extra allowed browser origins for API Gateway CORS. The configured frontend_domain is always included automatically."
  type        = list(string)
  default = [
    "http://localhost:5173",
  ]
}

variable "enable_cloudfront_ipv6" {
  description = "Whether to enable IPv6 for the CloudFront distribution."
  type        = bool
  default     = true
}
