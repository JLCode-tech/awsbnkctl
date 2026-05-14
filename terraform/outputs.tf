# ============================================================
# Root outputs (Sprint 0)
#
# Sprint 0 strips the legacy ROKS outputs (roks_cluster_id /
# transit_gateway_name / flo_trusted_profile_id / testing_*). The
# AWS-shaped equivalents (eks_cluster_name, cluster_endpoint, OIDC
# issuer URL, IRSA role ARNs, jumphost IPs against the new VPC) are
# re-emitted in Sprint 1 (EKS), Sprint 2 (S3 + IRSA), and Sprint 3
# (module port). Until then, only the eks_cluster stub outputs are
# surfaced so root-level `terraform output` is non-empty.
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

output "cluster_oidc_issuer_url" {
  description = "OIDC issuer URL (Sprint 2 IRSA input)"
  value       = module.eks_cluster.cluster_oidc_issuer_url
}

output "oidc_provider_arn" {
  description = "IAM OIDC provider ARN"
  value       = module.eks_cluster.oidc_provider_arn
}
