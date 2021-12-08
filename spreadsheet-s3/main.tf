# S3
resource "aws_s3_bucket" "spreadsheet" {
  bucket = var.bucket_name
  acl    = "private"

  versioning {
    enabled = true
  }

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

# IAM 

resource "aws_iam_role" "s3_access" {
  name                = "${var.bucket_name}-cross-account-access"
  description         = "Role granting cross-account access to test user spreadsheet in S3"
  assume_role_policy  = data.aws_iam_policy_document.ecs_task_assume_role.json
  managed_policy_arns = [aws_iam_policy.s3_access.arn]
}

data "aws_iam_policy_document" "ecs_task_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["${var.ecs_task_role_arn}"]
    }
    effect = "Allow"
  }
}

data "aws_iam_policy_document" "s3_access" {
  statement {
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["arn:aws:s3:::${var.bucket_name}/${var.spreadsheet_name}", ]
    effect    = "Allow"
  }
}

resource "aws_iam_policy" "s3_access" {
  name        = "${var.bucket_name}-s3-access"
  description = "Policy granting access to test user spreadsheet in S3"
  policy      = data.aws_iam_policy_document.s3_access.json
}


