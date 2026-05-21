---
name: worktree-complete
description: Finish a worktree task -- merge branch-local decisions, squash-merge to parent, clean up
---

**Run this when a worktree task is complete and ready to land on its parent branch.**

Policy authority: `docs/WORKTREE-CONVENTIONS.md`.
Use this skill as completion procedure only.

This is the MAF-specific completion flow that promotes selected branch-local
decisions into shared `.agent/DECISIONS.md`, then squash-merges branch work.

## Prerequisites

Before running this skill, confirm:
- All acceptance criteria in `.agent-local/TASK.md` are met
- All changes are committed on the current branch
- Tests/linting passes (if applicable)
- No unresolved blockers

## Variables

Derive these at the start:

```bash
BRANCH=$(git branch --show-current)
BRANCH_DIR=$(echo "$BRANCH" | tr '/' '-')
WORKTREE_PATH=$(git worktree list --porcelain | grep -B2 "branch refs/heads/$BRANCH" | grep "^worktree " | sed 's/^worktree //')

# Derive main repo from the .agent symlink target
MAIN_REPO=$(readlink .agent | sed 's|/.agent$||')
# If the symlink is relative, resolve it to absolute path
if [[ "$MAIN_REPO" != /* ]]; then
    MAIN_REPO=$(cd "$(dirname .agent)/$MAIN_REPO" && pwd)
fi
```

Read the **Base** field from `.agent-local/TASK.md` to get the parent branch.
Do NOT merge to `main` unless the task file explicitly says so.

## Steps

### 1. Merge branch-local decisions (if any)

If `.agent-local/decisions/` contains files:

a. Read each `.md` file in `.agent-local/decisions/`
b. Switch to main repo (where `.agent/` is NOT a symlink):
   ```bash
   cd <main-repo-path>
   ```
c. Append relevant content to `.agent/DECISIONS.md` with header:
   ```markdown
   ## D{N}: {Decision Title}
   
   **Date:** {date} | **Status:** Accepted | **Source:** {branch-name}
   {decision content}
   ```
d. Commit in main repo:
   ```bash
   git add .agent/DECISIONS.md
    git commit -m "docs: merge decisions from $BRANCH"
    ```

Only promote decisions that should become shared/accepted.

### 2. Archive the full branch-local state

Copy the entire `.agent-local/` directory to main repo's `.agent/tasks/done/{BRANCH_DIR}/`
for audit purposes. This preserves TASK.md, STATUS.md, and all decisions.

a. Copy the directory:
   ```bash
   cd "$MAIN_REPO"
   mkdir -p .agent/tasks/done/${BRANCH_DIR}
   cp -r "$WORKTREE_PATH/.agent-local/." .agent/tasks/done/${BRANCH_DIR}/
   ```

b. Prepend a completion header to the archived TASK.md:
   ```bash
   HEADER=$(cat <<HEADER_EOF
   > **Completed**: $(date +%Y-%m-%d)
   > **Branch**: $BRANCH
   > **Merged to**: $MERGE_TARGET
   > **Final commit**: $(git -C "$WORKTREE_PATH" rev-parse --short HEAD)

   HEADER_EOF
   )
   echo "$HEADER" | cat - .agent/tasks/done/${BRANCH_DIR}/TASK.md > /tmp/task_tmp \
       && mv /tmp/task_tmp .agent/tasks/done/${BRANCH_DIR}/TASK.md
   ```

c. Commit in main repo:
   ```bash
   git add .agent/tasks/done/${BRANCH_DIR}/
   git commit -m "docs: archive worktree state for $BRANCH_DIR"
   ```

### 3. Squash-merge to parent

Switch to the parent branch in the main repo (or another worktree that has it):

```bash
cd <main-repo-or-parent-worktree>
git checkout "$MERGE_TARGET"
git merge --squash "$BRANCH"
git commit -m "feat($BRANCH_DIR): <one-line summary>

<bullet list of changes from TASK.md>"
```

This creates a single commit on the parent with the full diff from the branch.

### 4. Verify the merge

```bash
git log --oneline -5
git diff HEAD~1 --stat
```

Confirm the merged files match what the task expected.

### 5. Remove the worktree

```bash
git worktree remove --force "$WORKTREE_PATH"
```

Use `--force` because `.agent` symlink and `.agent-local/` are untracked.

### 6. Delete the branch

The branch is now fully merged (squashed). Delete it:

```bash
git branch -D "$BRANCH"
```

Note: `-D` (force delete) is needed because git doesn't recognize squash-merges
as "merged" for `-d` safety checks. This is safe because we verified the merge
in step 4.

### 7. Report

Output:
- Squash commit hash on parent branch
- Files changed (from `git diff HEAD~1 --stat`)
- Archive location (`.agent/tasks/done/{BRANCH_DIR}/`)
- Archived files: TASK.md, STATUS.md, decisions/* (list them)
- Decisions merged to DECISIONS.md: yes/no (how many)
- Worktree removed: yes/no
- Branch deleted: yes/no
