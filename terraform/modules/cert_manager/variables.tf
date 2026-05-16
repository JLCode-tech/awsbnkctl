# ============================================================
# cert_manager — outer module variables (Sprint 3 / PRD 04 + PRD 07)
#
# AWS retarget of the inherited roksbnkctl wrapper. The inner
# `./modules/cert-manager` body (helm install of cert-manager via
# local-exec) is unchanged — only the outer parameter boundary
# moves from IBM-shaped to AWS-shaped per PRD 00 § "Inheritance
# map" (ports unchanged).
# ============================================================

variable "aws_region" {
  description = "AWS region hosting the EKS cluster"
  type        = string
}

variable "eks_cluster_name" {
  description = "EKS cluster name (replaces roks_cluster_name_or_id from the inherited module)"
  type        = string

  validation {
    condition     = length(var.eks_cluster_name) > 0
    error_message = "eks_cluster_name cannot be empty — an existing EKS cluster is required."
  }
}

variable "cert_manager_namespace" {
  description = "Kubernetes namespace for cert-manager"
  type        = string
  default     = "cert-manager"
}

variable "cert_manager_version" {
  description = "cert-manager Helm chart version"
  type        = string
  default     = "v1.17.3"
}

variable "create_eks_cluster" {
  description = "When true, cluster is being created by eks_cluster — skip plan-time cluster credential fetch"
  type        = bool
  default     = false
}

variable "eks_cluster_dependency_id" {
  description = "eks_cluster sentinel ID — when set, defers runtime_config fetch to apply time after eks_cluster completes"
  type        = string
  default     = null
}

# Persistent dir kept in the AWS shape for symmetry with the inherited
# wrapper. Inner helm/kubectl local-exec writes nothing here; the
# variable persists so consumers calling the outer module with the
# v0.x `kubeconfig_dir` knob continue to compile.
variable "kubeconfig_dir" {
  description = "Persistent, writable dir reserved for local-exec scratch space. Inherited from the v0.x wrapper for source compatibility."
  type        = string
  default     = "/work/.bnk/scratch/kubeconfig/cert_manager"
}
