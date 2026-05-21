# Engineering Standards

> **Builder: Read this before writing ANY code. Reviewer: Use this as your quality checklist.**
> This is the quality bar. Every commit must meet it.

---

## Core Principles

1. **Clarity over cleverness** — Maintainable, not impressive
2. **Explicit over implicit** — No magic. Make behaviour obvious
3. **Composition over inheritance** — Small units that combine
4. **Fail fast, fail loud** — Surface errors at the source
5. **Delete code** — Less code = fewer bugs. Question every addition
6. **Verify, don't assume** — Run it. Test it. Prove it works

---

## Before Writing Code (Think Before Coding)

1. **State assumptions explicitly** — If uncertain, ask rather than guess.
2. **If multiple interpretations exist, present them** — Don't pick silently.
3. **Push back when warranted** — If a simpler approach exists, say so.
4. **If something is unclear, stop** — Name what's confusing. Ask.
5. **Clarify requirements** — Restate the goal. Ask if ambiguous.
6. **Identify failure modes** — What can go wrong:
   - Invalid inputs, missing dependencies
   - Network/IO failures, concurrency issues
   - Resource exhaustion
7. **Classify by priority**:
   - **A. Core flow**: Happy path + direct error cases
   - **B. Edge cases**: Unusual but valid scenarios
   - **C. Out of scope**: Document, don't implement
8. **Check existing code** — Is this already solved? Extend, don't duplicate.

---

## Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

The test: Would a senior engineer say this is overcomplicated? If yes, simplify.

---

## Implementation Order (Goal-Driven Execution)

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Steps:
1. Write failing test for core happy path
2. Implement minimum code to pass
3. Write failing tests for error cases
4. Implement error handling
5. Refactor (tests stay green)
6. Edge case tests only after core is solid

---

## Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the task's requirements.

---

## Pre-Commit Checklist

- [ ] All tests pass
- [ ] No commented-out code
- [ ] No TODO without context (`// TODO: [reason] description`)
- [ ] Error messages are actionable
- [ ] No secrets, credentials, or hardcoded env-specific values
- [ ] Lint/format passes
- [ ] No unrelated "while I'm here" changes
- [ ] Every changed line traces to the task requirements
- [ ] Work file (work/v{N}.md) documents all changes

---

## DRY

1. Search first: `grep -r "pattern" src/`
2. Check shared/utils/common directories
3. Extract when logic appears 2+ times
4. But: don't over-abstract prematurely

---

## Code Organisation

### Dependency Direction

```
Features → Services → Core
    ↓          ↓        ↓
   UI      Business   Utilities
           Logic
```

- `Core/` depends on nothing internal
- `Services/` depends only on `Core/`
- `Features/` depends on `Services/` and `Core/`
- No circular dependencies
- Features are deletable without breaking unrelated code

### File Structure

- One type per file, filename matches type name
- Group by feature/domain, not by layer
- Shared utilities in `core/` or `common/` — zero domain dependencies

---

## Style

### Naming

| Type | Convention |
|------|------------|
| Types/Classes | `UpperCamelCase` |
| Functions/Variables | `lowerCamelCase` |
| Constants | `SCREAMING_SNAKE_CASE` (or follow language convention) |
| Files | `kebab-case` |

### Functions

- Do one thing. Name describes WHAT, not HOW.
- Max 3-4 parameters — beyond that, use a config/options object
- Avoid boolean parameters — they obscure intent at call sites:

```
// Bad — what do true, false mean?
build(scheme, true, false)

// Good — intent is clear
build(scheme, configuration: .release, clean: true)
```

### Imports

```
1. External packages
2. Internal modules (absolute paths)
3. Relative imports
4. Types
```

### Comments

- Explain WHY, not WHAT
- Delete comments that restate code
- TODO format: `// TODO: [context] description`
- Document non-obvious behaviour and workarounds

---

## Error Handling

1. Define domain-specific error types per module
2. Include context: what failed, with what inputs
3. Map external errors at boundaries — don't leak implementation details
4. Fail at the source — don't pass invalid state hoping someone handles it
5. Errors are API — design them like success paths

### Error Checklist

- [ ] Message helps diagnose the problem
- [ ] Includes relevant context (IDs, paths, values)
- [ ] Caller can distinguish error types programmatically
- [ ] Transient vs permanent failures are distinguishable

---

## CLI Mutation Safety

When a task adds/modifies a mutating binary or CLI, follow `docs/PROGRAMMATIC-FIRST-ADR.md`
(ADR-017) for the canonical dry-run/safe-preview policy. Implement and verify the preview path
alongside execute-path behavior.

---

## Testing

### Principles

- Tests are isolated: no shared state, no execution order dependencies
- One behaviour per test, descriptive names
- Tests run in parallel — design for it
- Test behaviour, not implementation
- Fast tests get run; slow tests get skipped

### Naming

```
test_{unit}_{condition}_{expectedResult}
```

Examples: `test_build_failsWhenSchemeNotFound`, `test_cache_fetchesAfterExpiration`

### Structure: Arrange-Act-Assert

```
func test_example() {
    // Arrange — setup preconditions

    // Act — execute behaviour under test

    // Assert — verify outcomes
}
```

### What to Test

| Test | Don't Test |
|------|------------|
| Public interface behaviour | Private implementation details |
| Error handling paths | Framework/language behaviour |
| State transitions | Trivial getters/setters |
| Business logic | Third-party library internals |

### Unit vs Integration

- **Unit**: Single component, mocked deps, fast
- **Integration**: Multiple components, real deps, slower
- Default to unit. Integration for: critical paths, complex interactions, external contracts

---

## Debugging

1. **Reproduce** — Can you trigger it consistently?
2. **Isolate** — Smallest input that fails?
3. **Read the error** — Full stack trace. Actually read it.
4. **Hypothesise** — One specific guess about the cause
5. **Test** — Add logging, write a test, inspect state
6. **Fix** — Change ONE thing. Confirm it's fixed.
7. **Regression test** — Ensure it can't silently break again

**Don't**: Change multiple things at once. Assume without evidence.
Delete error handling to "simplify". Fix symptoms instead of root causes.

---

## Refactoring

### When

- Before adding a feature (make the change easy, then make the easy change)
- After tests pass (not during implementation)
- When you touch code that's hard to understand

### When NOT

- While debugging
- Without test coverage
- Unrelated to the current task
- "While I'm here" changes — separate commit or ticket

### How

1. Ensure tests exist and pass
2. Make one structural change
3. Run tests
4. Repeat

Never change behaviour and structure in the same step.

---

## Dependencies

Before adding a new dependency:

1. Can we solve this in <100 lines ourselves?
2. Is it actively maintained?
3. What's the transitive dependency cost?
4. What's the license?
5. What if it disappears tomorrow?

Rules: Wrap third-party APIs behind interfaces you control.
Pin versions. Isolate imports to wrapper modules.

---

## Token Efficiency

- Never re-read files you just wrote. You know the contents.
- Never re-run commands to "verify" unless outcome was uncertain.
- Don't echo back large blocks of code unless asked.
- Batch related edits into single operations.
- Skip confirmations like "I'll continue..." — just do it.
- If a task needs 1 tool call, don't use 3. Plan before acting.

---

## When Uncertain

1. **Check existing patterns** — How does the codebase solve similar problems?
2. **Ask** — If the spec is unclear, escalate to lead. Don't guess at architecture.
3. **Smallest change** — Prefer minimal diff that solves the problem.
4. **Reversibility** — Prefer changes easy to undo.
5. **Prove it** — Run the code. Pass the tests. Don't guess.
