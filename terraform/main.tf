# ============================================================
# F5 BIG-IP Next for Kubernetes — Root Module (Sprint 3)
#
# Sprint 3 wires the full end-to-end lifecycle per PLAN.md
# § "Sprint 3":
#
#   eks_cluster ──► cert_manager
#               └─► s3_supply_chain + iam_irsa
#                     └─► flo
#                            └─► cne_instance
#                                   └─► license
#                                   └─► testing
#
# Execution-order target: `awsbnkctl up --dry-run` plans this full
# graph against fake AWS credentials. Live `terraform apply` is
# gated on the operator-run spike (PRD 07 § "Spike protocol").
# ============================================================


# ============================================================
# eks_cluster — Sprint 1 deliverable per PRD 07
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
# ============================================================

module "iam_irsa" {
  source = "./modules/iam_irsa"

  region                  = var.region
  cluster_name            = var.cluster_name
  oidc_provider_arn       = module.eks_cluster.oidc_provider_arn
  cluster_oidc_issuer_url = module.eks_cluster.cluster_oidc_issuer_url
  flo_namespace           = var.flo_namespace
  s3_bucket_arn           = module.s3_supply_chain.bucket_arn
  kms_key_arn             = module.s3_supply_chain.kms_key_arn
}


# ============================================================
# ecr_mirror — Sprint 2 scaffold per PRD 08 v1.0 stretch
# ============================================================

module "ecr_mirror" {
  source = "./modules/ecr_mirror"

  region            = var.region
  workspace_name    = var.workspace_name
  enable_ecr_mirror = var.enable_ecr_mirror
}


# ============================================================
# cert_manager — Sprint 3 deliverable (port unchanged)
# ============================================================

module "cert_manager" {
  source = "./modules/cert_manager"

  aws_region                = var.region
  eks_cluster_name          = module.eks_cluster.cluster_name
  cert_manager_namespace    = var.cert_manager_namespace
  cert_manager_version      = var.cert_manager_version
  eks_cluster_dependency_id = module.eks_cluster.cluster_ready_id
}


# ============================================================
# flo — Sprint 3 deliverable (S3 + IRSA retarget)
# ============================================================

module "flo" {
  source = "./modules/flo"

  aws_region                    = var.region
  eks_cluster_name              = module.eks_cluster.cluster_name
  far_repo_url                  = var.far_repo_url
  f5_bigip_k8s_manifest_version = var.f5_bigip_k8s_manifest_version
  flo_namespace                 = var.flo_namespace
  flo_utils_namespace           = var.flo_utils_namespace
  cert_manager_namespace        = var.cert_manager_namespace

  # PRD 08 supply-chain inputs
  s3_bucket_name         = module.s3_supply_chain.bucket_name
  s3_far_auth_object_key = module.s3_supply_chain.far_auth_object_key
  s3_jwt_object_key      = module.s3_supply_chain.jwt_object_key
  flo_irsa_role_arn      = module.iam_irsa.flo_role_arn

  deploy_bnk                 = var.deploy_bnk
  eks_cluster_dependency_id  = module.eks_cluster.cluster_ready_id
  cert_manager_dependency_id = module.cert_manager.cert_manager_ready_id
}


# ============================================================
# cne_instance — Sprint 3 deliverable (CRD unchanged)
# ============================================================

module "cne_instance" {
  source = "./modules/cne_instance"

  aws_region                       = var.region
  eks_cluster_name                 = module.eks_cluster.cluster_name
  far_repo_url                     = var.far_repo_url
  f5_bigip_k8s_manifest_version    = var.f5_bigip_k8s_manifest_version
  flo_namespace                    = var.flo_namespace
  flo_utils_namespace              = var.flo_utils_namespace
  flo_irsa_role_arn                = module.iam_irsa.flo_role_arn
  cneinstance_deployment_size      = var.cneinstance_deployment_size
  cneinstance_gslb_datacenter_name = var.cneinstance_gslb_datacenter_name
  cneinstance_network_attachments  = module.flo.cneinstance_network_attachments

  deploy_bnk                = var.deploy_bnk
  eks_cluster_dependency_id = module.eks_cluster.cluster_ready_id
  flo_dependency_id         = module.flo.flo_ready_id
}


# ============================================================
# license — Sprint 3 deliverable (JWT-from-S3 retarget)
# ============================================================

module "license" {
  source = "./modules/license"

  aws_region          = var.region
  eks_cluster_name    = module.eks_cluster.cluster_name
  s3_bucket_name      = module.s3_supply_chain.bucket_name
  s3_jwt_object_key   = module.s3_supply_chain.jwt_object_key
  flo_irsa_role_arn   = module.iam_irsa.flo_role_arn
  flo_utils_namespace = var.flo_utils_namespace
  license_mode        = var.license_mode

  deploy_bnk                = var.deploy_bnk
  eks_cluster_dependency_id = module.eks_cluster.cluster_ready_id
  cneinstance_dependency_id = module.cne_instance.cneinstance_ready_id
}


# ============================================================
# testing — Sprint 3 deliverable (jumphost fixtures)
# ============================================================

module "testing" {
  source = "./modules/testing"

  aws_region                       = var.region
  eks_cluster_name                 = module.eks_cluster.cluster_name
  aws_vpc_id                       = var.vpc_id
  aws_subnet_ids                   = var.subnet_ids
  testing_create_cluster_jumphosts = var.testing_create_cluster_jumphosts
  testing_ssh_key_name             = var.testing_ssh_key_name
  testing_jumphost_instance_type   = var.testing_jumphost_instance_type

  eks_cluster_dependency_id = module.eks_cluster.cluster_ready_id
}
