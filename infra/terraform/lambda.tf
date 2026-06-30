resource "aws_cloudwatch_log_group" "renderer_lambda" {
  name              = "/aws/lambda/${local.lambda_function_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_cloudwatch_log_group" "worker_lambda" {
  name              = "/aws/lambda/${local.worker_lambda_function_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_cloudwatch_log_group" "orchestrator_lambda" {
  name              = "/aws/lambda/${local.orchestrator_lambda_function_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_lambda_function" "renderer" {
  function_name = local.lambda_function_name
  description   = "Baseline Mandelbrot renderer for the portfolio demo."

  role    = aws_iam_role.renderer_lambda.arn
  runtime = "provided.al2023"
  handler = "bootstrap"

  filename         = var.lambda_package_path
  source_code_hash = try(filebase64sha256(var.lambda_package_path), null)

  memory_size = var.lambda_memory_size
  timeout     = var.lambda_timeout

  architectures = ["x86_64"]

  depends_on = [
    aws_cloudwatch_log_group.renderer_lambda,
    aws_iam_role_policy_attachment.renderer_lambda_basic,
  ]
}

resource "aws_lambda_function" "worker" {
  function_name = local.worker_lambda_function_name
  description   = "Distributed Mandelbrot tile worker for the portfolio demo."

  role    = aws_iam_role.renderer_lambda.arn
  runtime = "provided.al2023"
  handler = "bootstrap"

  filename         = var.lambda_package_path
  source_code_hash = try(filebase64sha256(var.lambda_package_path), null)

  memory_size = var.distributed_lambda_memory_size
  timeout     = var.distributed_lambda_timeout

  architectures = ["x86_64"]

  environment {
    variables = {
      MANDELBROT_HANDLER_MODE = "worker"
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.worker_lambda,
    aws_iam_role_policy_attachment.renderer_lambda_basic,
  ]
}

resource "aws_lambda_function" "orchestrator" {
  function_name = local.orchestrator_lambda_function_name
  description   = "Distributed Mandelbrot local orchestrator for the portfolio demo."

  role    = aws_iam_role.renderer_lambda.arn
  runtime = "provided.al2023"
  handler = "bootstrap"

  filename         = var.lambda_package_path
  source_code_hash = try(filebase64sha256(var.lambda_package_path), null)

  memory_size = var.distributed_lambda_memory_size
  timeout     = var.distributed_lambda_timeout

  architectures = ["x86_64"]

  environment {
    variables = {
      MANDELBROT_HANDLER_MODE = "orchestrator"
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.orchestrator_lambda,
    aws_iam_role_policy_attachment.renderer_lambda_basic,
  ]
}
