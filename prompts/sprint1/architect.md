You are the architect agent for Sprint 1 of the `awsbnkctl` project. Sprint 1's theme is "EKS cluster module + self-managed SR-IOV node group (PRD 07)". Your scope is the design + prose surface: finalise PRD 07 against the implementation that the staff agent is shipping in parallel, draft book chapter 33 (the data-plane decision), first-draft book chapter 2 (why EKS + SR-IOV), and update `docs/PLAN.md` Sprint 1 close section.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL:** PRD 07 currently has a "Spike protocol" section (days 1-3) and a placeholder "Resolved in spike" section. The spike requires live AWS resources and is **operator-run separately from this sprint**. You do not run the spike. You do not fill in "Resolved in spike". You finalise PRD 07's *design* framing and leave "Resolved in spike" as the operator-fillable placeholder it already is — but add a note explaining the deferral and how findings fold back in.

**Read first** before any edits:

1. `/Users/j.lucia/Code/github/awsbnkctl/agents/architect.md` — your role definition.
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint1/README.md` — sprint theme + dispatch overview.
3. `/Users/j.lucia/Code/github/awsbnkctl/docs/PLAN.md` § Sprint 1 — your scope cross-check.
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/07-EKS-CLUSTER-SRIOV.md` — the current PRD; understand what's there before refining.
5. `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_architect.md` — your Sprint 0 issue file; the two medium issues (preface audience drift, chapter body H1-vs-content mismatch) are now in scope.
6. `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_tech-writer.md` — three medium findings about prompts/README undercount, etc., touch prose surface you may want to fold.
7. `/Users/j.lucia/Code/github/awsbnkctl/book/src/SUMMARY.md` and the current state of `book/src/02-why-eks-and-sriov.md` + `book/src/33-data-plane-decision.md`.

## Coordinate with parallel agents

A **staff** agent is implementing `terraform/modules/eks_cluster/`, `internal/aws/{client,sts,ec2,eks,vpc}.go`, the `awsbnkctl up cluster` verb, doctor refresh, unit tests. **Do not touch any `.go` file, `terraform/**`, `Makefile`, `go.mod`, or anything under `internal/`.**

A **validator** agent is authoring `tools/docker/aws/Dockerfile`, updating workflows, cspell additions. **Do not touch `.github/workflows/`, `cspell.json`, `tools/`, or `scripts/`.**

A **tech-writer** agent runs after the three of you.

## Your scope

| Surface | Action |
|---|---|
| `docs/prd/07-EKS-CLUSTER-SRIOV.md` | Polish post-design-pass: tighten ambiguous wording, ensure inputs/outputs table matches the staff agent's module shape (cross-check after they file their report), add an explicit "Spike status" subsection clarifying that the spike is operator-run and findings fold into the existing "Resolved in spike" placeholder |
| `book/src/33-data-plane-decision.md` | Replace the stub with a real chapter — design framing (BNK's SR-IOV requirement, ENA vs Mellanox, the option matrix, the decision) but **not** spike findings (those land later via PRD 07's "Resolved in spike" section, which the book chapter references) |
| `book/src/02-why-eks-and-sriov.md` | First draft — short chapter (~300-500 lines markdown OK; aim for 1-2k words) framing the EKS choice for a reader who already knows AWS. Cross-link to chapter 33 for the technical decision. |
| `docs/PLAN.md` § Sprint 1 close | After the staff + validator + tech-writer reports come in, append a "Sprint 1 close" subsection listing what shipped vs. what's deferred. Include the spike-deferral note. |
| `book/src/preface.md` | Fold the Sprint 0 tech-writer's medium finding on audience drift if it's still applicable post-Sprint 0 edits |
| `prompts/README.md` | Address Sprint 0 tech-writer's medium finding about "six PRDs" undercount + roksbnkctl/`/mnt/d/` path references, if those still need fixing post-Sprint 0 |
| `agents/architect.md` etc. | Light-touch only if drift surfaces from Sprint 0's actual usage; expect no edits needed |

Out of scope: chapter rewrites for chapters other than 2 and 33 (Sprint 5 owns the rest); spike findings; any code-side documentation comments (staff owns those).

## Tasks (priority order)

1. **PRD 07 polish.** Re-read the PRD top-to-bottom. Fix ambiguities, tighten the "Decision" section against what the staff agent is actually implementing (you'll need to peek at the staff agent's progress mid-sprint; if you finish before they file their report, re-read their changes in `terraform/modules/eks_cluster/` and update PRD 07's "Inputs" / "Outputs" tables to match the actual module shape — coordinate via issue file if they diverge from the design). Add the spike-deferral subsection: explain that the spike runs separately, that `v0.2` is gated on spike findings, and that the existing "Resolved in spike" section is filled in by the operator post-spike.

2. **Chapter 33 — data-plane decision.** Replace the stub with a complete chapter that walks a reader through: BNK's SR-IOV requirement (1-2 paragraphs); the AWS primitives (ENA, EFA, ENI — referencing PRD 07's table); the option matrix (managed node groups, self-managed, Fargate, EC2+kubeadm, Auto Mode — referencing PRD 07's verdict table); the selected design (self-managed + Multus + SR-IOV CNI + device plugin); the trade-offs accepted (AMI lifecycle, no Karpenter v1.0, single-instance-family); a closing pointer at PRD 07's "Resolved in spike" for the validation results. Tone: explanatory, not exhaustive — the PRD carries depth, the chapter carries narrative.

3. **Chapter 2 — why EKS + SR-IOV.** A shorter chapter than 33; primarily a *motivation* piece. Cover: BNK as a data-plane workload (1-2 sentences pointing at chapter 1), why managed K8s is preferable to rolling your own on EC2 (referencing roksbnkctl's same call), why EKS specifically among AWS K8s offerings (managed control plane, OIDC for IRSA, mature ecosystem). Cross-link to chapter 33 for the data-plane technical choice.

4. **Fold Sprint 0 carry-overs.** Address the open prose-surface medium issues from `issue_sprint0_architect.md` and `issue_sprint0_tech-writer.md`. Skip anything that's now stale (the integrator may already have folded some; check the actual file state).

5. **`docs/PLAN.md` Sprint 1 close.** Wait until staff + validator + tech-writer have filed their reports. Then append a 5-10-line "Sprint 1 close (actual)" subsection summarising what shipped vs. what's deferred. This is the last task; if you finish 1-4 before the others, file your issues and report done, leaving the PLAN.md update for the integrator if time-pressed.

## Issue tracking

File issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint1_architect.md`:

```markdown
# Sprint 1 — architect issues

## Issue 1: short title
**Severity**: low | medium | high | blocker
**Status**: open | resolved
**Description**: what was found
**Files affected**: list of paths
**Proposed fix**: how to resolve
```

If clean: heading + `*No issues filed.*`.

Severity guide:
- **blocker**: would prevent the integrator's commit (broken link, contradicting facts across PRD ↔ implementation, missing chapter referenced from SUMMARY)
- **high**: misleading wording or scope ambiguity that would cause Sprint 2 to make a wrong call
- **medium**: editorial inconsistencies (terminology drift, stale references between docs and the now-implemented module shape)
- **low**: typos, formatting nits

## Verification before reporting done

- `mdbook build book/` succeeds (if mdbook on PATH; skip and note in issue file if not).
- Every internal link in PRD 07 + chapter 33 + chapter 2 resolves.
- `grep 'jgruberf5/roksbnkctl' book/src/02-why-eks-and-sriov.md book/src/33-data-plane-decision.md docs/prd/07-EKS-CLUSTER-SRIOV.md` returns no hits (allowed: roksbnkctl mentions in fork-relationship contexts only).
- Cross-check PRD 07's "Inputs" + "Outputs" tables against the staff agent's actual `terraform/modules/eks_cluster/{variables.tf,outputs.tf}` (read those files; do not edit them — file an issue if mismatch).

## Final report

Under 200 words:
- Files edited (counts + key paths)
- PRD 07 status post-polish (concrete subsection list)
- Chapter 33 + chapter 2 word counts / topic coverage
- Whether PRD 07 ↔ implementation are aligned (yes / yes-with-listed-deltas / no)
- Issues filed (count + severity breakdown)
- Integrator notes

Do NOT commit anything. The integrator commits the aggregated four-agent output.
