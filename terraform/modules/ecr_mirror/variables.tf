# ============================================================
# ecr_mirror — inputs (PRD 08 § "Implementation outline")
#
# v1.0 stretch module. PRD 08 § "Decision" lists this gated on
# var.enable_ecr_mirror; the root module wires `count` (Sprint 3
# follow-up) so the resources only materialise when the operator
# opts in. v1.0 Sprint 2 lands the module skeleton; the actual
# image enumeration + skopeo pipeline is tracked as a Sprint 3
# follow-up issue per the brief's deferral allowance.
# ============================================================

variable "region" {
  description = "AWS region; ECR is region-scoped."
  type        = string
}

variable "workspace_name" {
  description = "Workspace name; threaded into repository naming + tagging."
  type        = string
}

variable "enable_ecr_mirror" {
  description = "Master gate. Module emits nothing when false (default). When true, creates one ECR repository per image in var.images and runs skopeo via null_resource. v1.0 stretch (PRD 08)."
  type        = bool
  default     = false
}

variable "images" {
  description = "Source images to mirror, as <repo>:<tag> pairs. Empty = the module emits the repositories but skips the skopeo step (operator runs it out-of-band)."
  type        = list(string)
  default     = []
}

variable "source_authfile_path" {
  description = "Local path to a docker authfile (`skopeo login --authfile`) for the source registry. Empty = skopeo uses the ambient docker creds chain."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags applied to every ECR repository."
  type        = map(string)
  default     = {}
}
