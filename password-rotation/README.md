# password-rotation-runner

A Terraform module which deploys a scheduled ECS task on a cron schedule.  This task runs an application which pulls a test user spreadsheet from S3, rotates the Enterprise Portal passwords for the test users, updates the spreadsheet, and pushes it back to S3. To read more about the password rotation application, see the README (TBD)

## Usage
See the 'Inputs' section in [DOCS.md](DOCS.md) for variable descriptions and defaults
```
module "password-rotation" {
  source      = "github.com/CMSgov/portal-test-user-manager//password-rotation
  
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

The image run by the scheduled ECS task is managed by MAC FC in a separate account.  MAC FC will provide a value for the `repo_url` variable that tells the ECS task what ECR repo to pull from. 

After creating the S3, the test user spreadsheet must be uploaded to the bucket manually using the key specified by the `s3_key` variable.