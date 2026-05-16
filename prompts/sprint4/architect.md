You are the **architect** agent for Sprint 4 of awsbnkctl. Sprint 4 lands the test surface refresh + doctor polish + AWS E2E phases. Your scope: PRD 04 wording fix (HIGH carry-over), chapters 20-23 first drafts, PLAN.md Sprint 4 close.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/architect.md`
2. `prompts/sprint4/architect.md` (this) + `prompts/sprint4/README.md`
3. `docs/PLAN.md` § Sprint 4
4. `docs/prd/{04,05}-*.md` — PRD 04 architect refines; PRD 05 documents the test framework
5. Sprint 3 issue files (especially tech-writer Issue 1 — HIGH; carries to Sprint 4 architect)
6. `book/src/{20,21,22,23}-*.md` (currently stubs)
7. `internal/test/{dns,connectivity,throughput}.go` (READ-ONLY — verify what shape the chapters should describe)

## Off-limits

`.go`, `terraform/**`, `Makefile`, `go.mod`, `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.

## Tasks (priority order)

1. **PRD 04 wording fix.** Tech-writer Sprint 3 Issue 1 (HIGH): PRD claims `internal/cred.Resolver` delegates the AWS chain, but the AWS chain actually lives in `internal/aws/{client,sts}.go`. Re-read both surfaces; rewrite PRD 04's "Resolved in Sprint 3" section to accurately reflect the split: host-side AWS chain in `internal/aws/`, IRSA in-cluster shape is auto-injected by EKS pod-identity webhook (not in our code), `internal/cred/` retains the IBM-shaped resolver as deprecated for back-compat-string-naming-purposes-only.

2. **Chapter 20 — connectivity testing.** Replace stub with a real chapter (~800-1,200 words): what connectivity probes do (HTTPS reachability + load-balancer DNS + AWS NLB/ALB shape recognition), `awsbnkctl test connectivity` invocation, what failure modes mean.

3. **Chapter 21 — DNS testing for GSLB.** Real chapter (~800-1,200 words): the miekg/dns library, multi-vantage probe pattern, GSLB-aware divergence detection, AWS Route 53 specifics where they matter.

4. **Chapter 22 — throughput testing.** Real chapter (~800-1,200 words): iperf3-via-k8s-Job pattern, EKS Pod Security Admission requirements (no privileged containers), how to interpret results, what 'normal' looks like on c5n.4xlarge.

5. **Chapter 23 — E2E test plan.** Real chapter (~1,000-1,500 words): the phase-letter system (A-N), AWS-flavoured phases per PRD 05 Sprint 4 additions, what each phase exercises, how to run `scripts/e2e-test.sh` locally vs. CI.

6. **PLAN.md Sprint 4 close.** Last task — after siblings report.

## Issue tracking

`issues/issue_sprint4_architect.md`. Schema as before.

## Final report

Under 200 words. Do NOT commit.
