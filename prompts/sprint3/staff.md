You are the staff engineer agent for Sprint 3 of `awsbnkctl`. Sprint 3 ports the five inherited Terraform modules to AWS, wires `awsbnkctl up --dry-run` end-to-end, retargets cred/exec per PRD 04, retires Sprint 0 + Sprint 2 carry-overs.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL — CRITICAL.** No `terraform apply` against live AWS. Validation: `terraform validate`, mocked aws-sdk-go-v2 tests, `go build/test/vet/fmt`, `awsbnkctl up --dry-run` (the full lifecycle plan; must not touch real AWS).

**Read first:**
1. `agents/staff.md`
2. `prompts/sprint3/staff.md` (this) + `prompts/sprint3/README.md`
3. `docs/PLAN.md` § Sprint 3
4. `docs/prd/{04,07,08}-*.md` — primary specs; PRD 04 architect updates this sprint
5. Sprint 2 carry-over issue files (especially staff Issue 1, tech-writer Issue 4)
6. The five inherited modules: `terraform/modules/{cert_manager,flo,cne_instance,license,testing}/`

## Coordinate with parallel agents

**architect**: prose, PRD updates, chapter 26 draft. Off-limits: `docs/`, `book/`, `agents/`, `prompts/`.
**validator**: CI, scripts, cspell, tools. Off-limits: `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.

## Your scope

| Surface | Action |
|---|---|
| `terraform/modules/cert_manager/` | Rename `ibmcloud_*` inputs → `aws_*`; `roks_cluster_name_or_id` → `eks_cluster_name`; module body (kubernetes_manifest for cert-manager CRDs) unchanged |
| `terraform/modules/flo/` | Rename inputs; swap `ibmcloud_cos_bucket_*` → `s3_bucket_name` + `flo_role_arn` (from iam_irsa module); render FLO helm values with S3 URLs in place of COS endpoints; replace COS-auth env vars with IRSA-auto-injected `AWS_WEB_IDENTITY_TOKEN_FILE` references |
| `terraform/modules/cne_instance/` | Rename `flo_trusted_profile_id` → `flo_irsa_role_arn`; CNEInstance CRD body unchanged |
| `terraform/modules/license/` | Pull JWT from S3 (signed URL via IRSA) instead of COS; rename inputs |
| `terraform/modules/testing/` | Drop `roks_transit_gateway_name`; add `aws_vpc_id` + `aws_subnet_ids`; iperf3 + nginx fixtures otherwise unchanged |
| `terraform/main.tf` | Uncomment + rewire all five module calls; full dependency graph: eks_cluster → cert_manager / s3_supply_chain / iam_irsa → flo → cne_instance → license / testing |
| `terraform/variables.tf` | Add inputs that the new module call sites need (cert_manager_namespace, flo_namespace, far_repo_url, etc.) |
| `internal/cred/*.go` | PRD 04 retarget: replace IBMCLOUD_API_KEY env handling with AWS standard chain (env, profile, instance role, SSO). Drop IBM-named methods. |
| `internal/exec/*.go` | Same: drop IBM-shaped backend cred injection; AWS uses IRSA in-cluster (no env-var injection needed for k8s backend) + standard chain locally |
| `internal/config/workspace.go` | Remove `Workspace.IBMCloud` back-compat alias (Sprint 2 left this for PRD 04 retarget which is happening now). Update `MIGRATING.md` if needed |
| `internal/cli/legacy_helpers.go` | Delete or significantly trim. Carry-over from Sprint 0 |
| `internal/cli/doctor_backend.go` | Retarget the k8s ops-pod check from IBM TP to IRSA shape. Replace `IBMCLOUD_API_KEY` Secret check with IRSA-injected env var probe |
| `internal/cli/cluster.go` | Wire `awsbnkctl up` (no subcommand) to drive the full lifecycle, not just cluster. `--dry-run` plans the full graph |
| `internal/cli/lifecycle.go` | Update upCmd to call the full-lifecycle path; refresh Short/Long blobs |
| `internal/doctor/doctor.go` + `_test.go` | Relax workspace-nil gate on awsChecks per Sprint 2 tech-writer Issue 4. Update `TestRunWithWhy_StockDevBox_NoWorkspace` to allow the AWS credentials warning row on stock dev box (architect provides contract guidance via issue file if needed) |

## Tasks (priority order)

1. **PRD 04 cred/exec retarget** — first; every other change depends on cred chain being AWS-shaped
2. **Port the five Terraform modules** — variable renames + module body retargets
3. **Rewire `terraform/main.tf`** — uncomment + wire the full dependency graph
4. **Drop workspace IBMCloud alias** — clean break now that cred/exec is AWS-shaped
5. **Retire legacy_helpers + retarget doctor_backend**
6. **Wire `awsbnkctl up --dry-run` full lifecycle**
7. **Relax doctor visibility gate** + update test
8. **Build green gate** — full suite
9. **File Sprint 3 staff issues**

## Issue tracking

`issues/issue_sprint3_staff.md`.

## Verification before reporting done

- `go vet / build / test / gofmt` all clean
- `terraform validate` on root + all five ported modules
- `./bin/awsbnkctl up --dry-run` (with fake AWS creds) plans without panic; output shows the full module graph
- `./bin/awsbnkctl doctor` on stock dev box shows the AWS credentials warning (closes Sprint 2 tech-writer Issue 4)
- `grep -r 'IBMCloud\|IBMCLOUD\|ibmcloud' internal/` returns near-zero (test-skip strings allowed)

## Final report

Under 200 words. Do NOT commit.
