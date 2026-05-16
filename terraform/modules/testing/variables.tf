# ============================================================
# testing — outer module variables (Sprint 3 AWS retarget)
#
# Drops `roks_transit_gateway_name` (IBM-specific); replaces the
# client-VPC + transit-gateway pattern with an explicit
# aws_vpc_id + aws_subnet_ids the operator either supplies or
# reads from the eks_cluster module output. iperf3 + nginx
# fixtures (jumphost user_data) stay otherwise unchanged.
# ============================================================

variable "aws_region" {
  description = "AWS region for the test jumphost(s)"
  type        = string
}

variable "eks_cluster_name" {
  description = "EKS cluster name the jumphosts target"
  type        = string

  validation {
    condition     = length(var.eks_cluster_name) > 0
    error_message = "eks_cluster_name cannot be empty."
  }
}

variable "aws_vpc_id" {
  description = "VPC ID hosting the EKS cluster — jumphosts attach to subnets in this VPC"
  type        = string
}

variable "aws_subnet_ids" {
  description = "Subnet IDs (one per AZ) the jumphosts get placed in. >=1 required; HA jumphost-per-AZ uses len(aws_subnet_ids) instances."
  type        = list(string)
}

# ============================================================
# Feature Flags
# ============================================================

variable "testing_create_cluster_jumphosts" {
  description = "Create one jumphost per supplied subnet (one per AZ)"
  type        = bool
  default     = false
}

# ============================================================
# Shared Jumphost Configuration
# ============================================================

variable "testing_ssh_key_name" {
  description = "Name of an AWS-side EC2 key pair to inject into all jumphosts (empty = no SSH key, falls back to the generated jumphost_shared_key only)"
  type        = string
  default     = ""
}

variable "testing_jumphost_instance_type" {
  description = "EC2 instance type for jumphosts (e.g., t3.medium)"
  type        = string
  default     = "t3.medium"
}

variable "testing_cluster_jumphost_name_prefix" {
  description = "Name prefix for cluster jumphosts — subnet ID is appended (<prefix>-<subnet-id>)"
  type        = string
  default     = "tf-testing-jumphost-cluster"
}

variable "eks_cluster_dependency_id" {
  description = "eks_cluster sentinel ID — when set, defers VPC data-source reads to apply time after eks_cluster completes"
  type        = string
  default     = null
}

variable "create_eks_cluster" {
  description = "Set true when the EKS cluster is being created in this run — defers cluster-VPC-derived data sources"
  type        = bool
  default     = false
}
