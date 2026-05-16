# ============================================================
# cert_manager — outer providers (Sprint 3 AWS retarget)
#
# The IBM provider + `ibm_container_cluster_config` data-source
# fetch pattern is replaced with the EKS-native
# `aws_eks_cluster_auth` short-lived token + the
# `aws_eks_cluster` data source's endpoint + CA cert. Same shape
# the root module uses, repeated here so the outer cert_manager
# module is callable standalone or from terraform/main.tf.
# ============================================================

# Cluster gate — same convention as the v0.x wrapper: when
# eks_cluster is being created in the same apply, defer reads
# until the cluster's apply finishes.
resource "null_resource" "eks_cluster_gate" {
  triggers = {
    dep = var.eks_cluster_dependency_id != null ? var.eks_cluster_dependency_id : "direct-apply"
  }
}

data "aws_eks_cluster" "cluster" {
  count      = var.create_eks_cluster ? 0 : 1
  name       = var.eks_cluster_name
  depends_on = [null_resource.eks_cluster_gate]
}

data "aws_eks_cluster_auth" "cluster" {
  count      = var.create_eks_cluster ? 0 : 1
  name       = var.eks_cluster_name
  depends_on = [null_resource.eks_cluster_gate]
}
