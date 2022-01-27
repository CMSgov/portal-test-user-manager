# password-rotation-runner

A Terraform module which deploys a scheduled ECS task on a cron schedule.  This task runs an application which pulls a test user spreadsheet from S3, rotates the Enterprise Portal passwords for the test users, updates the spreadsheet, and pushes it back to S3. To read more about the password rotation application, see the README (TBD)

## Usage
See [variables.tf](variables.tf) for variable descriptions.
```
module "password-rotation" {
  source      = "github.com/CMSgov/portal-test-user-manager//password-rotation
  
  environment = ""

  repo_url                    = "" 
  ecs_vpc_id                  = "" 
  ecs_subnet_ids              = []
  # event_rule_enabled        = true

  s3_bucket            = ""
  s3_key               = ""
  username_header      = ""
  password_header      = ""

  sheet_name_dev       = ""
  sheet_name_val       = ""
  sheet_name_prod      = ""
}
```

The image run by the scheduled ECS task is managed by MAC FC in a separate account.  MAC FC will provide a value for the `repo_url` variable that tells the ECS task what ECR repo to pull from. 

After creating the S3 bucket, the test user spreadsheet must be uploaded to the bucket manually using the key specified by the `s3_key` variable.