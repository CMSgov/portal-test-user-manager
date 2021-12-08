module "password_rotation" {
  source = "../password-rotation"

  app_name    = "password-rotation"
  environment = "dev"
  task_name   = "scheduled-runner"

  repo_url                 = "156322662943.dkr.ecr.us-east-1.amazonaws.com/password-rotation"
  image_tag                = "latest"
  ecs_vpc_id               = "vpc-043ae3133b10db9a0"
  ecs_subnet_ids           = ["subnet-03f688f7435a936d7", "subnet-0fb2cb5b2036a5c6a"] // private subnets
  schedule_task_expression = "cron(0/2 * * * ? *)"                                    // every 2 minutes
  event_rule_enabled       = false

  // TODO add real values 
  s3_bucket          = "bharvey-test-same-account-bucket"
  s3_key             = "example.txt"
  file_name          = ""
  sheet_name         = ""
  username_header    = ""
  password_header    = ""
  portal_environment = ""
  portal_hostname    = ""
  idm_hostname       = ""
}

output "password_rotation_outputs" {
  value = module.password_rotation
}