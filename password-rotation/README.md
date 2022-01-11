# password-rotation-runner

A Terraform module which deploys a scheduled ECS task on a cron schedule.  This task runs an application which pulls a test user spreadsheet from S3, rotates the Enterprise Portal passwords for the test users, updates the spreadsheet, and pushes it back to S3. To read more about the password rotation application, see the README (TBD)

## Usage
See [variables.tf](variables.tf) for variable descriptions
```
module "password-rotation" {
  source      = "github.com/CMSgov/portal-test-user-manager//password-rotation
  
  app_name    = ""              // defaults to "password-rotation"
  environment = "dev"
  task_name   = ""               // defaults to "scheduled-runner"

  repo_url                 = "156322662943.dkr.ecr.us-east-1.amazonaws.com/password-rotation"
  image_tag                = "" // defaults to "latest"
  ecs_vpc_id               = "" 
  ecs_subnet_ids           = [] 
  schedule_task_expression = "" // defaults to "0 8 1 * ? *", monthly on the first day of the month at 8am UTC (3am UTC-5)
  event_rule_enabled       =    // defaults to true

  s3_bucket            = ""
  s3_key               = ""
  username_header      = "UserName"
  password_header      = "Password"

  sheet_name_dev       = "Portal-DEV"
  sheet_name_val       = "Portal-VAL"
  sheet_name_prod      = "Portal-PROD"

  portal_hostname_dev  = "portaldev.cms.gov"
  portal_hostname_val  = "portalval.cms.gov"
  portal_hostname_prod = "portal.cms.gov"
  
  idm_hostname_dev     = "test.idp.idm.cms.gov"
  idm_hostname_val     = "impl.idp.idm.cms.gov"
  idm_hostname_prod    = "idp.idm.cms.gov"
}
```

The image run by the scheduled ECS task is managed by MAC FC in a separate account.  MAC FC has provided a value for the `repo_url` variable that tells the ECS task what ECR repo to pull from. 

After creating the S3 bucket, the test user spreadsheet must be uploaded to the bucket manually using the key specified by the `s3_key` variable.