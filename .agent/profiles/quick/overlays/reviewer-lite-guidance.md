# Quick Reviewer Lite Guidance

Purpose: define a reduced-depth Quick review posture for low-risk, tightly bounded tasks while preserving actionable feedback standards and shared refinement-loop mechanics.

## Core Stance

Reviewer Lite is **lighter than Build in depth and breadth**, not in rigor of communication.

- Lighter depth: prioritize high-signal blocking issues first; avoid exhaustive edge-case exploration by default.
- Same rigor: when issues are raised, feedback must still include **location + problem + fix guidance**.
- Same protocol: keep the shared builder↔reviewer loop, revision caps, and lead verification unchanged.

## What Is Intentionally Lighter Than Build

1. **Rubric-first verification over full semantic audit**
   - Start with explicit acceptance criteria / verification rubric checks, then correctness, safety, and scope-fit blockers.
   - Do not default to broad architectural critique for small low-risk tasks.

2. **Bounded checklist over comprehensive sweep**
   - Validate only what is required to safely merge the scoped objective.
   - Defer non-blocking polish unless it creates near-term risk.

3. **Concise issue set over exhaustive commentary**
   - Prefer a short list of high-impact findings.
   - Avoid speculative “nice-to-have” feedback that slows Quick cycles.

## Required Review Floor (Never Relax)

Reviewer Lite must still verify and enforce:

1. **Task alignment**
   - Output matches objective + acceptance criteria.
   - No silent scope expansion.

2. **Correctness and safety basics**
   - No obvious logic flaws in changed paths.
   - No clear safety regressions (e.g., auth/permission checks removed, unsafe defaults introduced).

3. **Feedback specificity standard**
   - Every flagged issue includes:
     - **Location** (`file:line` or nearest identifiable section)
     - **Problem** (specific risk/defect)
     - **Fix guidance** (concrete next action)

4. **Shared loop contract**
   - Use the same review outcomes (`REVISE`, `APPROVE`, `ESCALATE`) and max revision behavior from `.agent/rules/refinement-loop.md`.
   - Do not alter approval flow or bypass lead verification.

## Quick Reviewer Lite Decision Heuristic

Use this order:

1. **Blockers first**: correctness/safety/scope breakage.
2. **Then merge-readiness**: is this safe for the stated bounded objective?
3. **Then optional notes**: include only if high-value and low-noise.

If (1) and (2) are satisfied, avoid inventing additional depth solely to mimic Build posture.

Quick evidence rule: if the scoped acceptance criteria are met and no blocking safety/correctness issue remains, treat that as sufficient evidence unless the task explicitly requests more.

## Escalation Conditions (Reviewer Lite → Build Posture)

Escalate to deeper Build-depth expectations (or recommend Quick→Build profile escalation) when any apply:

1. **Risk surface exceeds Quick fit**
   - Changes touch shared/high-impact surfaces or cross-module behavior.

2. **Uncertainty is material**
   - Reviewer cannot confidently determine correctness/safety from bounded evidence.

3. **Churn indicates non-trivial complexity**
   - Repeated critical findings across revisions, slow convergence, or likely stagnation.

4. **Spec or dependency ambiguity blocks confident approval**
   - Acceptance criteria unclear, hidden dependency assumptions, or unresolved requirement gaps.

5. **Potential systemic impact discovered**
   - Evidence suggests deeper architectural, reliability, or security implications beyond lite review depth.

When escalating:

- State explicit trigger(s) in review notes.
- Keep issues actionable (location/problem/fix) where possible.
- Preserve existing task protocol files; do not fork mechanics.

## Anti-Rubber-Stamp Guardrails

- “Looks good” without checking objective/risk is not valid review.
- Lack of blockers is not enough; reviewer must still confirm scope fit and basic safety.
- Keep reviewer tone concise, but never vague.

## Out of Scope for This Overlay

- Changes to shared refinement-loop rules.
- Changes to role IDs, lifecycle states, or revision caps.
- Builder-specific Quick implementation behavior (covered separately).
