# ============================================================
# iam_irsa — inputs (PRD 08 § "Implementation outline")
# ============================================================

variable "region" {
  description = "AWS region (for tagging + resource names)."
  type        = string
}

variable "cluster_name" {
  description = "EKS cluster name; threaded into the IAM role name for inspectability."
  type        = string
}

variable "oidc_provider_arn" {
  description = "OIDC provider ARN from the eks_cluster module (PRD 07 outputs)."
  type        = string
}

variable "cluster_oidc_issuer_url" {
  description = "OIDC issuer URL from the eks_cluster module (PRD 07 outputs). The trust policy's StringEquals condition is keyed on the issuer hostname (no scheme)."
  type        = string
}

variable "flo_namespace" {
  description = "Kubernetes namespace where the FLO service account lives."
  type        = string
  default     = "flo-system"
}

variable "flo_service_account_name" {
  description = "FLO service account name."
  type        = string
  default     = "flo-controller"
}

variable "s3_bucket_arn" {
  description = "Supply-chain bucket ARN (from the s3_supply_chain module). The role's permission policy grants s3:GetObject on <arn>/*."
  type        = string
}

variable "kms_key_arn" {
  description = "Supply-chain CMK ARN (from the s3_supply_chain module). The role's permission policy grants kms:Decrypt on this key so encrypted-at-rest objects can be read."
  type        = string
}

variable "role_name_override" {
  description = "Override the generated IAM role name. Empty = awsbnkctl-<cluster>-flo-supply-chain-reader."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags applied to the IAM role + policy."
  type        = map(string)
  default     = {}
}
