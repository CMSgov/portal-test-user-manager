variable "app_name" {
  type        = string
  description = "Name of the application"
  default     = "password-rotation"
}

variable "environment" {
  type        = string
  description = "Environment name"
}

variable "task_name" {
  type        = string
  description = "Name of the task to be run"
  default     = "scheduled-runner"
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
  default     = "cron(0 8 1 * ? *)" // monthly on the first day of the month at 8am UTC (3am UTC-5)
}

variable "event_rule_enabled" {
  type        = bool
  description = "Whether the event rule that triggers the task is enabled"
  default     = false
}

variable "s3_bucket" {
  type        = string
  description = "The name for the S3 bucket that will contain the test user spreadsheet"
}

variable "s3_key" {
  type        = string
  description = "The S3 key (path/filename) for the test user spreadsheet"
}

variable "username_header" {
  type        = string
  description = "Username header for the test user spreadsheet"
}

variable "password_header" {
  type        = string
  description = "Password header for the test user spreadsheet"
}

variable "portal_sheet_name_dev" {
  type        = string
  description = "Sheet name for the dev portal"
}

variable "portal_sheet_name_val" {
  type        = string
  description = "Sheet name for the val portal"
}

variable "portal_sheet_name_prod" {
  type        = string
  description = "Sheet name for the prod portal"
}


variable "portal_hostname_dev" {
  type        = string
  description = "Hostname for the dev CMS Enterprise Portal"
  default     = "portaldev.cms.gov"
}

variable "portal_hostname_val" {
  type        = string
  description = "Hostname for the val CMS Enterprise Portal"
  default     = "portalval.cms.gov"
}

variable "portal_hostname_prod" {
  type        = string
  description = "Hostname for the prod CMS Enterprise Portal"
  default     = "portal.cms.gov"
}

variable "idm_hostname_dev" {
  type        = string
  description = "Hostname for dev CMS Enterprise Portal IDM"
  default     = "test.idp.idm.cms.gov"
}

variable "idm_hostname_val" {
  type        = string
  description = "Hostname for val CMS Enterprise Portal IDM"
  default     = "impl.idp.idm.cms.gov"
}

variable "idm_hostname_prod" {
  type        = string
  description = "Hostname for prod CMS Enterprise Portal IDM"
  default     = "idm.cms.gov"
}

# MAIL variables
variable "smtp_host" {
  type    = string
  default = "internal-Enterpris-SMTPProd-I20YLD1GTM6L-357506541.us-east-1.elb.amazonaws.com"
}

variable "smtp_port" {
  type    = string
  default = "25"
}

variable "from_address" {
  type    = string
  default = "no-reply-macfin-ecs-job@cms.hhs.gov"
}

variable "sender_name" {
  type    = string
  default = "macfin-ecs-job"
}

variable "to_addresses" {
  description = "comma-separated email addresses"
  type        = string
  default     = "macfintestingteam@dcca.com"
}

variable "mail_enabled" {
  type    = string
  default = "false"
}

variable "devportal_testing_sheet_names" {
  description = "comma-separated sheet names"
  type        = string
  default     = ""
}

variable "valportal_testing_sheet_names" {
  description = "comma-separated sheet names"
  type        = string
  default     = ""
}

variable "prodportal_testing_sheet_names" {
  description = "comma-separated sheet names"
  type        = string
  default     = ""
}

