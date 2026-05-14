# ============================================================
# Root Terraform Variables (Sprint 0)
#
# Sprint 0 strips the IBM-Cloud variables and seeds the AWS-shaped
# inputs per PRD 07's input table
# (docs/prd/07-EKS-CLUSTER-SRIOV.md § "Inputs"). Module-specific
# variables (cert_manager_namespace, far_repo_url, etc.) stay
# stripped until their respective sprints (Sprint 2 = supply chain,
# Sprint 3 = module port).
# ============================================================


# ============================================================
# eks_cluster — Sprint 1 inputs (PRD 07)
# ============================================================

variable "region" {
  description = "AWS region for the EKS cluster (e.g. us-east-1)"
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
  description = "VPC ID hosting the cluster (Sprint 1 v1.x adds create_vpc = true to provision a fresh one)"
  type        = string
}

variable "subnet_ids" {
  description = "Private subnet IDs (>=3 AZs per PRD 07 HA target)"
  type        = list(string)
}

variable "node_instance_types" {
  description = "Self-managed node group instance types (default c5n.4xlarge per PRD 07 SR-IOV target)"
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
