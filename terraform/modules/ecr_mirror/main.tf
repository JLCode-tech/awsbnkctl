# ============================================================
# ecr_mirror — Sprint 2 scaffold per PRD 08
# (docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md § "Decision" + § "ecr_mirror"
# §"v1.0 stretch").
#
# Module shape:
#
#   - Gated on var.enable_ecr_mirror. When false, every resource's
#     for_each is empty so the module is a no-op. When true, one
#     aws_ecr_repository per element of var.images.
#   - skopeo copy null_resource: the actual image mirror execution
#     is deferred to a Sprint 3 follow-up issue per the staff brief
#     (the brief explicitly allows skipping the skopeo wiring if
#     budget is tight). The resource shape stays here so the
#     terraform plan diff is small when Sprint 3 fills the body.
#
# This is the deliberate "skeleton + tracking issue" shape PRD 08
# describes: the module exists, has its inputs + outputs pinned,
# and the skopeo pipeline lands when the validator's tools-image
# multi-arch work (Sprint 2 validator) provides a stable skopeo
# binary.
# ============================================================

locals {
  enabled    = var.enable_ecr_mirror
  image_set  = local.enabled ? { for img in var.images : img => img } : {}

  base_tags = merge(
    {
      "awsbnkctl.io/managed-by" = "awsbnkctl"
      "awsbnkctl.io/prd"        = "08"
      "awsbnkctl.io/workspace"  = var.workspace_name
    },
    var.tags,
  )
}

# One repository per source image. The repository name is derived
# from the source repo (strip the host + tag).
resource "aws_ecr_repository" "mirror" {
  for_each = local.image_set

  name = "awsbnkctl/${var.workspace_name}/${replace(split(":", each.key)[0], "/", "-")}"

  image_scanning_configuration {
    scan_on_push = true
  }

  image_tag_mutability = "IMMUTABLE"

  tags = merge(local.base_tags, {
    "awsbnkctl.io/source-image" = each.key
  })
}

# Sprint 3 follow-up: null_resource running skopeo copy. The
# trigger references the source image string so a tag change
# re-runs the copy.
#
# resource "null_resource" "mirror_copy" {
#   for_each = local.image_set
#   triggers = { source = each.value }
#
#   provisioner "local-exec" {
#     command = "skopeo copy ${var.source_authfile_path != "" ? "--src-authfile=${var.source_authfile_path}" : ""} docker://${each.value} docker://${aws_ecr_repository.mirror[each.key].repository_url}:${split(":", each.value)[1]}"
#   }
# }
