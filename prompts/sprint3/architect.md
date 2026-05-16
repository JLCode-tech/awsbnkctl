You are the architect agent for Sprint 3 of `awsbnkctl`. Sprint 3 ports the four reusable modules (`cert_manager`, `flo`, `cne_instance`, `license`, `testing`) and lands the first end-to-end `up --dry-run`. Your scope is the prose surface: doctor-visibility test-contract update, PRD 08 corrections, chapter 26 first-pass (troubleshooting), PLAN.md Sprint 3 close.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries — no live AWS.

**Read first:**
1. `agents/architect.md`
2. `prompts/sprint3/architect.md` (this) + `prompts/sprint3/README.md`
3. `docs/PLAN.md` § Sprint 3
4. `docs/prd/{07,08}-*.md` — your refs.
5. Sprint 2 issue files for carry-overs.
6. `book/src/26-troubleshooting.md` (stub) + `book/src/25-cos-supply-chain.md` (Sprint 2 architect output).

## Coordinate with parallel agents

**staff**: ports the five inherited Terraform modules, rewires top-level main.tf, retargets cred/exec, retires legacy_helpers + doctor_backend, relaxes doctor visibility gate. **Off-limits for you**: `.go`, `terraform/**`, `Makefile`, `go.mod`.

**validator**: CI matrix updates for full-up dry-run integration; e2e script phase markers; cspell. **Off-limits**: `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.

## Your scope

| Surface | Action |
|---|---|
| `docs/prd/04-CREDENTIALS.md` | Update inherited PRD with the AWS-cred-chain section (env / profile / instance role / SSO); reference IRSA as the in-cluster shape; mark "Resolved in Sprint 3" for the IBM→AWS retarget items |
| `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` | Tech-writer Issue 2: PRD says bucket versioning "off by default"; module forces enabled. Update PRD to match module (versioning is enabled — for FAR/JWT artefact history) |
| `book/src/26-troubleshooting.md` | First-pass draft (~1,000-1,500 words). Top-10-style: "FLO can't pull from S3" (IRSA misconfig), "CNEInstance stuck pending" (SR-IOV VFs unavailable), "EKS cluster Ready but kubectl says ResourceNotFound" (kubeconfig wrong), "terraform plan succeeds but apply fails on quota", etc. Cross-link to chapter 25 (supply chain) + chapter 33 (data-plane decision) |
| `book/src/25-cos-supply-chain.md` | Update chapter 26 cross-links from "stub" framing to actual references |
| `docs/PLAN.md` § Sprint 3 close | Append "Sprint 3 close (actual)" subsection — last task, after siblings file reports |
| `internal/doctor/doctor_test.go` | Coordinate via issue if you need the test contract updated — flag for staff to fold |

## Tasks (priority order)

1. PRD 04 update for AWS cred chain (replaces IBMCLOUD_API_KEY references with AWS standard chain + IRSA).
2. PRD 08 bucket versioning correction.
3. Chapter 26 first-pass — troubleshooting catalogue.
4. Chapter 25 cross-link refresh.
5. PLAN.md Sprint 3 close (last).

## Issue tracking

File to `issues/issue_sprint3_architect.md`. Schema as before.

## Final report

Under 200 words. Do NOT commit.
