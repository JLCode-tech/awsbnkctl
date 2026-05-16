# Sprint 4

**Theme:** Test surface + doctor refresh + AWS E2E phases

_Drafted from `docs/PLAN.md` Sprint 4 section._

Sprint 4 verifies the inherited test surface (`internal/test/{dns,connectivity,throughput}.go`) works against AWS-shaped deployments: EKS LoadBalancer service shape (ALB/NLB), Pod Security Admission compliance for the tools image on EKS 1.25+, AWS-region-aware DNS probes. Plus a doctor refresh (better failure messages, optional Service Quotas check for actual running-on-demand vCPU quota), AWS-flavoured E2E phases per PRD 05, and Sprint 3 carry-over folds.

End-of-sprint gate: `awsbnkctl test {dns,connectivity,throughput}` runs against a workspace config (mocked end-to-end at sprint dispatch time; live AWS validation in operator-run spike). `awsbnkctl doctor` covers all six AWS rows with clear failure-mode messages. AWS-flavoured E2E phases pass against mocked fixtures.

**SPIKE DEFERRAL** carries — no live AWS resources.

Carry-overs from Sprint 3:
1. **tech-writer Issue 1 (HIGH)** — PRD 04 wording drift (PRD says `internal/cred.Resolver` delegates AWS chain; actually lives in `internal/aws/`). Sprint 4 architect resolves.
2. **tech-writer Issue 3 (medium)** — `up --dry-run` first-run UX gap (missing tfvars surfacing terraform errors instead of friendly init-needed message). Sprint 4 staff folds into `runFullLifecyclePlan`.
3. **tech-writer Issue 2 (medium)** — IBM-residue tech-debt sweep (302 hits in tests + comments). Sprint 4 staff or Sprint 5 — file as low-priority cleanup.

Four-agent dispatch:

1. **architect** — PRD 04 wording fix (the high carry-over); drafts chapters 20-23 (connectivity, DNS, throughput, E2E test plan) — currently stubs; updates PLAN.md Sprint 4 close.
2. **staff** — verifies `internal/test/{dns,connectivity,throughput}.go` works on EKS shape (PSA compliance, NLB/ALB recognition); plumbs workspace region + cluster outputs into test commands; AWS-flavoured E2E phases per PRD 05; up --dry-run first-run UX fix; optional Service Quotas check in doctor.
3. **validator** — extends CI for `awsbnkctl test` dry-run; cspell for new test terminology; e2e marker refresh for test phases.
4. **tech-writer** — read-only at sprint close.

The integrator commits. Sprint 4 does not cut a tag.
