# ============================================================
# cne_instance — outputs (Sprint 3 AWS retarget)
# ============================================================

output "cneinstance_namespace" {
  description = "Namespace where CNEInstance is deployed"
  value       = var.flo_namespace
}

output "cneinstance_values" {
  description = "Rendered CNEInstance helm-values map (Sprint 4 helm_release consumes this)"
  value       = local.cneinstance_values
}

output "cneinstance_ready_id" {
  description = "Sentinel ID for apply-time ordering — (known after apply) on every apply"
  value       = null_resource.cneinstance_ready.id
}
