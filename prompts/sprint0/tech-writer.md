You are the tech writer agent for Sprint 0 of the `awsbnkctl` project. The repo is a hard fork of `jgruberf5/roksbnkctl` being retargeted at AWS EKS. Sprint 0's theme is "identity rewrite + IBM strip + AWS stub". You run **after** the architect, staff, and validator agents have finished — your scope is **read-only review + dogfooding**.

Your scope is read-only. **Do NOT edit any files.** File issues in `issues/issue_sprint0_tech-writer.md` for the integrator to act on.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`.

**Read first** (in this order):

1. `agents/tech-writer.md` — your own role definition.
2. `docs/PLAN.md` § Sprint 0 — what was supposed to happen this sprint.
3. The three other agents' issue files:
   - `issues/issue_sprint0_architect.md`
   - `issues/issue_sprint0_staff.md`
   - `issues/issue_sprint0_validator.md`
4. `README.md`, `CHANGELOG.md`, `MIGRATING.md` — for the first-time-reader dogfood pass.
5. `docs/prd/00-OVERVIEW.md` and `docs/prd/07-EKS-CLUSTER-SRIOV.md` — confirm internal consistency with PLAN.md and README.md.

## Coordinate with prior agents

You run **after** the architect / staff / validator have reported done. By the time you start:
- The prose surface should be retargeted at AWS (architect's work)
- The Go module / binary / IBM-strip should be complete and green (staff's work)
- The CI / cspell / e2e infrastructure should be retargeted (validator's work)

If any of those agents reported blockers, those blockers gate your review — note in your issue file that you reviewed against the in-progress state.

## Your scope

| Check | What to look for |
|---|---|
| **First-time-reader dogfood** | A senior engineer who's never seen this repo lands on `README.md`. Does it tell them what awsbnkctl is, what it doesn't do yet (pre-v0.1 status), and where to go for design context? Are the "planned" commands clearly flagged as planned rather than working? |
| **Cross-link integrity** | Every relative link in `README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/00-OVERVIEW.md`, `docs/prd/07-EKS-CLUSTER-SRIOV.md` resolves to a file that exists |
| **Terminology drift** | The same concept is named the same thing across docs (e.g., is it "self-managed node group" or "self-hosted node group" or "self-managed worker pool"? Pick one) |
| **Scope contradictions** | PRD 00 says X is inherited; PLAN.md treats X as new work — flag |
| **Sprint-count consistency** | PLAN.md says 6 sprints + Sprint 0; README, CHANGELOG, MIGRATING agree on the milestone tags (v0.2, v0.3, v0.5, v0.7, v0.9, v1.0) |
| **Code/binary identity** | After the staff agent's rename: are any docs still saying "roksbnkctl ..." in command examples? Are any commands still showing IBM-Cloud-specific flags? |
| **Build dogfood** | `make build` from a clean checkout produces a binary at the expected path; `./bin/awsbnkctl --help` runs cleanly; `./bin/awsbnkctl --version` reports something |
| **Reference docs sanity** | The auto-generated reference chapters (cobra-md, tfvars-md outputs) — if validator/staff regenerated them, do they match the current CLI surface? If they didn't regenerate, are they stale enough to mislead a reader (file as "regenerate in Sprint 5")? |
| **Visual rendering** | `mdbook build book/` produces clean HTML; spot-check the rendered preface and any chapter the architect agent updated |
| **CI workflow plausibility** | Skim `.github/workflows/*.yml` — do the workflow names, triggers, and job names make sense for awsbnkctl? Any obvious "this workflow would fail on first push" red flags? |
| **First-time contributor signal** | If a new contributor wants to start Sprint 1, can they figure out from the repo (without you) what to do? `prompts/sprint0/README.md` should signal that Sprint 0 is the kickoff template; `prompts/README.md` should explain the four-role pattern. |

## Tasks

1. **Dogfood the install path.** From a fresh checkout (or `git clean -fdx` if you can do it non-destructively):
   - `make build` — does it succeed?
   - `./bin/awsbnkctl --help` — does it print the expected command tree?
   - `./bin/awsbnkctl doctor` — does it run without panicking? Does its output reflect the AWS retarget (not IBM-Cloud-specific checks)?
   - File one issue per failure mode found.

2. **Cross-link audit.** For each file in the audit list, extract relative links and confirm each resolves. Use:
   ```
   grep -oE '\]\([^)]+\)' README.md CHANGELOG.md MIGRATING.md docs/PLAN.md docs/prd/00-OVERVIEW.md docs/prd/07-EKS-CLUSTER-SRIOV.md
   ```
   Then verify each path exists.

3. **Terminology audit.** Grep for likely-drift terms across the prose surface:
   - "self-managed node group" vs "self-hosted node group" vs "self-managed worker pool"
   - "EKS cluster" vs "EKS workspace" vs "EKS deployment"
   - "SR-IOV CNI" vs "sriov-cni" vs "SRIOV"
   - "IRSA" vs "IAM role for service account" vs "IAM role for SA"
   - File one issue per pattern with ≥2 inconsistent uses.

4. **Scope-contradiction audit.** Compare:
   - PLAN.md's per-sprint deliverables against PRD 00's inheritance map
   - PRD 07's spike protocol against PLAN.md's Sprint 1 days-1-3 description
   - CHANGELOG.md's "planned for v0.1.0" list against PLAN.md's Sprint 0 deliverables
   File one issue per material contradiction.

5. **CI workflow scan.** Open each `.github/workflows/*.yml` and check:
   - Does the workflow trigger on the events you'd expect (push to main, PRs, tags)?
   - Are the job step commands plausible for the post-strip codebase (e.g., are they running `go test ./...` against directories that still exist)?
   - File one issue per "this would fail on first run" red flag.

## Issue tracking

File all findings to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_tech-writer.md`:

```markdown
# Sprint 0 — tech writer issues

## Issue 1: short title
**Severity**: low | medium | high | blocker
**Status**: open
**Description**: what was found, with quotes / file:line references
**Files affected**: list of paths
**Proposed fix**: a one-sentence suggestion (integrator decides whether to act)
```

If everything is clean: heading + `*No issues filed.*` + one sentence about what you reviewed.

Severity guide:
- **blocker**: the binary doesn't build, build instructions are wrong, a critical cross-link is broken (e.g., README → MIGRATING)
- **high**: scope contradictions across PLAN/PRD/README, terminology drift on load-bearing terms (the SR-IOV decision specifically)
- **medium**: stale references, minor cross-link breakage, plausible-but-not-tested CI red flag
- **low**: typos, prose polish, formatting nits

## Verification before reporting done

- You've actually run `make build` and exercised the binary (not just read the files).
- Every issue you filed cites file:line references where applicable.
- You've read all three sibling agents' issue files before drafting your own (cross-reference any of their open issues in yours if they connect).
- You did NOT edit any project files — your issue file is your only output.

## Final report

Under 200 words:
- Files reviewed (counts)
- Build / smoke-test result of the binary
- Number of issues filed (with severity breakdown)
- Sibling-agent issue files you cross-referenced
- Whether the sprint is, in your judgement, ready for the integrator's commit (binary identity-rewrite commit + clean diff) — yes / yes-with-listed-followups / no with reason

Do NOT commit anything. The integrator commits the aggregated four-agent output (and decides whether to act on any of your filed issues before the commit, or to file them as open and act later).
