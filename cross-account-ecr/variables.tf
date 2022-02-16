variable "principal_arns" {
  type        = list(string)
  description = "ARNs to grant ECR access to"
}

variable "repo_name" {
  type        = string
  description = "Name of the ECR repo"
}