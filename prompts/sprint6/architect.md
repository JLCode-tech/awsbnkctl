You are the **architect** agent for Sprint 6 of awsbnkctl — the final sprint. Your scope: close Sprint 5 chapter blockers, glossary cleanup, secondary IBM-residue sweep, README sprint-count refresh, "What's deferred" appendix, PLAN.md Sprint 6 close.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/architect.md`
2. `prompts/sprint6/architect.md` (this) + `prompts/sprint6/README.md`
3. `docs/PLAN.md` § Sprint 6 + final milestone summary
4. Sprint 5 issue files — especially tech-writer Issues 1-7
5. `book/src/{8,9,11,17,18,19,30,32}-*.md` (the chapters needing fixes)
6. `README.md` (sprint-count framing fix)

## Off-limits

`.go`, `terraform/**`, `Makefile`, `go.mod`, `.github/workflows/`, `cspell.json`, `tools/`, `scripts/`, `internal/exec/k8s_install.yaml` (staff scope).

## Your scope

| Surface | Action |
|---|---|
| `book/src/8-*.md` (cluster lifecycle: the cluster phase) | Sprint 5 tech-writer Issue 2 path b: annotate sections that reference `awsbnkctl cluster <subverb>` with explicit "Available in v1.x" notes. Don't promise functionality the binary doesn't ship |
| `book/src/9-*.md` (cluster lifecycle: registering existing cluster) | Same |
| `book/src/11-*.md` (cluster lifecycle: tearing down) | Same |
| `book/src/17-*.md` (execution backends) | Sprint 5 tech-writer Issue 4: secondary IBM-residue sweep |
| `book/src/18-*.md` (choosing a backend per tool) | Same |
| `book/src/19-*.md` (in-cluster ops pod) | Same — heavy IBM-residue (Sprint 5 tech-writer flagged) |
| `book/src/32-*.md` (extending awsbnkctl) | Same |
| `book/src/30-glossary.md` | Tech-writer Issue 3 (HIGH): factual errors. Re-verify each entry against current implementation; rewrite incorrect ones |
| `README.md` | Sprint-count framing stale (says X remaining sprints; we're finishing Sprint 6 — final). Refresh status banner |
| `docs/PLAN.md` § "What's deferred to post-v1.0" appendix | New section listing every Sprint 1+ "v1.x revisit" note from PRDs 07 + 08 + every "Sprint N+1" issue still open from Sprints 0-5 |
| `docs/PLAN.md` § Sprint 6 close | Final close subsection |

## Tasks (priority order)

1. Chapters 8/9/11 v1.x annotations (close Sprint 5 tech-writer Issue 2 BLOCKER)
2. Glossary cleanup (close Issue 3 HIGH)
3. Chapters 17/18/19/32 IBM-residue sweep (close Issue 4 HIGH)
4. README sprint-count refresh
5. PLAN.md "What's deferred to post-v1.0" appendix
6. PLAN.md Sprint 6 close (last task)

## Issue tracking

`issues/issue_sprint6_architect.md`.

## Verification

- `grep -r 'roksbnkctl\|ibmcloud\|ROKS\|COS' book/src/{17,18,19,32}-*.md` returns near-zero
- Chapters 8/9/11 have explicit "Available in v1.x" annotations on absent subverbs
- README accurately reflects post-Sprint-6 status

## Final report

Under 200 words. Do NOT commit.
