# ============================================================
# eks_cluster — Sprint 1 outputs (PRD 07)
#
# Output shape sourced from PRD 07 § "Outputs"
# (docs/prd/07-EKS-CLUSTER-SRIOV.md). Downstream modules (Sprint 2's
# s3_supply_chain + iam_irsa, Sprint 3's cert_manager / flo /
# cne_instance) consume cluster_oidc_issuer_url + oidc_provider_arn +
# cluster_ready_id.
# ============================================================

output "cluster_name" {
  description = "EKS cluster name (echoed for downstream depends_on convenience)"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API endpoint URL"
  value       = module.eks.cluster_endpoint
}

output "cluster_ca_data" {
  description = "EKS cluster CA cert (base64-encoded)"
  value       = module.eks.cluster_certificate_authority_data
}

output "cluster_oidc_issuer_url" {
  description = "OIDC issuer URL for IRSA (Sprint 2 input)"
  value       = module.eks.cluster_oidc_issuer_url
}

output "oidc_provider_arn" {
  description = "IAM OIDC provider ARN (Sprint 2 IRSA wiring consumes this)"
  value       = module.eks.oidc_provider_arn
}

output "node_group_role_arn" {
  description = "IAM role ARN attached to the self-managed node group"
  value       = try(values(module.eks.self_managed_node_groups)[0].iam_role_arn, "")
}

output "cluster_ready_id" {
  description = "Empty-resource ID for downstream depends_on (carries through the roksbnkctl convention)"
  value       = null_resource.cluster_ready.id
}
