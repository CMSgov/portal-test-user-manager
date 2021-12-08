variable "bucket_name" {
  type        = string
  description = "The name of the S3 bucket containing the test user spreadsheet"
}

variable "spreadsheet_name" {
  type        = string
  description = "The filename of the test user spreadsheet"
}

variable "ecs_task_role_arn" {
  type        = string
  description = "The ARN of the ECS task role used by the password rotation application"
}