terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.30.0"
    }
  }

  required_version = ">= 0.13"

  # backend "s3" {
  #   bucket         = "aws-cms-oit-iusg-spe-cmcs-macbis-dev-tf-state-us-east-1"
  #   key            = "mac-fc-infra/portal-test-user-manager/terraform.tfstate"
  #   dynamodb_table = "terraform-state-lock"
  #   region         = "us-east-1"
  #   encrypt        = "true"
  #   role_arn       = "arn:aws:iam::037370603820:role/atlantis"
  # }
}