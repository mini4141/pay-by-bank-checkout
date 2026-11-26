terraform {
  required_version = ">= 1.9"
  required_providers {
    aws    = { source = "hashicorp/aws", version = "~> 6.0" }
    random = { source = "hashicorp/random", version = "~> 3.6" }
  }
}

provider "aws" {
  region  = var.region
  profile = "pay-by-bank-checkout-deployer"

  default_tags {
    tags = {
      Project   = "pay-by-bank-checkout"
      Ephemeral = "true"
      ManagedBy = "terraform"
      Owner     = "alvieirvansyah"
    }
  }
}
