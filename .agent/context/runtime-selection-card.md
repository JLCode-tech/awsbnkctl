# Runtime Selection Card (On-Demand)

Use this card only when selecting or explaining task runtime.
Do not treat it as startup-required context.

## Runtime Tuple

`profile / pattern / harness_mode`

Baseline fallback: `build / orchestrator-subagent / balanced`

## Precedence

Explicit human intent > task shape + risk heuristics > project default > global baseline

## Profile Selection

- `quick`: speed-first, tightly bounded, low-risk, explicit acceptance criteria
- `build`: normal product delivery with standard quality depth
- `hardening`: security/reliability/performance risk-first, deeper evidence burden

## Pattern Selection

- `generator-verifier`: use only when all true — one bounded output, explicit pass/fail criteria, low blast radius, low ambiguity
- `orchestrator-subagent`: clear decomposition with bounded subtasks + synthesis
- `agent-team`: broad work with mostly independent long-running partitions
- `shared-state`: agents need each other's findings continuously while working
- `message-bus`: event-driven routing/trigger semantics dominate workflow
- Fallback when uncertain: `orchestrator-subagent`

## GV Escalation Triggers

Escalate GV -> `orchestrator-subagent` immediately if any appear:
- second substantial objective
- acceptance criteria become ambiguous
- shared/high-impact surfaces enter scope
- verification churn exceeds iteration 2 without convergence

## Harness Selection

- `strict`: strong audit/security controls and explicit boundary enforcement
- `balanced`: default delivery; framework sets major guardrails, model handles local flow
- `model-led`: exploratory work where rigid orchestration adds friction

## Communication Rule

- Default human communication: posture-first concise language.
- Include full tuple when asked, when non-defaults are material, or when cost/latency/evidence impact is significant.
