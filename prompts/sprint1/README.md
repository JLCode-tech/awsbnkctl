# Sprint 1

**Theme:** EKS cluster module + self-managed SR-IOV node group (PRD 07)

_Drafted from `docs/PLAN.md` Sprint 1 section._

Sprint 1 lands the load-bearing technical work of awsbnkctl: an EKS cluster Terraform module that provisions a self-managed node group on ENA-enabled instance types and layers Multus + SR-IOV CNI + SR-IOV device plugin DaemonSets on top of the AWS VPC CNI. Plus the `internal/aws/` Go SDK helpers, the `awsbnkctl up cluster` lifecycle verb, and an AWS-aware refresh of `awsbnkctl doctor`.

End-of-sprint gate: `terraform -chdir=terraform/modules/eks_cluster validate` succeeds; `awsbnkctl up cluster --dry-run` plans the resources correctly; `internal/aws/` Go unit tests pass against mocked SDK clients; `awsbnkctl doctor` reports AWS-shaped checks. The closing milestone (`v0.2`) is gated on the **operator-run spike** validating the design hypothesis against live AWS — that runs separately from this sprint's agent dispatch.

## Spike deferral

**Important:** PRD 07's "Spike protocol" (days 1-3) requires live AWS resources and incurs cost (~$5-15 for a few hours of EKS cluster + 2 × c5n.4xlarge instances). **No agent in this sprint runs `terraform apply` against live AWS.** The spike is operator-run separately, with findings folded into PRD 07's "Resolved in spike" section.

This sprint focuses on:
- Authoring the Terraform module against the PRD 07 design hypothesis
- Authoring `internal/aws/` Go code with unit-level mocking
- Wiring the `awsbnkctl up cluster` cobra verb
- Refreshing `awsbnkctl doctor` for AWS-shaped checks
- Validating via `terraform validate` (not `apply`) and Go unit tests

If the spike later surfaces a hypothesis mismatch, Sprint 1.5 (or Sprint 2 rework) addresses it; the v0.2 tag is gated on spike validation, not on this sprint's agent dispatch.

Carry-overs from Sprint 0 (open issues in `issues/issue_sprint0_*.md` that touch Sprint 1 scope):
1. Validator's medium: AWS tools-image (`tools/docker/aws/Dockerfile`) needs Sprint 1 authoring → validator agent's task 4.
2. Staff's open: `internal/cred/` + `internal/exec/` still carry `IBMCloud`/`IBMCLOUD_API_KEY` identifiers — touched only at the AWS-credential adapter level this sprint; full retarget is Sprint 2 per PRD 04.
3. Staff's open: `internal/cli/doctor_backend.go` has stale ops-pod references — staff agent's task 5.

Four-agent dispatch (parallel):

1. **architect** — finalises `docs/prd/07-EKS-CLUSTER-SRIOV.md` post-design-pass (no `Resolved in spike` content yet — that's operator output); drafts `book/src/33-data-plane-decision.md` (design framing; spike findings fold later); first-draft `book/src/02-why-eks-and-sriov.md`; updates `docs/PLAN.md` Sprint 1 close section with what actually shipped.
2. **staff** — implements `terraform/modules/eks_cluster/`; implements `internal/aws/{client,sts,ec2,eks,vpc}.go` with aws-sdk-go-v2; wires `awsbnkctl up cluster` cobra verb; refreshes `awsbnkctl doctor` with AWS checks; unit tests across the new surface.
3. **validator** — authors `tools/docker/aws/Dockerfile` + matching `build-aws` Makefile target + `tools-images.yml` matrix entry; integration-test scaffolding (mocked AWS or localstack-backed); CI matrix updates for the new packages; cspell additions for any new AWS terminology the staff agent introduces.
4. **tech-writer** — read-only at sprint close: dogfood the `awsbnkctl up cluster --dry-run` path, cross-link audit on PRD 07 + book chapter 33, flag PRD 07's "Resolved in spike" section as awaiting operator output.

The integrator (you, or another agent) folds the four agents' issue files into a single Sprint 1 commit on `main`. The commit does **not** tag `v0.2` — that tag waits on operator-run spike validation.
