// Package aws wraps aws-sdk-go-v2 for awsbnkctl's cloud-side surface.
//
// Sprint 0 placeholder. PRD 07 (`docs/prd/07-EKS-CLUSTER-SRIOV.md`)
// pins the implementation contract; this directory is empty in Sprint
// 0 so the IBM-package strip lands against a clean base.
//
// Per PRD 07 § "internal/aws/", the Sprint 1+ surface is:
//
//   - client.go — aws-sdk-go-v2 client constructor; resolves
//     credentials via the standard chain (env / profile / instance
//     role / SSO).
//   - sts.go    — caller-identity for doctor; OIDC provider ARN
//     derivation for IRSA wiring (Sprint 2).
//   - ec2.go    — describe instance type availability per region;
//     SR-IOV / ENA capability flags; quota lookup for the chosen
//     instance family.
//   - eks.go    — describe-cluster (post-apply); kubeconfig
//     generation (no shell-out to `aws eks update-kubeconfig`);
//     cluster auth token via `sts:GetCallerIdentity` presigned URL.
//   - vpc.go    — optional VPC discovery / validation.
//   - s3.go     — Sprint 2; FAR pull-key + JWT licence upload/fetch
//     under the workspace bucket.
//   - iam.go    — Sprint 2; OIDC provider lookup + IRSA role
//     existence checks for doctor.
//
// All five v1.0 callers (`awsbnkctl init`, `awsbnkctl up`, `awsbnkctl
// doctor`, `awsbnkctl down`, the terraform-exec wrapper's post-apply
// kubeconfig fetch) consume this package. The IBM equivalent
// (`internal/ibm/`) was deleted in Sprint 0; until Sprint 1 lands the
// EKS cluster module the cred-resolver chain stops at "env API key
// only" and the doctor reports "AWS support coming in Sprint 1".
package aws
