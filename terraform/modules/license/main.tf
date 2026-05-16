# ============================================================
# license — outer module body (Sprint 3 AWS retarget)
#
# Resolves the License CR's JWT URL to an S3 path under the
# supply-chain bucket. The inner `./modules/license` body that
# emits the License CR is preserved on disk per PRD 00
# § "Inheritance map"; Sprint 4 wires the kubernetes_manifest
# License CR via the AWS-native kubernetes provider once the
# operator-run spike validates the EKS path.
# ============================================================

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
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
  # The FLO pod (IRSA-annotated) consumes this URL via the
  # aws-sdk-go's S3 GetObject — the EKS pod-identity webhook
  # injected AWS_WEB_IDENTITY_TOKEN_FILE, the SDK exchanges
  # that with STS for short-lived AWS creds, the GetObject
  # call hits S3 directly. No presigned URL needed in the
  # in-cluster path (presigned is the v1.x out-of-cluster path).
  license_jwt_s3_url = "s3://${var.s3_bucket_name}/${var.s3_jwt_object_key}"

  license_values = {
    namespace            = var.flo_utils_namespace
    license_mode         = var.license_mode
    jwt_s3_url           = local.license_jwt_s3_url
    s3_bucket_name       = var.s3_bucket_name
    s3_jwt_object_key    = var.s3_jwt_object_key
    flo_irsa_role_arn    = var.flo_irsa_role_arn
  }
}

resource "null_resource" "license_ready" {
  triggers = {
    enabled       = var.deploy_bnk ? "on" : "off"
    cluster       = var.eks_cluster_name
    s3_bucket     = var.s3_bucket_name
    s3_jwt_key    = var.s3_jwt_object_key
    flo_irsa_role = var.flo_irsa_role_arn
  }

  depends_on = [
    null_resource.eks_cluster_gate,
    null_resource.cneinstance_gate,
  ]
}
