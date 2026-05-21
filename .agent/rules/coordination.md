# Coordination Rules

> How agents avoid stepping on each other. Read before modifying shared files.

## File Safety

| Category | Examples | Rule |
|----------|----------|------|
| **Safe** | Feature code in task scope | Agent works freely |
| **Shared** | package.json, config, schema | One agent at a time, noted in STATE.md |
| **Never touch** | Lock files, .git | No agent modifies these |

## Task Isolation

- Each task defines its scope in TASK.md (`Scope:` field)
- Builder MUST NOT modify files outside the task scope
- If a fix requires out-of-scope changes, escalate to lead

## Concurrent Work

- Only one builder task active at a time (Phase 1 simplification)
- Reviewer does not review while builder is actively revising
- Lead manages sequencing via delegation queue in STATE.md

## Shared File Protocol

When a task needs to modify a shared file (package.json, config, schema):
1. Builder notes it in `work/v{N}.md`
2. Reviewer specifically checks the shared file changes
3. Lead verifies shared file changes during verification loop

## State File Ownership

| File | Owner | Others |
|------|-------|--------|
| MEMORY.md | Lead writes | All read |
| CURRENT_WORK.md | Lead writes | All read |
| LESSONS.md | Any agent writes | All read |
| PATTERNS.md | Lead/Architect write | All read |
| agents/{role}/STATE.md | That role only | Lead reads |
| tasks/{id}/* | See task protocol | Role-based access |
