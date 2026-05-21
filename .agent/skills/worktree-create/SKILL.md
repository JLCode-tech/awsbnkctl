---
name: worktree-create
description: Create a git worktree with MAF branch-local scaffolding and shared links. Use when starting a new branch in a sibling worktree.
---

**Run this when starting work on a new branch that needs its own worktree.**

Policy authority: `docs/WORKTREE-CONVENTIONS.md`.
Use this skill as the create/setup checklist.

`bin/maf-worktree.sh create` is the recommended single command for this workflow.
Use the manual steps below only when you need finer control.

This skill creates the git worktree, then runs `bin/maf-worktree.sh setup` to apply
the rerun-safe shared/local layout:

- shared `.agent/` symlink
- shared `.opencode/` symlink (when present)
- real root config copies (`opencode.json`, `CLAUDE.md`, `AGENTS.md`)
- branch-local `.agent-local/` scaffold (`TASK.md`, `STATUS.md`, `decisions/`)

## Prerequisites

Before running this skill, confirm:
- You are in the canonical/main repo checkout
- The main repo has `bin/maf-worktree.sh`
- You know the branch name and base branch

## Variables

Derive or ask for:

| Variable | Source | Example |
|----------|--------|---------|
| `BRANCH` | user/task spec | `feat/worktree-rerun-hardening` |
| `BASE` | user/task spec | `staging` |
| `MAIN_REPO` | current repo root | `/path/to/repo` |
| `WORKTREE_ROOT` | sibling convention | `/path/to/repo-worktrees` |
| `WORKTREE_PATH` | `$WORKTREE_ROOT/$BRANCH` or sanitized variant | `/path/to/repo-worktrees/feat/worktree-rerun-hardening` |

## Steps

### Recommended: single command

Run from the canonical main repo checkout:

```bash
bin/maf-worktree.sh create "$BRANCH" --base "$BASE"
# or with an explicit path:
bin/maf-worktree.sh create "$BRANCH" --base "$BASE" --path "$WORKTREE_PATH"
```

This handles fetch → worktree add → setup → next-steps in one call.

### Manual (when you need step-level control)

#### 1) Create the worktree

```bash
git -C "$MAIN_REPO" fetch origin
git -C "$MAIN_REPO" worktree add "$WORKTREE_PATH" -b "$BRANCH" "origin/$BASE"
```

If the branch already exists locally:

```bash
git -C "$MAIN_REPO" worktree add "$WORKTREE_PATH" "$BRANCH"
```

#### 2) Run MAF setup/reconcile

```bash
bin/maf-worktree.sh setup "$WORKTREE_PATH" "$MAIN_REPO"
# backward-compat form also accepted:
# bash "$MAIN_REPO/bin/maf-worktree.sh" "$WORKTREE_PATH" "$MAIN_REPO"
```

#### 3) Verify scaffold

```bash
git -C "$MAIN_REPO" worktree list
ls -la "$WORKTREE_PATH/.agent"
ls -la "$WORKTREE_PATH/.agent-local"
```

#### 4) Optional: seed branch task

If task details are known, update `.agent-local/TASK.md` and `.agent-local/STATUS.md`.

## Output

Report:
- Worktree path
- Branch and base branch
- `maf-worktree.sh` status (success/blockers)
- Whether `.agent-local/TASK.md` was populated
