# ============================================================
# eks_cluster — Sprint 1 inputs (PRD 07)
#
# Schema sourced from PRD 07 § "Inputs"
# (docs/prd/07-EKS-CLUSTER-SRIOV.md). The full input contract is below;
# the Sprint 1 module body composes terraform-aws-modules/eks/aws ~> 20.x
# plus a self-managed launch template + Multus / SR-IOV manifests.
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

variable "enable_multus" {
  description = "Install upstream Multus CNI DaemonSet (k8snetworkplumbingwg/multus-cni v4.x thick plugin). Required for SR-IOV chaining; default true per PRD 07."
  type        = bool
  default     = true
}

variable "enable_sriov" {
  description = "Install upstream SR-IOV CNI + SR-IOV device plugin DaemonSets. Default true per PRD 07; turn off only if the spike surfaces a hypothesis mismatch."
  type        = bool
  default     = true
}

variable "sriov_resource_name" {
  description = "Resource key the SR-IOV device plugin advertises VFs under (default intel.com/sriov per PRD 07; BNK's CNEInstance reconciler expects this key)."
  type        = string
  default     = "intel.com/sriov"
}
