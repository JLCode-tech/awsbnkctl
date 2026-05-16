# Sprint 2

**Theme:** S3 supply chain + IRSA workload identity (PRD 08)

_Drafted from `docs/PLAN.md` Sprint 2 section._

Sprint 2 lands the cloud-side primitives BNK consumes at runtime: an S3 bucket (KMS-encrypted) for the FAR pull-key archive + JWT licence, and IRSA (IAM Roles for Service Accounts) binding the FLO service account to an IAM role with scoped S3 read permission. Plus the `internal/aws/{s3,iam}.go` Go helpers and the `awsbnkctl init` AWS path that uploads local FAR + JWT files at workspace creation.

End-of-sprint gate: `terraform validate` succeeds on the new `s3_supply_chain` + `iam_irsa` modules; `internal/aws/s3.go` + `iam.go` unit tests pass against mocked SDK clients; `awsbnkctl init --dry-run` runs the wizard offline (no live AWS) and writes a workspace config; `awsbnkctl doctor` reports the new S3 + IRSA pre-flight rows when a workspace is configured.

**SPIKE DEFERRAL** (carries from Sprint 1). No agent runs `terraform apply` against live AWS. The Sprint 1 operator-run spike still gates v0.2; Sprint 2 closes more *offline* work on top.

Carry-overs from Sprint 1:
1. **tech-writer Issue 1 + staff Issue 3** — workspace schema retarget (`IBMCloud` field → `AWS`). Sprint 2 staff retargets the config schema; doctor visibility issue follows.
2. **staff Issue 2** — EC2 vCPU quota probe (deferred from Sprint 1 doctor refresh; now wireable since workspace + region exist post-`init`).
3. **staff Issue 4** — `legacy_helpers.go` + `doctor_backend.go` retirement (depends on cred + exec retarget per PRD 04).
4. **validator Issue 5** — pre-written notes for Sprint 2 validator (S3 tools-image extension, localstack option, cspell additions).
5. **tech-writer Issue 6** — `tools/docker/aws/Dockerfile` multi-arch. Sprint 2 validator picks this up.
6. **tech-writer Issue 5** — `awsbnkctl up cluster --help` cosmetic — Sprint 2 staff includes in their carry-over folds.

Four-agent dispatch (parallel architect/staff/validator → tech-writer after):

1. **architect** — finalises PRD 08, full rewrite of `book/src/25-cos-supply-chain.md` for S3 + IRSA, updates PLAN.md Sprint 2 close.
2. **staff** — authors `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}/`; `internal/aws/{s3,iam}.go` + tests; workspace schema retarget; `awsbnkctl init` AWS path; doctor S3 + IRSA + vCPU rows.
3. **validator** — Dockerfile multi-arch (TARGETARCH-aware); CI integration test scaffolding; cspell IRSA/OIDC/KMS/skopeo additions.
4. **tech-writer** — read-only at sprint close.

The integrator commits the aggregated four-agent output. Sprint 2 does **not** cut a tag — v0.2 still gates on operator-run spike.
