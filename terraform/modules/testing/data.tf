# ============================================================
# testing — outer data sources (Sprint 3 AWS retarget)
#
# Resolves the latest Ubuntu 22.04 AMI Canonical publishes (mirrors
# the inherited Ubuntu-22.04 minimal pin). Single SSH-key lookup
# kept symmetric with the v0.x module.
# ============================================================

data "aws_ami" "ubuntu_22_04" {
  count       = var.testing_create_cluster_jumphosts ? 1 : 0
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_vpc" "cluster_vpc" {
  count      = var.testing_create_cluster_jumphosts && !var.create_eks_cluster ? 1 : 0
  id         = var.aws_vpc_id
  depends_on = [null_resource.eks_cluster_gate]
}
