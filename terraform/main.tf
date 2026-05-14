# ============================================================
# F5 BIG-IP Next for Kubernetes — Root Module (Sprint 0)
#
# Sprint 0 strips the IBM-Cloud module wiring (roks_cluster +
# cert_manager / flo / cne_instance / license / testing modules that
# consumed IBM outputs) and replaces it with a single TODO marker per
# downstream sprint. The four reusable modules (cert_manager, flo,
# cne_instance, license, testing) survive on disk; their AWS-shaped
# rewire is Sprint 3 work per docs/PLAN.md § "Sprint 3".
#
# Execution-order target (Sprint 3 deliverable):
#
#   eks_cluster ──► cert_manager
#                └─► s3_supply_chain + iam_irsa (Sprint 2, PRD 08)
#                       └─► flo
#                              └─► cne_instance
#                                     └─► license
#                                     └─► testing
#
# Sprint 1 lands only `module "eks_cluster"` — the rest stay TODO'd
# until their consuming sprints. Sprint 0 wiring intentionally
# fail-stops at the eks_cluster apply (see modules/eks_cluster/main.tf)
# rather than silently succeed with no infra.
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
}


# ============================================================
# Sprint 2 — S3 supply chain + IRSA (PRD 08)
# ============================================================
#
# TODO(Sprint 2, docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md):
#
#   module "s3_supply_chain" {
#     source     = "./modules/s3_supply_chain"
#     bucket_name = ...
#     kms_key_arn = ...
#     ...
#   }
#
#   module "iam_irsa" {
#     source                  = "./modules/iam_irsa"
#     cluster_oidc_issuer_url = module.eks_cluster.cluster_oidc_issuer_url
#     oidc_provider_arn       = module.eks_cluster.oidc_provider_arn
#   }


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
# the Sprint 0 tree doesn't fail on undefined wiring.
#
#   module "cert_manager" {
#     source = "./modules/cert_manager"
#     # AWS-shaped inputs: region, role ARN, cluster id from eks_cluster
#   }
#
#   module "flo" {
#     source = "./modules/flo"
#     # AWS-shaped inputs: s3 bucket name, flo IRSA role ARN
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
