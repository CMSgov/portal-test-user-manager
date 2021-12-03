output "task_execution_role_arn" {
  description = "ARN for the IAM role that is executing the scanner"
  value       = aws_iam_role.task_execution_role.arn
}

output "ecs_cluster_arn" {
  description = "ARN for the ECS cluster where this profile will execute"
  value       = var.ecs_cluster_arn
}