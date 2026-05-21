# Task: {task-id}

Created: {timestamp}
Created-By: lead
Assigned-To: builder
Parent: {module-id or "standalone"}

## Runtime Hints (Optional)
- Human-Intent-Profile: {inherit-project-default|quick|build|hardening|mixed/undetermined}
- Risk-Posture: {low|normal|high|critical}
- Pattern-Hint: generator-verifier
- Harness-Hint: {auto|strict|balanced|model-led}
- Notes: {Optional bounded-task intent from human/lead}

## Objective
{Single bounded objective with low ambiguity.}

## Acceptance Criteria
- [ ] {Criterion 1 — explicit and testable}
- [ ] {Criterion 2 — explicit and testable}

## Verification Rubric
- Pass if: {explicit condition verifier can evaluate deterministically}
- Fail if: {explicit fail condition}
- Evidence expected: {tests, output diff, command result, etc.}

## Expected Output
{Concrete deliverable shape. Keep concise and specific.}

## Constraints
- Max-Iterations: 3
- Scope: {files/directories that can be modified}
- Do-Not-Modify: {files/systems explicitly off-limits}
- Escalate-To: lead

## Context
{Only the minimum background needed for bounded generator-verifier execution.}
