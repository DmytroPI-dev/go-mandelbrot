data "aws_caller_identity" "current" {}

data "aws_acm_certificate" "frontend" {
  provider = aws.acm

  domain      = var.certificate_domain
  statuses    = ["ISSUED"]
  most_recent = true
}

