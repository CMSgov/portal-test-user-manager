module "scheduled-runner" {
  source = "./scheduled-runner"

  app_name    = "password-rotation"
  environment = "dev"
  task_name   = "scheduled-runner"


  image          = "" // TODO add image once it is pushed to ECR
  ecs_vpc_id     = "" // TODO add VPC ID
  ecs_subnet_ids = [] // TODO add private subnet IDs

  schedule_task_expression = "cron(30 9 * * ? *)"

  s3_bucket          = "" // TODO add MACFin S3 bucket
  s3_key             = "" // TODO add MACFin S3 key
  s3_access_role_arn = "" // TODO add MACFin cross-account role arn
}