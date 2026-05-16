# ============================================================
# license — outer module variables (Sprint 3 AWS retarget)
#
# Pulls the JWT from S3 (via the IRSA-bound FLO role) instead of
# IBM COS. The inherited inner `./modules/license` body is
# preserved on disk per PRD 00 § "Inheritance map" but the outer
# wrapper renders the License CR with the AWS-shaped JWT URL.
# ============================================================

variable "aws_region" {
  description = "AWS region hosting the EKS cluster"
  type        = string
}

variable "eks_cluster_name" {
  description = "EKS cluster name (replaces roks_cluster_name_or_id)"
  type        = string

  validation {
    condition     = length(var.eks_cluster_name) > 0
    error_message = "eks_cluster_name cannot be empty — an existing EKS cluster is required."
  }
}

# ============================================================
# S3 supply-chain (replaces ibmcloud_cos_*; PRD 08)
# ============================================================

variable "s3_bucket_name" {
  description = "Supply-chain S3 bucket holding the subscription JWT (output of s3_supply_chain)"
  type        = string
}

variable "s3_jwt_object_key" {
  description = "S3 key of the subscription JWT (output of s3_supply_chain)"
  type        = string
}

variable "flo_irsa_role_arn" {
  description = "FLO IRSA role ARN — the FLO pod assumes this role to GetObject the JWT from S3 via a presigned URL"
  type        = string
  default     = ""
}

# ============================================================
# Namespace + license metadata
# ============================================================

variable "flo_utils_namespace" {
  description = "Namespace for F5 utility components (License CR lives here)"
  type        = string
  default     = "f5-utils"
}

variable "license_mode" {
  description = "License operation mode (connected or disconnected)"
  type        = string
  default     = "connected"
}

variable "create_eks_cluster" {
  description = "When true, the EKS cluster is being created in the same apply — defers credential fetch to apply time"
  type        = bool
  default     = false
}

variable "eks_cluster_dependency_id" {
  description = "eks_cluster sentinel ID — when set, defers credential fetch to apply time"
  type        = string
  default     = null
}

variable "cneinstance_dependency_id" {
  description = "cneinstance_ready_id — ensures License CRD is available before applying License CR"
  type        = string
  default     = null
}

variable "deploy_bnk" {
  description = "Deploy BIG-IP Next for Kubernetes; when false no License resources are created"
  type        = bool
  default     = true
}

variable "kubeconfig_dir" {
  description = "Reserved scratch dir for any future local-exec staging."
  type        = string
  default     = "/work/.bnk/scratch/kubeconfig/license"
}
