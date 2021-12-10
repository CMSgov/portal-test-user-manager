# password-rotation-runner

A Terraform module which deploys a scheduled ECS task on a cron schedule.  This task runs an application which pulls a test user spreadsheet from S3, rotates the Enterprise Portal passwords for the test users, updates the spreadsheet, and pushes it back to S3. To read more about the password rotation application, see the README (TBD)

## Usage
See 'Inputs' below for variable descriptions and defaults
```
module "password-rotation" {
  source      = "github.com/CMSgov/portal-test-user-manager-runner/password-rotation
  
  app_name    = ""
  environment = ""
  task_name   = ""

  repo_url                 = ""
  image_tag                = ""
  ecs_vpc_id               = "" 
  ecs_subnet_ids           = [] 
  schedule_task_expression = "" 
  event_rule_enabled       = # true/false

  s3_bucket          = ""
  s3_key             = ""
  sheet_name         = ""
  username_header    = ""
  password_header    = ""
  portal_environment = ""
  portal_hostname    = ""
  idm_hostname       = ""
}
```

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 0.13 |

The image run by the scheduled ECS task is managed by MAC FC in a separate account.  MAC FC will provide a value for the `repo_url` variable that tells the ECS task what ECR repo to pull from. 

After creating the S3, the test user spreadsheet must be uploaded to the bucket manually using the key specified by the `s3_key` variable.

The Portal Test User Manager application requires a password that it uses to lock the portion of the test user spreadsheet that it manages. This password is stored in System Manager Parameter Store in a parameter  called `{app_name}-${environment}-sheet-password`. The value of this parameter must be set manually after the parameter is created by this module. 

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_app_name"></a> [app\_name](#input\_app\_name) | Name of the application | `string` | `"password-rotation"` | no |
| <a name="input_ecs_subnet_ids"></a> [ecs\_subnet\_ids](#input\_ecs\_subnet\_ids) | Subnet IDs for the ECS task | `list(string)` | n/a | yes |
| <a name="input_ecs_vpc_id"></a> [ecs\_vpc\_id](#input\_ecs\_vpc\_id) | VPC ID to be used by ECS | `string` | n/a | yes |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment name | `string` | n/a | yes |
| <a name="input_event_rule_enabled"></a> [event\_rule\_enabled](#input\_event\_rule\_enabled) | Whether the event rule that triggers the task is enabled | `bool` | `true` | no |
| <a name="input_idm_hostname"></a> [idm\_hostname](#input\_idm\_hostname) | Hostname for CMS Enterprise Portal IDM | `string` | n/a | yes |
| <a name="input_image_tag"></a> [image\_tag](#input\_image\_tag) | Tag of the image to be run by the task definition | `string` | `"latest"` | no |
| <a name="input_password_header"></a> [password\_header](#input\_password\_header) | Password header for the test user spreadsheet | `string` | n/a | yes |
| <a name="input_portal_environment"></a> [portal\_environment](#input\_portal\_environment) | Target environment for the CMS Enterprise Portal | `string` | n/a | yes |
| <a name="input_portal_hostname"></a> [portal\_hostname](#input\_portal\_hostname) | Hostname for the CMS Enterprise Portal | `string` | n/a | yes |
| <a name="input_repo_url"></a> [repo\_url](#input\_repo\_url) | URL of the ECR repo that hosts the password rotation image | `string` | n/a | yes |
| <a name="input_s3_bucket"></a> [s3\_bucket](#input\_s3\_bucket) | The name for the S3 bucket that will contain the test user spreadsheet | `string` | n/a | yes |
| <a name="input_s3_key"></a> [s3\_key](#input\_s3\_key) | The S3 key (path/filename) for the test user spreadsheet | `string` | n/a | yes |
| <a name="input_schedule_task_expression"></a> [schedule\_task\_expression](#input\_schedule\_task\_expression) | Cron based schedule task to run on a cadence | `string` | `"0 3 1 * ? *"` | no |
| <a name="input_sheet_name"></a> [sheet\_name](#input\_sheet\_name) | Sheet name for the test user spreadsheet | `string` | n/a | yes |
| <a name="input_task_name"></a> [task\_name](#input\_task\_name) | Name of the task to be run | `string` | `"scheduled-runner"` | no |
| <a name="input_username_header"></a> [username\_header](#input\_username\_header) | Username header for the test user spreadsheet | `string` | n/a | yes |

## Resources

| Name | Type |
|------|------|
| [aws_cloudwatch_event_rule.run_command](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_rule) | resource |
| [aws_cloudwatch_event_target.ecs_scheduled_task](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_target) | resource |
| [aws_cloudwatch_log_group.ecs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_ecs_cluster.app](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_cluster) | resource |
| [aws_ecs_task_definition.scheduled_task_def](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ecs_task_definition) | resource |
| [aws_iam_policy.parameter_store](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_policy) | resource |
| [aws_iam_policy.s3_access](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_policy) | resource |
| [aws_iam_policy_attachment.parameter_store](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_policy_attachment) | resource |
| [aws_iam_role.cloudwatch_target_role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.task_execution_role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role.task_role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy_attachment.container_service_events](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_iam_role_policy_attachment.ecs_task_execution](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_s3_bucket.spreadsheet](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket) | resource |
| [aws_security_group.ecs_sg](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group_rule.app_ecs_allow_outbound](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group_rule) | resource |
| [aws_ssm_parameter.sheet_password](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_ecs_task_definition.scheduled_task_def](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ecs_task_definition) | data source |
| [aws_iam_policy_document.ecs_assume_role_policy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.events_assume_role_policy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.s3_access](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_partition.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/partition) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Outputs

No outputs.
