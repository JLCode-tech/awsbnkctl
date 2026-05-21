---
name: worktree-start
description: Orient in a git worktree -- detect shared vs local state, find branch task, start work
---

**Run this when starting work in a git worktree.**

This skill replaces `session-start` for worktree contexts.

Policy authority: `docs/WORKTREE-CONVENTIONS.md`.
Use this skill as an execution checklist, not as the policy source.

## Detection

You are in a worktree if:
- `.git` is a **file** (not a directory) — contains `gitdir: /path/to/.git/worktrees/...`
- `.agent/` is a **symlink** pointing to the main repo
- `.agent-local/` **exists** with branch-specific files (TASK.md, STATUS.md, decisions/)

If `.agent-local/` does **not** exist, tell the user to run:
```bash
bin/maf-worktree.sh setup <this-worktree-path> <main-repo-path>
# backward-compat form also accepted:
# bin/maf-worktree.sh <this-worktree-path> <main-repo-path>
```

For a quick CLI orientation check (non-mutating), the user can also run:
```bash
bin/maf-worktree.sh start [<this-worktree-path>]
```

If current directory is canonical main checkout, do not start feature
implementation there. Create/switch to sibling worktree per
`docs/WORKTREE-CONVENTIONS.md`.

## Orientation Steps

Run these in sequence:

1. **Read shared framework** — `.agent/AGENTS.md` (via symlink)
   Output: role structure, coordination rules

2. **Read branch task** — `.agent-local/TASK.md`
   Output: objective, acceptance criteria, resume section

3. **Read branch state** — `.agent-local/STATUS.md`
   Output: current phase, blockers, next action

4. **Check git state** — run:
   ```bash
   git branch --show-current && git log --oneline -5 && git status --short
   ```
   Output: branch name, recent commits, dirty files

5. **(On demand) Read shared memory** — `.agent/MEMORY.md`, `.agent/LESSONS.md`
   Only if needed for context. Don't read unless relevant.

## File Location Rules

**Shared state** (read from `.agent/` symlink):
- AGENTS.md, MEMORY.md, LESSONS.md, PATTERNS.md, DECISIONS.md
- rules/, templates/, agents/

**Branch-local state** (read/write in `.agent-local/`):
- TASK.md — branch objective and acceptance criteria
- STATUS.md — branch work state
- decisions/ — branch-specific ADRs

**Important:**
- Branch-local/provisional decisions go in `.agent-local/decisions/`
- Shared accepted decisions belong in `.agent/DECISIONS.md`
- Promotion from local → shared happens during completion (`worktree-complete`)

## Summary Output

After orientation, output:
- Branch name and base branch
- Current task objective (from TASK.md)
- Next action (from STATUS.md or TASK.md resume)
- Any blockers

Then ask: "Ready to continue?"
