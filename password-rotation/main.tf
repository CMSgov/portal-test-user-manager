locals {
  awslogs_group = "/aws/ecs/${var.app_name}-${var.environment}-${var.task_name}"
}

data "aws_partition" "current" {}

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

## IAM ## 

# CloudWatch Event role

data "aws_iam_policy_document" "events_assume_role_policy" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    effect = "Allow"
  }
}

resource "aws_iam_role" "cloudwatch_target_role" {
  name                = "cw-target-role-${var.app_name}-${var.environment}-${var.task_name}"
  description         = "Role allowing CloudWatch Events to run the task"
  assume_role_policy  = data.aws_iam_policy_document.events_assume_role_policy.json
  managed_policy_arns = ["arn:aws:iam::aws:policy/service-role/AmazonEC2ContainerServiceEventsRole"]
}

## ECS roles

# Trust relationship for task roles

data "aws_iam_policy_document" "ecs_assume_role_policy" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }

    effect = "Allow"
  }
}

# ECS task role

resource "aws_iam_role" "task_role" {
  name                = "ecs-task-role-${var.app_name}-${var.environment}-${var.task_name}"
  description         = "Role granting permissions to the ECS container task"
  assume_role_policy  = data.aws_iam_policy_document.ecs_assume_role_policy.json
  managed_policy_arns = [aws_iam_policy.s3_access.arn]
}

data "aws_iam_policy_document" "s3_access" {
  statement {
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["arn:aws:s3:::${var.s3_bucket}/${var.s3_key}", ]
    effect    = "Allow"
  }
}

resource "aws_iam_policy" "s3_access" {
  name        = "${var.s3_bucket}-s3-access"
  description = "Policy granting access to the S3 bucket containing the test user spreadsheet"
  policy      = data.aws_iam_policy_document.s3_access.json
}

# ECS task execution role

resource "aws_iam_role" "task_execution_role" {
  name                = "ecs-task-exec-role-${var.app_name}-${var.environment}-${var.task_name}"
  description         = "Role granting permissions to the ECS container agent/Docker daemon"
  assume_role_policy  = data.aws_iam_policy_document.ecs_assume_role_policy.json
  managed_policy_arns = [aws_iam_policy.parameter_store.arn, "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"]
}

data "aws_iam_policy_document" "parameter_store" {
  statement {
    actions   = ["ssm:GetParameters"]
    resources = ["${aws_ssm_parameter.automated_sheet_password.arn}", ]
    effect    = "Allow"
  }
}

resource "aws_iam_policy" "parameter_store" {
  name        = "${var.app_name}-${var.environment}-${var.task_name}-parameter-store"
  description = "Policy granting access to parameter store"
  policy      = data.aws_iam_policy_document.parameter_store.json
}

## CloudWatch ##

resource "aws_cloudwatch_event_rule" "run_command" {
  name                = "${var.task_name}-${var.environment}"
  description         = "Scheduled task for ${var.task_name} in ${var.environment}"
  schedule_expression = var.schedule_task_expression
  is_enabled          = var.event_rule_enabled
}

resource "aws_cloudwatch_event_target" "ecs_scheduled_task" {
  target_id = "run-scheduled-task-${var.task_name}-${var.environment}"
  arn       = aws_ecs_cluster.app.arn
  rule      = aws_cloudwatch_event_rule.run_command.name
  role_arn  = aws_iam_role.cloudwatch_target_role.arn

  ecs_target {
    launch_type         = "FARGATE"
    task_count          = 1
    task_definition_arn = aws_ecs_task_definition.scheduled_task_def.arn

    network_configuration {
      subnets         = var.ecs_subnet_ids
      security_groups = [aws_security_group.ecs_sg.id]
    }
  }
}

## ECS ##

# ECS cluster 

resource "aws_ecs_cluster" "app" {
  name = "${var.app_name}-${var.environment}-${var.task_name}"
}

# ECS security group

resource "aws_security_group" "ecs_sg" {
  name        = "ecs-${var.app_name}-${var.environment}"
  description = "${var.app_name}-${var.environment} container security group"
  vpc_id      = var.ecs_vpc_id

  tags = {
    Name        = "ecs-${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

resource "aws_security_group_rule" "app_ecs_allow_outbound" {
  description       = "Allow all outbound"
  security_group_id = aws_security_group.ecs_sg.id

  type        = "egress"
  from_port   = 0
  to_port     = 0
  protocol    = "-1"
  cidr_blocks = ["0.0.0.0/0"]
}

# ECS task details

resource "aws_ecs_task_definition" "scheduled_task_def" {
  family       = "${var.app_name}-${var.environment}-${var.task_name}"
  network_mode = "awsvpc"

  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "1024"

  task_role_arn      = aws_iam_role.task_role.arn
  execution_role_arn = aws_iam_role.task_execution_role.arn

  container_definitions = templatefile("${path.module}/container-definitions.json",
    {
      app_name    = var.app_name,
      task_name   = var.task_name,
      environment = var.environment,
      repo_url    = var.repo_url
      image_tag   = var.image_tag

      s3_bucket                           = var.s3_bucket,
      s3_key                              = var.s3_key,
      username_header                     = var.username_header
      password_header                     = var.password_header
      automated_sheet_password_param_name = aws_ssm_parameter.automated_sheet_password.name

      sheet_name_dev  = var.sheet_name_dev
      sheet_name_val  = var.sheet_name_val
      sheet_name_prod = var.sheet_name_prod

      portal_hostname_dev  = var.portal_hostname_dev
      portal_hostname_val  = var.portal_hostname_val
      portal_hostname_prod = var.portal_hostname_prod

      idm_hostname_dev  = var.idm_hostname_dev
      idm_hostname_val  = var.idm_hostname_val
      idm_hostname_prod = var.idm_hostname_prod

      awslogs_group  = local.awslogs_group,
      awslogs_region = data.aws_region.current.name
    }
  )
}

# CloudWatch log group

resource "aws_cloudwatch_log_group" "ecs" {
  name = local.awslogs_group
}

# Systems Manager parameter 

resource "aws_ssm_parameter" "automated_sheet_password" {
  name  = "${var.app_name}-${var.environment}-automated-sheet-password"
  type  = "SecureString"
  value = "set_manually_after_creation"

  lifecycle {
    ignore_changes = [value]
  }
}

# S3 bucket
resource "aws_s3_bucket" "spreadsheet" {
  bucket = var.s3_bucket
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