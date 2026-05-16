# ============================================================
# iam_irsa — outputs (PRD 08 § "Implementation outline")
# ============================================================

output "flo_role_arn" {
  description = "IAM role ARN. Annotate the FLO ServiceAccount with eks.amazonaws.com/role-arn = <this value>."
  value       = aws_iam_role.flo_supply_chain_reader.arn
}

output "flo_role_name" {
  description = "IAM role name (the inspectable side of flo_role_arn)."
  value       = aws_iam_role.flo_supply_chain_reader.name
}
