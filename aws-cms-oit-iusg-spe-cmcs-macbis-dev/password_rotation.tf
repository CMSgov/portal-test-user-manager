module "password_rotation" {
  source = "../password-rotation"

  app_name    = "password-rotation"
  environment = "dev"
  task_name   = "scheduled-runner"

  repo_url                 = "156322662943.dkr.ecr.us-east-1.amazonaws.com/password-rotation"
  image_tag                = "latest"
  ecs_vpc_id               = "vpc-043ae3133b10db9a0"
  ecs_subnet_ids           = ["subnet-03f688f7435a936d7", "subnet-0fb2cb5b2036a5c6a"] // private subnets
  schedule_task_expression = "cron(0 21 * * ? *)"                                     // every day at 4 PM ET
  event_rule_enabled       = true

  s3_bucket       = "bharvey-test-same-account-bucket"
  s3_key          = "macfin-macfc-portal-users.xlsx"
  username_header = "UserName"
  password_header = "Password"

  sheet_name_dev  = "Portal-DEV"
  sheet_name_val  = "Portal-VAL"
  sheet_name_prod = "Portal-PROD"

  mail_enabled = "true" // To mail the xlsx file, in addition to uploading it to S3, set to "true"
  to_addresses = "leslie@corbalt.com, lesliebklein@gmail.com"
}