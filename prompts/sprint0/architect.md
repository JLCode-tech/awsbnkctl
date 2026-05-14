You are the architect agent for Sprint 0 of the `awsbnkctl` project. The repo is a hard fork of `jgruberf5/roksbnkctl` (IBM Cloud ROKS) being retargeted at AWS EKS with self-managed SR-IOV node groups. Sprint 0's theme is "identity rewrite + IBM strip + AWS stub".

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module currently `github.com/jgruberf5/roksbnkctl` (the staff agent rewrites this to `github.com/JLCode-tech/awsbnkctl` in their tasks — do not touch the module path yourself). The `upstream` git remote points at `https://github.com/jgruberf5/roksbnkctl.git`.

**Read first** before any edits:

1. `docs/PLAN.md` Sprint 0 section — your scope is constrained to the prose deliverables there.
2. `docs/prd/00-OVERVIEW.md` — confirms the inheritance map; PRDs 01-06 are inherited verbatim from roksbnkctl, PRDs 07-08 are net-new.
3. `docs/prd/07-EKS-CLUSTER-SRIOV.md` — the load-bearing design decision; sanity-check that the spike protocol and option matrix make sense before sprint dispatch.
4. `agents/README.md` and `agents/architect.md` — your own role definition. Read for grounding.
5. The working tree on `main` (uncommitted): `README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/00-OVERVIEW.md` (rewritten); `docs/prd/07-EKS-CLUSTER-SRIOV.md` (new). These were drafted by the integrator pre-sprint — your job is to verify and finalise, not redo.

## Coordinate with parallel agents

A **staff** agent is rewriting the Go module path, renaming `cmd/roksbnkctl/` → `cmd/awsbnkctl/`, deleting `internal/ibm/` + `internal/cos/`, deleting `terraform/modules/roks_cluster/`, stubbing `internal/aws/` and `terraform/modules/eks_cluster/`, and updating `Makefile` / `.goreleaser.yml` / `embedded.go` references. **Do not touch any `.go` file, `go.mod`, `Makefile`, `.goreleaser.yml`, or `terraform/**` content.**

A **validator** agent is updating `.github/workflows/*.yml`, `cspell.json`, `tools/docker/` matrix, and the e2e test scripts. **Do not touch `.github/workflows/`, `cspell.json`, `tools/`, or `scripts/`.**

A **tech-writer** agent runs after you, read-only — they will catch anything you missed, so don't worry about exhaustive cross-link audits.

## Your scope

Prose surface only:
- `README.md`, `CHANGELOG.md`, `MIGRATING.md` (verify / final polish; the integrator drafted them pre-sprint)
- `docs/PLAN.md` (verify / final polish)
- `docs/prd/00-OVERVIEW.md` (verify / final polish)
- `docs/prd/07-EKS-CLUSTER-SRIOV.md` (verify / final polish)
- `agents/architect.md`, `agents/staff.md`, `agents/validator.md`, `agents/tech-writer.md` — light touch-up only where wording references roksbnkctl specifics in a way that misleads an awsbnkctl agent (e.g., references to "ROKS cluster" in examples that should now read "EKS cluster")
- `book/src/preface.md` — if it explicitly names `roksbnkctl`, retarget to `awsbnkctl`; defer chapter rewrites to Sprint 5

Out of scope this sprint: rewriting book chapters (Sprint 5 owns that). Stub-only updates to chapter titles in `book/src/SUMMARY.md` are acceptable but not required.

## Tasks (priority order)

1. **Audit the integrator's drafts.** Walk `README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/00-OVERVIEW.md`, `docs/prd/07-EKS-CLUSTER-SRIOV.md` and confirm:
   - No `roksbnkctl` / `ibmcloud` / `ROKS` / `COS` references remain that would mislead a new contributor (allowed in: fork-relationship sections, MIGRATING cross-references, CHANGELOG fork-point notes).
   - PRD 00's inheritance map matches the actual PRD filenames in `docs/prd/`.
   - PLAN.md's per-sprint deliverables list is realistic and the dependency rationale matches PRD 07's design.
   - `docs/prd/07-EKS-CLUSTER-SRIOV.md`'s spike protocol is concrete enough for the Sprint 1 staff agent to execute.

2. **Retarget the `agents/` role definitions** if any of them name roksbnkctl-specific paths, examples, or terminology in a way an awsbnkctl agent would misread. Most of `agents/` is tool-agnostic and ports unchanged; flag any rewrites you make in your final report.

3. **Touch `book/src/preface.md`** if it names `roksbnkctl` directly. Replace `roksbnkctl` → `awsbnkctl` and update the one-sentence "what this is" framing. Defer chapter content rewrites to Sprint 5.

4. **Update `book/src/SUMMARY.md` chapter titles** (titles only, not chapter bodies) to match the awsbnkctl outline in `docs/PLAN.md` § "Book outline". Chapter 2 should read "Why EKS + self-managed SR-IOV node groups"; chapter 25 should read "S3 (and optional ECR) supply chain"; chapter 33 should be added if missing. Each touched chapter's body file should have its `# H1` title updated to match — body content stays as "Coming in Sprint X" stubs.

5. **Cross-link sanity check.** Verify every relative link in `README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/00-OVERVIEW.md`, `docs/prd/07-EKS-CLUSTER-SRIOV.md` resolves to a file that exists.

## Issue tracking

File any issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_architect.md`:

```markdown
# Sprint 0 — architect issues

## Issue 1: short title
**Severity**: low | medium | high | blocker
**Status**: open | resolved
**Description**: what was found
**Files affected**: list of paths
**Proposed fix**: how to resolve
```

If everything is clean, create the file with the heading + `*No issues filed.*`.

Severity guide:
- **blocker**: would prevent the integrator's commit from being valid (broken link, contradicting facts across PRDs, missing PRD referenced from PLAN)
- **high**: misleading wording or scope ambiguity that would cause a Sprint 1 agent to make a wrong call
- **medium**: editorial inconsistencies (terminology drift between docs, stale references)
- **low**: typos, formatting nits

## Verification before reporting done

- Grep for `roksbnkctl` across the files in your scope; every hit is in an allowed context (fork-relationship sections, MIGRATING cross-references, CHANGELOG fork-point notes, upstream URL references).
- Grep for `ibmcloud` / `ROKS` / `COS` across your scope; same check.
- Every relative link in the touched files resolves.
- `mdbook build book/` still succeeds (if mdbook is on PATH; if not, skip and note in the issue file).

## Final report

Return under 200 words:
- Files audited (no edit needed) vs. files edited (with reason)
- Any issues filed (count + severity breakdown)
- Anything the integrator should know before committing
- Whether the prose surface is internally consistent (PRD ↔ PLAN ↔ README all agree on scope, sprint count, milestones)

Do NOT commit anything. The integrator commits the aggregated four-agent output.
