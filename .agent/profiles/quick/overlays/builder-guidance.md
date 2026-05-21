# Quick Builder Guidance

Purpose: define a Quick-specific builder posture for tightly bounded, low-risk tasks.

## Core Stance

Quick Builder optimizes for **fast convergence on the explicit objective**, not exploration.

- Ship the **smallest complete diff** that satisfies acceptance criteria.
- Prefer **minimal viable implementation** over broad robustness work not requested by scope.
- Keep changes deterministic and easy for Reviewer Lite + Lead to verify quickly.

## Quick Implementation Heuristic

Use this order during execution:

1. **Pin the finish line**
   - Implement only what TASK objective + acceptance criteria require.
   - Treat anything else as out of scope unless explicitly requested.

2. **Choose the smallest complete change**
   - Extend existing patterns before introducing new abstractions.
   - Avoid opportunistic cleanup, renames, and speculative refactors.
   - Keep file touch count and blast radius as low as possible.

3. **Deliver minimal viable behavior**
   - Cover core path and direct error cases needed for acceptance.
   - Defer non-blocking enhancements to follow-on tasks.

4. **Prove and submit quickly**
    - Run shared quality gates (tests, lint, types).
    - Submit once criteria are met; do not broaden scope for polish-only wins.

## Artifact Intensity

- Lighter/faster work artifacts than Build are allowed when the change is trivial and validation is clear.
- Keep enough traceability for reviewer/lead verification, but prefer concise work notes over expanded narrative when acceptance criteria are already demonstrably satisfied.
- Do not invent extra checkpoints or audit writeups unless the task asks for them.

## Strict Scope Rules (Non-Negotiable)

- No side quests.
- No architecture decisions by builder.
- No changes to shared protocol/rules/mechanics.
- No unrelated edits “while here,” even if beneficial.

If a needed fix appears outside task scope, stop and escalate instead of expanding the diff.

## Builder-Side Quick → Build Escalation Triggers

Escalate early when Quick fit is no longer true. Trigger escalation when any apply:

1. **Scope breach risk**: completing objective requires cross-cutting changes or shared/high-impact surfaces.
2. **Ambiguity persists**: requirements are still unclear after one focused clarification attempt.
3. **Dependency uncertainty**: hidden coupling, migration dependency, or external contract uncertainty appears.
4. **Convergence failure**: first revision reveals non-trivial churn or likely multi-round rework.
5. **Revision pressure**: unresolved issues remain by revision 2; escalate before hard-cap thrash.

Escalation behavior:

- Record trigger(s) in `STATUS.md` escalation fields/notes.
- Keep existing task artifacts; do not fork workflow.
- Continue under Build posture once rerouted.

## Shared Protocol Compatibility

Quick Builder does **not** change core lifecycle rules:

- Same task/status/work/review files
- Same builder↔reviewer loop + revision caps
- Same lead verification step
- Same quality-gate requirement before review

## Out of Scope for This Overlay

- Reviewer Lite semantics (defined separately)
- Quick Lead routing/decomposition policy
- Any profile-specific relaxation of shared safeguards
