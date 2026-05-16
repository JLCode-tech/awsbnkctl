# ============================================================
# testing — outer module body (Sprint 3 AWS retarget)
#
# Drops the IBM transit-gateway jumphost shape; AWS jumphosts live
# directly in the cluster VPC (one per supplied subnet). iperf3 +
# nginx fixtures in the user_data script otherwise unchanged.
# Provisions:
#
#   - a per-apply shared SSH key pair (TLS-generated, embedded in
#     each jumphost's authorized_keys so jumphosts can SSH each
#     other passwordlessly — same convention as the v0.x module)
#   - one security group permitting SSH inbound from the world +
#     all outbound
#   - one EC2 instance per supplied subnet (HA: one per AZ)
#   - one Elastic IP per instance for ingress
#
# user_data installs the testing toolchain (iperf3, kubectl, helm,
# awscli, nginx) and pulls the EKS kubeconfig via
# `aws eks update-kubeconfig`.
# ============================================================

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.0.0"
    }
  }
}

locals {
  jumphost_user_data = <<-EOF
    #!/bin/bash
    set -e

    mkdir -p /home/ubuntu/.ssh /root/.ssh
    chmod 700 /home/ubuntu/.ssh /root/.ssh
    echo "${trimspace(tls_private_key.jumphost_shared_key.public_key_openssh)}" >> /home/ubuntu/.ssh/authorized_keys
    echo "${trimspace(tls_private_key.jumphost_shared_key.public_key_openssh)}" >> /root/.ssh/authorized_keys
    chmod 600 /home/ubuntu/.ssh/authorized_keys /root/.ssh/authorized_keys
    chown ubuntu:ubuntu /home/ubuntu/.ssh /home/ubuntu/.ssh/authorized_keys

    apt-get update
    apt-get install -y curl wget gnupg lsb-release software-properties-common \
        iperf3 dnsutils net-tools netcat-openbsd nginx unzip

    # AWS CLI v2
    curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
    unzip -q awscliv2.zip
    ./aws/install
    rm -rf aws awscliv2.zip

    # kubectl
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl

    # helm
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

    # Wire kubeconfig from the EKS cluster (relies on the EC2 instance role
    # carrying eks:DescribeCluster against the named cluster, or AWS creds
    # supplied via instance metadata).
    mkdir -p /root/.kube
    aws eks update-kubeconfig --region "${var.aws_region}" --name "${var.eks_cluster_name}" || true
    if [ -f /root/.kube/config ]; then
      chmod 600 /root/.kube/config
      mkdir -p /home/ubuntu/.kube
      cp /root/.kube/config /home/ubuntu/.kube/config
      chown -R ubuntu:ubuntu /home/ubuntu/.kube
      chmod 600 /home/ubuntu/.kube/config
    fi

    # Shared private key — written to both root and ubuntu so the
    # passwordless inter-jumphost SSH pattern works either side.
    echo "${base64encode(tls_private_key.jumphost_shared_key.private_key_openssh)}" | base64 -d > /home/ubuntu/.ssh/id_rsa
    cp /home/ubuntu/.ssh/id_rsa /root/.ssh/id_rsa
    chmod 600 /home/ubuntu/.ssh/id_rsa /root/.ssh/id_rsa
    chown ubuntu:ubuntu /home/ubuntu/.ssh/id_rsa

    echo "${trimspace(tls_private_key.jumphost_shared_key.public_key_openssh)}" > /home/ubuntu/.ssh/id_rsa.pub
    echo "${trimspace(tls_private_key.jumphost_shared_key.public_key_openssh)}" > /root/.ssh/id_rsa.pub
    chmod 644 /home/ubuntu/.ssh/id_rsa.pub /root/.ssh/id_rsa.pub
    chown ubuntu:ubuntu /home/ubuntu/.ssh/id_rsa.pub

    # nginx default page — confirms the jumphost is up + reachable on :80
    echo "awsbnkctl testing jumphost ready at $(date)" > /var/www/html/index.html
    systemctl enable nginx
    systemctl restart nginx

    echo "Setup completed at $(date)" > /var/log/jumphost-setup.log
    EOF
}

# Shared SSH key (one per apply, embedded in every jumphost)
resource "tls_private_key" "jumphost_shared_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Security group permitting SSH inbound + all outbound + nginx :80 inbound
resource "aws_security_group" "cluster_jumphost_sg" {
  count       = var.testing_create_cluster_jumphosts ? 1 : 0
  name_prefix = "${var.testing_cluster_jumphost_name_prefix}-sg-"
  description = "Cluster jumphost SG (Sprint 3; SSH + nginx :80)"
  vpc_id      = var.aws_vpc_id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "SSH"
  }
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "nginx (testing fixture)"
  }
  ingress {
    from_port   = 5201
    to_port     = 5201
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "iperf3 (testing fixture)"
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "all outbound"
  }

  tags = {
    "awsbnkctl.io/managed-by" = "awsbnkctl"
    "awsbnkctl.io/role"       = "testing-jumphost"
  }
}

# One jumphost per supplied subnet
resource "aws_instance" "cluster_jumphost" {
  for_each = var.testing_create_cluster_jumphosts ? toset(var.aws_subnet_ids) : toset([])

  ami                         = data.aws_ami.ubuntu_22_04[0].id
  instance_type               = var.testing_jumphost_instance_type
  subnet_id                   = each.key
  vpc_security_group_ids      = [aws_security_group.cluster_jumphost_sg[0].id]
  associate_public_ip_address = true
  key_name                    = var.testing_ssh_key_name != "" ? var.testing_ssh_key_name : null
  user_data                   = local.jumphost_user_data

  tags = {
    Name                      = "${var.testing_cluster_jumphost_name_prefix}-${substr(each.key, 0, 12)}"
    "awsbnkctl.io/managed-by" = "awsbnkctl"
    "awsbnkctl.io/role"       = "testing-jumphost"
  }

  depends_on = [null_resource.eks_cluster_gate]
}

# Elastic IP per jumphost (for stable public-facing testing URLs)
resource "aws_eip" "cluster_jumphost_eip" {
  for_each = aws_instance.cluster_jumphost

  instance = each.value.id
  domain   = "vpc"

  tags = {
    Name = "${var.testing_cluster_jumphost_name_prefix}-${substr(each.key, 0, 12)}-eip"
  }
}
