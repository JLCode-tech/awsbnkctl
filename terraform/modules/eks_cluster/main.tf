# ============================================================
# eks_cluster — Sprint 0 placeholder
#
# This module is a Sprint 1 deliverable per PRD 07
# (docs/prd/07-EKS-CLUSTER-SRIOV.md). Sprint 0 ships an empty stub so
# the top-level terraform/ tree still validates after the IBM-modules
# strip — but applying it MUST fail loudly rather than silently
# succeed (a silent no-op would mask a Sprint-1-broken plan as
# "working").
#
# When Sprint 1 lands, this module wraps `terraform-aws-modules/eks/aws
# ~> 20.x` and composes the self-managed SR-IOV node group + Multus +
# SR-IOV CNI + SR-IOV device plugin stack. Inputs and outputs are
# already declared in variables.tf + outputs.tf so the call-site shape
# in terraform/main.tf is stable across the sprint boundary.
# ============================================================

resource "null_resource" "eks_cluster_sprint_one_placeholder" {
  provisioner "local-exec" {
    command = <<-EOT
      echo ""
      echo "ERROR: terraform/modules/eks_cluster/ is a Sprint 1 deliverable."
      echo "       The module body lands per PRD 07."
      echo "       See: docs/prd/07-EKS-CLUSTER-SRIOV.md"
      echo "            docs/PLAN.md \$ \"Sprint 1\""
      echo ""
      exit 1
    EOT
  }
}
