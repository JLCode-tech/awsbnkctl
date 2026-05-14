# Sprint 0 — staff engineer issues

## Issue 1: Go toolchain absent on the working host
**Severity**: medium
**Status**: resolved
**Description**: The macOS host where Sprint 0 was executed had no `go` binary on PATH (no `/opt/homebrew/opt/go`, no `~/sdk`, no `~/go`, no system install). Without Go the verification gauntlet (`go build`, `go vet`, `go test`) cannot run. Resolved by `brew install go` (1.26.3). The integrator should confirm Go is on their PATH before re-running `make build` from a fresh clone.
**Files affected**: none — host-side
**Proposed fix**: Already resolved. Future agents should assume Go may need installing.

## Issue 2: Inherited tfvars-md smoke test relied on the ROKS variable schema
**Severity**: high
**Status**: resolved
**Description**: `tools/refgen/tfvars-md/main_test.go::TestRun_SmokeOutput` asserted that the live `terraform/variables.tf` contained `ibmcloud_api_key` and a `roks_cluster` module section. Sprint 0 stripped both. Per the brief, inherited tests retargeting in a later sprint get `t.Skip(...)` rather than a delete; the test now skips with `inherited test — retargets in Sprint 3` (Sprint 3 is when the AWS-shaped variables.tf is reauthored alongside the module port). The integrator should review the skip and confirm Sprint 3's brief picks it up. The two other tests in the same file (`TestParseFile_BasicShape`, `TestFindMatchingBrace`) still execute against synthetic fixtures and pass.
**Files affected**: `tools/refgen/tfvars-md/main_test.go`
**Proposed fix**: Restore in Sprint 3 once the AWS-shaped variables.tf and the eks_cluster module body land; update the asserted variable names + module section header.

## Issue 3: Residual IBM coupling in cred/exec/config packages (out of Sprint 0 scope)
**Severity**: medium
**Status**: open
**Description**: The brief's verification step states `grep -r 'ibmcloud\|IBMCLOUD' --include='*.go' .` should return no hits. That target is not reachable from Sprint 0's scope because `internal/cred`, `internal/exec`, `internal/config`, and `internal/doctor` all carry IBM-named surface (e.g., `Resolver.IBMCloudAPIKey`, `Credentials.IBMCloudAPIKey`, `IBMCloudCfg` struct, the docker backend's `ibmcloud` credential-shim entry, the resolver's `IBMCLOUD_API_KEY` env-var probe). The four directories the brief named (`internal/ibm/`, `internal/cos/`, `terraform/modules/roks_cluster/`, `tools/docker/ibmcloud/`) are deleted; the deep-coupled packages are flagged for Sprint 2's AWS-credential-adapter work (PRD 04 inheritance edits per `docs/prd/00-OVERVIEW.md`). Build, vet, gofmt, and the test suite are all green; the residual strings are functional names inherited from roksbnkctl that need retargeting, not active IBM API calls.
**Files affected**: `internal/cred/resolver.go`, `internal/exec/creds.go`, `internal/exec/docker.go`, `internal/exec/k8s.go`, `internal/exec/local.go`, `internal/config/workspace.go`, `internal/config/secrets.go`
**Proposed fix**: Sprint 2 brief includes a "retarget the cred chain" item — rename `IBMCloudAPIKey` → `AWSCredential` (or similar), drop the `IBMCLOUD_API_KEY` env-var probe, replace the docker backend's `ibmcloud` shim with the AWS SDK-direct equivalent. The `ibmcloud` row in the docker `dockerImageBinary` map and the k8s `jobToolCmdOverride` map should also drop (PRD 00 § "Inheritance map" says no AWS CLI passthrough is planned).

## Issue 4: Inherited CLI verbs deleted alongside `internal/ibm`/`internal/cos`
**Severity**: medium
**Status**: resolved
**Description**: The brief explicitly authorised deleting CLI verb files that depend on the four removed packages. The following Go files were deleted via `git rm -f`: `internal/cli/cos.go`, `internal/cli/cluster.go`, `internal/cli/cluster_phase.go`, `internal/cli/bnk_phase.go`, `internal/cli/bnk_phase_test.go`, `internal/cli/ops.go`, `internal/cli/ops_test.go`, `internal/cli/ops_integration_test.go`. `init.go` and `lifecycle.go` were rewritten as Sprint 1-defer stubs that keep the cobra command tree (`up`/`plan`/`apply`/`down`/`init`) but error cleanly when invoked. A new file `internal/cli/legacy_helpers.go` carries forward four small helpers (`workspaceEnv`, `resolveBackendSpecWith`, `podReady`, `refDescription`) that surviving CLI files (`test.go`, `tfvars.go`, `doctor_backend.go`) still call. The integrator should review the new stubs + helpers for adequacy.
**Files affected**: `internal/cli/init.go`, `internal/cli/lifecycle.go`, `internal/cli/legacy_helpers.go` (new), plus the deletions enumerated above.
**Proposed fix**: Sprint 1 re-implements `init` + `up` + `plan` + `apply` + `down` against `internal/aws` and `terraform/modules/eks_cluster/`. The helpers in `legacy_helpers.go` retire when the per-tool AWS credential adapter (Sprint 2) replaces the IBM env-var passthrough.

## Issue 5: `internal/cli/doctor_backend.go` still references the deleted ops-pod surface
**Severity**: low
**Status**: open
**Description**: The ops-pod doctor checks (under `--backend k8s`) probe for `awsbnkctl-ops`-named Kubernetes resources and the trusted-profile Secret. The ops-install command was deleted alongside `ops.go`, so these doctor rows always fail when `--backend k8s` is passed. Build is green because the deleted-package import was the only compile-time dependency; runtime behaviour just reports "not found — run `awsbnkctl ops install`" which is now a dead reference.
**Files affected**: `internal/cli/doctor_backend.go`
**Proposed fix**: Either delete `doctor_backend.go` in Sprint 1 (when the AWS-shaped ops pod is reauthored, if at all) or rewrite its probe surface against an AWS-shaped ops pod model. Sprint 1's brief should mention this. No user-visible impact today — the `--backend k8s` doctor path is not part of the Sprint 0 smoke test.

## Issue 6: Workspace config still carries `IBMCloud` field shape
**Severity**: medium
**Status**: open
**Description**: `internal/config/workspace.go` defines `Workspace.IBMCloud IBMCloudCfg` with `Region`, `ResourceGroup`, `APIKeySource`, `APIKeyB64`. Sprint 0 stub callers (`init`, `lifecycle`) no longer touch it, but `workspaces.go`, `inspect.go`, etc. still serialise and load it. A new workspace YAML will still write `ibmcloud:` under each workspace until Sprint 1 retargets the struct shape to `aws:` with `region`, `vpc_id`, `subnet_ids`, etc. Per PRD 07's input table this is Sprint 1 work.
**Files affected**: `internal/config/workspace.go`, `internal/config/secrets.go`, `internal/cli/workspaces.go`, `internal/cli/inspect.go`
**Proposed fix**: Sprint 1 adds an `AWS AWSCfg` field; Sprint 2 finalises the migration (rename + remove the `IBMCloud` field). Keeping the old shape in Sprint 0 was a deliberate choice — touching the struct ripples through every package's serialisation tests.
