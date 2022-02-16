module "cross_account_ecr" {
  source = "../cross-account-ecr"

  principal_arns = [
    "arn:aws:iam::037370603820:root", # macbis-dev
    "arn:aws:iam::741306476019:root"  # MACFin
  ]
  repo_name = "password-rotation"
}

output "cross_account_ecr_outputs" {
  value = module.cross_account_ecr
}