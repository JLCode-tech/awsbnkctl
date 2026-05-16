# Changelog

All notable changes to `awsbnkctl` are documented in this file. Format follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) convention; the project uses [semantic versioning](https://semver.org/spec/v2.0.0.html).

Per-sprint design rationale lives in [`docs/PLAN.md`](docs/PLAN.md); per-PRD design specs live under [`docs/prd/`](docs/prd/). This file is the user-facing summary of what changed between releases.

awsbnkctl began as a hard fork of [`jgruberf5/roksbnkctl`](https://github.com/jgruberf5/roksbnkctl) at its `v1.2.1` tag. roksbnkctl's pre-fork changelog is preserved in git history on `upstream/main` and at <https://github.com/jgruberf5/roksbnkctl/blob/main/CHANGELOG.md>; this file starts fresh at the awsbnkctl fork point.

## Unreleased

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
