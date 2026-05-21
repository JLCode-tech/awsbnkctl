# Agent Instructions

> **KEEP LEAN**: This file < 50 lines. Details in agents/, rules/, docs/.

## Roles (see `.opencode/models.env` for model assignments)

| Role | Tier | Purpose |
|------|------|---------|
| Lead | Frontier | Orchestrator — delegates, verifies, talks to human |
| Architect | Frontier | Validates decomposition, writes specs |
| Builder | Fast | Writes code following specs. Fast iteration. |
| Reviewer | Workhorse | Reviews code. Read-only. Finds issues. |

## Startup (All Agents)

1. Read this file; read your ROLE.md (`.agent/agents/{role}/ROLE.md`)
2. Read `.agent/ACTIVE_CONTEXT.md` — tiny mandatory checkpoint
3. Read `.agent/agents/{role}/STATE.md` — your own context
4. On demand: MEMORY.md / LESSONS.md / PATTERNS.md / DECISIONS.md (when relevant)
   - Architect: also read `.agent/context/architecture-language.md` (canonical vocabulary)
   - Skills referencing task system: read `.agent/config/task-system.md`
5. Check `.agent/tasks/active/` for assigned work

## Four Loops + Memory

1. **Decomposition**: Lead + Architect → phases → modules → tasks
2. **Build**: Builder codes following task spec
3. **Review**: Builder ↔ Reviewer inner loop (max 3 rounds)
4. **Verification**: Lead checks work against original spec. See `docs/DESIGN.md`.

- **Shared**: ACTIVE_CONTEXT.md (always); MEMORY / LESSONS / PATTERNS / DECISIONS (on demand)
- **Per-agent**: `.agent/agents/{role}/STATE.md`; **Tasks**: `.agent/tasks/active/{task-id}/`
- **Skills**: `.agent/skills/` (harness-agnostic behaviors; sync via `bin/maf-skills.sh`)
- **Config**: `.agent/config/` (task-system indirection, domain config)

## CLI-First Principle

Prefer `bin/maf-*` CLIs: `maf-status.sh` (state), `maf-context.sh` (bundles),
`maf-check.sh` (validation), `maf-brief.sh` (inspection), `maf-workspace.sh` (cross-project),
`maf-task.sh` (scaffolding), `maf-git.sh` / `maf-gh.sh` (git/GitHub),
`maf-skills.sh sync` (deploy skills to active harness). Fewer tool calls = less token waste.

## Rules

Read `rules/refinement-loop.md` (cycles), `rules/coordination.md` (shared files),
`rules/cost-control.md` (escalation), `rules/context-budget.md` (long context),
`rules/state-hygiene.md` (state expansion). Search before writing (DRY). Stay in task scope.
