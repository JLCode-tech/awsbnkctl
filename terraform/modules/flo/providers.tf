# ============================================================
# flo — outer providers (Sprint 3 AWS retarget)
#
# The IBM provider + `ibm_container_cluster_config` data-source
# pattern is replaced with the EKS-native
# `aws_eks_cluster_auth` short-lived token + `aws_eks_cluster`
# endpoint + CA cert. Same shape the root module uses.
# ============================================================

resource "null_resource" "eks_cluster_gate" {
  triggers = {
    dep = var.eks_cluster_dependency_id != null ? var.eks_cluster_dependency_id : "direct-apply"
  }
}

resource "null_resource" "cert_manager_gate" {
  triggers = {
    dep = var.cert_manager_dependency_id != null ? var.cert_manager_dependency_id : "direct-apply"
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
