# Phased development & testing plan

Execution plan for `awsbnkctl` — the AWS-retargeted port of [`roksbnkctl`](https://github.com/jgruberf5/roksbnkctl). Synthesises the PRDs in [`docs/prd/`](./prd/) into sequenced work, development and testing interleaved per sprint. References the PRDs by number; read those for the *what*, this for the *when* and *how*.

> **Fork inheritance.** This plan starts from a hard fork of `jgruberf5/roksbnkctl@v1.2.1`. The Go scaffolding (cobra CLI, terraform-exec wrapper, client-go k8s wrapper, miekg/dns probe, four-role sprint pattern, mdBook framework, doctor framework, execution backends, cred resolver) is **inherited intact**. Sprint 0 strips IBM-specific surface; subsequent sprints layer AWS-specific surface on top. The roksbnkctl PRDs 01-05 (SSH/`--on`, internalised `kubectl`, execution backends, credentials, E2E plan) are inherited and their implementations carried forward; only `06-CLUSTER-TRIAL-PHASE-SPLIT.md` and the to-be-authored PRDs 07+ (EKS cluster, S3 supply chain, SR-IOV data plane) are net-new.

## Goals & top-level milestones

| Milestone | Tag | Outcome |
|---|---|---|
| **M0** | _(no tag)_ | Sprint 0: identity rewrite complete; `cmd/awsbnkctl/` builds green; IBM-specific surface stripped or stubbed; CI matrix passes on Linux + macOS. |
| **M1** | `v0.2` | Sprint 1: `terraform/modules/eks_cluster/` stands up an EKS cluster with a self-managed node group on ENA-enabled instances (`c5n`/`m5n`); Multus + SR-IOV CNI + SR-IOV device plugin land via DaemonSets; nodes advertise SR-IOV VFs to the scheduler. |
| **M2** | `v0.3` | Sprint 2: S3 supply chain (FAR pull keys, JWT licence, optional ECR mirror) replaces IBM COS; IRSA via the EKS OIDC provider replaces the IBM Trusted Profile workload-identity wiring. |
| **M3** | `v0.5` | Sprint 3: `cert_manager`, `flo`, `cne_instance`, `license` modules ported to consume AWS-shaped inputs (S3 bucket, IRSA role ARN). `awsbnkctl up` runs the full lifecycle against a real AWS account; BNK comes up healthy. |
| **M4** | `v0.7` | Sprint 4: `awsbnkctl test` (DNS + connectivity + throughput) passes against the AWS deployment; `awsbnkctl doctor` reports AWS-shaped checks (STS caller-identity, EKS describe-cluster permissions, instance-type quotas). |
| **M5** | `v0.9` | Sprint 5: book retargeted at AWS — every chapter that references IBM-Cloud-specific behaviour rewritten; new chapters for the SR-IOV node-group decision and the S3/ECR supply chain. Web book published at `https://JLCode-tech.github.io/awsbnkctl/book/`. |
| **M6** | `v1.0` | Sprint 6: E2E phases pass on a stock dev host (no `kubectl`/`aws` installed); security review clean; first stable release with goreleaser-built binaries for Linux/macOS/Windows × amd64/arm64. |

Estimated calendar time: **~13 weeks** (Sprint 0 ≈ 1 week + six 2-week sprints) for a single focused engineer who already knows EKS, Terraform, and the roksbnkctl codebase. Doubling for "real-world with reviews, distractions, integration debt, and SR-IOV surprises" puts M6 around **6 months out**.

### What we inherit (vs. roksbnkctl's 14-week build-from-scratch plan)

awsbnkctl's plan is shorter than roksbnkctl's because the following work is **already done** in the inherited tree and does not need redoing — only retargeting where it touches cloud-specific surface:

- SSH client + `--on` flag (roksbnkctl PRD 01)
- Internalised `kubectl` via client-go (roksbnkctl PRD 02)
- Execution backends: local, docker, k8s, ssh (roksbnkctl PRD 03)
- Credential resolver chain (roksbnkctl PRD 04) — needs AWS adapter
- E2E test framework (roksbnkctl PRD 05) — needs AWS test phases
- Workspace config + cobra CLI scaffolding
- mdBook framework + `make book` / `make book-serve` targets
- goreleaser config + release CI workflow
- Doctor framework — needs AWS-shaped checks

## Phase overview — sequencing decisions

```
┌────────────────────────────────────────────────────────────────────┐
│ Sprint 0 (week 0)        Identity rewrite + IBM strip              │
│                          README/CHANGELOG/MIGRATING done; module   │
│                          path + cmd/ rename; remove internal/ibm,  │
│                          internal/cos, terraform/modules/roks_*;   │
│                          stub internal/aws; green build + CI       │
├────────────────────────────────────────────────────────────────────┤
│ Sprint 1 (weeks 1-2)     EKS cluster + self-managed SR-IOV node    │
│                          group (PRD 07 - to author). Spike first:  │
│                          can we get SR-IOV VFs schedulable on a    │
│                          c5n.4xlarge node? Then productise.        │
│   ↓                                                                │
│ Sprint 2 (weeks 3-4)     S3 supply chain + IRSA (PRD 08 - to       │
│                          author). FAR pull-key upload, JWT licence │
│                          handling, optional ECR mirror, FLO        │
│                          service-account → IAM role binding.       │
│   ↓                                                                │
│ Sprint 3 (weeks 5-6)     Port the four reusable modules:           │
│                          cert_manager, flo, cne_instance, license. │
│                          Swap IBM inputs for AWS inputs; first     │
│                          end-to-end `awsbnkctl up` against AWS.    │
│   ↓                                                                │
│ Sprint 4 (weeks 7-8)     Test surface + doctor: AWS-shaped checks, │
│                          DNS/connectivity/throughput tests pass    │
│                          against the AWS deployment, AWS-flavoured │
│                          E2E phases.                               │
│   ↓                                                                │
│ Sprint 5 (weeks 9-10)    Book retarget: rewrite all IBM-specific   │
│                          chapters, add SR-IOV + S3/ECR chapters,   │
│                          publish at GitHub Pages.                  │
│   ↓                                                                │
│ Sprint 6 (weeks 11-12)   Hardening, security review, E2E gate,     │
│                          v1.0 cut with goreleaser binaries.        │
└────────────────────────────────────────────────────────────────────┘
```

**Dependency rationale:**
- The SR-IOV node group (Sprint 1) blocks every subsequent BNK deployment — if it can't work on EKS without surprise, the whole plan needs to revisit the data-plane decision.
- S3 + IRSA (Sprint 2) shapes how `flo` and `cne_instance` modules are parameterised in Sprint 3, so it must precede module porting.
- Module porting (Sprint 3) is the first sprint that produces a working `awsbnkctl up`. Everything before is enablement.
- Test surface (Sprint 4) gates the v0.7 milestone and surfaces correctness issues that drive Sprint 5/6 polish.
- Book retarget (Sprint 5) is large but mostly mechanical — done in one focused sprint so dev sprints aren't slowed by chapter writes.
- Hardening (Sprint 6) is the final v1.0 gate.

---

## Sprint 0 — identity rewrite + IBM strip (week 0)

### Goal

Make the fork look and build like `awsbnkctl`: module path, binary name, repo identity, CI green. Strip the IBM-specific surface so subsequent sprints layer AWS-specific surface against a clean base.

### Code deliverables

| Item | Detail |
|---|---|
| Identity rewrite | `README.md`, `CHANGELOG.md`, `MIGRATING.md` (done, uncommitted at the time of writing) |
| Go module path | `go.mod`: `module github.com/JLCode-tech/awsbnkctl`. Update every import; `gofmt`/`go vet` clean. |
| Binary rename | `cmd/roksbnkctl/` → `cmd/awsbnkctl/`. Update `Makefile`, `.goreleaser.yml`, `embedded.go`, CI workflows. |
| IBM strip | Delete `internal/ibm/`, `internal/cos/`, `terraform/modules/roks_cluster/`. Delete `tools/docker/ibmcloud/`. |
| AWS stub | Empty `internal/aws/` package with a doc.go explaining future scope. Empty `terraform/modules/eks_cluster/` with a placeholder `main.tf` that errors out cleanly. |
| Variable + reference cleanup | Strip `ibmcloud_*` variables from `terraform/variables.tf`; replace top-level `main.tf` module wiring with TODO stubs that fail-loud rather than silently misbehave. |
| Build green | `make build && make test && go vet ./...` all pass. The binary doesn't *do* anything yet — but it builds, parses flags, and prints help. |

### Documentation deliverables

| Item | Detail |
|---|---|
| `README.md` / `CHANGELOG.md` / `MIGRATING.md` | Already drafted in this fork's working tree; commit alongside the rename. |
| `docs/PLAN.md` | This file. |
| `docs/prd/00-OVERVIEW.md` | Inherited from roksbnkctl; rewrite the top section to frame awsbnkctl's scope (AWS, not IBM Cloud). PRDs 01-05 stay inherited; PRD 06 stays inherited; flag PRDs 07-08 as "to author in Sprints 1-2". |
| `agents/` | Inherited unchanged — the role definitions are tool-agnostic. |
| `prompts/sprint0/{architect,staff,validator,tech-writer}.md` | Author from scratch using roksbnkctl's `prompts/sprint0/` as the template. Sprint scope = identity rewrite + IBM strip + AWS stub. |

### Test deliverables

- Existing test suite still green after the strip (most IBM-specific tests deleted alongside their packages; the `internal/cli`, `internal/tf`, `internal/k8s` test suites should pass unchanged).
- CI matrix runs: Linux + macOS, `go test`, `gofmt`, `go vet`, `staticcheck`.
- `mdbook build book/` still passes (chapter content will be rewritten in Sprint 5; the framework stays).

### Gate to Sprint 1

- `awsbnkctl --help` runs and prints the expected command list.
- `awsbnkctl doctor` runs (may report "not implemented for AWS yet" — but no panics).
- No `ibmcloud` / `roks` / `cos` string references remain in the build path (allowed in `book/` and `CHANGELOG.md` historical sections).
- A working tree on `main` with one or more commits attributable to "fork + retarget".

### Risks

- Import-path rewrite touches every Go file. Budget half a day; use a single `gofmt -w` + `find . -name '*.go' -exec sed -i …` pass, then `go build` to find stragglers.
- Some inherited tests assume IBM environment vars or live ROKS connectivity. Skip with `t.Skip("requires AWS retarget in Sprint N")` rather than deleting — flag for replacement in the relevant later sprint.

---

## Sprint 1 — EKS cluster + SR-IOV node group (PRD 07 — to author)

### Goal

Stand up an EKS cluster with a self-managed node group that exposes SR-IOV VFs to the scheduler. Validate the load-bearing data-plane decision before any further work depends on it.

### Spike (days 1-3)

Before productising in Terraform, hand-stand-up the target shape:

1. Create an EKS cluster (1.30+) via `eksctl` or raw `aws` CLI.
2. Add a self-managed node group on `c5n.4xlarge` (or `m5n.4xlarge`) with a custom launch template that exposes ENA + the SR-IOV-capable ENIs.
3. Install Multus (`multus-cni`) + SR-IOV CNI + SR-IOV device plugin via the upstream manifests.
4. Verify VFs appear as `intel.com/sriov` (or whatever `sriov_resource_name` the eks_cluster module is configured with) in `kubectl describe node`.
5. Schedule a test pod that requests one VF; confirm it gets an additional interface from the SR-IOV pool.

If the spike fails or surfaces blockers (e.g., AWS ENI VF semantics don't match BNK's expectations), pause and revisit the data-plane decision in PRD 07.

### Productisation (days 4-10)

| Item | Detail |
|---|---|
| `terraform/modules/eks_cluster/` | Wraps `terraform-aws-modules/eks/aws ~> 20.x`. Inputs: `region`, `cluster_name`, `cluster_version`, `vpc_id` (or create-new), `node_instance_types` (default `["c5n.4xlarge"]`), `node_min_size`, `node_max_size`. Outputs: `cluster_name`, `cluster_endpoint`, `cluster_ca_data`, `oidc_provider_arn`, `cluster_ready_id`. |
| Self-managed node group | Custom launch template (`aws_launch_template`) with ENA + SR-IOV ENI configuration; uses the EKS-optimised AL2023 AMI for the cluster K8s version. |
| Multus + SR-IOV stack | Deployed via the `kubernetes` provider as part of the module: Multus DaemonSet, SR-IOV CNI DaemonSet, SR-IOV device plugin DaemonSet with a `ConfigMap` declaring the VF pool. |
| `internal/aws/` | aws-sdk-go-v2 client for STS (caller identity), EC2 (describe instance type availability + quotas), EKS (describe-cluster, fetch kubeconfig post-apply). |
| `awsbnkctl up cluster` | Drives `terraform/modules/eks_cluster/` only. `awsbnkctl up` (no subcommand) still gated — Sprint 3 closes the full lifecycle. |

### Documentation deliverables

- `docs/prd/07-EKS-CLUSTER-SRIOV.md` — PRD authored in parallel with the spike; codifies the SR-IOV-on-self-managed-nodes decision, the instance-type matrix, the upgrade path implications.

### Test deliverables

- Spike post-mortem written into the PRD (what worked, what surprised).
- Integration test: `awsbnkctl up cluster` against a real AWS account in a CI-controlled sub-account; cluster reports `Ready`; SR-IOV device plugin advertises VFs.
- Unit tests for the new `internal/aws/` helpers (STS caller identity, region/AZ enumeration).

### Gate to Sprint 2

- A `c5n.4xlarge` node in the cluster reports SR-IOV VFs as schedulable resources.
- `awsbnkctl up cluster --workspace test-sriov` is idempotent (rerun is a no-op when state is current).
- PRD 07 committed; spike learnings folded in.

### Risks

- **AWS ENI VF semantics may not match what BNK expects.** AWS exposes "Elastic Network Adapter" SR-IOV, which is not always interchangeable with the Mellanox-flavoured SR-IOV that BNK is typically validated against. Surface this in the spike; if BNK rejects the VF, fall back plan is the `vpc-cni` + multi-ENI shape (worse perf but possibly sufficient).
- Quota limits on `c5n` / `m5n` family in greenfield AWS accounts often hit `Running On-Demand instances` limits at low numbers. Doctor should pre-flight check.
- Launch-template AMI lifecycle is now user-managed. Document the upgrade path.

---

## Sprint 2 — S3 supply chain + IRSA (PRD 08 — to author)

### Goal

Replace the IBM COS supply chain with S3 (or ECR mirror) for FAR pull keys, JWT licence, and any other BNK artefact. Wire FLO's service account to an IAM role via IRSA so no static API key lives in cluster.

### Code deliverables

| Item | Detail |
|---|---|
| `terraform/modules/s3_supply_chain/` | S3 bucket (server-side encrypted with `aws:kms`), bucket policy restricting access to the FLO IRSA role, `aws_s3_object` resources for FAR pull-keys + JWT licence. Inputs: `bucket_name`, `kms_key_arn`, `far_repo_url`, `jwt_file_local_path`, `far_auth_file_local_path`. |
| `terraform/modules/ecr_mirror/` (optional) | Stretch: mirror FAR images into ECR via `aws_ecr_repository` + a `null_resource` running `skopeo copy`. Drives the v1.x ECR option without blocking v0.5. |
| `terraform/modules/iam_irsa/` | IAM OIDC provider (referencing the EKS OIDC URL from Sprint 1), trust-policy-shaped role for FLO. Outputs: `flo_role_arn` consumed by the `flo` module in Sprint 3. |
| `internal/aws/s3.go` | Wraps aws-sdk-go-v2 S3 for put/get; used by `awsbnkctl init` to upload local pull-key + JWT files. |
| `internal/aws/iam.go` | OIDC provider lookup, role existence checks for doctor. |
| `awsbnkctl init` AWS path | Wizard prompts: region, VPC, instance types, bucket name (default derived from workspace), and either "upload my local FAR tarball" or "I'll point at an existing S3 path". |

### Documentation deliverables

- `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md`.
- Note in CHANGELOG: the FAR artefacts that previously lived under `keys/` (per roksbnkctl convention) now live in S3; local-only fallback is a `tf_source.local` mode.

### Test deliverables

- Integration test: upload a dummy FAR tarball to S3 via `awsbnkctl init`; assert it's readable via the IRSA role from an in-cluster pod.
- Unit tests for `internal/aws/s3` and `internal/aws/iam`.

### Gate to Sprint 3

- A pod running under the FLO service account can read the S3-stored FAR + JWT artefacts using IRSA (no static credentials in the pod spec).
- PRD 08 committed.

### Risks

- IRSA trust policy is fiddly — typos in the OIDC subject claim are silent failures. Add `doctor` checks that test the assume-role flow end-to-end.
- BNK's licence artefact format hasn't (as of this writing) been validated against an S3 source by F5 directly. Spike: read the FLO source to confirm it accepts an HTTPS URL (S3 presigned) for licence pulls; if not, the `cne_instance` module needs a download-step shim.

---

## Sprint 3 — port the four reusable modules; first end-to-end `up` (weeks 5-6)

### Goal

`awsbnkctl up` runs end-to-end against an AWS account and lands a healthy BNK deployment. The four "reusable" Terraform modules — `cert_manager`, `flo`, `cne_instance`, `license` — get parameter-swapped to consume AWS inputs.

### Code deliverables

| Item | Detail |
|---|---|
| `terraform/modules/cert_manager/` | Port: replace `ibmcloud_*` inputs with `aws_*` equivalents (region, role ARNs). Module body is unchanged (pure k8s manifests). |
| `terraform/modules/flo/` | Port: swap `ibmcloud_cos_*` for `s3_bucket_name` + `flo_role_arn` (IRSA). FLO values rendered to point at S3 URLs instead of COS endpoints. |
| `terraform/modules/cne_instance/` | Port: takes `flo_namespace`, `flo_trusted_profile_id` → renamed to `flo_irsa_role_arn`. CNEInstance CRD body unchanged. |
| `terraform/modules/license/` | Port: licence JWT pulled from S3 (presigned URL) instead of COS. |
| `terraform/modules/testing/` | Port: replaces `roks_transit_gateway_name` input with `aws_vpc_id` + `aws_subnet_ids`. |
| Top-level `terraform/main.tf` | Wire `eks_cluster` → `cert_manager` / `iam_irsa` / `s3_supply_chain` → `flo` → `cne_instance` → `license` / `testing`. Mirrors roksbnkctl's dependency graph. |
| `awsbnkctl up` (full lifecycle) | Runs init → plan → confirm → apply → fetch kubeconfig → wait for CNEInstance to report `Ready`. |
| `awsbnkctl down` | Runs the reverse: destroy modules in dependency-graph order, with stuck-finalizer cleanup baked in (lifted from roksbnkctl). |

### Documentation deliverables

- Update `docs/prd/06-CLUSTER-TRIAL-PHASE-SPLIT.md` if the phase-split semantics changed.
- Mark PRDs 07 + 08 as `Resolved in Sprint 3` where Sprint 3 closes their consumption.

### Test deliverables

- End-to-end smoke test on a CI-controlled AWS sub-account: `awsbnkctl init → up → kubectl get cneinstance → down`. Total runtime budget: ~30 min on EKS.
- Unit tests for terraform-exec wrapper updates (new inputs, new module wiring).

### Gate to Sprint 4

- `awsbnkctl up` on a clean AWS account completes inside 30 minutes and lands a healthy `CNEInstance`.
- `awsbnkctl down` on the same workspace returns the account to clean state (no leaked S3 objects, IAM roles, ENIs, or VPCs).

### Risks

- Module wiring order is fragile — terraform's dependency graph cares about explicit `depends_on` for cross-module readiness gates. roksbnkctl's pattern (each module exports a `ready_id` consumed downstream) carries over verbatim.
- BNK CNEInstance reconciliation can take 8-15 min; CI test timeouts need explicit budget.

### Sprint 3 close (actual)

What shipped (architect surface — confirmed at this commit):

- **PRD 04 update** — added a top-level §"Resolved in Sprint 3" section documenting the AWS standard credential chain (env / profile / SSO / IMDS / container / web-identity), the IRSA replacement of the upstream IBM Trusted Profile path, the AWS-retargeted backend × credential matrix, the AWS-shaped doctor rows, and the migration steps from `roksbnkctl`. The inherited "Resolved in Sprint 9" IBM-cloud sections are retained as historical context (the upstream's v1.2 surface).
- **PRD 08 versioning correction** — Decision § now documents that `aws_s3_bucket_versioning.status = "Enabled"` ships unconditionally (no `var.enable_bucket_versioning` toggle), with the audit-trail-outweighs-storage-cost rationale and a corresponding entry in the Trade-offs accepted section. Resolves the Sprint 2 tech-writer Issue 2 PRD ↔ module drift.
- **Chapter 26 first-pass** — `book/src/26-troubleshooting.md` replaces the inherited IBM-flavoured catalogue with ~1,700 words of AWS-shaped symptom → root-cause → fix entries across cluster + node group (SR-IOV VF advertisement, CNEInstance pending), AWS creds + auth (STS chain resolution, IRSA `AccessDenied`), EKS access (kubeconfig context, access-entry mapping), terraform + quotas (vCPU `VcpuLimitExceeded`, two-AZ subnet requirement, orphan ENI cleanup), and CI-specific (provider-cache invalidation, cred-leak audit). Cross-links to chapters 23, 25, 33 + PRD 04.
- **Chapter 25 cross-link refresh** — chapter 25's chapter-26 cross-reference now points at a concrete section (`§"AWS credentials + auth"`) instead of the prior forward-reference framing.

What was deferred (carries into Sprint 4 or later):

- **Operator-run spike** (PRD 07 § Spike status) still gates `v0.2`. Sprint 3 closes offline work; live `awsbnkctl apply` against AWS waits on the operator.
- **Doctor refresh** (`internal/doctor/aws.go`) is Sprint 4. The AWS-shaped doctor rows the PRD 04 update documents land then; PRD 04 codifies the contract Sprint 4 implements.
- **ECR mirror v1.x first-class** (PRD 08 § Trade-offs).
- **Chapter 14 deep rewrite** (current chapter targets `IBMCLOUD_API_KEY`; Sprint 5 retargets at the AWS chain) — the PRD 04 cred-chain section is the design surface that rewrite will draw from.
- **Chapter 25 filename rename** (`25-cos-supply-chain.md` → `25-s3-supply-chain.md`) — Sprint 5 architect, per Sprint 2 architect Issue 2 cascade plan.

Sibling agent surfaces (staff terraform module ports, validator CI matrix, tech-writer read-only pass) report independently; see the per-agent issue files at `issues/issue_sprint3_{staff,validator,tech-writer}.md` for what landed on those surfaces. The integrator reconciles at sprint close.

---

## Sprint 4 — test surface + doctor (weeks 7-8)

### Goal

`awsbnkctl test` and `awsbnkctl doctor` give AWS-shaped feedback. DNS / connectivity / throughput tests pass against the Sprint 3 deployment.

### Code deliverables

| Item | Detail |
|---|---|
| `internal/doctor/aws.go` | New checks: AWS credentials present (env / profile / instance role / SSO); STS caller-identity resolves; EKS describe-cluster permission; EC2 quota for `c5n` / `m5n` family in the chosen region; S3 PutObject permission on the workspace bucket. |
| `internal/test/throughput.go` | Inherited from roksbnkctl; verify the `--backend k8s` Job pattern works on EKS (no OpenShift-specific SCC assumptions). |
| `internal/test/dns.go` | Inherited; the multi-vantage probe works against AWS-hosted GSLB targets without modification. Add an AWS-flavoured vantage if needed. |
| `internal/test/connectivity.go` | Inherited; validate against EKS LoadBalancer service shape (ALB/NLB). |
| `awsbnkctl test` AWS-aware | Plumb workspace region + cluster outputs through so the test surface picks them up automatically. |

### Documentation deliverables

- Update the troubleshooting chapter (Sprint 5 will fold these into the book) with common Sprint 4-surfaced issues.

### Test deliverables

- All three test verbs (`dns`, `connectivity`, `throughput`) pass against a Sprint 3 deployment.
- Doctor reports green on a stock dev box with only `terraform` and AWS credentials installed.

### Gate to Sprint 5

- `awsbnkctl doctor` reports green on the CI runner.
- `awsbnkctl test` passes end-to-end on the Sprint 3 deployment.

### Risks

- OpenShift's SCC-based pod security (which roksbnkctl's tools image accommodates) doesn't apply on EKS; instead, EKS 1.25+ enforces Pod Security Admission. Verify the tools image runs unprivileged under `restricted` PSS.

---

## Sprint 5 — book retarget (weeks 9-10)

### Goal

The book at `book/src/` reads as awsbnkctl's documentation, not roksbnkctl's. Web book publishes at `https://JLCode-tech.github.io/awsbnkctl/book/`.

### Documentation deliverables

| Chapter | Action |
|---|---|
| 1. What is BNK | Inherited; minor edits for AWS context |
| 2. Why ROKS | **Rewrite** as "Why EKS + self-managed SR-IOV node groups"; cite PRD 07 |
| 3. What awsbnkctl does | Rewrite |
| 4. Installation | Edit: drop `ibmcloud` mentions; update download URL |
| 5. Doctor | Edit: replace IBM-flavoured checks with AWS checks |
| 6. Workspaces | Inherited |
| 7. Quick start | Rewrite around `awsbnkctl up` AWS path |
| 8-11. Cluster lifecycle | Edit per AWS specifics |
| 12. Workspace config | Edit: new fields (`aws_region`, `node_instance_types`) |
| 13. Terraform variables | Auto-regenerate via `tools/tfvars-md` |
| 14. Credentials and the resolver chain | Edit: AWS credential chain replaces IBM Cloud chain |
| 15. SSH targets | Inherited |
| 16-19. Remote execution | Inherited |
| 20-23. Testing | Inherited mostly; edit DNS chapter for AWS GSLB nuances |
| 24. Day-2 ops | Edit per AWS specifics |
| 25. COS supply chain | **Rewrite** as "S3 (and optional ECR) supply chain"; cite PRD 08 |
| 26. Troubleshooting | Rewrite top-N issues for AWS |
| 27-30. Reference | Auto-regenerate via `tools/cobra-md` + `tools/tfvars-md` |
| 31-32. Contributing | Edit: update build-from-source instructions |
| _new_ 33. The data-plane decision | **New chapter**: walk through SR-IOV options on EKS, why self-managed node groups won, AMI lifecycle implications |

### Code deliverables

- GitHub Actions workflow `.github/workflows/book.yml` — already inherited; update the `gh-pages` deploy target so it lands at JLCode-tech.github.io.
- Update `tools/cobra-md` and `tools/tfvars-md` reference generators to emit awsbnkctl-shaped output.

### Test deliverables

- `mdbook test book/` passes (link integrity).
- `cspell` clean on `book/src/**/*.md`.
- Visual spot-check: walk Chapters 1-7 as a first-time reader; record issues in `issues/issue_sprint5_tech-writer.md`.

### Gate to Sprint 6

- Book builds clean, deploys to GitHub Pages, no broken cross-links.
- A first-time reader can follow the quick start without referring back to roksbnkctl's docs.

### Risks

- Book rewrites are scope-creep magnets. Stick to the chapter-by-chapter triage above; defer rewording to v1.x polish.

---

## Sprint 6 — hardening + v1.0 cut (weeks 11-12)

### Goal

E2E phases pass on a stock dev host. Security review clean. v1.0 binary attached to a GitHub Release.

### Code deliverables

| Item | Detail |
|---|---|
| E2E test phases | Adapt roksbnkctl's E2E phases A-N + L-DNS to AWS shapes. Document in `docs/prd/05-E2E-TEST-PLAN.md`. |
| Security audit | `gosec ./...` clean; secrets scan clean (no AWS keys, no JWT bodies, no FAR tarballs accidentally committed). |
| Doctor refresh | Final pass: every check has a useful failure-mode message. |
| `awsbnkctl self update` | Validate the inherited self-update path against awsbnkctl's release tags + checksums. |
| Release prep | `make release` end-to-end; goreleaser builds Linux/macOS/Windows × amd64/arm64; book PDF attached. |

### Documentation deliverables

- Final book pass — diagrams, screenshots, cross-link integrity.
- `CHANGELOG.md` rolls up the seven-sprint history into a `v1.0.0` entry.
- `docs/PLAN.md` (this file) gets a "v1.x deferred work" appendix.

### Test deliverables

- All E2E phases pass on a stock dev host (no `kubectl`, no `aws` CLI, no `iperf3`, no `dig` installed).
- CI matrix green on Linux + macOS (Windows compile check is a stretch).

### Gate to v1.0

- E2E green.
- Security review green.
- Book published and dogfooded.
- A tagged `v1.0.0` Release with binaries + checksums + book PDF.

### Risks

- Late-discovered AWS quotas or instance-type availability issues in untested regions. Mitigate: declare a "tested regions" list (us-east-1, us-west-2, eu-west-1) in the docs; everything else is best-effort.

---

## v1.x deferred work (post-v1.0)

These deliberately defer past v1.0 to keep the M6 scope finite:

- **ECR mirror story closure.** Sprint 2 ships an optional ECR mirror module; v1.x makes it the default for air-gapped customers and adds an automated FAR-image sync workflow.
- **EKS Auto Mode + AWS Karpenter integration.** The Sprint 1 spike validates against a fixed node group; v1.x adds support for elastic / Karpenter-managed node groups if AWS ENA-SR-IOV semantics permit.
- **AWS-native cred backends.** v1.x evaluates SSO and IAM Identity Center first-class support in the cred resolver chain.
- **Homebrew tap.** Same as roksbnkctl — on the v1.x roadmap.
- **Cross-region GSLB testing.** Sprint 4's DNS probe handles single-region GSLB; multi-region is a v1.x stretch.

---

## Book outline (the full SUMMARY.md target)

Sprint 5 lands the rewrites. The chapter map below is **the SUMMARY.md target** — chapters carry through from roksbnkctl unless noted, with the AWS-specific additions called out.

```
PART I — CONCEPTS
  1. What is BIG-IP Next for Kubernetes (BNK)
  2. Why EKS + self-managed SR-IOV node groups            [rewritten]
  3. What awsbnkctl does (and doesn't do)                 [rewritten]

PART II — GETTING STARTED
  4. Installation
  5. Doctor: checking your environment                    [AWS-flavoured]
  6. Workspaces
  7. Quick start: from AWS credentials to deployed BNK    [rewritten]

PART III — CLUSTER LIFECYCLE
  8. The cluster phase (cluster up/down)
  9. Registering an existing cluster
  10. Deploying BNK trials on top
  11. Tearing down

PART IV — CONFIGURATION
  12. Workspace config (config.yaml)                      [new AWS fields]
  13. Terraform variables (terraform.tfvars)              [auto-regen]
  14. Credentials and the AWS resolver chain              [rewritten]
  15. SSH targets

PART V — REMOTE EXECUTION
  16. The --on flag and SSH jumphosts
  17. Execution backends: local, docker, k8s, ssh
  18. Choosing a backend per tool
  19. The in-cluster ops pod

PART VI — TESTING
  20. Connectivity testing
  21. DNS testing for GSLB
  22. Throughput testing
  23. The E2E test plan

PART VII — OPERATIONS
  24. Day-2 ops: status, logs, k get/apply/exec
  25. S3 (and optional ECR) supply chain                  [rewritten]
  26. Troubleshooting                                     [AWS-flavoured]

PART VIII — REFERENCE
  27. Command reference                                   [auto-regen]
  28. Configuration reference                             [auto-regen]
  29. Terraform variable reference                        [auto-regen]
  30. Glossary

PART IX — CONTRIBUTING
  31. Building from source
  32. Extending awsbnkctl

PART X — DESIGN DECISIONS
  33. The data-plane decision: SR-IOV on EKS              [new]
```

---

## How this plan is maintained

- **Sprint kickoff:** the integrator drafts four `prompts/sprint<N>/{architect,staff,validator,tech-writer}.md` briefs referencing the relevant sections above, then dispatches the four-role parallel-agent pattern per [`prompts/README.md`](../prompts/README.md).
- **Sprint close:** the architect agent updates this file's per-sprint section with what actually shipped vs. planned, and folds any deferred work into the v1.x section.
- **PRD drift:** when a sprint surfaces a design decision that conflicts with a PRD, the PRD is updated in the same commit as the implementation. The integrator catches PRD ↔ PLAN drift at tag-cut time.
