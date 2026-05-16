# Changelog

All notable changes to `awsbnkctl` are documented in this file. Format follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) convention; the project uses [semantic versioning](https://semver.org/spec/v2.0.0.html).

Per-sprint design rationale lives in [`docs/PLAN.md`](docs/PLAN.md); per-PRD design specs live under [`docs/prd/`](docs/prd/). This file is the user-facing summary of what changed between releases.

awsbnkctl began as a hard fork of [`jgruberf5/roksbnkctl`](https://github.com/jgruberf5/roksbnkctl) at its `v1.2.1` tag. roksbnkctl's pre-fork changelog is preserved in git history on `upstream/main` and at <https://github.com/jgruberf5/roksbnkctl/blob/main/CHANGELOG.md>; this file starts fresh at the awsbnkctl fork point.

## Unreleased

### Added — Sprint 6 (Hardening + ops-pod IRSA retarget + v0.9-rc release-artefact prep)

**Final sprint.** The repository is now structurally complete for `v0.9-rc1`. The first stable `v1.0` tag waits on the operator-run PRD 07 spike (validating SR-IOV VFs on a real EKS `c5n.4xlarge` node) per `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Spike status".

- **Sprint 5 BLOCKER closures (staff):** `internal/exec/k8s_install.yaml` retargeted end-to-end from IBM Trusted-Profile shape to AWS IRSA shape. Dropped the `roksbnkctl-ibm-creds` Secret + `IBMCLOUD_API_KEY` mount + IBM SA-annotation patch hook; added `eks.amazonaws.com/role-arn` so the EKS pod-identity webhook auto-injects `AWS_ROLE_ARN` + `AWS_WEB_IDENTITY_TOKEN_FILE`. Namespace/label/SA names retargeted to `awsbnkctl-*`. ClusterRole `secrets:get` → namespaced `secrets:{create,delete,patch}` in `awsbnkctl-test` (per-Job ephemeral files). Closes Sprint 5 tech-writer Issue 1.
- **Sprint 5 chapter blocker closures (architect):** chapters 8, 9, 11 gained explicit "Available in v1.x" banners on `awsbnkctl cluster <subverb>` / `awsbnkctl bnk <subverb>` references that don't ship in v0.9. Chapter 19 banner updated to reflect Sprint 6's YAML retarget. Closes Sprint 5 tech-writer Issue 2.
- **Glossary (architect):** chapter 30 rewritten — CRN deleted, S3 / CIS / EKS / OpenShift / Trusted Profile / VPE/VPC-endpoint / TGW / VSI/EC2-instance / SCC entries corrected; new PSA, IRSA, IMDS, redactor, envFrom, Secret, runAsNonRoot, restricted-v2, cred-resolver-chain entries. Cross-link anchors to chapters 14/25/26 fixed. Closes Sprint 5 tech-writer Issue 3.
- **Secondary IBM-residue sweep (architect):** chapters 17, 18, 32 swept clean of IBM Cloud / ROKS / COS prose; `Credentials.IBMCloudAPIKey` references deleted, Secret names rewritten to `awsbnkctl-aws-creds`, `toolImages` / `toolPackages` examples retargeted, AWS-CLI-v2 install recipe fixed. Closes Sprint 5 tech-writer Issue 4.
- **README sprint-count refresh (architect):** status banner now reads "Sprint 6 complete; v0.9-rc ready; v1.0 awaits spike". Quick start no longer prefixed "Planned"; fork-relationship section updated.
- **PLAN.md "What's deferred to post-v1.0" appendix (architect):** consolidates every "v1.x revisit" note from PRDs 07-08 and every Sprint 0-5 open issue into one place. Eight subsections covering ops-pod IRSA auto-provision, ECR mirror first-class story, Karpenter / EKS Auto Mode evaluation, Calico CNI alternative, AWS Secrets Manager for JWT, multi-region GSLB, air-gapped install, Homebrew tap.
- **Security audit (staff):** `gosec ./...` 55 findings catalogued (10 HIGH known-design/false-positives + 20 G301/G306 user-config file-perm posture accepted; rationale in staff Issue 1). `govulncheck` clean post-bump of `golang.org/x/net` v0.50.0 → v0.53.0 (closes GO-2026-4918 + GO-2026-4559). Secrets scan: no static credentials in source.
- **goreleaser (staff):** verified `goreleaser build --snapshot --clean` produces six binary archives — linux/macOS/windows × amd64/arm64. `.goreleaser.yml` two surviving "IBM Cloud ROKS" strings retargeted to "AWS EKS".
- **security-audit CI job (validator):** gosec + govulncheck + gitleaks, fail-on-finding, runs on every PR + main push. `book-build` cspell step extended to also cover `docs/**/*.md`.
- **cspell (validator):** +49 entries (AWS / Go / k8s domain vocab, British spellings, project placeholders, `subverb`/`subverbs`). Total 416 words. Zero unknown-word findings across 48 files.
- **e2e-full.yml (validator):** header + step banner refreshed from Sprint 4 framing to Sprint 6 / v1.0-cut framing; preserved trigger surface verbatim.

### Integrator folds (Sprint 6 tech-writer blockers)

- **Chapter 13 (Terraform variables)** — retargeted stale `roks_workers_per_zone` / `openshift_cluster_name` / `roks_min_worker_vcpu_count` references at AWS-shaped `node_*` / `cluster_*` / `node_instance_types`. S3 HMAC keys paragraph rewritten for IRSA-shaped supply chain. Closes Sprint 6 tech-writer Issue 3 (BLOCKER).
- **Chapter 19 banner** — corrected the stale "do not run ops install" banner to reflect Sprint 6 staff's actual YAML retarget. Closes Sprint 6 tech-writer Issue 2 (BLOCKER).
- **CHANGELOG Sprint 6 entry** — this section. Closes Sprint 6 tech-writer Issue 1 (BLOCKER).

### v0.9-rc1 ready

All six AWS-retarget sprints (Sprint 0 → Sprint 6) have landed. Repository state:
- `go build / vet / test / gofmt` clean across all 13 packages.
- `terraform validate` clean on root + all 8 modules (`eks_cluster`, `cert_manager`, `s3_supply_chain`, `iam_irsa`, `ecr_mirror`, `flo`, `cne_instance`, `license`, `testing`).
- `awsbnkctl --help` / `init --dry-run` / `up --dry-run` / `down --dry-run` / `test {connectivity,dns,throughput} --dry-run` / `doctor` all work end-to-end offline.
- `awsbnkctl doctor` reports six AWS pre-flight rows (credentials configured, STS caller-identity, EKS DescribeCluster permission, EC2 vCPU quota, S3 PutObject feasibility, IAM:GetRole FLO IRSA) when a workspace is configured.
- Book published-ready (`mdbook build book/` clean in CI; cspell zero findings).
- goreleaser snapshot produces six binary archives.
- `gosec` + `govulncheck` clean (open findings documented and accepted).
- CI matrix: 10 jobs (test × Linux/macOS, windows-build, integration, aws-mocked, full-up-dryrun, test-dryrun, security-audit, book-build, spellcheck, goreleaser-check).

### Added — Sprint 5 (Book retarget + IBM-residue sweep + reference chapter regen)

- **Book retarget (architect):** chapters 1, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14 rewritten or substantially edited from IBM Cloud / ROKS / COS framing to AWS / EKS / S3 framing. Mechanical sweeps applied to chapters 2, 3, 13, 15-19, 24, 28, 30-33 (`roksbnkctl`→`awsbnkctl`, `ROKS`→`EKS`, `COS`→`S3`, `IBMCLOUD_API_KEY`→`AWS_ACCESS_KEY_ID`, `IBM Cloud`→`AWS`). Chapter 26 troubleshooting gained sub-anchors (`### AWS LoadBalancer`, `### DNS`) closing Sprint 4 tech-writer Issue 4.
- **IBM-residue tech-debt sweep (staff):** `.go` surface from **297 → 1 hit**. Deleted `internal/cred/` package entirely; removed `Credentials.IBMCloudAPIKey` + per-backend cred-shim plumbing; deleted 9 obsolete IBM-shim test files; trimmed 214 lines of legacy API-key code from `internal/config/secrets.go`. The remaining hit is `/ibmcloud_api_key` keychain user-key in `DeleteAPIKeyFromKeychain` — intentional v0.x→v0.9 migration helper. Closes Sprint 3 tech-writer Issue 2.
- **Reference chapter regeneration (staff):** `book/src/27-command-reference.md` (858 lines) regenerated via `tools/refgen/cobra-md` against current AWS-shaped CLI surface; `book/src/29-terraform-variable-reference.md` (206 lines) regenerated via `tools/refgen/tfvars-md` against current Terraform variables.
- **CI book gate (validator):** `.github/workflows/ci.yml` gains a `book-build` job — `mdbook build book/` + `cspell` hard-fail on `book/src/**/*.md`. Runs on every PR.
- **cspell dictionary (validator):** +136 entries across British spellings (`behaviour`, `customise`, `optimised`), cloud / k8s / AWS domain vocabulary (`batchv`, `corev`, `clientset`, `cneinstance`, `kustomization`, `resourcegroupstaggingapi`, `vcpu`, `Gbps`), and chapter-specific terms (`retargeted`, `tunables`, `unparseable`). Total: 366 words; zero unknown-word findings on `book/src/**/*.md`.

### Carried into Sprint 6 (blockers per tech-writer review)

- `internal/exec/k8s_install.yaml` still entirely roksbnkctl-shaped (ops-pod manifest the IBM-residue grep missed because it's YAML not Go). Sprint 6 staff retargets.
- Chapters 8, 9, 11 reference `cluster` / `bnk` subverbs that don't exist on the binary surface today. Sprint 6 architect either retargets prose or annotates as v1.x roadmap.
- Chapters 17, 18, 19, 32 carry secondary IBM-residue prose (cluster-lifecycle subtree). Sprint 6 architect sweep.
- Glossary (chapter 30) has factual errors — Sprint 6 architect cleanup.
- README.md sprint-count framing stale (says 5 remaining sprints; actually finishing Sprint 6 next).

### Added — Sprint 4 (Test surface refresh + doctor polish + AWS E2E phases)

- `internal/cli/test.go` + `internal/cli/test_dryrun.go` — `awsbnkctl test {connectivity,dns,throughput,all}` plumbed with workspace-derived region + cluster + namespace defaults; `--dry-run` flag plans the probe without executing.
- `internal/k8s/iperf3.go` — Pod Security Admission (`restricted` profile) compliance: `runAsNonRoot: true`, `runAsUser: 1000`, `seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`. EKS 1.25+ ready.
- `internal/aws/servicequotas.go` + tests — optional `RunningOnDemandVCPUQuota` probe (Service Quotas `L-1216C47A`). Feature-flag gated (off by default).
- `internal/doctor/aws.go` — feature-flagged Service Quotas row alongside the existing six AWS pre-flight checks.
- `internal/cli/cluster.go::runFullLifecyclePlan` — first-run UX fix: catches "missing tfvars" terraform error, returns friendly "workspace not initialised — run `awsbnkctl init -w <name>` first" message. Closes Sprint 3 tech-writer Issue 3.
- `docs/prd/04-CREDENTIALS.md` § "Where the AWS chain lives in the tree" — rewritten to accurately describe the host-side cred chain (`internal/aws/{client,sts}.go`) vs. in-cluster IRSA (auto-injected by EKS pod-identity webhook). Closes Sprint 3 tech-writer Issue 1 (HIGH).
- `book/src/20-connectivity.md` — full chapter (~1,490 words): HTTPS probes, AWS NLB/ALB shape recognition, failure-mode interpretation.
- `book/src/21-dns-testing-gslb.md` — full chapter (~1,700+ words): miekg/dns vantages, GSLB divergence detection, Route 53 specifics.
- `book/src/22-throughput.md` — full chapter (~1,900+ words): iperf3-via-k8s-Job, PSA requirements, baseline expectations on c5n.4xlarge.
- `book/src/23-e2e-test-plan.md` — full chapter (~2,200+ words): phase-letter system A-N, AWS-flavoured phases, local-vs-CI execution.
- `.github/workflows/ci.yml` — new `test-dryrun` job: materialises a fake workspace under `~/.awsbnkctl/ci-dryrun/config.yaml`, runs all three test verbs with `--dry-run`, asserts exit-0.
- `cspell.json` — additions for `Route53`, `iperf3`, `PSA`, `seccomp`, `SCC`, `vantage`, `vantages`, `GSLB`, `divergence`.
- `scripts/e2e-test-backends.sh` + `scripts/e2e-test.sh` — phase markers K-N refined for Sprint 4 status.

### Added — Sprint 3 (Five reusable modules ported + first end-to-end `up --dry-run`)

- `terraform/modules/{cert_manager,flo,cne_instance,license,testing}/` — five inherited modules ported to consume AWS-shaped inputs (`aws_*` instead of `ibmcloud_*`, `eks_cluster_*` instead of `roks_cluster_*`, `s3_*` instead of `cos_*`, `irsa_role_*` instead of `trusted_profile_*`). Module bodies unchanged where PRD 00 said "ports unchanged"; outer wrappers rebuilt.
- `terraform/main.tf` — full dependency graph rewired: `eks_cluster` → `cert_manager` / `s3_supply_chain` / `iam_irsa` → `flo` → `cne_instance` → `license` / `testing`.
- `awsbnkctl up --dry-run` (no subcommand) — first end-to-end plan against the full graph. Live `apply` still gates on operator-run spike per PRD 07.
- `awsbnkctl plan` aliases `up --dry-run`; `awsbnkctl down --dry-run` plans the destroy graph.
- PRD 04 cred/exec retarget: `internal/cred/` + `internal/exec/` dropped `IBMCLOUD_API_KEY` env handling; AWS standard chain (env / profile / instance role / SSO) replaces it. IRSA is the in-cluster shape; no env-var injection needed for k8s backend.
- `internal/cli/doctor_backend.go` — retargeted from IBM Trusted-Profile ops-pod check to IRSA shape (probes `eks.amazonaws.com/role-arn` annotation + `AWS_WEB_IDENTITY_TOKEN_FILE` env).
- `Workspace.IBMCloud` back-compat alias removed (clean break). `internal/cli/legacy_helpers.go` trimmed.
- `internal/doctor/doctor.go` — `awsChecks` call relaxed from workspace-nil-gate to unconditional (closes Sprint 2 tech-writer Issue 4). `TestRunWithWhy_StockDevBox_NoWorkspace` updated. Six AWS rows now render on stock dev box (credentials warning + downstream skipped).
- `docs/prd/04-CREDENTIALS.md` — top-of-file "Resolved in Sprint 3" section: AWS standard credential chain documented; IRSA in-cluster shape; AWS backend × credential matrix; doctor surface; migration steps from IBM-API-key chain.
- `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` — versioning correction (PRD now matches `s3_supply_chain` module's "enabled unconditionally" for FAR/JWT artefact history).
- `book/src/26-troubleshooting.md` — full first-pass (~1,700 words): SR-IOV VF advertisement, CNEInstance pending, STS chain, IRSA AccessDenied, EKS kubeconfig context, vCPU quotas, two-AZ subnet rule, orphan ENIs, CI provider-cache, cred-leak audit.
- `book/src/25-cos-supply-chain.md` — chapter 26 cross-link refreshed.
- `.github/workflows/ci.yml` — new `full-up-dryrun` job runs `awsbnkctl up --dry-run` against fake AWS creds; asserts exit-0 and that plan output mentions all 8 modules.
- `scripts/e2e-test.sh` — phase markers refined (cluster phases A-H: "Sprint 3 implements dry-run; spike validates apply"; BNK trial phases: "live apply gates on spike"). All scripts still `exit 0`.
- `scripts/test-integration-aws.sh` — full-up-dryrun gate added alongside per-package tests.
- `cspell.json` — +33 module-terminology entries; staff `.tf` cspell findings reduced 46 → 4.

### Added — Sprint 2 (S3 supply chain + IRSA per PRD 08)

- `terraform/modules/s3_supply_chain/` — KMS-encrypted S3 bucket with bucket policy scoping `s3:GetObject` to the FLO IRSA role; `aws_s3_object` resources for the FAR pull-key archive + subscription JWT, sourced from local paths the operator provides at `awsbnkctl init` time.
- `terraform/modules/iam_irsa/` — IAM OIDC provider lookup (from PRD 07's `eks_cluster` outputs); trust-policy IAM role bound to the FLO service account (`system:serviceaccount:flo-system:flo-controller`); permission policy granting `s3:GetObject` on the supply-chain bucket + `kms:Decrypt` on the CMK.
- `terraform/modules/ecr_mirror/` — optional (gated on `var.enable_ecr_mirror`, default false): per-image ECR repositories + `skopeo copy` from F5's FAR registry. v1.0 stretch; v1.x first-class for air-gapped customers.
- `internal/aws/s3.go` + `internal/aws/iam.go` — aws-sdk-go-v2 helpers (`PutObject`, `HeadObject`, `HeadBucket`, `GetOIDCProvider`, `HasIRSARole`) + mocked unit tests.
- `awsbnkctl init` AWS-shaped wizard — region, VPC, FAR archive path, JWT path, FLO namespace prompts; `--dry-run` runs offline.
- `awsbnkctl doctor` — three new checks: `aws ec2 vCPU quota`, `aws s3:PutObject feasibility`, `aws iam:GetRole (FLO IRSA)`.
- Workspace schema retarget: `Workspace.AWS` (region, profile, vpc_id, subnet_ids, supply_chain) replaces `Workspace.IBMCloud` as the primary path. Back-compat alias retained; full retirement gates on PRD 04 cred/exec retarget (Sprint 3).
- `tools/docker/aws/Dockerfile` — multi-arch (TARGETARCH-aware download URLs + dual-arch sha256 pins) for awscli v2, kubectl, helm. `tools-images.yml` builds + pushes linux/amd64 + linux/arm64 manifest list. Closes Sprint 1 tech-writer Issue 6.
- `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` — full PRD authored.
- `book/src/25-cos-supply-chain.md` — full chapter rewrite (~2,500 words) covering S3 bucket shape, IRSA trust chain, init upload flow, ECR mirror option, day-2 ops (JWT / FAR / IRSA role / CMK rotation).
- `cspell.json` — added KMS, CMK, OIDC, OpenID, presigned, webhook, aarch64.

### Notes

- Sprint 2 does NOT cut a tag. The first release tag (`v0.2`) gates on operator-run validation of PRD 07's spike per `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Spike status". Sprint 2's modules + Go code + doctor checks are all offline-validatable (`terraform validate`, mocked aws-sdk-go-v2 unit tests, `awsbnkctl init --dry-run`).
- Open Sprint 2 tech-writer findings carry into Sprint 3's read-first list (notably the inherited doctor visibility test contract from Sprint 1).


### Added

- Fork point established from `jgruberf5/roksbnkctl@v1.2.1`. Repository identity rewritten (`README.md`, `CHANGELOG.md`, `MIGRATING.md`) to reflect the AWS EKS retargeting scope. `upstream` git remote retained against `jgruberf5/roksbnkctl` for cherry-picking shared-surface improvements.

### Planned for v0.2 (Sprint 0 prep → Sprint 1 close)

Sprint 0 lands the strip-and-retarget on `main` without a tag (M0 in `docs/PLAN.md`). The first tagged release is `v0.2`, gated on Sprint 1 closing the EKS cluster + SR-IOV node-group module per [`docs/prd/07-EKS-CLUSTER-SRIOV.md`](docs/prd/07-EKS-CLUSTER-SRIOV.md). The milestone-to-tag mapping is in [`docs/PLAN.md`](docs/PLAN.md) — Sprints 1–6 ship `v0.2 → v1.0`.

- Strip IBM-specific surface: remove `internal/ibm/`, `internal/cos/`, `terraform/modules/roks_cluster/`. Replace with `internal/aws/` (aws-sdk-go-v2) and `terraform/modules/eks_cluster/` (`terraform-aws-modules/eks/aws` + custom launch template for self-managed SR-IOV node groups).
- Rename Go module path from `github.com/jgruberf5/roksbnkctl` to `github.com/JLCode-tech/awsbnkctl`. Rename `cmd/roksbnkctl/` to `cmd/awsbnkctl/`.
- Rewrite `docs/PLAN.md` as an awsbnkctl sprint roadmap; author `docs/prd/00-OVERVIEW.md` and `docs/prd/07-EKS-CLUSTER-SRIOV.md` (SR-IOV-on-self-managed-nodes decision).
- Author `prompts/sprint0/{architect,staff,validator,tech-writer}.md` for the IBM-strip-and-AWS-seed cycle.
- Reuse unchanged: `cert_manager`, `flo`, `cne_instance`, `license`, `testing` Terraform modules; the Go scaffolding under `internal/{cli,tf,k8s,exec,cred,test,doctor,ui,remote,config}/`; the `agents/` role definitions; the mdBook framework (chapter content to be rewritten in later sprints).
