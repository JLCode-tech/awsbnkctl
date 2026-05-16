# ============================================================
# flo — outputs (Sprint 3 AWS retarget)
# ============================================================

output "flo_namespace" {
  description = "Namespace where the FLO ServiceAccount is provisioned"
  value       = var.flo_namespace
}

output "flo_utils_namespace" {
  description = "Namespace where FLO utility components will be installed (Sprint 4 helm_release)"
  value       = var.flo_utils_namespace
}

output "flo_service_account_name" {
  description = "FLO ServiceAccount name (annotated with the IRSA role ARN)"
  value       = var.flo_service_account_name
}

output "flo_irsa_role_arn" {
  description = "IRSA role ARN bound to the FLO ServiceAccount (echoed for downstream depends_on)"
  value       = var.flo_irsa_role_arn
}

output "flo_helm_values" {
  description = "Rendered FLO helm-values map (Sprint 4 helm_release consumes this)"
  value       = local.flo_helm_values
}

output "flo_ready_id" {
  description = "Sentinel ID for apply-time ordering — (known after apply) on every apply"
  value       = null_resource.flo_ready.id
}

output "cneinstance_network_attachments" {
  description = "Carries through the inherited convention so cne_instance can read the NAD names FLO advertised. Sprint 3 stubs to the default ipvlan + macvlan pair pending Sprint 4 helm_release wiring."
  value       = ["ens5-ipvlan-l2", "macvlan-conf"]
}
