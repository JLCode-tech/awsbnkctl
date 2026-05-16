You are the tech-writer agent for Sprint 3 of `awsbnkctl`. The architect/staff/validator agents have completed. Read-only review + dogfooding gate.

**Do NOT edit files** except `issues/issue_sprint3_tech-writer.md` (overwrite stale fork content).

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**Read first:**
1. `agents/tech-writer.md`
2. `prompts/sprint3/tech-writer.md` (this) + `prompts/sprint3/README.md`
3. `docs/PLAN.md` § Sprint 3
4. `docs/prd/04-CREDENTIALS.md` (architect updated)
5. Sibling Sprint 3 issue files
6. Five ported modules + rewired `terraform/main.tf`
7. New chapter 26

## Your scope (read-only audits)

| Check | What to look for |
|---|---|
| **Module port correctness** | Each of cert_manager / flo / cne_instance / license / testing has `aws_*` inputs (not `ibmcloud_*`); module body unchanged where PRD 00 said "ports unchanged" |
| **`terraform/main.tf` graph** | Full dependency wired: eks_cluster → cert_manager / s3_supply_chain / iam_irsa → flo → cne_instance → license / testing |
| **`awsbnkctl up --dry-run`** | Plan output shows all 7 modules; no panic; non-zero exit if creds missing, zero exit on plan |
| **PRD 04 cred chain** | Reflects AWS standard chain (env, profile, instance role, SSO); IRSA documented as in-cluster shape |
| **Workspace clean break** | `grep -r 'IBMCloud' internal/` returns near-zero (only test-skip strings tolerated) |
| **Doctor visibility** | Sprint 2 tech-writer Issue 4 closed: on stock dev box without workspace, `awsbnkctl doctor` shows AWS credentials warning row |
| **legacy_helpers retirement** | Mostly or fully deleted; cleanup confirmed |
| **Chapter 26 narrative** | Top-N troubleshooting items make sense; cross-links to chapter 25 + 33 resolve |
| **Cross-link audit** | PRD 04 + 07 + 08 + chapter 25 + 26 + 33 + README + CHANGELOG all link cleanly |

## Tasks

1. PRD 04 ↔ implementation reconciliation
2. Five-module port verification (variable rename sweep)
3. `terraform/main.tf` full-graph plan run with fake creds
4. Workspace clean-break audit
5. Doctor visibility verification (closes Sprint 2 tech-writer Issue 4)
6. Chapter 26 read-through
7. Cross-link audit

## Final report

Under 200 words: files reviewed, dogfood results, issues filed (severity), sibling cross-refs, PRD ↔ impl verdict, workspace clean-break verdict, doctor visibility verdict, chapter 26 verdict, ready-for-integrator verdict.
