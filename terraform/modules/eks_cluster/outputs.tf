# ============================================================
# eks_cluster — Sprint 0 placeholder outputs
#
# Output shape sourced from PRD 07
# (docs/prd/07-EKS-CLUSTER-SRIOV.md § "Outputs"). Sprint 0 returns
# empty strings; Sprint 1 wires these to the upstream
# terraform-aws-modules/eks/aws outputs.
#
# Downstream modules (Sprint 3's cert_manager / flo / cne_instance /
# license / testing) consume cluster_oidc_issuer_url + oidc_provider_arn
# + cluster_ready_id; declaring them here in Sprint 0 lets the root
# module reference them by name from the moment the stub lands.
# ============================================================

output "cluster_name" {
  description = "EKS cluster name (echoed)"
  value       = ""
}

output "cluster_endpoint" {
  description = "EKS API endpoint URL"
  value       = ""
}

output "cluster_ca_data" {
  description = "EKS CA cert (base64)"
  value       = ""
}

output "cluster_oidc_issuer_url" {
  description = "OIDC issuer URL for IRSA (Sprint 2 input)"
  value       = ""
}

output "oidc_provider_arn" {
  description = "IAM OIDC provider ARN"
  value       = ""
}

output "node_group_role_arn" {
  description = "IAM role ARN for the self-managed node group"
  value       = ""
}

output "cluster_ready_id" {
  description = "Empty-resource ID for downstream depends_on (carries through roksbnkctl convention)"
  value       = null_resource.eks_cluster_sprint_one_placeholder.id
}
