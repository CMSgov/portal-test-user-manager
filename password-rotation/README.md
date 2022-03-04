# password-rotation-runner

A Terraform module which deploys a scheduled ECS task on a cron schedule.  This task runs an application which pulls a test user spreadsheet from S3, rotates the Enterprise Portal passwords for the test users, updates the spreadsheet, and pushes it back to S3. To read more about the password rotation application, see the README (TBD)

## Usage
See [variables.tf](variables.tf) for variable descriptions.
To enable the scheduled task to run, uncomment `event_rule_enabled = true`. To disable it, comment out the line.
To enable the application to send email, uncomment `mail_enabled = "true"` . To disable it, comment out the line.

### Configure the application to update passwords in each testing sheet associated with each portal.
Each portal may be associated with 0 or more sheets used for running tests. Each test sheet contains usernames and passwords. The application updates all passwords in each test sheet if the test sheet is configured for the portal.

For example, the dev portal may be associated with 4 testing sheets. The test sheets are named
DEV, TEST, IMPL, DEVP. By associatng the dev portal sheet (say, `Portal-DEV`) with the test sheets, the application updates usernames in the test sheets with the current password.
For this example, set the variable `devportal_testing_sheet_names` to a string of comma-separated testing sheet names.

If the portal has no associated testing sheets then set the variable to an empty string `""` or remove the variable from the module since it's default value is `""`.
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

  portal_sheet_name_dev       = ""
  portal_sheet_name_val       = ""
  portal_sheet_name_prod      = ""

  # mail_enabled = "true"
  to_addresses = "macfintestingteam@dcca.com" // Separate multiple addresses with a comma.

  # update_env_sheets_enabled = "true"

  devportal_testing_sheet_names  = "" # For ex: "DEV,TEST,IMPL,DEVP"
  valportal_testing_sheet_names  = "" # For ex: "TRAINING,IMPLP"
  prodportal_testing_sheet_names = "" # For ex: "PROD"
}
```

The image run by the scheduled ECS task is managed by MAC FC in a separate account.  MAC FC will provide a value for the `repo_url` variable that tells the ECS task what ECR repo to pull from. 

After creating the S3 bucket, the test user spreadsheet must be uploaded to the bucket manually using the key specified by the `s3_key` variable.