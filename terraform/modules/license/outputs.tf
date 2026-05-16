# ============================================================
# license — outputs (Sprint 3 AWS retarget)
# ============================================================

output "license_namespace" {
  description = "Namespace where the License CR will be deployed"
  value       = var.flo_utils_namespace
}

output "license_jwt_s3_url" {
  description = "S3 URL the FLO pod will fetch via IRSA (s3://<bucket>/<key>)"
  value       = local.license_jwt_s3_url
}

output "license_values" {
  description = "Rendered License-CR values map (Sprint 4 kubernetes_manifest consumes this)"
  value       = local.license_values
}

output "license_ready_id" {
  description = "Sentinel ID for apply-time ordering — (known after apply) on every apply"
  value       = null_resource.license_ready.id
}
