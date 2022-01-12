# cross-account-ecr

This module sets up an ECR repo and the IAM permissions for a different account to pull images from that repo

## Usage

```
module "cross_account_ecr" {
  source = ""

  account_id  = ""    // AWS account ID to grant pull access
  repo_name    = ""   // Name of the ECR repo 
}

output "cross_account_ecr_outputs" {
  value = module.cross_account_ecr
}
```

## Outputs

`cross_account_ecr_outputs`: the URL of the ECR repo that the module creates