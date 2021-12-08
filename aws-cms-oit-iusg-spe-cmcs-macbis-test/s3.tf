module "spreadsheet_s3" {
  source = "../spreadsheet-s3"

  bucket_name       = "bharvey-test-spreadsheet-bucket"
  spreadsheet_name  = "example.txt"
  ecs_task_role_arn = "arn:aws:iam::037370603820:role/ecs-task-role-password-rotation-dev-scheduled-runner"
}

output "spreadsheet_s3" {
  value = module.spreadsheet_s3
}