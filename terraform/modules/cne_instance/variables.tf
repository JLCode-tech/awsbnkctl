# ============================================================
# cne_instance — outer module variables (Sprint 3 AWS retarget)
#
# Rename map per the Sprint 3 staff brief:
#   roks_cluster_name_or_id    → eks_cluster_name
#   flo_trusted_profile_id     → flo_irsa_role_arn
#   ibmcloud_*                 → aws_*  (only region carries through)
#
# The inherited inner `./modules/cneinstance` body remains on disk
# per PRD 00 § "Inheritance map" (ports unchanged). Sprint 3
# preserves the CNEInstance CRD body unchanged; only the parameter
# boundary moves to AWS.
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

# ============================================================
# FLO Namespace Configuration
# ============================================================

variable "flo_namespace" {
  description = "Namespace for F5 Lifecycle Operator"
  type        = string
  default     = "flo-system"
}

variable "flo_utils_namespace" {
  description = "Namespace for F5 utility components"
  type        = string
  default     = "f5-utils"
}

variable "f5_bigip_k8s_manifest_version" {
  description = "Version of f5-bigip-k8s-manifest chart"
  type        = string
  default     = "2.3.0-3.2598.3-0.0.170"
}

variable "flo_irsa_role_arn" {
  description = "IAM role ARN for the FLO IRSA service account (replaces flo_trusted_profile_id)"
  type        = string
  default     = ""
}

variable "flo_cluster_issuer_name" {
  description = "mTLS certificate issuer name"
  type        = string
  default     = ""
}

# ============================================================
# CNEInstance Configuration
# ============================================================

variable "cneinstance_deployment_size" {
  description = "Deployment size for CNEInstance (Small, Medium, Large)"
  type        = string
  default     = "Small"
}

variable "cneinstance_gslb_datacenter_name" {
  description = "GSLB datacenter name for CNEInstance (optional)"
  type        = string
  default     = ""
}

variable "cneinstance_network_attachments" {
  description = "Multus Network Attachment Definitions for CNEInstance TMM deployments"
  type        = list(string)
  default     = ["ens5-ipvlan-l2", "macvlan-conf"]
}

variable "create_eks_cluster" {
  description = "When true, the EKS cluster is being created in the same apply — defers credential fetch to apply time"
  type        = bool
  default     = false
}

variable "eks_cluster_dependency_id" {
  description = "eks_cluster sentinel ID — when set, defers runtime_config fetch to apply time after eks_cluster completes"
  type        = string
  default     = null
}

variable "flo_dependency_id" {
  description = "flo_ready sentinel ID — defer cne_instance until flo completes and CRDs are registered"
  type        = string
  default     = null
}

variable "deploy_bnk" {
  description = "Deploy BIG-IP Next for Kubernetes; when false no CNEInstance resources are created"
  type        = bool
  default     = true
}

variable "kubeconfig_dir" {
  description = "Reserved scratch dir for any future local-exec staging."
  type        = string
  default     = "/work/.bnk/scratch/kubeconfig/cne_instance"
}
