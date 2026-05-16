# ============================================================
# s3_supply_chain — Sprint 2 implementation per PRD 08
# (docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md § "Decision" + § "Implementation
# outline").
#
# Three resources land here:
#
#   1. aws_kms_key (conditional) — customer-managed CMK for SSE-KMS.
#      Skipped if var.kms_key_arn is supplied.
#   2. aws_s3_bucket + bucket policy — the supply-chain bucket. Policy
#      denies non-TLS requests + denies non-AES256/aws:kms uploads;
#      the actual GetObject grant lives in the iam_irsa module's role
#      policy (PRD 08 § "Decision" §"Bucket policy restricts
#      s3:GetObject to the FLO IRSA role").
#   3. aws_s3_object — the FAR archive + JWT, sourced from local paths
#      the operator supplied via `awsbnkctl init`.
#
# PRD 08 § "Open questions" resolved here: explicit Deny-by-default
# posture on non-TLS + insecure uploads, plus the future-proof
# defensive Deny in the bucket policy.
# ============================================================

resource "random_id" "bucket_suffix" {
  byte_length = 4
}

locals {
  # PRD 08 § "Decision" §"Bucket name pattern".
  bucket_name = (
    var.bucket_name_override != "" ?
    var.bucket_name_override :
    "awsbnkctl-${var.workspace_name}-${random_id.bucket_suffix.hex}"
  )

  create_kms = var.kms_key_arn == ""
  kms_key_arn = (
    local.create_kms ?
    try(aws_kms_key.cmk[0].arn, "") :
    var.kms_key_arn
  )

  base_tags = merge(
    {
      "awsbnkctl.io/managed-by" = "awsbnkctl"
      "awsbnkctl.io/prd"        = "08"
      "awsbnkctl.io/workspace"  = var.workspace_name
    },
    var.tags,
  )
}

# ----------------------------------------------------------------
# Customer-managed KMS CMK. Created when var.kms_key_arn is empty;
# the iam_irsa module's permission policy grants kms:Decrypt against
# whichever ARN we ultimately surface (local.kms_key_arn).
#
# PRD 08 § "Trade-offs accepted" §"Customer-managed KMS key by
# default" — costs $1/month but the JWT is sensitive enough that
# SSE-S3 (AWS-managed) isn't the default v1.0 posture.
# ----------------------------------------------------------------
resource "aws_kms_key" "cmk" {
  count                   = local.create_kms ? 1 : 0
  description             = "awsbnkctl supply-chain CMK for workspace ${var.workspace_name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = local.base_tags
}

resource "aws_kms_alias" "cmk" {
  count         = local.create_kms ? 1 : 0
  name          = "alias/awsbnkctl-${var.workspace_name}-supply-chain"
  target_key_id = aws_kms_key.cmk[0].key_id
}

# ----------------------------------------------------------------
# The supply-chain bucket. PRD 08 § "Goal" + § "Decision".
# ----------------------------------------------------------------
resource "aws_s3_bucket" "supply_chain" {
  bucket = local.bucket_name

  # force_destroy = true is intentionally NOT set. The bucket holds
  # the JWT + FAR-pull credentials; an accidental terraform destroy
  # must not silently nuke them. Operators who really want to delete
  # run `aws s3 rm s3://<bucket> --recursive` first.

  tags = merge(local.base_tags, {
    "Name" = local.bucket_name
  })
}

resource "aws_s3_bucket_versioning" "supply_chain" {
  bucket = aws_s3_bucket.supply_chain.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "supply_chain" {
  bucket = aws_s3_bucket.supply_chain.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = local.kms_key_arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "supply_chain" {
  bucket = aws_s3_bucket.supply_chain.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# ----------------------------------------------------------------
# Bucket policy: Deny-by-default posture.
#
# PRD 08 § "Open questions" resolved: an explicit Deny pair (no-TLS +
# insecure upload) layered on top of S3's default-deny. The actual
# GetObject grant lives in the iam_irsa module's role-permission
# policy — keeping the grant alongside the IRSA role (rather than
# in the bucket policy) makes the trust chain inspectable from the
# IAM side without re-reading bucket policy JSON.
# ----------------------------------------------------------------
data "aws_iam_policy_document" "bucket_policy" {
  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.supply_chain.arn,
      "${aws_s3_bucket.supply_chain.arn}/*",
    ]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }

  statement {
    sid    = "DenyUnencryptedUploads"
    effect = "Deny"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.supply_chain.arn}/*"]

    condition {
      test     = "StringNotEquals"
      variable = "s3:x-amz-server-side-encryption"
      values   = ["aws:kms"]
    }
  }
}

resource "aws_s3_bucket_policy" "supply_chain" {
  bucket = aws_s3_bucket.supply_chain.id
  policy = data.aws_iam_policy_document.bucket_policy.json
}

# ----------------------------------------------------------------
# Objects. PRD 08 § "Open questions" resolved:
#
#   "Should `awsbnkctl init` upload via aws-sdk-go-v2 directly (no
#   terraform) or via a terraform `aws_s3_object` resource?
#   Decided: `aws_s3_object` so the supply chain is reproducible
#   from `terraform apply` alone."
#
# The init wizard writes the local paths to terraform.tfvars; this
# module then renders them as `aws_s3_object` resources on apply.
# The hash-based etag ensures terraform re-uploads when the local
# file changes (so a refreshed FAR archive on disk triggers a new
# object version).
# ----------------------------------------------------------------
resource "aws_s3_object" "far_auth" {
  bucket = aws_s3_bucket.supply_chain.id
  key    = var.far_auth_object_key
  source = var.far_auth_file_local_path
  etag   = filemd5(var.far_auth_file_local_path)

  server_side_encryption = "aws:kms"
  kms_key_id             = local.kms_key_arn

  tags = local.base_tags
}

resource "aws_s3_object" "jwt" {
  bucket = aws_s3_bucket.supply_chain.id
  key    = var.jwt_object_key
  source = var.jwt_file_local_path
  etag   = filemd5(var.jwt_file_local_path)

  server_side_encryption = "aws:kms"
  kms_key_id             = local.kms_key_arn

  tags = local.base_tags
}
