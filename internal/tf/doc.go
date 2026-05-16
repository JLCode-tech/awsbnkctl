// Package tf wraps hashicorp/terraform-exec to drive `terraform init /
// plan / apply / destroy` against the pinned TF source for a awsbnkctl
// workspace.
//
// Three layers:
//
//   - source.go    — resolve "what tag should we run?" (GitHub Releases API)
//   - fetch.go     — turn that into a local directory of .tf files (tarball
//     download, or use local Path for type=local)
//   - vars.go      — render config.yaml into terraform.tfvars
//   - terraform.go — Workspace.{Init,Plan,Apply,Destroy,Output} via tfexec
//
// The `terraform` binary is a runtime dep on PATH (>= 1.5). Module
// fetching for the upstream TF's own dependencies (providers, child
// modules) is delegated to `terraform init`.
//
// Secrets policy: cloud credentials are NEVER written to
// terraform.tfvars. AWS credentials resolve via the SDK chain and
// reach the terraform child process through the standard AWS provider
// env vars (AWS_ACCESS_KEY_ID etc.) inherited from the awsbnkctl
// process env.
package tf
