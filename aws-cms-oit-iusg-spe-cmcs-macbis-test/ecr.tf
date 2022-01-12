module "cross_account_ecr" {
  source = "../cross-account-ecr"

  account_id = "037370603820"
  repo_name  = "password-rotation"
}

output "cross_account_ecr_outputs" {
  value = module.cross_account_ecr
}