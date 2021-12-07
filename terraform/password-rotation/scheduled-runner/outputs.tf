output "ecr_repo_arn" {
  description = "ARN for ECR repo used by the scheduled runner"
  value       = aws_ecr_repository.app.arn
}