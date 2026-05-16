# ============================================================
# license — outer providers (Sprint 3 AWS retarget)
# ============================================================

resource "null_resource" "eks_cluster_gate" {
  triggers = {
    dep = var.eks_cluster_dependency_id != null ? var.eks_cluster_dependency_id : "direct-apply"
  }
}

resource "null_resource" "cneinstance_gate" {
  triggers = {
    dep = var.cneinstance_dependency_id != null ? var.cneinstance_dependency_id : "direct-apply"
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
