# ============================================================
# Root provider configurations (Sprint 0)
#
# aws  — primary cloud provider. Credentials resolve via the standard
#         chain (env / shared config / profile / instance role / SSO);
#         Sprint 2 adds the IRSA-aware variants for in-cluster pods.
#
# null — required because Sprint 0's eks_cluster stub uses a
#         null_resource to fail-stop the apply with a Sprint-1-defer
#         message. Sprint 3's cert_manager / flo / cne_instance modules
#         also keep null_resource provisioners for the post-apply
#         k8s-readiness probes carried over from roksbnkctl.
# ============================================================

provider "aws" {
  region = var.region
}

provider "null" {}
