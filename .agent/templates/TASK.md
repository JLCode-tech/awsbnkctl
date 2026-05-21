# Task: {task-id}

Created: {timestamp}
Created-By: lead
Assigned-To: builder
Parent: {module-id or "standalone"}

## Runtime Hints (Optional)
- Human-Intent-Profile: {inherit-project-default|quick|build|hardening|mixed/undetermined}
- Risk-Posture: {low|normal|high|critical}
- Pattern-Hint: {auto|orchestrator-subagent|generator-verifier|agent-team|shared-state|message-bus}
- Harness-Hint: {auto|strict|balanced|model-led}
- Notes: {Optional plain-language intent from human/lead. Example: "Security-focused pass on auth/session flow."}

<!-- Runtime hints guidance (advisory-only):
- Pre-fill hints when human intent/risk is explicit or likely material for runtime choice.
- Leave Pattern-Hint/Harness-Hint as 'auto' when no strong signal exists.
- Leave Human-Intent-Profile as 'inherit-project-default' when no explicit posture intent is present.
- Hints inform lead selection but do not define effective runtime.
- Per ADR-020, STATUS.md Runtime Selection is canonical; TASK.md hints are optional/advisory.
-->

## Category
{bug | enhancement | refactor | spike | chore}

<!-- Category guidance:
- bug: broken behavior that needs correcting
- enhancement: new capability built on existing system
- refactor: internal restructure with no behavior change
- spike: time-boxed investigation/prototype, produces findings not production code
- chore: maintenance (deps, cleanup, config, docs)
- Choose exactly one. This informs review focus and escalation posture.
-->

## Summary
{One-line description of what changes. Example: "Add token expiry validation to the JWT middleware so expired tokens return a structured 401."}

<!-- Summary guidance:
- Single sentence, active voice, describes the end state.
- Durability over precision: describe behavior and interfaces, not file paths or line numbers.
- Builder reads this first — make it the clearest signal in the task.
-->

## Current behavior
{For bug/refactor: what happens now (the broken or messy behavior).
For enhancement: the status quo the feature builds on.
For spike/chore: the existing state being investigated or cleaned up.}

<!-- Current behavior guidance:
- Describe observable behavior, not implementation details.
- For bugs: include the failure mode and any known reproduction conditions.
- For enhancements: describe the gap — what the system cannot do today.
- Do NOT reference file paths or line numbers; describe interfaces and contracts instead.
-->

## Desired behavior
{What the system does after the work is complete.
Be specific about edge cases and error conditions.}

<!-- Desired behavior guidance:
- Describe outcomes, not steps. Builder decides how to implement.
- Cover error paths and boundary conditions explicitly.
- Good: "When token expiry is in the past, the middleware returns {error: 'token_expired', code: 401}."
- Bad: "Check the expiry field and return an error if it's wrong."
- Do NOT reference file paths or line numbers.
-->

## Key interfaces
- {TypeName or functionSignature() — what needs to change and why}
- {Config shape — any new configuration options needed}

<!-- Key interfaces guidance:
- Bulleted list of types, function signatures, and config shapes that need to change.
- Behavioral, not file-path-based. Name the contract, not the location.
- Example: "- SkillConfig type — add optional schedule field of type CronExpression"
- Example: "- validateToken() return type — currently throws on expiry; should return Result<Claims, TokenError>"
- Leave blank if no interface changes are needed (pure implementation fix).
-->

## Objective
{Clear, specific description. Not "build auth" but "Implement JWT token validation
middleware that checks signature AND expiry, returning structured 401 errors."}

## Acceptance Criteria
- [ ] {Criterion 1 — testable and unambiguous}
- [ ] {Criterion 2}
- [ ] {Tests pass}

<!-- If the task adds/modifies a mutating binary/CLI, include acceptance criteria that enforce
ADR-017 dry-run/safe-preview requirements (see docs/PROGRAMMATIC-FIRST-ADR.md). -->

## Expected Output
{What the deliverable looks like. "A middleware function exported from
src/auth/middleware.ts that returns 401 with {error: string, code: number}."}

## Out of Scope
- {Thing that should NOT be changed or addressed in this task}
- {Adjacent feature that might seem related but is separate}

<!-- Out of scope guidance:
- Required section — do not leave empty. Add at least one bullet.
- Prevents gold-plating and scope drift. Builder must not touch items listed here.
- Reviewer checks this list when assessing whether changes stay within bounds.
- Examples: "Migrating existing callers to the new interface", "Adding rate limiting (separate task)"
-->

## Tools & Sources
- Reference: {existing files to follow as patterns}
- Test with: {specific test command}
- Follow patterns in: .agent/PATTERNS.md
- {Any MCP servers or external resources to use}

## Execution Budget
- Complexity: small
- Target-Model-Tier: fast
- Expected-Turns: <= 12
- Restart-Threshold: after 2 failed revisions OR obvious context bloat
- Escalation-Before-Upgrade: narrow scope, checkpoint, start fresh session, add missing examples/tests

## Constraints
- Max-Revisions: 3
- Scope: {which files/directories CAN be modified}
- Do-Not-Modify: {files explicitly off-limits}
- Escalate-To: lead

## Context
{Background, dependencies, links to MODULE.md if applicable,
related tasks that feed into this one}
