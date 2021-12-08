module "scheduled-runner" {
  source = "./scheduled-runner"

  app_name    = "password-rotation"
  environment = "dev"
  task_name   = "scheduled-runner"

  image                    = "194f2c9a"
  ecs_vpc_id               = "vpc-043ae3133b10db9a0"
  ecs_subnet_ids           = ["subnet-03f688f7435a936d7", "subnet-0fb2cb5b2036a5c6a"] // private subnets
  schedule_task_expression = "cron(0/5 * * * ? *)"                                    // every 5 minutes
  event_rule_enabled       = true
  // TODO add MACFin cross-account role arn
  s3_access_role_arn = "arn:aws:iam::156322662943:role/bharvey-test-spreadsheet-bucket-cross-account-access"
  // TODO add real values 
  s3_bucket          = "bharvey-test-spreadsheet-bucket"
  s3_key             = "example.txt"
  file_name          = ""
  sheet_name         = ""
  username_header    = ""
  password_header    = ""
  portal_environment = ""
  portal_hostname    = ""
  idm_hostname       = ""
}