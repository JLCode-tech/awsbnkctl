# Sprint 2 — staff engineer issues

Sprint 2 lands PRD 08 (S3 supply chain + IRSA) + the workspace schema
retarget that closes Sprint 1 staff Issue 3 + tech-writer Issue 1.

## Issue 1: legacy_helpers.go IBM cred plumbing not yet retired

**Severity**: low
**Status**: open (Sprint 3 — depends on PRD 04 cred + exec retarget)
**Description**: The Sprint 2 brief targeted full retirement of
`internal/cli/legacy_helpers.go`. The file's surviving consumers
(`internal/cli/test.go`, `internal/cli/tfvars.go`,
`internal/cli/doctor_backend.go`) compile against it, but the deeper
IBMCLOUD_API_KEY plumbing flows through `internal/cred` +
`internal/exec` — those packages still expose
`Credentials.IBMCloudAPIKey` and the docker/k8s execution backends
still bind-mount `/run/secrets/ibmcloud_api_key`. Retiring
`legacy_helpers.go` cleanly requires the cred + exec retarget per
PRD 04, which is Sprint 3's scope. Sprint 2 refreshed the in-file
narrative to reflect the new context but kept the functions live.

**Files affected**: `internal/cli/legacy_helpers.go`,
`internal/cli/doctor_backend.go`, `internal/cred/resolver.go`,
`internal/exec/creds.go`, `internal/exec/docker.go`.
**Proposed fix**: Sprint 3 ports `Credentials.IBMCloudAPIKey` →
`Credentials.AWSCredentials` (the SDK chain output) and updates the
docker/k8s injection paths. Once those don't reference the IBM
field names, the `legacy_helpers.go` shim retires alongside
`doctor_backend.go`'s IBMCLOUD_API_KEY ops-pod env probe.

## Issue 2: Workspace schema retarget kept back-compat alias instead of clean break

**Severity**: low (informational)
**Status**: resolved-as-designed
**Description**: Per the brief's "back-compat alias OR clean break —
your call, document" guidance, Sprint 2 staff chose the back-compat
alias path. `Workspace.IBMCloud` stays as a deprecated yaml field
so legacy on-disk workspaces continue to load; the new
`Workspace.AWS` block is the first-class shape and is what
`awsbnkctl init` writes. Doctor + tf vars renderer + cli/inspect.go
+ cli/workspaces.go read from AWS first, fall back to
IBMCloud.Region for the region field only.

Rationale: a clean break would have broken every existing on-disk
workspace (and the cred + exec packages that still consume
`IBMCloud.APIKeyB64`); the alias keeps the change additive and
revisable at Sprint 3 retirement time.

**Files affected**: `internal/config/workspace.go`,
`internal/doctor/aws.go`, `internal/tf/vars.go`,
`internal/cli/inspect.go`, `internal/cli/workspaces.go`.
**Proposed fix**: integrator notes the back-compat shape; Sprint 3
deletes the `IBMCloud` field outright once cred + exec retarget.

## Issue 3: ECR mirror module skopeo pipeline deferred to Sprint 3

**Severity**: roadmap (PRD 08 v1.0 stretch)
**Status**: open by design
**Description**: PRD 08 § "Decision" explicitly allows deferring the
ECR mirror module's `null_resource` + `skopeo copy` pipeline; the
Sprint 2 brief restates this latitude. The module's variables.tf,
outputs.tf, and `aws_ecr_repository` resources land this sprint
behind `var.enable_ecr_mirror = false`; the `local-exec` skopeo
provisioner stays commented out in `main.tf` with a Sprint 3 TODO.
Reason: the validator agent's Sprint 2 multi-arch tools-image work
provides the stable skopeo binary the provisioner needs, and that
work was parallel to staff — coupling them mid-sprint risked both.

**Files affected**: `terraform/modules/ecr_mirror/main.tf`.
**Proposed fix**: Sprint 3 staff fills the commented
`null_resource mirror_copy` block once the validator's tools-image
(containing skopeo) is published. Tracking: Sprint 3 staff issue,
to be filed at Sprint 3 dispatch.

## Issue 4: init wizard's S3 upload path runs only when bucket recorded in workspace

**Severity**: low (UX nit)
**Status**: open
**Description**: `awsbnkctl init` without `--dry-run` reads
`ws.AWS.SupplyChain.BucketName` to know where to PutObject.
Pre-`awsbnkctl up`, that field is empty (the s3_supply_chain module
generates the random-suffixed name at apply time), so the init
wizard correctly declines to upload — but the message ("terraform
apply will create it + upload via aws_s3_object") presumes the
operator's mental model already includes the two-phase flow. PRD 08
§ "Open questions" documents the design decision (declarative
supply chain via `aws_s3_object`), but a first-time user may wonder
why init didn't upload.

**Files affected**: `internal/cli/init.go` (the
`uploadSupplyChain` skip branch).
**Proposed fix**: Sprint 3 architect adds a one-line link to PRD 08
§ "Open questions" in the wizard's skip-message OR documents the
two-phase flow in the book chapter 25 architect is drafting. Likely
the latter — the wizard message is already clear about the next
step.

## Issue 5: ec2 vCPU quota row probes ec2:DescribeAccountAttributes, not the live quota

**Severity**: roadmap
**Status**: closes Sprint 1 staff Issue 2 partially
**Description**: The doctor's new `aws ec2 vCPU quota` row resolves
the IAM-permission-feasibility half of Sprint 1 staff Issue 2 (the
caller can talk to EC2). The actual live "Running On-Demand c5n
instances" quota lives in the Service Quotas API (separate from EC2);
querying that requires adding `aws-sdk-go-v2/service/servicequotas`
to internal/aws. The Sprint 2 doctor row surfaces the path
("check Service Quotas for the Running On-Demand c5n quota") rather
than auto-resolving it.

**Files affected**: `internal/doctor/aws.go` (the ec2 vCPU row).
**Proposed fix**: Sprint 4 doctor refresh adds the
`servicequotas:GetServiceQuota` call (cheap; one API hop). Tracked
as Sprint 4 doctor work; not a Sprint 2 deliverable per the brief's
"closes Sprint 1 staff Issue 2" criterion (the brief asked for the
EC2 vCPU row to land, not for the live-quota auto-resolution).

## Issue 6: Live-AWS validation deferred to operator spike

**Severity**: roadmap (informational, not actionable this sprint)
**Status**: open by design
**Description**: Per the Sprint 2 brief's SPIKE DEFERRAL section, no
agent runs `terraform apply` against live AWS. Validation tools
used: `terraform validate` (succeeded on root + each new module),
`go test ./...` (passed; mocked AWS clients), `go vet`, `gofmt`,
`awsbnkctl init --dry-run` (wizard ran offline, wrote the workspace
config, skipped the PutObject step). The Sprint 1 operator-run
spike still gates v0.2; Sprint 2 lands more offline work on top
without unblocking the spike.

**Files affected**: `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` §
"Resolved in spike" (architect's surface — staff does not edit).
**Proposed fix**: operator runs the day-by-day spike protocol
(carried from PRD 07); findings fold into both PRDs; v0.2 tag
follows.

## Issue 7: gofmt fixup landed on internal/cli/cluster.go during build-green sweep

**Severity**: trivial
**Status**: resolved-during-sprint
**Description**: The Sprint 1 `internal/cli/cluster.go` had a
trailing-newline drift (`gofmt -d -l .` flagged a single missing
trailing newline). Sprint 2 staff ran `gofmt -w` on the file as
part of the build-green gate. No semantic change.

**Files affected**: `internal/cli/cluster.go`.
**Proposed fix**: applied; no follow-up.
