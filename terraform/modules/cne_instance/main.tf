# ============================================================
# cne_instance — outer module body (Sprint 3 AWS retarget)
#
# The CNEInstance CRD body lives in the inner
# `./modules/cneinstance` and is preserved unchanged per PRD 00
# § "Inheritance map". Sprint 3's job is the parameter-boundary
# port — `cneinstance_ibm_trusted_profile_id` becomes
# `cneinstance_irsa_role_arn` for FLO-side consumption.
#
# Sprint 4 wires the inner module's kubernetes_manifest CR via the
# AWS-native helm + kubernetes providers once the live spike
# (PRD 07 § "Spike protocol") validates the cluster shape. Until
# then this outer body provisions only the cross-module locals +
# the readiness sentinel.
# ============================================================

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
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

locals {
  cneinstance_values = {
    deployment_size      = var.cneinstance_deployment_size
    gslb_datacenter_name = var.cneinstance_gslb_datacenter_name
    network_attachments  = var.cneinstance_network_attachments

    flo_namespace         = var.flo_namespace
    utils_namespace       = var.flo_utils_namespace
    far_repo_url          = var.far_repo_url
    manifest_version      = var.f5_bigip_k8s_manifest_version
    cluster_issuer_name   = var.flo_cluster_issuer_name
    flo_irsa_role_arn     = var.flo_irsa_role_arn

    # AWS-shaped cloud env replacing the v0.x IBM block.
    cloud_provider = "aws"
    cloud_region   = var.aws_region
  }
}

resource "null_resource" "cneinstance_ready" {
  triggers = {
    enabled         = var.deploy_bnk ? "on" : "off"
    flo_irsa_role   = var.flo_irsa_role_arn
    cluster         = var.eks_cluster_name
    network_attach  = jsonencode(var.cneinstance_network_attachments)
  }

  depends_on = [
    null_resource.eks_cluster_gate,
    null_resource.flo_gate,
  ]
}
