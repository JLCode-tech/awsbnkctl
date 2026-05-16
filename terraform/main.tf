# ============================================================
# F5 BIG-IP Next for Kubernetes — Root Module (Sprint 2)
#
# Sprint 2 layers s3_supply_chain + iam_irsa (PRD 08) on top of the
# Sprint 1 eks_cluster module. The ecr_mirror module is wired here
# behind var.enable_ecr_mirror (default false) per PRD 08's v1.0
# stretch posture; the actual skopeo pipeline lands in Sprint 3.
#
# Execution-order target (Sprint 3 deliverable):
#
#   eks_cluster ──► cert_manager
#                └─► s3_supply_chain + iam_irsa  (Sprint 2, PRD 08)
#                       └─► flo
#                              └─► cne_instance
#                                     └─► license
#                                     └─► testing
# ============================================================


# ============================================================
# eks_cluster — Sprint 1 deliverable per PRD 07
# (docs/prd/07-EKS-CLUSTER-SRIOV.md)
# ============================================================

module "eks_cluster" {
  source = "./modules/eks_cluster"

  region              = var.region
  cluster_name        = var.cluster_name
  cluster_version     = var.cluster_version
  vpc_id              = var.vpc_id
  subnet_ids          = var.subnet_ids
  node_instance_types = var.node_instance_types
  node_min_size       = var.node_min_size
  node_max_size       = var.node_max_size
  node_desired_size   = var.node_desired_size
  enable_multus       = var.enable_multus
  enable_sriov        = var.enable_sriov
  sriov_resource_name = var.sriov_resource_name
}


# ============================================================
# s3_supply_chain — Sprint 2 deliverable per PRD 08
# (docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md § "Implementation outline")
#
# Holds the FAR pull-key archive + subscription JWT. Encrypted via
# a customer-managed KMS CMK created in-module (override via
# var.kms_key_arn for governance scenarios).
# ============================================================

module "s3_supply_chain" {
  source = "./modules/s3_supply_chain"

  region                   = var.region
  workspace_name           = var.workspace_name
  kms_key_arn              = var.kms_key_arn
  far_auth_file_local_path = var.far_auth_file_local_path
  jwt_file_local_path      = var.jwt_file_local_path
}


# ============================================================
# iam_irsa — Sprint 2 deliverable per PRD 08
# (docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md § "Implementation outline")
#
# Trust-policy IAM role bound to the FLO ServiceAccount via the EKS
# OIDC provider; permission policy grants s3:GetObject on the
# s3_supply_chain bucket + kms:Decrypt on the CMK.
# ============================================================

module "iam_irsa" {
  source = "./modules/iam_irsa"

  region                   = var.region
  cluster_name             = var.cluster_name
  oidc_provider_arn        = module.eks_cluster.oidc_provider_arn
  cluster_oidc_issuer_url  = module.eks_cluster.cluster_oidc_issuer_url
  flo_namespace            = var.flo_namespace
  s3_bucket_arn            = module.s3_supply_chain.bucket_arn
  kms_key_arn              = module.s3_supply_chain.kms_key_arn
}


# ============================================================
# ecr_mirror — Sprint 2 scaffold per PRD 08 v1.0 stretch
#
# Gated on var.enable_ecr_mirror (default false). Module body is
# a no-op when disabled; when enabled it creates one ECR repository
# per image and (Sprint 3 follow-up) runs skopeo copy via
# null_resource.
# ============================================================

module "ecr_mirror" {
  source = "./modules/ecr_mirror"

  region            = var.region
  workspace_name    = var.workspace_name
  enable_ecr_mirror = var.enable_ecr_mirror
}


# ============================================================
# Sprint 3 — port the four reusable modules onto AWS inputs
# (docs/PLAN.md § "Sprint 3")
# ============================================================
#
# TODO(Sprint 3): rewire each module to consume AWS-shaped inputs.
# The module directories survive on disk (cert_manager, flo,
# cne_instance, license, testing) and their bodies are mostly pure
# k8s manifests — only the parameter boundary changes. Until then
# the call sites are commented out so `terraform validate` against
# the Sprint 2 tree doesn't fail on undefined wiring.
#
#   module "cert_manager" {
#     source = "./modules/cert_manager"
#     # AWS-shaped inputs: region, role ARN, cluster id from eks_cluster
#   }
#
#   module "flo" {
#     source = "./modules/flo"
#     # AWS-shaped inputs: s3 bucket name, flo IRSA role ARN
#     flo_role_arn   = module.iam_irsa.flo_role_arn
#     s3_bucket_name = module.s3_supply_chain.bucket_name
#   }
#
#   module "cne_instance" {
#     source = "./modules/cne_instance"
#     # AWS-shaped inputs: flo_irsa_role_arn (replaces flo_trusted_profile_id)
#   }
#
#   module "license" {
#     source = "./modules/license"
#     # AWS-shaped inputs: presigned S3 URL for JWT (replaces COS endpoint)
#   }
#
#   module "testing" {
#     source = "./modules/testing"
#     # AWS-shaped inputs: vpc_id, subnet_ids (replaces transit gateway)
#   }
