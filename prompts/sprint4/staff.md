You are the **staff engineer** agent for Sprint 4 of awsbnkctl. Sprint 4 verifies + AWS-shapes the inherited test surface, polishes doctor, lands AWS E2E phases per PRD 05, folds Sprint 3 carry-overs.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL — CRITICAL.** No live AWS. Validation: `terraform validate`, mocked AWS unit tests, `go build/test/vet/fmt`, `awsbnkctl test {dns,connectivity,throughput} --dry-run` (the dry-run flag should plan the test without actually executing the probes).

**Read first:**
1. `agents/staff.md`
2. `prompts/sprint4/staff.md` (this) + `prompts/sprint4/README.md`
3. `docs/PLAN.md` § Sprint 4
4. `docs/prd/05-E2E-TEST-PLAN.md` — primary spec
5. Sprint 3 carry-over issue files (especially tech-writer Issue 3 — `up --dry-run` first-run UX)
6. Current `internal/test/{dns,connectivity,throughput}.go` (inherited from roksbnkctl)
7. Current `internal/cli/cluster.go` `runFullLifecyclePlan`

## Off-limits

`docs/`, `book/`, `agents/`, `prompts/`, `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.

## Your scope

| Surface | Action |
|---|---|
| `internal/test/connectivity.go` | Verify works against EKS LoadBalancer (NLB/ALB) shape. Add awareness for `service.beta.kubernetes.io/aws-load-balancer-type` annotation if present. |
| `internal/test/dns.go` | Verify miekg/dns probe works against AWS Route 53. Add a "vantage" for AWS public resolvers (Amazon's 8.8.8.8-equivalent: `169.254.169.253` for VPC; `205.251.192.0/24` block for public) if useful. |
| `internal/test/throughput.go` | Verify iperf3-via-k8s-Job works under EKS Pod Security Admission `restricted` profile (no privileged, runAsNonRoot, drop ALL capabilities). The tools image already uses uid 1000 from Sprint 1; verify the Job spec matches. |
| `internal/test/test.go` (or equivalent dispatch) | Plumb workspace region + cluster outputs into test commands so test {dns,connectivity,throughput} default to workspace-derived targets. |
| `internal/test/*_test.go` | Mocked unit tests for any new dispatch logic. |
| `internal/cli/test.go` | If exists, extend the cobra surface for `awsbnkctl test {dns,connectivity,throughput,all}` with a `--dry-run` flag that plans the probe without executing. |
| `internal/cli/cluster.go` `runFullLifecyclePlan` | **Sprint 3 carry-over (tech-writer Issue 3 medium)**: catch the "missing tfvars" terraform error and translate to a friendly message: "workspace not initialised — run `awsbnkctl init -w <name>` first, or pass --var-file" |
| `internal/doctor/aws.go` | **Optional Service Quotas check**: probe `servicequotas:GetServiceQuota` for `L-1216C47A` (Running On-Demand Standard instances) — if the operator's IAM permits, surface the actual quota; if not, fall back to the existing "default 5 instances / 80 vCPU" pointer. Gated by an internal feature flag for now (off by default). |
| `scripts/e2e-test-backends.sh` | If inherited from roksbnkctl, refresh per Sprint 4 PRD 05 AWS phases. Each phase: header echo + skip-marker citing AWS-phase-not-yet-live status. |
| Test fixtures | Add a small `testdata/` directory if helpful for mocked AWS responses. |

## Tasks (priority order)

1. **Verify inherited `internal/test/*.go` builds + tests pass** — quick health check before edits.
2. **PSA compliance for throughput.go** — ensure Job spec uses `runAsNonRoot: true`, `seccompProfile: RuntimeDefault`, `capabilities.drop: [ALL]`. This is the load-bearing EKS compatibility check.
3. **Workspace plumbing** — test commands default to workspace-derived targets (region, cluster name, FLO namespace).
4. **First-run UX fix** for `up --dry-run` (Sprint 3 tech-writer Issue 3 medium).
5. **Service Quotas check** — optional, gated by feature flag.
6. **E2E AWS phases** — `scripts/e2e-test-backends.sh` refresh.
7. **Build green gate** — full suite.
8. **File Sprint 4 staff issues**.

## Issue tracking

`issues/issue_sprint4_staff.md`.

## Verification

- `go vet / build / test / gofmt` clean
- `./bin/awsbnkctl test --help` shows the test subcommands
- `./bin/awsbnkctl test connectivity --dry-run --workspace test` plans the probe without executing
- `./bin/awsbnkctl up --dry-run` on a workspace missing tfvars now returns a friendly "init needed" message (closes Sprint 3 tech-writer Issue 3)

## Final report

Under 200 words. Do NOT commit.
