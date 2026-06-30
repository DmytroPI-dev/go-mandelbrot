resource "aws_cloudwatch_log_group" "api_gateway" {
  name              = "/aws/apigateway/${local.api_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_apigatewayv2_api" "renderer" {
  name          = local.api_name
  protocol_type = "HTTP"

  cors_configuration {
    allow_credentials = false
    allow_headers     = ["content-type"]
    allow_methods     = ["GET", "OPTIONS"]
    allow_origins     = local.api_cors_origins
    max_age           = 3600
  }
}

resource "aws_apigatewayv2_integration" "renderer" {
  api_id = aws_apigatewayv2_api.renderer.id

  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.renderer.invoke_arn
  integration_method     = "POST"
  payload_format_version = "1.0"
}

resource "aws_apigatewayv2_integration" "orchestrator" {
  api_id = aws_apigatewayv2_api.renderer.id

  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.orchestrator.invoke_arn
  integration_method     = "POST"
  payload_format_version = "1.0"
}

resource "aws_apigatewayv2_route" "render_get" {
  api_id = aws_apigatewayv2_api.renderer.id

  route_key = "GET /render"
  target    = "integrations/${aws_apigatewayv2_integration.renderer.id}"
}

resource "aws_apigatewayv2_route" "render_distributed_get" {
  api_id = aws_apigatewayv2_api.renderer.id

  route_key = "GET /render-distributed"
  target    = "integrations/${aws_apigatewayv2_integration.orchestrator.id}"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id = aws_apigatewayv2_api.renderer.id

  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gateway.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
      integrationErr = "$context.integrationErrorMessage"
    })
  }
}

resource "aws_lambda_permission" "allow_api_gateway" {
  statement_id  = "AllowExecutionFromApiGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.renderer.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.renderer.execution_arn}/*/*"
}

resource "aws_lambda_permission" "allow_api_gateway_orchestrator" {
  statement_id  = "AllowExecutionFromApiGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.orchestrator.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.renderer.execution_arn}/*/*"
}
