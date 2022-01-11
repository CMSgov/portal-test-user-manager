terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.30.0"
    }
  }

  required_version = ">= 0.13"

  backend "s3" {
    bucket         = "aws-cms-oit-iusg-spe-cmcs-macbis-dev-tf-state-us-east-1"
    key            = "mac-fc-infra/password-rotation/terraform.tfstate"
    dynamodb_table = "terraform-state-lock"
    region         = "us-east-1"
    encrypt        = "true"
  }
}