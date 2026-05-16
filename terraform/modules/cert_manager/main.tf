# ============================================================
# cert_manager — outer module body (Sprint 3 AWS retarget)
#
# Calls the inner ./modules/cert-manager body unchanged (PRD 00
# § "Inheritance map": ports unchanged). The kube_host + kube_token
# inputs the inner module consumes come from the EKS data sources
# in providers.tf, not from `ibm_container_cluster_config`.
# ============================================================

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
    time = {
      source  = "hashicorp/time"
      version = ">= 0.9.0"
    }
  }
}

module "cert_manager" {
  source = "./modules/cert-manager"

  depends_on = [null_resource.eks_cluster_gate]

  enabled               = true
  namespace             = var.cert_manager_namespace
  chart_version         = var.cert_manager_version
  post_deployment_delay = 30
  kube_host             = try(data.aws_eks_cluster.cluster[0].endpoint, "")
  kube_token            = try(data.aws_eks_cluster_auth.cluster[0].token, "")
}
