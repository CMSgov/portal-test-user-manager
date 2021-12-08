module "spreadsheet-s3" {
  source = "./module"

  bucket_name       = "bharvey-test-spreadsheet-bucket"
  spreadsheet_name  = "example.txt"
  ecs_task_role_arn = "arn:aws:iam::037370603820:role/ecs-task-exec-role-password-rotation-dev-scheduled-runner"
}