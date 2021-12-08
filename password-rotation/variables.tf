variable "app_name" {
  type        = string
  description = "Name of the application"
}

variable "environment" {
  type        = string
  description = "Environment name"
}

variable "task_name" {
  type        = string
  description = "Name of the task to be run"
}

variable "image_tag" {
  type        = string
  description = "Tag of the image to be run by the task definition"
  default     = "latest"
}

variable "repo_url" {
  type        = string
  description = "URL of the ECR repo that hosts the password rotation image"
}

variable "ecs_vpc_id" {
  type        = string
  description = "VPC ID to be used by ECS"
}

variable "ecs_subnet_ids" {
  type        = list(string)
  description = "Subnet IDs for the ECS task."
}

variable "schedule_task_expression" {
  type        = string
  description = "Cron based schedule task to run on a cadence"
}

variable "event_rule_enabled" {
  type        = bool
  description = "Whether the event rule that triggers the task is enabled"
}

variable "s3_bucket" {
  type        = string
  description = "S3 bucket that will contain the test user spreadsheet"
}

variable "s3_key" {
  type        = string
  description = "The S3 key (path/filename) for the test user spreadsheet"
  default     = ""
}

variable "file_name" {
  type        = string
  description = "File name for the test user spreadsheet"
}

variable "sheet_name" {
  type        = string
  description = "Sheet name for the test user spreadsheet"
}

variable "username_header" {
  type        = string
  description = "Username header for the test user spreadsheet"
}

variable "password_header" {
  type        = string
  description = "Password header for the test user spreadsheet"
}

variable "portal_environment" {
  type        = string
  description = "Target environment for the CMS Enterprise Portal"
}

variable "portal_hostname" {
  type        = string
  description = "Hostname for the CMS Enterprise Portal"
}

variable "idm_hostname" {
  type        = string
  description = "Hostname for CMS Enterprise Portal IDM "
}