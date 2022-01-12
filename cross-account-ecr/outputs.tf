output "ecr_url" {
  description = "URL of the ECR repo"
  value       = aws_ecr_repository.app.repository_url
}