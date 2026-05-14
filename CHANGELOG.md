# Changelog

All notable changes to `awsbnkctl` are documented in this file. Format follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) convention; the project uses [semantic versioning](https://semver.org/spec/v2.0.0.html).

Per-sprint design rationale lives in [`docs/PLAN.md`](docs/PLAN.md); per-PRD design specs live under [`docs/prd/`](docs/prd/). This file is the user-facing summary of what changed between releases.

awsbnkctl began as a hard fork of [`jgruberf5/roksbnkctl`](https://github.com/jgruberf5/roksbnkctl) at its `v1.2.1` tag. roksbnkctl's pre-fork changelog is preserved in git history on `upstream/main` and at <https://github.com/jgruberf5/roksbnkctl/blob/main/CHANGELOG.md>; this file starts fresh at the awsbnkctl fork point.

## Unreleased

### Added

- Fork point established from `jgruberf5/roksbnkctl@v1.2.1`. Repository identity rewritten (`README.md`, `CHANGELOG.md`, `MIGRATING.md`) to reflect the AWS EKS retargeting scope. `upstream` git remote retained against `jgruberf5/roksbnkctl` for cherry-picking shared-surface improvements.

### Planned for v0.2 (Sprint 0 prep → Sprint 1 close)

Sprint 0 lands the strip-and-retarget on `main` without a tag (M0 in `docs/PLAN.md`). The first tagged release is `v0.2`, gated on Sprint 1 closing the EKS cluster + SR-IOV node-group module per [`docs/prd/07-EKS-CLUSTER-SRIOV.md`](docs/prd/07-EKS-CLUSTER-SRIOV.md). The milestone-to-tag mapping is in [`docs/PLAN.md`](docs/PLAN.md) — Sprints 1–6 ship `v0.2 → v1.0`.

- Strip IBM-specific surface: remove `internal/ibm/`, `internal/cos/`, `terraform/modules/roks_cluster/`. Replace with `internal/aws/` (aws-sdk-go-v2) and `terraform/modules/eks_cluster/` (`terraform-aws-modules/eks/aws` + custom launch template for self-managed SR-IOV node groups).
- Rename Go module path from `github.com/jgruberf5/roksbnkctl` to `github.com/JLCode-tech/awsbnkctl`. Rename `cmd/roksbnkctl/` to `cmd/awsbnkctl/`.
- Rewrite `docs/PLAN.md` as an awsbnkctl sprint roadmap; author `docs/prd/00-OVERVIEW.md` and `docs/prd/07-EKS-CLUSTER-SRIOV.md` (SR-IOV-on-self-managed-nodes decision).
- Author `prompts/sprint0/{architect,staff,validator,tech-writer}.md` for the IBM-strip-and-AWS-seed cycle.
- Reuse unchanged: `cert_manager`, `flo`, `cne_instance`, `license`, `testing` Terraform modules; the Go scaffolding under `internal/{cli,tf,k8s,exec,cred,test,doctor,ui,remote,config}/`; the `agents/` role definitions; the mdBook framework (chapter content to be rewritten in later sprints).
