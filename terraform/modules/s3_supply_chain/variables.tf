# ============================================================
# s3_supply_chain — inputs (PRD 08 § "Implementation outline")
# ============================================================

variable "region" {
  description = "AWS region for the supply-chain bucket + KMS CMK."
  type        = string
}

variable "workspace_name" {
  description = "Workspace name; threaded into bucket naming + tagging (PRD 08 § \"Decision\")."
  type        = string
}

variable "kms_key_arn" {
  description = "Existing CMK ARN for SSE-KMS. Empty creates a fresh CMK in this module."
  type        = string
  default     = ""
}

variable "far_auth_file_local_path" {
  description = "Local path to the FAR pull-key archive (f5cne-far-auth-*.tar.gz)."
  type        = string
}

variable "jwt_file_local_path" {
  description = "Local path to the subscription JWT (f5cne-subscription-*.jwt)."
  type        = string
}

variable "bucket_name_override" {
  description = "Override the generated bucket name. Empty = awsbnkctl-<workspace>-<random>."
  type        = string
  default     = ""
}

variable "far_auth_object_key" {
  description = "S3 key for the FAR archive. Defaults to far-auth.tar.gz."
  type        = string
  default     = "far-auth.tar.gz"
}

variable "jwt_object_key" {
  description = "S3 key for the subscription JWT. Defaults to subscription.jwt."
  type        = string
  default     = "subscription.jwt"
}

variable "tags" {
  description = "Additional tags applied to every resource. Module always adds awsbnkctl.io/managed-by + prd."
  type        = map(string)
  default     = {}
}
