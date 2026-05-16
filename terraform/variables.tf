# ============================================================
# Root Terraform Variables (Sprint 1)
#
# Sprint 0 stripped the IBM-Cloud variables and seeded the AWS-shaped
# inputs per PRD 07 § "Inputs" (docs/prd/07-EKS-CLUSTER-SRIOV.md).
# Sprint 1 adds the Multus / SR-IOV gating + resource-name variables
# matching the eks_cluster module surface. Module-specific variables
# (cert_manager_namespace, far_repo_url, etc.) stay stripped until
# their respective sprints (Sprint 2 = supply chain, Sprint 3 = module
# port).
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

variable "enable_multus" {
  description = "Install upstream Multus CNI DaemonSet (default true). See PRD 07 §\"Multus + SR-IOV stack\"."
  type        = bool
  default     = true
}

variable "enable_sriov" {
  description = "Install upstream SR-IOV CNI + device plugin DaemonSets (default true). See PRD 07 §\"Multus + SR-IOV stack\"."
  type        = bool
  default     = true
}

variable "sriov_resource_name" {
  description = "Resource key the SR-IOV device plugin advertises VFs under (default intel.com/sriov per PRD 07)."
  type        = string
  default     = "intel.com/sriov"
}


# ============================================================
# s3_supply_chain + iam_irsa — Sprint 2 inputs (PRD 08)
# ============================================================

variable "workspace_name" {
  description = "awsbnkctl workspace name; threaded into the s3_supply_chain bucket name + iam_irsa role name for inspectability. PRD 08 § \"Decision\"."
  type        = string
  default     = "default"
}

variable "kms_key_arn" {
  description = "Existing customer-managed KMS CMK for the supply-chain bucket. Empty creates one in-module (PRD 08 § \"Trade-offs accepted\")."
  type        = string
  default     = ""
}

variable "far_auth_file_local_path" {
  description = "Local path to the FAR pull-key archive (f5cne-far-auth-*.tar.gz). Supplied by `awsbnkctl init`."
  type        = string
}

variable "jwt_file_local_path" {
  description = "Local path to the subscription JWT (f5cne-subscription-*.jwt). Supplied by `awsbnkctl init`."
  type        = string
}

variable "flo_namespace" {
  description = "Kubernetes namespace where the FLO service account lives. PRD 08 default flo-system."
  type        = string
  default     = "flo-system"
}


# ============================================================
# ecr_mirror — Sprint 2 scaffold gate (PRD 08 v1.0 stretch)
# ============================================================

variable "enable_ecr_mirror" {
  description = "Enable the optional ECR mirror module (PRD 08 v1.0 stretch). Default false; the module is a no-op when disabled. Set true to mirror FAR images into ECR for air-gapped deployments."
  type        = bool
  default     = false
}


# ============================================================
# Sprint 3 — port-the-five-modules inputs
# (cert_manager, flo, cne_instance, license, testing)
# ============================================================

variable "cert_manager_namespace" {
  description = "Kubernetes namespace where cert-manager is installed. Default 'cert-manager'."
  type        = string
  default     = "cert-manager"
}

variable "cert_manager_version" {
  description = "Helm chart version for cert-manager. Default per Sprint 3 module."
  type        = string
  default     = "v1.17.3"
}

variable "far_repo_url" {
  description = "FAR Repository URL for Docker and Helm registry. Default repo.f5.com."
  type        = string
  default     = "repo.f5.com"
}

variable "f5_bigip_k8s_manifest_version" {
  description = "Version of the f5-bigip-k8s-manifest chart (FLO + CIS versions are extracted from this)."
  type        = string
  default     = "2.3.0-3.2598.3-0.0.170"
}

variable "flo_utils_namespace" {
  description = "Kubernetes namespace where the F5 utilities (License CR, etc.) live. Default 'f5-utils'."
  type        = string
  default     = "f5-utils"
}

variable "cneinstance_deployment_size" {
  description = "CNEInstance deployment size (Small | Medium | Large)."
  type        = string
  default     = "Small"
}

variable "cneinstance_gslb_datacenter_name" {
  description = "GSLB datacenter name for CNEInstance (empty = no GSLB)."
  type        = string
  default     = ""
}

variable "license_mode" {
  description = "License operation mode (connected | disconnected)."
  type        = string
  default     = "connected"
}

variable "deploy_bnk" {
  description = "Deploy BIG-IP Next for Kubernetes (FLO + CNEInstance + License). Default true."
  type        = bool
  default     = true
}

variable "testing_create_cluster_jumphosts" {
  description = "Provision the testing-fixture jumphosts (one per supplied subnet). Default false."
  type        = bool
  default     = false
}

variable "testing_ssh_key_name" {
  description = "AWS-side EC2 key pair name to inject into the testing jumphosts."
  type        = string
  default     = ""
}

variable "testing_jumphost_instance_type" {
  description = "EC2 instance type for the testing jumphosts."
  type        = string
  default     = "t3.medium"
}
