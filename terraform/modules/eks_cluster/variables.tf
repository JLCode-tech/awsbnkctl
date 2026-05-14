# ============================================================
# eks_cluster — Sprint 0 placeholder inputs
#
# Schema sourced from PRD 07
# (docs/prd/07-EKS-CLUSTER-SRIOV.md § "Inputs"). The Sprint 0 stub
# accepts these so the root module's call site is the same shape it
# will be in Sprint 1 — only the implementation body changes.
# ============================================================

variable "region" {
  description = "AWS region for the EKS cluster"
  type        = string
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
}

variable "cluster_version" {
  description = "Kubernetes version (default 1.30 per PRD 07)"
  type        = string
  default     = "1.30"
}

variable "vpc_id" {
  description = "VPC ID hosting the cluster; set create_vpc = true (Sprint 1) to provision a fresh one"
  type        = string
}

variable "subnet_ids" {
  description = "Private subnet IDs (>=3 AZs per PRD 07)"
  type        = list(string)
}

variable "node_instance_types" {
  description = "Self-managed node group instance types (default c5n.4xlarge per PRD 07)"
  type        = list(string)
  default     = ["c5n.4xlarge"]
}

variable "node_min_size" {
  description = "Node group minimum size"
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Node group maximum size"
  type        = number
  default     = 10
}

variable "node_desired_size" {
  description = "Node group initial size"
  type        = number
  default     = 3
}
