locals {
  name_prefix = "${var.project}-${var.environment}"

  frontend_bucket_name = "${local.name_prefix}-frontend-${data.aws_caller_identity.current.account_id}"

  lambda_function_name              = "${local.name_prefix}-renderer"
  worker_lambda_function_name       = "${local.name_prefix}-worker"
  orchestrator_lambda_function_name = "${local.name_prefix}-orchestrator"
  api_name                          = "${local.name_prefix}-api"

  api_cors_origins = distinct(concat(
    var.allowed_cors_origins,
    ["https://${var.frontend_domain}"]
  ))

  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Repository  = "go-mandelbrot"
  }
}
