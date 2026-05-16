# ============================================================
# flo — outer module body (Sprint 3 AWS retarget)
#
# Provisions the FLO ServiceAccount + namespace + IRSA-annotated
# SA in the EKS cluster. The FLO helm release proper (the FAR
# image pull, the f5-bigip-k8s-manifest chart, the CIS controller
# install) stays out of this validate-only sprint — the inner
# `./modules/flo/modules/flo` body that handles the helm install
# is preserved on disk per PRD 00 § "Inheritance map" but is not
# called from this wrapper. Sprint 4 wires the helm_release once
# the operator-run spike validates the EKS + Multus + SR-IOV path
# (PRD 07 § "Spike protocol").
#
# Sprint 3 deliverable per the brief:
#
#   - render FLO helm values with S3 URLs in place of COS endpoints
#   - replace COS-auth env vars with IRSA-auto-injected
#     AWS_WEB_IDENTITY_TOKEN_FILE references
#
# The kubernetes_namespace + kubernetes_service_account resources
# below carry the IRSA wiring; helm-side rendering of S3 URLs is
# parameterised via locals so a Sprint 4 helm_release can consume
# them unchanged.
# ============================================================

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.30"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
    time = {
      source  = "hashicorp/time"
      version = ">= 0.9.0"
    }
  }
}

locals {
  # FLO helm values templating seam — Sprint 4 wires this map into
  # a helm_release. The S3 URLs replace the v0.x COS endpoints; the
  # FLO pod consults these via the IRSA-injected
  # AWS_WEB_IDENTITY_TOKEN_FILE env var to assume the IRSA role and
  # GetObject the keys from S3.
  flo_helm_values = {
    far_repo_url    = var.far_repo_url
    manifest_version = var.f5_bigip_k8s_manifest_version
    namespace       = var.flo_namespace
    utils_namespace = var.flo_utils_namespace

    # PRD 08 § "Decision" — supply chain via S3 + IRSA.
    s3_bucket_name        = var.s3_bucket_name
    s3_far_auth_object    = var.s3_far_auth_object_key
    s3_jwt_object         = var.s3_jwt_object_key
    s3_far_auth_url       = "s3://${var.s3_bucket_name}/${var.s3_far_auth_object_key}"
    s3_jwt_url            = "s3://${var.s3_bucket_name}/${var.s3_jwt_object_key}"

    irsa_role_arn          = var.flo_irsa_role_arn
    service_account_name   = var.flo_service_account_name
    cert_manager_namespace = var.cert_manager_namespace
  }
}

resource "kubernetes_namespace" "flo" {
  count = var.deploy_bnk ? 1 : 0

  metadata {
    name = var.flo_namespace
    labels = {
      "awsbnkctl.io/managed-by" = "awsbnkctl"
      "awsbnkctl.io/prd"        = "08"
    }
  }

  depends_on = [null_resource.eks_cluster_gate]
}

resource "kubernetes_namespace" "flo_utils" {
  count = var.deploy_bnk ? 1 : 0

  metadata {
    name = var.flo_utils_namespace
    labels = {
      "awsbnkctl.io/managed-by" = "awsbnkctl"
      "awsbnkctl.io/prd"        = "08"
    }
  }

  depends_on = [null_resource.eks_cluster_gate]
}

resource "kubernetes_service_account" "flo_controller" {
  count = var.deploy_bnk ? 1 : 0

  metadata {
    name      = var.flo_service_account_name
    namespace = var.flo_namespace
    annotations = {
      # The load-bearing annotation — PRD 08 § "Architecture (rendered)"
      # § "In the cluster". The EKS pod-identity webhook reads this and
      # injects AWS_ROLE_ARN + AWS_WEB_IDENTITY_TOKEN_FILE into the
      # FLO pod's env at admission time.
      "eks.amazonaws.com/role-arn" = var.flo_irsa_role_arn
    }
  }

  depends_on = [
    kubernetes_namespace.flo,
    null_resource.cert_manager_gate,
  ]
}

# Sentinel: token rotates on every apply, so this null_resource is
# replaced every apply. Its ID is (known after apply), giving
# downstream modules a reliable apply-time dependency on FLO
# completing — regardless of whether FLO's other resources changed.
resource "null_resource" "flo_ready" {
  triggers = {
    token        = try(data.aws_eks_cluster_auth.cluster[0].token, "direct-apply")
    irsa_role    = var.flo_irsa_role_arn
    s3_bucket    = var.s3_bucket_name
    sa_namespace = var.flo_namespace
  }
  depends_on = [kubernetes_service_account.flo_controller]
}
