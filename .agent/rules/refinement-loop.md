# Refinement Loop Rules

> How the builder↔reviewer inner loop works, when to escalate, and how to prevent infinite cycles.

## The Inner Loop

```
Builder submits work → Reviewer reviews
                         ↓ REVISE → Builder fixes → Reviewer re-reviews (max 3 rounds)
                         ↓ APPROVE → Lead verifies against spec
                         ↓ ESCALATE → Lead handles
```

**Key principle**: The reviewer sends work back to the builder directly.
The lead is NOT involved in the inner loop. This keeps the cycle fast.

## Quality Gate Order

Run cheap checks first. Don't waste reviewer time on code that doesn't compile:

1. **Task protocol valid?** (`bin/maf-check.sh task <task-id>`) → No → Builder fixes task artifacts/protocol
2. **Tests pass?** → No → Builder auto-fixes (no reviewer needed)
3. **Lint clean?** → No → Builder auto-fixes
4. **Types pass?** → No → Builder auto-fixes
5. **Gate evidence valid?** (`bin/maf-check.sh gates <task-id>`) → No → Builder fixes work evidence
6. All pass → **Reviewer does semantic review**
7. Revision 2+ → Reviewer runs **stagnation check** (`bin/maf-check.sh stagnation <task-id>`)
8. Reviewer approves → **Lead verifies against original spec**

## Escalation Triggers

Any of these triggers escalation to the lead:

1. **Hard cap**: `revisions_used >= max_revisions` (default: 3)
2. **Stagnation**: >80% of critical issues in review N match review N-1 (check with `bin/maf-check.sh stagnation <task-id>`)
3. **No progress**: Issue count not decreasing across 2 consecutive reviews
4. **Scope expansion**: Fix requires changes outside the task's defined scope
5. **Ambiguity**: Reviewer cannot determine if output meets spec

## Revision Strategy Escalation

Don't just retry the same way. Escalate the approach:

- **Retry 1**: Builder (fast-tier) + specific feedback from reviewer
- **Retry 2**: Builder (fast-tier) + additional context (patterns, examples, test output)
- **Retry 3**: First ask whether checkpoint + fresh session would help; if not, upgrade to builder-upgraded (workhorse-tier)
- **Beyond 3**: Escalate to lead

## Cost-Aware Escalation

Before upgrading model tier, check whether failure is caused by context/spec/scope issues first.

See `.agent/rules/cost-control.md` for canonical escalation policy and tier defaults.

## Preventing Infinite Loops

1. **Hard cap is non-negotiable**. Default 3, ceiling 5. No exceptions.
2. **Track issue identity across reviews**. If the same issue appears in r1 and r2, that's stagnation.
3. **Quality must improve monotonically**. If issue count goes up, something is wrong.
4. **Wall clock awareness**. If a task has been looping for an unreasonable duration, escalate.

## What Good Feedback Looks Like

Reviews must be **specific, locatable, and actionable**:

- Include exact location (`file:line`), concrete problem statement, and specific fix guidance.
- Avoid vague comments that do not tell the builder what to change.
- Every issue should be directly actionable in one pass.

## After Approval: Lead Verification

Reviewer APPROVE does not mean done. The lead performs a final check:

1. Read original TASK.md acceptance criteria
2. Check each criterion against the actual work
3. **PASS**: Task is done. Archive it.
4. **FAIL**: Write specific notes → send back to builder (counts as a new revision)
5. **RE-SCOPE**: The spec itself was wrong. Update TASK.md and restart.

This catches **intent drift** — code that works and passes review but doesn't
solve the original problem.
