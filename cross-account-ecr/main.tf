resource "aws_ecr_repository" "app" {
  name                 = "${var.app_name}"
  # name                 = "${var.app_name}-${var.environment}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

data "aws_iam_policy_document" "cross_account_access" {
  statement {
    actions = [
      "ecr:GetAuthorizationToken",
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage"
    ]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${var.account_id}:root"]
    }
    effect = "Allow"
  }
}

resource "aws_ecr_repository_policy" "cross_account_access" {
  repository = aws_ecr_repository.app.name
  policy     = data.aws_iam_policy_document.cross_account_access.json
}