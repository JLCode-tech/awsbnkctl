You are the **architect** agent for Sprint 5 of awsbnkctl. Sprint 5 is the book retarget sprint — rewrite chapter bodies that still carry IBM Cloud / ROKS / COS framing.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries — book describes design as-implemented.

**Read first:**
1. `agents/architect.md`
2. `prompts/sprint5/architect.md` (this) + `prompts/sprint5/README.md`
3. `docs/PLAN.md` § Sprint 5
4. Current `book/src/SUMMARY.md` for full chapter outline
5. Sprint 0 architect's chapter-title-only updates — your job now is body content
6. `docs/prd/00-OVERVIEW.md` for what each chapter should describe

## Off-limits

`.go`, `terraform/**`, `Makefile`, `go.mod`, `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`.

## Your scope

Every book/src/*.md chapter whose body still carries roksbnkctl/IBM/ROKS/COS framing. Confirmed Sprint 5 retargets:

| Chapter | Title | Current state | Sprint 5 action |
|---|---|---|---|
| 1 | What is BIG-IP Next for Kubernetes (BNK) | inherited | Light edit: drop any IBM-specific examples |
| 4 | Installation | inherited | Replace IBM Cloud + ROKS install steps with AWS account + EKS prerequisites |
| 5 | Doctor | inherited | Document the six AWS rows + Service Quotas optional row |
| 6 | Workspaces | inherited | Update for AWS workspace schema (region, vpc_id, subnet_ids, supply_chain) |
| 7 | Quick start | inherited | Full rewrite around `awsbnkctl init/up/test/down` AWS path |
| 8-11 | Cluster lifecycle | inherited | Edit per AWS specifics — EKS, self-managed node groups, IRSA |
| 12 | Workspace config | inherited | New AWS fields documentation |
| 14 | Credentials | Sprint 2 architect added IRSA paragraph; deep rewrite due | AWS standard chain (env / profile / SSO / instance role) + IRSA in-cluster |
| 15 | SSH targets | inherited | Light edit if any IBM references remain |
| 16-19 | Remote execution | inherited | Light edit |
| 24 | Day-2 ops | inherited | Edit per AWS specifics |
| 26 | Troubleshooting | Sprint 3 draft | Add sub-anchors per Sprint 4 tech-writer Issue (chapter 22 cross-links) |
| 27-30 | Reference (Command/Config/TF Variable/Glossary) | auto-generated; coordinate with staff who regenerates these | Light edit + glossary updates |
| 31 | Building from source | inherited | Update build instructions for awsbnkctl |
| 32 | Extending awsbnkctl | inherited | Rewrite for AWS context |

Chapters that already had Sprint-specific rewrites (2, 7-Sprint1 work, 20-23, 25, 33) — verify still coherent post-other-chapter retargets.

## Tasks (priority order)

1. Walk the SUMMARY.md outline; for each chapter, `grep` for `roksbnkctl`, `ibmcloud`, `ROKS`, `COS`, `IBM Cloud`, `oc`, `OpenShift` references and rewrite where they appear in body content.
2. Chapter 26 sub-anchors fix (Sprint 4 carry-over).
3. Cross-link audit — verify every `[link](./XX-*.md)` resolves.
4. PLAN.md Sprint 5 close subsection.

## Issue tracking

`issues/issue_sprint5_architect.md`. Schema as before.

## Verification

- `mdbook build book/` succeeds (if available; skip and note otherwise)
- `grep -r 'roksbnkctl\|ibmcloud\|ROKS\|IBM Cloud\|COS' book/src/` returns near-zero hits (allowed: fork-relationship sections in chapter 1 or 32; explicit migration notes referencing roksbnkctl in chapter 4)

## Final report

Under 200 words. Do NOT commit.
