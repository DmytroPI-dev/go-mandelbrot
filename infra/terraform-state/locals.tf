data "aws_caller_identity" "current" {}

locals {
  name_prefix       = "${var.project}-${var.environment}"
  state_bucket_name = var.state_bucket_name != "" ? var.state_bucket_name : "${local.name_prefix}-terraform-state-${data.aws_caller_identity.current.account_id}"

  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Component   = "terraform-state"
  }
}
