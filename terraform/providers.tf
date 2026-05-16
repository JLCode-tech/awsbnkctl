# ============================================================
# Root provider configurations (Sprint 1)
#
# aws        — primary cloud provider. Credentials resolve via the
#              standard chain (env / shared config / profile / instance
#              role / SSO); Sprint 2 adds IRSA-aware variants for
#              in-cluster pods.
#
# kubernetes — configured against the EKS cluster's API endpoint +
#              CA data + a temporary auth token computed by the
#              `aws_eks_cluster_auth` data source (which presigns
#              `sts:GetCallerIdentity` under the hood — the same
#              mechanism the in-binary kubeconfig generator uses, see
#              `internal/aws/eks.go`).
#
# helm       — points at the same EKS cluster. Used by Sprint 3's
#              cert_manager / flo modules via `helm_release` (replaces
#              the inherited null_resource + local-exec helm calls).
#
# null + time — small support providers retained for null_resource
#              triggers and time_sleep ordering.
#
# Provider initialisation against a not-yet-existing cluster: the
# kubernetes + helm provider blocks reference module.eks_cluster
# outputs that are only populated after `terraform apply` reconciles
# the EKS control plane. `terraform validate` doesn't evaluate
# expressions so this is fine at validate time; `terraform plan`
# evaluates them but tolerates `null` (provider connection is
# deferred until first resource access).
# ============================================================

provider "aws" {
  region = var.region
}

# `aws_eks_cluster_auth` data source surfaces a short-lived auth token
# (15 minutes) signed via `sts:GetCallerIdentity` — same auth model the
# in-binary kubeconfig generator emits (see `internal/aws/eks.go`).
data "aws_eks_cluster_auth" "cluster" {
  name = module.eks_cluster.cluster_name
}

provider "kubernetes" {
  host                   = module.eks_cluster.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks_cluster.cluster_ca_data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

provider "helm" {
  kubernetes {
    host                   = module.eks_cluster.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks_cluster.cluster_ca_data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

provider "null" {}
