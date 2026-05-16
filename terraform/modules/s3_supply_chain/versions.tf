# ============================================================
# s3_supply_chain — provider requirements
#
# PRD 08 § "Implementation outline" §"terraform/modules/s3_supply_chain/".
# Sprint 2 module; consumed by root main.tf after eks_cluster reconciles.
# ============================================================

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5"
    }
  }
}
