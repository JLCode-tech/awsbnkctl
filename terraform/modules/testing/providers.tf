# ============================================================
# testing — providers (Sprint 3 AWS retarget)
#
# Drops the dual-region IBM provider pattern (vpc_region alias);
# AWS jumphosts live in the same VPC + region as the cluster.
# ============================================================

resource "null_resource" "eks_cluster_gate" {
  triggers = {
    dep = var.eks_cluster_dependency_id != null ? var.eks_cluster_dependency_id : "direct-apply"
  }
}
