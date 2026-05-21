# Status: {task-id}

State: draft
Current-Version: v0
Revisions-Used: 0 / 3
Assigned-To: builder
Last-Updated: {timestamp}

## Cost / Context Signals
Context-Risk: low
Restart-Recommended: no
Escalation-Reason:
Notes: Prefer checkpoint + fresh session before model upgrade when context grows.

## Runtime Selection
Profile: build
Pattern: orchestrator-subagent
Harness-Mode: balanced
Model-Led-Evidence:
Runtime-Suggestion: {suggested tuple from maf-suggest-runtime, e.g. build / orchestrator-subagent / balanced}
Runtime-Suggestion-Rationale: {compact advisory rationale from maf-suggest-runtime output}
Runtime-Suggestion-Decision: {accepted|overridden|not-run}
Runtime-Suggestion-Override-Rationale: {required if lead overrides suggestion; otherwise leave blank}
Project-Default-Profile: build
Session-Intent: inherit-project-default
Task-Override: none
Selection-Rationale: {Why this tuple is active for this task. Keep compact.}

<!-- Task-Override guidance:
- Canonical values: none | inherit-project-default | quick | build | hardening | mixed/undetermined
- Use 'none' when task inherits session/default posture.
- Use 'inherit-project-default' when you need explicit inherited semantics recorded in task metadata.
- Use other values only when this task intentionally differs from session/default posture.
- STATUS.md is canonical for effective runtime (ADR-020); TASK.md runtime hints are advisory only.
- Runtime-Suggestion* fields record advisory output from `bin/maf-suggest-runtime.sh`; they do not auto-mutate the effective runtime tuple.
 -->

## Runtime Change Log
| Timestamp | From (P/P/H) | To (P/P/H) | Trigger | Reason |
|-----------|---------------|------------|---------|--------|

<!-- Add one row per runtime escalation/demotion/update.
Required fields per row: Timestamp, From tuple, To tuple, Trigger, Reason.
Tuple format: <profile>/<pattern>/<harness-mode> (e.g., build/orchestrator-subagent/balanced).
Profile segment may use `none` when recording current/default no-override semantics.
-->

## State Transition Log
| Timestamp | From State | To State | Trigger |
|-----------|------------|----------|---------|

<!-- Required when Harness-Mode is strict.
Record every state transition with a parseable timestamp and concise trigger.
-->

## Revision History
| Version | Submitted | Reviewed | Verdict | Notes |
|---------|-----------|----------|---------|-------|

## Lead Verification
Verified: pending
Notes:
