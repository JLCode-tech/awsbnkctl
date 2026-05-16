# ============================================================
# iam_irsa — Sprint 2 implementation per PRD 08
# (docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md § "Decision" §"IAM role").
#
# Three resources:
#
#   1. data.aws_iam_openid_connect_provider — looks up the EKS OIDC
#      provider that PRD 07's eks_cluster module created. The data
#      source ensures terraform reconciles this module after the
#      eks_cluster apply; failing-fast here with a clear error beats
#      a downstream "trust policy invalid" from the role create.
#   2. aws_iam_role — trust policy bound to system:serviceaccount:<ns>:<sa>
#      via the OIDC provider. PRD 08's JSON example pinned the
#      sub + aud condition shape.
#   3. aws_iam_role_policy — permission policy with s3:GetObject on
#      the supply-chain bucket + kms:Decrypt on the CMK. Inline policy
#      (not aws_iam_policy + aws_iam_role_policy_attachment) so the
#      role + permission lifecycle stays atomic.
# ============================================================

# Confirm the OIDC provider actually exists. data source so module
# consumers fail at plan time if eks_cluster hasn't finished.
data "aws_iam_openid_connect_provider" "cluster" {
  arn = var.oidc_provider_arn
}

locals {
  # PRD 08's trust-policy example renders the StringEquals condition
  # against the OIDC issuer hostname (no "https://" prefix). The PRD 07
  # eks_cluster module surfaces the full URL; we strip the scheme here.
  oidc_issuer_host = replace(var.cluster_oidc_issuer_url, "https://", "")

  role_name = (
    var.role_name_override != "" ?
    var.role_name_override :
    "awsbnkctl-${var.cluster_name}-flo-supply-chain-reader"
  )

  base_tags = merge(
    {
      "awsbnkctl.io/managed-by" = "awsbnkctl"
      "awsbnkctl.io/prd"        = "08"
      "awsbnkctl.io/cluster"    = var.cluster_name
    },
    var.tags,
  )
}

# ----------------------------------------------------------------
# Trust policy. PRD 08 § "Decision" §"IAM role" §"trust policy".
# The Condition's StringEquals on `:sub` + `:aud` is what locks the
# role to the specific service account — without it any pod with
# a kube-mounted serviceaccount token could exchange it for the
# role.
# ----------------------------------------------------------------
data "aws_iam_policy_document" "trust" {
  statement {
    sid     = "FLOServiceAccountAssumeRoleWithWebIdentity"
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [data.aws_iam_openid_connect_provider.cluster.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_issuer_host}:sub"
      values   = ["system:serviceaccount:${var.flo_namespace}:${var.flo_service_account_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_issuer_host}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "flo_supply_chain_reader" {
  name               = local.role_name
  assume_role_policy = data.aws_iam_policy_document.trust.json

  tags = local.base_tags
}

# ----------------------------------------------------------------
# Permission policy. PRD 08 § "Decision" §"Role permission policy".
# s3:GetObject + kms:Decrypt — nothing else.
# ----------------------------------------------------------------
data "aws_iam_policy_document" "permissions" {
  statement {
    sid     = "ReadSupplyChainObjects"
    effect  = "Allow"
    actions = ["s3:GetObject"]
    resources = [
      "${var.s3_bucket_arn}/*",
    ]
  }

  # ListBucket is intentionally NOT granted — PRD 08 § "Decision"
  # pins read-by-key only. FLO knows the object keys from the
  # generated workspace config / tf outputs, not from bucket
  # enumeration.

  statement {
    sid     = "DecryptSupplyChainObjects"
    effect  = "Allow"
    actions = ["kms:Decrypt"]
    resources = [
      var.kms_key_arn,
    ]
  }
}

resource "aws_iam_role_policy" "permissions" {
  name   = "${local.role_name}-policy"
  role   = aws_iam_role.flo_supply_chain_reader.id
  policy = data.aws_iam_policy_document.permissions.json
}
