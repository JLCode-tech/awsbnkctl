# ============================================================
# s3_supply_chain — outputs (PRD 08 § "Implementation outline")
#
# iam_irsa consumes bucket_arn + kms_key_arn for its permission policy.
# ============================================================

output "bucket_name" {
  description = "The supply-chain S3 bucket name (generated or overridden)."
  value       = aws_s3_bucket.supply_chain.id
}

output "bucket_arn" {
  description = "The supply-chain S3 bucket ARN."
  value       = aws_s3_bucket.supply_chain.arn
}

output "far_auth_object_key" {
  description = "S3 key of the FAR archive object."
  value       = aws_s3_object.far_auth.key
}

output "jwt_object_key" {
  description = "S3 key of the subscription JWT object."
  value       = aws_s3_object.jwt.key
}

output "kms_key_arn" {
  description = "KMS CMK ARN used for SSE-KMS (created here if var.kms_key_arn was empty)."
  value       = local.kms_key_arn
}
