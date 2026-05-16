# ============================================================
# flo — outer module variables (Sprint 3 AWS retarget)
#
# Retargeted onto S3 + IRSA per PRD 08. The inherited inner
# `./modules/flo` body (IBM-COS bucket download + IBM trusted
# profile creation) is preserved on disk per PRD 00 § "Inheritance
# map" but no longer called from this wrapper — the AWS path
# materialises the FLO helm values via the EKS-native kubernetes
# + helm providers, annotating the FLO ServiceAccount with the
# IRSA role ARN so the EKS pod-identity webhook injects
# AWS_WEB_IDENTITY_TOKEN_FILE in place of the v0.x
# IBMCLOUD_API_KEY env var.
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
# FAR / Registry Configuration
# ============================================================

variable "far_repo_url" {
  description = "FAR Repository URL for Docker and Helm registry"
  type        = string
  default     = "repo.f5.com"
}

variable "f5_bigip_k8s_manifest_version" {
  description = "Version of the f5-bigip-k8s-manifest chart (FLO/CIS versions extracted from this)"
  type        = string
  default     = "2.3.0-3.2598.3-0.0.170"
}

# ============================================================
# S3 supply-chain (replaces ibmcloud_cos_*; PRD 08)
# ============================================================

variable "s3_bucket_name" {
  description = "Name of the supply-chain S3 bucket holding the FAR archive + JWT (output of the s3_supply_chain module)"
  type        = string
}

variable "s3_far_auth_object_key" {
  description = "S3 key of the FAR pull-key archive (output of the s3_supply_chain module)"
  type        = string
}

variable "s3_jwt_object_key" {
  description = "S3 key of the subscription JWT (output of the s3_supply_chain module)"
  type        = string
}

# ============================================================
# IRSA wiring (replaces flo_trusted_profile_id; PRD 08)
# ============================================================

variable "flo_irsa_role_arn" {
  description = "IAM role ARN the FLO ServiceAccount is annotated with (output of the iam_irsa module)"
  type        = string
}

# ============================================================
# Namespace + readiness gating
# ============================================================

variable "flo_namespace" {
  description = "Namespace for the F5 Lifecycle Operator"
  type        = string
  default     = "flo-system"
}

variable "flo_utils_namespace" {
  description = "Namespace for F5 utility components"
  type        = string
  default     = "f5-utils"
}

variable "flo_service_account_name" {
  description = "FLO ServiceAccount name (IRSA-annotated)"
  type        = string
  default     = "flo-controller"
}

variable "cert_manager_namespace" {
  description = "Kubernetes namespace where cert-manager is installed (PRD 07 cert_manager output)"
  type        = string
  default     = "cert-manager"
}

variable "create_eks_cluster" {
  description = "When true, the EKS cluster is being created in the same apply — defers credential fetch to apply time"
  type        = bool
  default     = false
}

variable "eks_cluster_dependency_id" {
  description = "eks_cluster sentinel ID — defer FLO until eks_cluster completes"
  type        = string
  default     = null
}

variable "cert_manager_dependency_id" {
  description = "cert_manager ready sentinel ID — blocks FLO until cert-manager CRDs are registered"
  type        = string
  default     = null
}

variable "deploy_bnk" {
  description = "Deploy BIG-IP Next for Kubernetes; when false no FLO resources are created"
  type        = bool
  default     = true
}

# ============================================================
# Inherited scratch dir knobs (kept for v0.x source compat)
# ============================================================

variable "kubeconfig_dir" {
  description = "Reserved scratch dir for any future local-exec staging."
  type        = string
  default     = "/work/.bnk/scratch/kubeconfig/flo"
}

variable "scratch_dir" {
  description = "Reserved scratch dir; carried for v0.x source compatibility."
  type        = string
  default     = "/work/.bnk/scratch"
}
