# ============================================================
# ecr_mirror — outputs
# ============================================================

output "repository_uris" {
  description = "Map of source image (repo:tag) to mirrored ECR repository URI. Empty when enable_ecr_mirror = false."
  value       = { for k, r in aws_ecr_repository.mirror : k => r.repository_url }
}

output "repository_names" {
  description = "Map of source image to ECR repository name."
  value       = { for k, r in aws_ecr_repository.mirror : k => r.name }
}
