module "password_rotation" {
  source = "../password-rotation"

  app_name    = "password-rotation"
  environment = "dev"
  task_name   = "scheduled-runner"

  repo_url                 = "156322662943.dkr.ecr.us-east-1.amazonaws.com/password-rotation"
  image_tag                = "latest"
  ecs_vpc_id               = "vpc-043ae3133b10db9a0"
  ecs_subnet_ids           = ["subnet-03f688f7435a936d7", "subnet-0fb2cb5b2036a5c6a"] // private subnets
  schedule_task_expression = "cron(0/1 * * * ? *)"                                    // every 1 minute
  event_rule_enabled       = false

  s3_bucket       = "bharvey-test-same-account-bucket"
  s3_key          = "example.txt"
  username_header = "UserName"
  password_header = "Password"

  sheet_name_dev  = "Portal-DEV"
  sheet_name_val  = "Portal-VAL"
  sheet_name_prod = "Portal-PROD"

  portal_hostname_dev  = "portaldev.cms.gov"
  portal_hostname_val  = "portalval.cms.gov"
  portal_hostname_prod = "portal.cms.gov"

  idm_hostname_dev  = "test.idp.idm.cms.gov"
  idm_hostname_val  = "impl.idp.idm.cms.gov"
  idm_hostname_prod = "idp.idm.cms.gov"
}