# Sprint 6 — architect issues

Sprint 6 architect scope: close Sprint 5 chapter-prose blockers (cluster-subverb chapters via path b — "Available in v1.x" annotation); glossary cleanup against the as-shipped AWS shape; secondary IBM-residue sweep across chapters 17 / 18 / 19 / 32; README sprint-count refresh; PLAN.md "What's deferred to post-v1.0" appendix; PLAN.md Sprint 6 close subsection.

Off-limits surfaces (`.go`, `terraform/**`, `Makefile`, `go.mod`, `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`, `internal/exec/k8s_install.yaml`) respected.

**SPIKE DEFERRAL** carries — chapters describe the design as-shipped (awsbnkctl AWS retarget), not as-validated against live AWS.

---

## Issue 1 (MEDIUM) — chapter 19's "Available in v1.x" annotation closes the prose-correctness gap but the staff retarget of `internal/exec/k8s_install.yaml` is the load-bearing closure

**Severity**: medium
**Status**: open (staff)

**Description**: The architect surface this sprint closed chapter 19 via path b (top-of-chapter annotation flagging the inherited `roksbnkctl`-shaped installer; redirecting first-time readers away from `awsbnkctl ops install` against real EKS). That closes the **prose** gap from Sprint 5 tech-writer Issue 1 — first-time readers no longer try to map the chapter prose onto a non-existent AWS-shaped installer.

The **implementation** gap remains: `internal/exec/k8s_install.yaml` is still entirely IBM-shaped (`roksbnkctl-ops` namespace, `roksbnkctl-ibm-creds` Secret, `IBMCLOUD_API_KEY` env var, IBM Cloud Trusted Profile flow under `--trusted-profile=auto`). This sprint's staff brief calls out `internal/exec/k8s_install.yaml` as staff scope — when that retarget lands, the chapter 19 v1.x banner is no longer needed; the deferred-work appendix entry under "Ops-pod surface" likewise rolls forward into the v1.x roadmap.

**Files affected**: `internal/exec/k8s_install.yaml`, `internal/cli/ops.go`, `internal/cli/doctor_backend.go`, `internal/exec/k8s.go` (`K8sOpsSecretName` constant) — all staff surfaces.

**Proposed fix**: Sprint 6 staff retargets the YAML + Go surfaces per the architect brief. If the retarget lands cleanly this sprint, the integrator pulls the chapter 19 v1.x annotation banner before tagging v0.9-rc1 (or pulls it post-rc1 in the v1.0 finalisation pass). If staff defers, the banner stays and the deferred-work appendix entry carries.

## Issue 2 (LOW) — chapter 25 filename rename (`25-cos-supply-chain.md` → `25-s3-supply-chain.md`) still pending

**Severity**: low (cosmetic; mdbook serves the file fine)
**Status**: open (deferred to Sprint 6 integrator)

**Description**: Sprint 5 architect Issue 3 → Sprint 5 tech-writer Issue 8 → carried into Sprint 6 architect scope per the architect brief's task table. The rename was not executed this sprint because (a) the architect brief did not explicitly authorise the rename (the `book/src/25-*.md` row isn't in the architect's task list — chapters 8/9/11/17/18/19/30/32 are), and (b) the rename is mechanically a single `git mv` + a 17-cross-link-sweep that's appropriate for an integrator commit rather than mid-sprint architect work.

The chapter content reads as "S3 (and optional ECR) supply chain"; only the slug carries the old name.

**Files affected**: `book/src/25-cos-supply-chain.md` (rename target), `book/src/SUMMARY.md` (TOC entry), the 17 chapters cross-linking to that file.

**Proposed fix**: Sprint 6 integrator at sprint close: `git mv book/src/25-cos-supply-chain.md book/src/25-s3-supply-chain.md`; `grep -rln 25-cos-supply-chain.md book/src/` → sed-sweep across the 17 cross-links; commit atomically before any new content lands. The validator's `book-build` CI job gates against breakage.

## Issue 3 (LOW) — chapter 02 / 03 / 32 filename slugs preserve the `roks` / `roksbnkctl` upstream lineage

**Severity**: low (cosmetic; mdbook + the SUMMARY.md labels supply the AWS-correct human-readable surface)
**Status**: open (post-v1.0 cleanup)

**Description**: Chapter filenames `02-why-roks.md`, `03-what-roksbnkctl-does.md`, `32-extending-roksbnkctl.md` preserve the upstream-fork slugs even though the chapter titles (and SUMMARY.md TOC entries) read as "Why EKS + self-managed SR-IOV node groups", "What awsbnkctl does (and doesn't do)", "Extending awsbnkctl". Sprint 5 tech-writer's Per-prose-surface verdict notes the title vs. filename mismatch is fine for mdbook but the slug should eventually catch up.

This sprint left the slugs alone because cross-link sweep cost is moderate and the chapter brief specifically called out chapter 32 for content surgery (which landed clean), not the filename rename.

**Files affected**: `book/src/02-why-roks.md`, `book/src/03-what-roksbnkctl-does.md`, `book/src/32-extending-roksbnkctl.md`, `book/src/SUMMARY.md`, every chapter cross-linking to those slugs.

**Proposed fix**: post-v1.0 housekeeping pass — same atomic-rename pattern as Issue 2 (`git mv` + sed-sweep + SUMMARY.md update). Defer to the v1.0 finalisation pass or to v1.0.1 cleanup; not a v0.9-rc blocker.

## Issue 4 (LOW) — v1.x annotations on chapters 8 / 9 / 11 frame the subverb subtrees as a "post-v1.0 staff lift"; integrator must spot-check this assumption at sprint close

**Severity**: low (forward-statement wording)
**Status**: open (verify against the integrator's Sprint 6 close)

**Description**: The architect-surface chapter banners landed this sprint reference the closure path for the `cluster *` / `bnk *` subverbs as "the staff lift from `roksbnkctl/internal/cli/cluster.go` + `bnk.go` retargeted at the EKS shape" with a `docs/PLAN.md` §"What's deferred to post-v1.0" cross-link. The deferred-work appendix entry under "Subverb subtrees" describes the same lift as post-v1.0 work.

The Sprint 6 staff brief in `prompts/sprint6/architect.md` calls out staff scope as "retargets `internal/exec/k8s_install.yaml` (ops-pod manifest IBM → IRSA-injected AWS-creds shape); runs `gosec ./...` and folds findings; secrets scan; verifies goreleaser config" — **not** the `cluster *` / `bnk *` subverb lift. The annotation framing assumes the subverb lift is a v1.x roadmap item (correct, per the deferred-work appendix) rather than Sprint 6 work.

The chapter-banner prose is correct in framing the subverb subtrees as v1.x roadmap; my issue here is just to flag the wording for the integrator to spot-check at sprint close — if there's mid-sprint scope creep where staff does pick up the cluster-subverb lift, the chapter-8/9/11 banners would need a "lands in v0.9.x" framing tweak. Until then the banners read correctly.

**Files affected**: `book/src/08-cluster-phase.md`, `book/src/09-registering-existing-cluster.md`, `book/src/11-tearing-down.md` (all banners), `docs/PLAN.md` § "Sprint 6 close" + § "What's deferred to post-v1.0".

**Proposed fix**: integrator spot-check: confirm Sprint 6 staff did not lift `cluster *` / `bnk *` (per the architect brief's task table). If confirmed, the banners + deferred-work appendix are accurate. If staff did pick up the lift, pull the v1.x framing from the relevant banners and add a Sprint 6 close-section line documenting the surprise.

## Issue 5 (LOW) — chapter 19's v1.x banner directs readers to "alternative backends per Chapter 18 §"Decision tree""; the decision tree itself doesn't yet call out "don't use k8s on v0.9 EKS"

**Severity**: low (chapter-cross-link friction; no first-time-reader bounce)
**Status**: open (post-Sprint-6 architect or Sprint 7 if v0.9-rc carries it)

**Description**: The chapter 19 v1.x annotation banner this sprint added directs readers away from `awsbnkctl ops install` on real EKS clusters and points at "alternative backends per [Chapter 18 §"Decision tree"](./18-choosing-backend.md#decision-tree)". Chapter 18's decision tree doesn't currently surface a "don't use `--backend k8s` against a v0.9 EKS cluster (ops pod is IBM-shaped)" branch — readers following the cross-link will see the standard decision tree (k8s for cluster-bandwidth measurement, k8s for cluster-side ad-hoc shell, etc.) without the v0.9 EKS-specific caveat.

This is a follow-on friction surface, not a first-time-reader bounce: readers from chapter 19 already know not to run `ops install` against real EKS; they're looking at the decision tree to find the right alternative. The decision tree's recommendations (local, ssh:<bastion>, docker) still work fine for v0.9; the missing surface is just the "and don't use k8s here in v0.9" callout.

**Files affected**: `book/src/18-choosing-backend.md` § "Decision tree".

**Proposed fix**: post-Sprint-6 architect pass (or Sprint 7 if v0.9-rc1 carries Sprint 6 architect's work into the next cycle) adds a "v0.9 caveat" callout to the decision tree's "I want a cluster-side ad-hoc shell" entry and to the "I'm running `aws` from a network that…" entry, pointing at chapter 19's v1.x banner. Mechanically a 4-line addition to two of the existing decision-tree sections.

## Issue 6 (LOW) — PLAN.md "What's deferred to post-v1.0" appendix is the canonical post-v1.0 backlog; future architects must fold discovered items into it rather than into ad-hoc lists

**Severity**: low (process-shape note)
**Status**: open (architect convention going forward)

**Description**: The Sprint 6 architect surface adds the "What's deferred to post-v1.0" appendix as the canonical list — every deliberate scope-cut, every Sprint-N+1 carry-over, every PRD "v1.x revisit" note now lives in that single section. The shape is intentional: future architects working on v0.9.x / v1.0 / v1.x sprints have one place to fold deferred items into, and the integrator has one place to scan when grooming the v1.x backlog.

Sprint 6 left this appendix as a complete-at-time-of-Sprint-6-close snapshot. Anything discovered between v0.9-rc1 cut and v1.0 cut (or post-v1.0) should be folded into this appendix as part of the relevant sprint-close architect pass, rather than into a new ad-hoc section.

This is a note for the future, not an actionable issue against Sprint 6.

**Files affected**: `docs/PLAN.md` § "What's deferred to post-v1.0".

**Proposed fix**: integrator notes the convention in the v1.0 release-prep checklist. Future architects: append, don't fork.

---

## Verification (against the architect brief)

| Verification gate | Status |
|---|---|
| `grep -r 'roksbnkctl\|ibmcloud\|ROKS\|COS' book/src/{17,18,19,32}-*.md` returns near-zero | ✓ pass. Chapters 17 / 18 / 32 return 0 hits. Chapter 19 returns 1 hit, contained in the explicit "Available in v1.x" banner that documents the inherited shape — the brief's "near-zero" framing accommodates this deliberate v1.x-roadmap annotation. |
| Chapters 8 / 9 / 11 have explicit "Available in v1.x" annotations on absent subverbs | ✓ pass. Top-of-chapter banners landed; each explicitly enumerates the absent subverbs (`cluster up`/`down`/`show`/`register`, `bnk up`/`down`) and directs readers to the v0.9 alternatives. |
| README accurately reflects post-Sprint-6 status | ✓ pass. Status banner reads "Sprint 6 complete; v0.9-rc ready; v1.0 awaits spike"; the "Planned quick start (post-Sprint 1)" is now a real quick-start section; the `book/` row in the layout reads "AWS-retargeted in Sprint 5; published at GitHub Pages"; the fork-relationship paragraph leads with the awsbnkctl book URL. |

## Issues filed: 6

- **0 blocker**
- **1 medium** (Issue 1 — chapter 19 v1.x banner is correct on the prose side; staff retarget is the implementation closure)
- **5 low** (Issue 2 — chapter 25 rename pending integrator; Issue 3 — chapter 02/03/32 slug rename post-v1.0; Issue 4 — banner-wording spot-check at integrator close; Issue 5 — decision-tree v0.9 caveat callout; Issue 6 — deferred-work appendix convention note)
- **0 roadmap**

Sprint 5 tech-writer Issues 1-6 closed end-to-end on the architect surface; Issue 7 (CHANGELOG Sprint 5 entry) was integrator-scope per its filing and is folded by the Sprint 6 integrator pre-commit pass; Issue 8 (chapter 25 rename) is this sprint's Issue 2.

## Sibling cross-references

- **staff Issue 1** (`internal/exec/k8s_install.yaml` IRSA retarget) — chapter 19's v1.x banner is the architect-side correctness gate; staff's YAML retarget is the implementation closure. When staff lands, the integrator pulls the banner.
- **tech-writer Sprint 6** — read-only final pass; the chapter banners + glossary rewrite + secondary sweep + README + PLAN appendix all expect a tech-writer dogfood pass to confirm zero remaining stuck-points.
- **validator Sprint 6** — full security audit + book PDF build verification + cspell pass. The v1.x annotation banners use no new vocabulary; cspell should pass clean.
