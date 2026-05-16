# Sprint 5

**Theme:** Book retarget at AWS — full chapter rewrites + GitHub Pages publish

_Drafted from `docs/PLAN.md` Sprint 5 section._

Sprint 5 retargets the book (`book/src/*.md`) from roksbnkctl/IBM Cloud framing to awsbnkctl/AWS EKS framing. The chapter outline carries from Sprint 0; Sprint 5 rewrites the chapter *bodies*. Auto-regenerated reference chapters (cobra-md, tfvars-md) are refreshed against the current AWS-shaped CLI + Terraform variables. The book publishes at `https://JLCode-tech.github.io/awsbnkctl/book/`.

End-of-sprint gate: `mdbook build book/` clean; web book deploys to GitHub Pages; cross-link audit clean; cspell clean.

**SPIKE DEFERRAL** carries — book content describes the design as-implemented, not as-validated-against-live-AWS. PRD 07's "Resolved in spike" placeholder is still operator-fillable.

Carry-overs from Sprint 4:
1. **tech-writer Issue 1 (medium)** — chapter 22 documents the bundled `awsbnkctl-tools-iperf3` image but `Iperf3DefaultImage = networkstatic/iperf3:latest`. One-line image tag swap or chapter wording fix. Sprint 5 staff folds.
2. **Sprint 3 tech-writer Issue 2 (medium)** — 302 IBM-residue hits in `internal/` tests + comments. Sprint 5 tech-debt sweep.
3. **Sprint 4 architect chapter 22 / 26 stub-anchor issues** — sub-anchors needed in chapter 26 for chapter 22's cross-links to resolve.

Four-agent dispatch:

1. **architect** — rewrites all remaining book chapters that need AWS retarget (parts I-IX); cross-link audit; SUMMARY.md final shape; updates PLAN.md Sprint 5 close.
2. **staff** — chapter 22 image-tag fix; IBM-residue sweep (delete dead test files / retarget comments); `tools/refgen/{cobra-md,tfvars-md}` regenerated output committed; chapter 26 sub-anchors fix.
3. **validator** — book CI workflow validates `mdbook build` on every push; cspell sweep over all rewritten chapters; CHANGELOG entry; GitHub Pages deployment verified.
4. **tech-writer** — read-only at sprint close. First-time-reader dogfood across the full book; cross-link audit; ready-for-v0.9 verdict.

The integrator commits. Sprint 5 closes M5 (v0.9 milestone in PLAN.md) but the tag waits on Sprint 6 hardening.
