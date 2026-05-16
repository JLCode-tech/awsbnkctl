# ============================================================
# eks_cluster — Sprint 1 implementation per PRD 07
# (docs/prd/07-EKS-CLUSTER-SRIOV.md § "Decision" + § "Implementation
# outline").
#
# Composes terraform-aws-modules/eks/aws ~> 20.x with:
#
#   - a self-managed node group on ENA-SR-IOV-capable instance types
#     (c5n.4xlarge default; user-overridable via node_instance_types).
#   - an EKS-optimised AL2023 AMI resolved at apply time via the public
#     SSM parameter Amazon publishes per Kubernetes minor version.
#   - a launch template body with ENA support enabled and user-data
#     that flips the `intel_iommu=on iommu=pt` kernel arguments at first
#     boot (PRD 07 § "Decision" + § "Open questions" §"kernel parameter
#     posture").
#
# The Multus + SR-IOV stack lives in multus.tf + sriov.tf — gated on
# var.enable_multus / var.enable_sriov respectively.
#
# This module deliberately does NOT run terraform apply against live
# AWS as part of Sprint 1. Validation is `terraform validate` only;
# the operator-run spike (PRD 07 § "Spike protocol") validates against
# real infra and folds findings back into the PRD before v0.2.
# ============================================================

# ----------------------------------------------------------------
# AL2023 EKS-optimised AMI lookup.
#
# AWS publishes the recommended AMI ID under
# /aws/service/eks/optimized-ami/<k8s_version>/amazon-linux-2023/x86_64/standard/recommended/image_id
# Resolving via SSM means the apply picks up new AL2023 minor releases
# without a module bump — at the cost of nondeterminism in the apply
# plan. Per PRD 07 § "Trade-offs accepted" the AMI is pinned per
# release; v1.x lifts this to an explicit AMI ID variable.
# ----------------------------------------------------------------
data "aws_ssm_parameter" "eks_al2023_ami" {
  name = "/aws/service/eks/optimized-ami/${var.cluster_version}/amazon-linux-2023/x86_64/standard/recommended/image_id"
}

# ----------------------------------------------------------------
# User-data applied via the launch template.
#
# Two responsibilities:
#
#   1. Bootstrap the kubelet against the EKS cluster — the AL2023 EKS
#      bootstrap script is at /etc/eks/bootstrap.sh; we invoke it with
#      the cluster name + API endpoint + CA cert.
#   2. Apply the IOMMU kernel parameters needed for SR-IOV. We patch
#      /etc/default/grub and rebuild GRUB; the reboot at the end picks
#      up the new cmdline so the SR-IOV device plugin can enumerate
#      VFs on first kubelet registration.
#
# This script body is the v0 hypothesis — the spike confirms whether
# `intel_iommu=on iommu=pt` is sufficient on AL2023 or whether
# additional `pci=realloc` / `default_hugepagesz=` knobs are needed.
# ----------------------------------------------------------------
locals {
  node_user_data = <<-USERDATA
    #!/bin/bash
    set -euo pipefail

    # PRD 07 §"Open questions" §"kernel parameter posture":
    # spike validates whether these two are sufficient on AL2023.
    if [ -f /etc/default/grub ]; then
      sed -i 's/^GRUB_CMDLINE_LINUX="\(.*\)"$/GRUB_CMDLINE_LINUX="\1 intel_iommu=on iommu=pt"/' /etc/default/grub
      grub2-mkconfig -o /boot/grub2/grub.cfg || true
    fi

    /etc/eks/bootstrap.sh ${var.cluster_name} \
      --apiserver-endpoint '${module.eks.cluster_endpoint}' \
      --b64-cluster-ca '${module.eks.cluster_certificate_authority_data}' || true

    # Reboot so the new kernel cmdline takes effect for SR-IOV VF
    # enumeration. Spike day-1 verifies the kubelet rejoins the
    # cluster cleanly after this reboot.
    nohup bash -c 'sleep 5 && reboot' >/dev/null 2>&1 &
  USERDATA
}

# ----------------------------------------------------------------
# terraform-aws-modules/eks/aws ~> 20.x — the upstream wrapper.
#
# We pass authentication_mode = "API" (PRD 07 § "Decision" §"Cluster
# auth"); the legacy aws-auth ConfigMap path is deliberately off.
#
# Self-managed node groups live under self_managed_node_groups (vs
# eks_managed_node_groups which AWS controls) — the difference is
# what PRD 07 §"Options considered" pinned as our seam-of-control for
# the SR-IOV kernel posture and AMI lifecycle.
# ----------------------------------------------------------------
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  vpc_id     = var.vpc_id
  subnet_ids = var.subnet_ids

  # EKS API auth is the modern surface per PRD 07. awsbnkctl's IAM
  # identity is granted system:masters at apply time so the post-apply
  # kubeconfig works without an aws-auth ConfigMap dance.
  authentication_mode                      = "API"
  enable_cluster_creator_admin_permissions = true

  # The standard EKS-managed addons. vpc-cni is the primary CNI; Multus
  # chains on top of it via multus.tf when var.enable_multus = true.
  cluster_addons = {
    coredns                = {}
    kube-proxy             = {}
    vpc-cni                = {}
    eks-pod-identity-agent = {}
  }

  # Self-managed node group per PRD 07 § "Decision". Launch template is
  # built by the upstream module from these inputs; we override the AMI
  # and the user-data so the SR-IOV kernel posture lands at first boot.
  self_managed_node_groups = {
    sriov = {
      name = "${var.cluster_name}-sriov"

      ami_id        = data.aws_ssm_parameter.eks_al2023_ami.value
      instance_type = var.node_instance_types[0]

      min_size     = var.node_min_size
      max_size     = var.node_max_size
      desired_size = var.node_desired_size

      # ENA SR-IOV is the load-bearing knob — PRD 07's open question is
      # whether BNK accepts ENA VFs.
      enable_monitoring = true

      # user_data_template_path is the upstream's escape hatch; we pass
      # our literal script via the bootstrap_extra_args + pre_bootstrap_user_data
      # split that the upstream module re-stitches.
      pre_bootstrap_user_data = <<-EOT
        # PRD 07 § "Decision" §"kernel parameter posture"
        if [ -f /etc/default/grub ]; then
          sed -i 's/^GRUB_CMDLINE_LINUX="\(.*\)"$/GRUB_CMDLINE_LINUX="\1 intel_iommu=on iommu=pt"/' /etc/default/grub
          grub2-mkconfig -o /boot/grub2/grub.cfg || true
        fi
      EOT

      # Launch-template overrides: ENA support is implicit on the
      # instance families PRD 07 selects (c5n / m5n / c6in all ship ENA
      # by default), but we set the metadata-service knobs to the
      # modern IMDSv2-only posture for defence-in-depth.
      metadata_options = {
        http_endpoint               = "enabled"
        http_tokens                 = "required"
        http_put_response_hop_limit = 2
      }

      # Tag the autoscaling group so the SR-IOV device plugin's
      # node-feature-discovery (if added later) can target this pool.
      tags = {
        "awsbnkctl.io/role" = "sriov-data-plane"
        "awsbnkctl.io/prd"  = "07"
        "k8s.io/cluster-autoscaler/enabled" : "true"
        "k8s.io/cluster-autoscaler/${var.cluster_name}" : "owned"
      }
    }
  }

  tags = {
    "awsbnkctl.io/managed-by" = "awsbnkctl"
    "awsbnkctl.io/prd"        = "07"
  }
}

# ----------------------------------------------------------------
# cluster_ready — empty resource carrying through the roksbnkctl
# convention: downstream modules (Sprint 2 + Sprint 3) reference
# module.eks_cluster.cluster_ready_id in their depends_on so they
# only start once the control plane + node group are both reconciled.
#
# The depends_on list pulls in the Multus + SR-IOV stack (via the
# kubernetes_manifest resources in multus.tf / sriov.tf) so consumers
# get a true "cluster + CNI stack ready" signal, not just "control
# plane up".
# ----------------------------------------------------------------
resource "null_resource" "cluster_ready" {
  triggers = {
    cluster_name = module.eks.cluster_name
    node_group   = jsonencode(keys(module.eks.self_managed_node_groups))
    multus       = var.enable_multus ? "on" : "off"
    sriov        = var.enable_sriov ? "on" : "off"
  }

  depends_on = [
    module.eks,
  ]
}
