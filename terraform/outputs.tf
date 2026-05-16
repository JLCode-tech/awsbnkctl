# ============================================================
# Root outputs (Sprint 1)
#
# Sprint 0 stripped the legacy ROKS outputs; Sprint 1 surfaces the
# eks_cluster module's contract (cluster name + endpoint + CA data +
# OIDC issuer + IRSA provider ARN). Sprint 2 layers s3_supply_chain
# outputs on top; Sprint 3 adds the testing module's jumphost IPs.
# ============================================================


# ============================================================
# eks_cluster — Sprint 1 outputs (PRD 07)
# ============================================================

output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks_cluster.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API endpoint URL"
  value       = module.eks_cluster.cluster_endpoint
}

output "cluster_ca_data" {
  description = "EKS cluster CA cert (base64-encoded)"
  value       = module.eks_cluster.cluster_ca_data
  sensitive   = true
}

output "cluster_oidc_issuer_url" {
  description = "OIDC issuer URL (Sprint 2 IRSA input)"
  value       = module.eks_cluster.cluster_oidc_issuer_url
}

output "oidc_provider_arn" {
  description = "IAM OIDC provider ARN"
  value       = module.eks_cluster.oidc_provider_arn
}

output "node_group_role_arn" {
  description = "IAM role ARN for the self-managed node group"
  value       = module.eks_cluster.node_group_role_arn
}

output "cluster_ready_id" {
  description = "Empty-resource ID for downstream depends_on (carries through the roksbnkctl convention)"
  value       = module.eks_cluster.cluster_ready_id
}
