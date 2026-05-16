You are the staff engineer agent for Sprint 2 of the `awsbnkctl` project. Sprint 2's theme is "S3 supply chain + IRSA workload identity (PRD 08)". You own the implementation: `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}/`, `internal/aws/{s3,iam}.go`, the workspace schema retarget (PRD 04 fold), the `awsbnkctl init` AWS path, and doctor extensions.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL — CRITICAL.** No live AWS this sprint. Validation tools (same as Sprint 1):
- `terraform validate` (no provider auth)
- `terraform plan` with fake creds: `AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1`
- Go unit tests with mocked aws-sdk-go-v2 clients
- `go build ./... && go test ./... && go vet ./...`
- `./bin/awsbnkctl init` runs the wizard with mocked S3 PutObject

**Read first** before any edits:

1. `/Users/j.lucia/Code/github/awsbnkctl/agents/staff.md`
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/staff.md` — yourself, of course
3. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/README.md`
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/PLAN.md` § Sprint 2
5. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` — primary spec.
6. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Outputs" — your inputs come from PRD 07.
7. Sprint 1 carry-overs: `issues/issue_sprint1_staff.md` (Issues 2, 3, 4 are Sprint 2 scope).
8. Current state: `internal/aws/{client,sts,ec2,eks,vpc}.go` (Sprint 1), `internal/cli/init.go` (inherited; touches the workspace wizard).

## Coordinate with parallel agents

An **architect** agent is finalising PRD 08, drafting chapter 25. **Do not touch `docs/`, `book/`, `agents/`, `prompts/`.**

A **validator** agent is updating Dockerfile multi-arch + CI + cspell. **Do not touch `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.**

## Your scope

| Surface | Action |
|---|---|
| `terraform/modules/s3_supply_chain/{main,variables,outputs,versions}.tf` | New: S3 bucket (KMS-encrypted), bucket policy restricting `s3:GetObject` to FLO IRSA role, `aws_s3_object` for FAR archive + JWT |
| `terraform/modules/iam_irsa/{main,variables,outputs,versions}.tf` | New: looks up the OIDC provider via `data.aws_iam_openid_connect_provider`; creates trust-policy IAM role bound to FLO SA; permission policy with `s3:GetObject` on the supply-chain bucket + `kms:Decrypt` on the CMK |
| `terraform/modules/ecr_mirror/{main,variables,outputs,versions}.tf` | New (gated on `var.enable_ecr_mirror` = false default): ECR repositories per FAR image, `null_resource` running `skopeo copy`. If time constrained, file as Sprint 3 follow-up issue and skip — PRD 08 explicitly allows this. |
| `terraform/main.tf` | Wire `s3_supply_chain` + `iam_irsa` calls; both consume `eks_cluster` outputs (OIDC ARN, issuer URL) |
| `terraform/variables.tf` | Add inputs: `far_auth_file_local_path`, `jwt_file_local_path`, `kms_key_arn`, `enable_ecr_mirror` |
| `internal/aws/s3.go` | `PutObject(ctx, bucket, key, body)`, `HeadObject(ctx, bucket, key)` |
| `internal/aws/iam.go` | `GetOIDCProvider(ctx, arn)`, `HasIRSARole(ctx, roleName)`, helper to derive the role ARN from cluster name + account |
| `internal/aws/s3_test.go`, `iam_test.go` | Mocked unit tests |
| `internal/config/workspace.go` (or equivalent) | Retarget `Workspace.IBMCloud` → `Workspace.AWS` per PRD 04 fold. Fields: `Region`, `Profile`, `VPCID`, `SubnetIDs`. Carry a back-compat alias for one release (`IBMCloud` deprecated; reads from `AWS` first, falls back to old field) **OR** clean break with `MIGRATING.md` note — your call, document in issue |
| `internal/cli/init.go` (AWS path) | Wizard: region prompt, VPC discovery (or "create new VPC" path), FAR archive path prompt, JWT path prompt, FLO namespace prompt (default `flo-system`). At wizard close: `aws.NewClients` + `PutObject` to upload FAR + JWT. **Mock-friendly** — `--dry-run` flag skips PutObject |
| `internal/doctor/aws.go` | New rows: `aws s3:PutObject permission` (probe via HeadBucket); `aws iam:GetOpenIDConnectProvider permission`; `aws ec2 vCPU quota for c5n family` (closes Sprint 1 staff Issue 2). Tests in `aws_test.go` |
| `internal/cli/cluster.go` | Wire the up cluster Long blob to mention `--workspace` (closes Sprint 1 tech-writer Issue 5) — one-sentence append per the tech-writer recommendation |
| `internal/cli/legacy_helpers.go` | Retire the IBM-named shims now that the workspace schema is retargeted (Sprint 1 staff Issue 4). Removal may need staged work — if any consumer breaks, file an issue and skip that consumer |
| `internal/cli/doctor_backend.go` | Replace IBM ops-pod check stub with placeholder citing Sprint 2's IRSA design — full retarget to IRSA-flavoured ops-pod check lives Sprint 3 |

## Tasks (priority order)

1. **Workspace schema retarget.** Rename `Workspace.IBMCloud` → `Workspace.AWS` with new field set. Update every consumer (`internal/doctor/aws.go`, `internal/cli/init.go`, `internal/tf/vars.go`, anywhere else `grep -r 'IBMCloud' internal/` finds). Make this the first task — every other change depends on it.

2. **Author Terraform modules.** Start with `variables.tf` + `outputs.tf` for both `s3_supply_chain` + `iam_irsa` matching PRD 08's tables. Then `main.tf` bodies. Then top-level `terraform/main.tf` rewire. `terraform validate` must succeed.

3. **Author `internal/aws/{s3,iam}.go` + tests.** Same pattern as Sprint 1's other AWS files — small interfaces for mocking, unit tests via aws-sdk-go-v2 middleware-test or manual mock injection.

4. **Wire `awsbnkctl init` AWS path.** Prompt-driven cobra command. Use `golang.org/x/term` or inherited prompt code. Confirm `awsbnkctl init --dry-run` runs without touching AWS.

5. **Doctor refresh.** Add S3 + IRSA + EC2 vCPU quota rows. Run against fake creds; existing `TestRunWithWhy_StockDevBox_NoWorkspace` must still pass (no workspace = no AWS rows).

6. **Sprint 1 carry-over folds.** `up cluster --help` cosmetic; `legacy_helpers.go` retirement (file issues for what you can't cleanly retire).

7. **Build green gate (offline).** `go vet`, `gofmt`, `go build`, `go test`, `terraform validate` on root + new modules.

8. **File Sprint 2 staff issues.** Schema same as Sprint 1.

## Issue tracking

File issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint2_staff.md`.

## Verification before reporting done

- `go vet ./...` clean
- `gofmt -d -l .` empty
- `go build ./...` succeeds
- `go test ./...` passes
- `terraform validate` on root + each new module
- `./bin/awsbnkctl init --dry-run` runs the wizard without touching live AWS
- `./bin/awsbnkctl doctor` reports the new S3 + IRSA + vCPU rows when a workspace exists

## Final report

Under 200 words — counts of files created/modified, schema retarget result (back-compat or clean break), module shape ↔ PRD 08 alignment, build/test results, tests skipped, carry-overs retired, issues filed, integrator notes. Do NOT commit.
