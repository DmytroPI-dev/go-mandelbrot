output "api_url" {
  description = "HTTP API render endpoint for VITE_API_URL."
  value       = "${aws_apigatewayv2_api.renderer.api_endpoint}/render"
}

output "distributed_api_url" {
  description = "HTTP API distributed render endpoint."
  value       = "${aws_apigatewayv2_api.renderer.api_endpoint}/render-distributed"
}

output "cloudfront_domain_name" {
  description = "Generated CloudFront distribution domain name."
  value       = aws_cloudfront_distribution.frontend.domain_name
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID for invalidations."
  value       = aws_cloudfront_distribution.frontend.id
}

output "frontend_bucket_name" {
  description = "Private S3 bucket for frontend assets."
  value       = aws_s3_bucket.frontend.bucket
}

output "frontend_domain" {
  description = "Custom frontend domain configured on CloudFront."
  value       = var.frontend_domain
}

output "lambda_function_name" {
  description = "Baseline renderer Lambda function name."
  value       = aws_lambda_function.renderer.function_name
}

output "worker_lambda_function_name" {
  description = "Distributed tile worker Lambda function name."
  value       = aws_lambda_function.worker.function_name
}

output "orchestrator_lambda_function_name" {
  description = "Distributed local orchestrator Lambda function name."
  value       = aws_lambda_function.orchestrator.function_name
}

output "certificate_arn" {
  description = "ACM certificate ARN used by CloudFront."
  value       = data.aws_acm_certificate.frontend.arn
}
