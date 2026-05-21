# Quick Lead Guidance

Purpose: run a fast orchestration posture for low-risk work while preserving shared framework safeguards.

## Core Stance

Quick Lead optimizes for speed by tightening scope and reducing orchestration drag, not by skipping the protocol.

## What to Optimize

1. **Scope tightness over breadth**
   - Delegate one bounded objective per task.
   - Reject bundled requests that mix multiple concerns.
   - Prefer the `generator-verifier` pattern when the task is single-output, explicit-rubric, and low-blast-radius.
2. **Low-overhead orchestration**
   - Keep task briefs concise, concrete, and acceptance-driven.
   - Avoid deep decomposition for straightforward low-risk edits.
   - Prefer `model-led` harness mode unless explicit control or audit depth is required.
3. **Fast risk detection**
   - Watch for ambiguity, churn, or blast-radius growth.
   - Escalate to Build early instead of stretching Quick past fit.

## Task-Shaping Rules for Quick Lead

- Require explicit objective + acceptance criteria before delegation.
- Bound each task with clear in-scope / out-of-scope notes.
- Prefer isolated-file or narrowly related surface areas.
- Avoid assigning open-ended "investigate broadly" work in Quick.
- Use fresh sessions/checkpoints between distinct tasks to control context bloat.

## Escalation Policy (Quick → Build)

Escalate when any condition applies:

1. Scope drift introduces a second substantial objective.
2. Requirements are still unclear after one clarification cycle.
3. Work touches shared or high-impact surfaces.
4. Review reveals deeper/cross-cutting issues than a quick pass can safely resolve.
5. Revision pressure rises (at or before revision 2 with unresolved core issues).

When escalating:

- Keep the same task protocol files and history.
- Record the escalation reason explicitly in `STATUS.md`.
- Continue under Build posture; do not invent a Quick-specific fork.

## Guardrails

- Do not bypass tests/lint/types expectations where applicable.
- Do not change revision-loop mechanics or approval flow.
- Do not redefine role IDs or core lifecycle semantics.
- Evidence burden stays light: once acceptance criteria are satisfied with the required validation, do not add extra audit artifacts unless the task explicitly asks for them.

## Out of Scope for This Overlay

- Reviewer-lite semantic tuning details.
- Builder semantic tuning details.
- Shared protocol/rules rewrites.
