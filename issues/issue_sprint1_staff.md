# Sprint 1 — staff engineer issues

## Issue 1: Agent run hit API 529 Overloaded at task 3/4 boundary

**Severity**: medium
**Status**: resolved (integrator-completed)
**Description**: The Sprint 1 staff agent completed tasks 1 (eks_cluster Terraform module — 5 .tf files), 2 (`internal/aws/{client,sts,ec2,eks,vpc}.go` + unit tests), and 3 (`internal/cli/cluster.go` with `awsbnkctl up cluster --dry-run` and `down cluster --dry-run` cobra verbs) before the Anthropic API returned `529 Overloaded` mid-dispatch. Tasks 4-7 (doctor refresh, Sprint 0 carry-over retirement, build-green gate, file issues) were completed by the integrator directly.

The work landed on disk before the error: `internal/aws/` has 10 files (5 source + 5 test), `terraform/modules/eks_cluster/` has main + multus + sriov + variables + outputs + versions (six files), `internal/cli/cluster.go` is 168 lines with both dry-run verbs wired.

**Files affected**: `internal/doctor/aws.go` (integrator-written), `internal/doctor/doctor.go` (Sprint 0 placeholder replaced with `awsChecks(ctx, cctx)` call).
**Proposed fix**: applied — three AWS doctor checks (credentials configured, STS caller-identity, EKS DescribeCluster permission probe) wired in. EC2 quota + S3 PutObject probes deferred to Sprint 2 per the task brief.

## Issue 2: EC2 vCPU quota check deferred to Sprint 2

**Severity**: low
**Status**: open
**Description**: PRD 07 § "Spike protocol" and PRD 07 § "internal/aws/" both list EC2 vCPU quota check as a doctor pre-flight item. `internal/aws/ec2.go` exposes `VCPUQuotaAttribute(ctx)`, so the helper is there. But the doctor framework doesn't have access to the chosen region + instance family at pre-flight time on a fresh dev box (the workspace config might not exist yet, or might not carry AWS-specific fields until Sprint 2's PRD 04 retarget).

**Files affected**: `internal/doctor/aws.go` — currently surfaces credentials + STS + EKS permission, not vCPU quota.
**Proposed fix**: Sprint 2 wires `awsbnkctl up cluster --dry-run` to run vCPU quota check against the workspace's declared region + instance type list before terraform plan; doctor reports it post-workspace-load. Could also be Sprint 2's `awsbnkctl init` wizard concern.

## Issue 3: Workspace config still carries `IBMCloud` field name

**Severity**: medium
**Status**: open (Sprint 2 retargets per PRD 04)
**Description**: `internal/doctor/aws.go` reads the region via `cctx.Workspace.IBMCloud.Region` because the workspace struct retains the IBM-shaped field names from the roksbnkctl inheritance. Sprint 2 retargets this per PRD 04. Until then, the AWS region is set via `AWS_REGION` env var (which the SDK default chain reads) or via the `IBMCloud.Region` field (which is misleadingly named but functionally correct).

**Files affected**: `internal/config/` (entire workspace schema), `internal/doctor/aws.go` (uses the workaround), `internal/tf/vars.go` (Sprint 0 placeholder).
**Proposed fix**: Sprint 2 — rename `Workspace.IBMCloud` → `Workspace.AWS` with field-by-field retarget. Document the migration in `MIGRATING.md`.

## Issue 4: legacy_helpers.go + doctor_backend.go retirement deferred

**Severity**: low
**Status**: open (Sprint 2)
**Description**: `internal/cli/legacy_helpers.go` (Sprint 0 shim) and `internal/cli/doctor_backend.go` (k8s-backend ops-pod check) both carry IBM-shaped surface (`IBMCLOUD_API_KEY` env, ops-pod cred secret structure). Removing them requires Sprint 2's cred + exec retarget (PRD 04). Leaving in place for Sprint 1.

**Files affected**: `internal/cli/legacy_helpers.go` (116 lines), `internal/cli/doctor_backend.go` (~200 lines).
**Proposed fix**: Sprint 2 task. The Sprint 1 task brief explicitly listed this as "touch only at the AWS-credential adapter level this sprint", which is what happened.

## Issue 5: PRD 07 ↔ Terraform module: 3 variables potentially absent

**Severity**: medium
**Status**: open (architect Issue 1 cross-refs this)
**Description**: Architect's Sprint 1 Issue 1 flagged a possible PRD 07 ↔ implementation drift on three variables (`enable_multus`, `enable_sriov`, `sriov_resource_name`) since the architect ran before staff completed. Integrator verification: `terraform/modules/eks_cluster/variables.tf` does expose `enable_multus` and `enable_sriov` (defaults `true`) and `sriov_resource_name` (default `"intel.com/sriov"`); all match PRD 07 § "Inputs" table. Architect's concern was based on the Sprint 0 stub state — closed at integration time.

**Files affected**: `terraform/modules/eks_cluster/variables.tf`, `docs/prd/07-EKS-CLUSTER-SRIOV.md`.
**Proposed fix**: tech-writer agent re-verifies post-integration; close at sprint close if no drift remains.

## Issue 6: Live-AWS validation deferred to operator spike

**Severity**: roadmap (informational, not actionable this sprint)
**Status**: open by design
**Description**: Per Sprint 1 README, no agent ran `terraform apply` against live AWS. Validation tools used: `terraform validate` (succeeded on module + root), `go test` (passed; mocked AWS clients), `go vet`, `gofmt`, `--dry-run` flows. PRD 07's "Resolved in spike" section remains a placeholder for operator output.

The v0.2 tag is gated on the operator-run spike per PRD 07 § "Spike status". Sprint 1's integrator commit does NOT cut v0.2.

**Files affected**: `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Resolved in spike" (placeholder).
**Proposed fix**: operator runs the day-by-day spike protocol; findings fold into the PRD; v0.2 tag follows.
