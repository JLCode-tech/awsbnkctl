# Sprint 0 — architect issues

## Issue 1: forward-reference link to PRD 08 does not yet resolve
**Severity**: low
**Status**: open
**Description**: Both `docs/prd/00-OVERVIEW.md` (PRD inheritance map row + dependency-graph reference) and `docs/prd/07-EKS-CLUSTER-SRIOV.md` (cross-references section) link to `./08-S3-SUPPLY-CHAIN-IRSA.md`. The file doesn't exist yet — it is explicitly flagged "to author (Sprint 2)" in PRD 00's status column. The link will 404 in `mdbook build` and in any GitHub-rendered view until Sprint 2 lands. The forward reference is intentional (every PRD inheritance map row links to its target file), so the fix is to create the file, not remove the link.
**Files affected**: `docs/prd/00-OVERVIEW.md`, `docs/prd/07-EKS-CLUSTER-SRIOV.md`
**Proposed fix**: Sprint 2 architect authors `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` per the deliverable in PLAN.md § Sprint 2. No Sprint 0 action required; resolves automatically when Sprint 2 lands. Alternatively, drop a placeholder stub `08-S3-SUPPLY-CHAIN-IRSA.md` now containing only `# PRD 08 — Coming in Sprint 2` so the link resolves immediately — defer to integrator preference.

## Issue 2: mdbook not on PATH; book build not verified locally
**Severity**: low
**Status**: open
**Description**: `which mdbook` returned not-found in this agent's environment, so `mdbook build book/` was not executed locally to verify the SUMMARY.md edits + new chapter 33 stub render cleanly. All structural checks were performed instead: every relative link in the touched files was resolved against the actual filesystem (only the two PRD 08 forward-references above 404, both expected); SUMMARY.md syntax matches the existing file's pattern; the new chapter 33 file exists and is correctly slugged. The first CI run on a branch touching `book/**` will exercise the GitHub Actions workflow (which installs mdbook via `peaceiris/actions-mdbook@v2`) and surface any rendering issues. This issue carries forward from `resolved_sprint0_*_architect.md` in the inherited tree.
**Files affected**: `book/src/SUMMARY.md`, `book/src/preface.md`, `book/src/33-data-plane-decision.md`
**Proposed fix**: Either install mdbook locally (`cargo install mdbook` or download release) or rely on the first CI run on `main`. No code changes required.

## Issue 3: preface.md still describes IBM Cloud audience and OpenShift specifics
**Severity**: medium
**Status**: open
**Description**: Per the brief, preface.md got a light `roksbnkctl` → `awsbnkctl` swap, a retargeted "what this is" framing, and a scaffolding banner pointing readers at Sprint 5 for the chapter rewrites. However, three audience / prerequisite paragraphs still reference "IBM Cloud account", "ROKS as Kubernetes with a thin SCC + project overlay", and the OpenShift-specific gotcha cross-references in chapters 22 and 26 — all inherited from the roksbnkctl original. The scaffolding banner at the top of the file acknowledges this and tells the reader to swap terminology mentally. This is acceptable for Sprint 0 (brief explicitly defers chapter content rewrites to Sprint 5) but a first-time reader will hit cognitive dissonance between the "awsbnkctl on AWS EKS" foreword and the "IBM Cloud account" prerequisite three paragraphs later.
**Files affected**: `book/src/preface.md`
**Proposed fix**: Sprint 5 architect rewrites the audience + prerequisites + book-conventions sections to match the awsbnkctl scope. The scaffolding banner can be removed at that point.

## Issue 4: book chapter bodies still describe roksbnkctl behaviour despite renamed H1s
**Severity**: medium
**Status**: open
**Description**: Per the brief, the H1 of chapters 2, 3, 7, 14, 25, and 32 was updated to match the renamed SUMMARY entries (e.g. `# Why EKS + self-managed SR-IOV node groups`, `# S3 (and optional ECR) supply chain`). The chapter bodies, however, remain the inherited roksbnkctl prose — chapter 2's first paragraph still says "this book and the `roksbnkctl` tool target ROKS"; chapter 25 still describes "IBM Cloud Object Storage (COS)" as the supply chain. The brief expected these to be Sprint X stubs but the inherited tree shipped them as full prose. The H1-vs-body mismatch is glaring and would confuse a first-time reader who arrives at chapter 2 expecting EKS / SR-IOV content. Sprint 5 owns the rewrite per PLAN.md § Sprint 5, so this is a known scoped deferral, not a Sprint 0 fix candidate.
**Files affected**: `book/src/02-why-roks.md`, `book/src/03-what-roksbnkctl-does.md`, `book/src/07-quick-start.md`, `book/src/14-credentials-resolver.md`, `book/src/25-cos-supply-chain.md`, `book/src/32-extending-roksbnkctl.md`
**Proposed fix**: Sprint 5 architect rewrites the chapter bodies to match the renamed H1s, per the chapter triage in PLAN.md § Sprint 5. Until then, the preface scaffolding banner sets expectations.

## Issue 5: book/src/32-extending-roksbnkctl.md filename retains roksbnkctl despite H1 rename
**Severity**: low
**Status**: open
**Description**: The H1 in `book/src/32-extending-roksbnkctl.md` was updated to `# Extending awsbnkctl` per the SUMMARY.md rename, but the filename itself remains `32-extending-roksbnkctl.md` (and SUMMARY.md still links to it under that filename). Renaming the file in Sprint 0 would require updating every cross-reference to it (preface.md and any in-chapter "see Chapter 32" link), which is outside Sprint 0's "minimal touch on book chapters" scope. Defer the file rename to Sprint 5's chapter-by-chapter rewrite pass.
**Files affected**: `book/src/32-extending-roksbnkctl.md` (filename), `book/src/preface.md` (link target), `book/src/SUMMARY.md` (link target)
**Proposed fix**: Sprint 5 renames the file to `32-extending-awsbnkctl.md` as part of the chapter rewrite, updating all cross-references in the same commit.

## Issue 6: PRD 07 line 5 status says "draft — Sprint 1" but PRD is being polished in Sprint 0
**Severity**: low
**Status**: open
**Description**: `docs/prd/07-EKS-CLUSTER-SRIOV.md` opens with a status callout: "draft — Sprint 1. Authored in parallel with the Sprint 1 spike". In practice the PRD was drafted by the integrator pre-sprint (Sprint 0) so the spike has a target to validate, and Sprint 1 folds spike findings into the "Resolved-in-spike" section. The "Sprint 1" status framing is forward-looking and slightly understates the current state ("draft authored Sprint 0; spike-derived edits Sprint 1" would be more precise). Not misleading enough to change in this sprint — the intent is clear from context — but worth a small wording polish next time the file is edited.
**Files affected**: `docs/prd/07-EKS-CLUSTER-SRIOV.md`
**Proposed fix**: Sprint 1 architect updates the status block to read "draft — authored Sprint 0; spike findings folded Sprint 1" at the same time they fold in the spike post-mortem content.

## Issue 7: PLAN.md line 5 enumerates three "PRDs 07+" concepts but only two PRDs (07, 08) are scoped
**Severity**: low
**Status**: open
**Description**: PLAN.md fork-inheritance callout reads "the to-be-authored PRDs 07+ (EKS cluster, S3 supply chain, SR-IOV data plane) are net-new." PRD 07 covers both EKS cluster *and* SR-IOV data plane (it's `07-EKS-CLUSTER-SRIOV.md`); PRD 08 covers S3 supply chain. So the three enumerated topic areas map to two PRD files, not three. A reader counting PRDs against the topic list might expect a third PRD that doesn't exist. Wording polish, not a factual error.
**Files affected**: `docs/PLAN.md`
**Proposed fix**: Tighten the prose to "PRDs 07-08 (EKS cluster + SR-IOV data plane, S3 supply chain) are net-new", or leave as-is if the topic-area enumeration is preferred to the file-count enumeration. Defer to integrator polish.
