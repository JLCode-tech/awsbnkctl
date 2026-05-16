You are the architect agent for Sprint 2 of the `awsbnkctl` project. Sprint 2's theme is "S3 supply chain + IRSA workload identity (PRD 08)". Your scope is the design + prose surface: finalise PRD 08, draft the S3-supply-chain book chapter (chapter 25 retarget), update PLAN.md Sprint 2 close section.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL** carries from Sprint 1 — no live AWS this sprint. PRD 08's "Resolved in spike" cross-reference (back to PRD 07) is the existing operator-run dependency.

**Read first** before any edits:

1. `/Users/j.lucia/Code/github/awsbnkctl/agents/architect.md`
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/README.md` — sprint theme + dispatch overview.
3. `/Users/j.lucia/Code/github/awsbnkctl/docs/PLAN.md` § Sprint 2 — scope cross-check.
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` — integrator-drafted; verify + polish.
5. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/07-EKS-CLUSTER-SRIOV.md` — supplies the OIDC outputs PRD 08 consumes.
6. Sprint 1 carry-over issue files (your task brief lists which apply).
7. `/Users/j.lucia/Code/github/awsbnkctl/book/src/25-cos-supply-chain.md` (note: filename retained from inherited tree; chapter title + body content needs retarget at S3).

## Coordinate with parallel agents

A **staff** agent is implementing `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}/`, `internal/aws/{s3,iam}.go`, workspace schema retarget, `awsbnkctl init` AWS path, doctor extensions. **Do not touch `.go`, `terraform/**`, `Makefile`, `go.mod`, `internal/`.**

A **validator** agent is updating the Dockerfile for multi-arch, CI integration tests, cspell. **Do not touch `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.**

A **tech-writer** agent runs after the three of you.

## Your scope

| Surface | Action |
|---|---|
| `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` | Polish post-implementation: tighten ambiguities, ensure inputs/outputs tables match the staff agent's module shape (cross-check after they file their report) |
| `book/src/25-cos-supply-chain.md` | Full rewrite: title becomes "S3 (and optional ECR) supply chain"; body covers FAR archive + JWT + bucket policy + IRSA trust chain + ECR mirror option. ~1,500-2,000 words, narrative + diagrams |
| `book/src/SUMMARY.md` | Update chapter 25 title to match (chapter filename can stay `25-cos-supply-chain.md` per the Sprint 1 architect Issue 3 cascade plan — Sprint 5 deferred filename rewrites) |
| `docs/PLAN.md` § Sprint 2 close | Append a "Sprint 2 close (actual)" subsection at the end |
| `book/src/14-credentials-resolver.md` | If still inherited content, briefly retarget the "AWS credentials" section to reference IRSA as the in-cluster cred shape (vs. host-side env / profile / instance role / SSO). Defer deep rewrite to Sprint 5. |
| Sprint 1 carry-overs in prose | Fold tech-writer Issue 5 (`up cluster --help` cosmetic) if cluster.go's Long blob still mentions `--workspace` invisibly — but this is in staff scope; you can file as their issue if you see drift |

## Tasks (priority order)

1. **PRD 08 polish.** Re-read top-to-bottom against the staff agent's actual `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}/variables.tf` + `outputs.tf` (read-only). File issue if inputs/outputs tables don't match.

2. **Chapter 25 rewrite.** Replace inherited COS content with S3 content. Sections:
   - "What's in the supply chain" (FAR archive + JWT — quick recap)
   - "The S3 bucket shape" (KMS, bucket policy, naming)
   - "IRSA trust chain" (FLO SA → IRSA role → OIDC provider → IAM)
   - "Uploading via `awsbnkctl init`"
   - "ECR mirror (optional)" — the v1.0 stretch feature
   - "Day-2 ops: rotating the FAR archive / JWT"
   Cross-link to PRD 07 (cluster + OIDC), PRD 08 (this), and chapter 14 (AWS cred chain).

3. **Chapter 14 minor retarget.** Add a one-paragraph "AWS in-cluster credentials (IRSA)" section pointing at chapter 25. Defer deep rewrite to Sprint 5.

4. **PLAN.md Sprint 2 close.** Last task — after staff + validator + tech-writer report. 5-10 lines summarising what shipped.

## Issue tracking

File issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint2_architect.md` per the standard schema.

## Verification before reporting done

- `mdbook build book/` succeeds (if mdbook on PATH).
- Every relative link in PRD 08 + chapter 25 resolves.
- `grep 'roksbnkctl\|ibmcloud\|cos:' book/src/25-cos-supply-chain.md` returns no hits except fork-relationship mentions.
- PRD 08 ↔ implementation cross-check filed as issue if drift exists (you don't edit `.go` or `terraform/**`).

## Final report

Under 200 words — files edited, PRD 08 status, chapter 25 word count, issues filed, integrator notes. Do NOT commit.
