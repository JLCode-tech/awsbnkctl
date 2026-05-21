# Quick Profile Routing

Quick mode is for tightly bounded, low-risk tasks where turnaround speed is the primary objective.
It is an overlay on the shared framework, not a protocol variant.

## Shared-Protocol Constraint (Non-Negotiable)

Quick keeps the same core mechanics used by Build:

- Same task protocol (`TASK.md`, `STATUS.md`, `work/v{N}.md`, `reviews/r{N}.md`)
- Same builder ↔ reviewer loop and revision cap contract
- Same lead verification step against task acceptance criteria

Quick changes orchestration posture, not lifecycle mechanics.

## Entry Gate (All Must Be True)

Route to Quick only if all checks pass:

1. **Single objective**: one concrete outcome, no bundled sub-projects.
2. **Clear finish line**: acceptance criteria are explicit and testable.
3. **Low blast radius**: isolated surfaces, no shared protocol/rule changes.
4. **Low ambiguity**: requirements are understood without architectural decisions.
5. **Low dependency risk**: no uncertain external dependencies or migration coupling.

If any check fails, route directly to Build.

## Operating Posture (Quick Lead)

- Keep decomposition shallow (task-level, not broad multi-phase planning).
- Prefer `generator-verifier` for bounded tasks with explicit pass/fail criteria.
- Prefer `model-led` harness mode for low-risk execution unless stricter framework control is materially needed.
- Prefer one bounded task at a time over batching.
- Minimize orchestration overhead: short task specs, explicit scope boundaries, no speculative side quests.
- Enforce context budget aggressively: checkpoint between tasks rather than carrying long-running session history.

## Escalation Triggers (Quick → Build)

Escalate immediately when one or more are true:

1. Scope expands beyond the original bounded objective.
2. Requirements remain ambiguous after one clarification attempt.
3. Task touches shared/high-impact surfaces (core rules, protocol, model/deploy plumbing).
4. Review loop indicates non-trivial churn (critical issue or cross-cutting rework).
5. Revision count reaches 2 with unresolved issues (escalate before hard cap pressure).
6. Risk posture changes during execution (dependency uncertainty, hidden coupling, or broader blast radius discovered).

## Escalation Mechanics

- Preserve the same task artifacts; do not fork files or invent a Quick-only workflow.
- Update routing decision to Build and record why in `STATUS.md` notes/escalation fields.
- Continue execution under Build posture from the current task state.

## Role Shape (Quick)

- **Quick Lead**: strict scope gatekeeping + low-overhead orchestration + early escalation.
- **Builder**: minimal viable implementation for the explicit objective.
- **Reviewer Lite**: lighter review depth than Build reviewer posture, without bypassing core guardrails.
