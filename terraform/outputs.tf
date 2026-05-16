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


# ============================================================
# s3_supply_chain — Sprint 2 outputs (PRD 08)
# ============================================================

output "supply_chain_bucket_name" {
  description = "Supply-chain S3 bucket name (FAR archive + JWT)."
  value       = module.s3_supply_chain.bucket_name
}

output "supply_chain_bucket_arn" {
  description = "Supply-chain S3 bucket ARN."
  value       = module.s3_supply_chain.bucket_arn
}

output "supply_chain_far_auth_key" {
  description = "S3 key of the FAR archive object."
  value       = module.s3_supply_chain.far_auth_object_key
}

output "supply_chain_jwt_key" {
  description = "S3 key of the subscription JWT object."
  value       = module.s3_supply_chain.jwt_object_key
}

output "supply_chain_kms_key_arn" {
  description = "KMS CMK ARN used for SSE-KMS on the supply-chain bucket."
  value       = module.s3_supply_chain.kms_key_arn
}


# ============================================================
# iam_irsa — Sprint 2 outputs (PRD 08)
# ============================================================

output "flo_irsa_role_arn" {
  description = "IAM role ARN to annotate on FLO's ServiceAccount as eks.amazonaws.com/role-arn."
  value       = module.iam_irsa.flo_role_arn
}

output "flo_irsa_role_name" {
  description = "FLO IRSA role name (inspectable side of flo_irsa_role_arn)."
  value       = module.iam_irsa.flo_role_name
}


# ============================================================
# ecr_mirror — Sprint 2 outputs (PRD 08 v1.0 stretch)
# ============================================================

output "ecr_mirror_repository_uris" {
  description = "Map of source image to ECR repository URI (empty when enable_ecr_mirror = false)."
  value       = module.ecr_mirror.repository_uris
}


# ============================================================
# Sprint 3 — full lifecycle outputs
# ============================================================

output "cert_manager_namespace" {
  description = "Namespace where cert-manager is deployed."
  value       = module.cert_manager.cert_manager_namespace
}

output "cert_manager_ready_id" {
  description = "Sentinel ID; (known after apply) until cert-manager CRDs are registered."
  value       = module.cert_manager.cert_manager_ready_id
}

output "flo_namespace" {
  description = "Namespace where FLO is provisioned."
  value       = module.flo.flo_namespace
}

output "flo_ready_id" {
  description = "Sentinel ID; (known after apply) once FLO IRSA ServiceAccount is ready."
  value       = module.flo.flo_ready_id
}

output "cneinstance_ready_id" {
  description = "Sentinel ID; (known after apply) once CNEInstance values are rendered."
  value       = module.cne_instance.cneinstance_ready_id
}

output "license_namespace" {
  description = "Namespace where the License CR will be deployed."
  value       = module.license.license_namespace
}

output "license_jwt_s3_url" {
  description = "S3 URL the FLO IRSA-bound pod fetches the JWT from."
  value       = module.license.license_jwt_s3_url
}

output "testing_jumphost_public_ips" {
  description = "Map of subnet id to public IP for the testing jumphosts."
  value       = module.testing.testing_cluster_jumphost_public_ips
}
