# State Hygiene Rules

> Active-state files exist to resume work quickly, not to preserve every historical detail.

## Core Rule

If a detail is not needed to choose the **next action**, it does not belong in an active-state file.

## Active-State Files

Treat these as operational surfaces:

- `ACTIVE_CONTEXT.md`
- `CURRENT_WORK.md`
- `.agent/agents/{role}/STATE.md`
- `.agent/tasks/active/`

They should stay lean, current, and action-oriented.

## Durable Reference Files

Treat these as read-on-demand references:

- `MEMORY.md`
- `LESSONS.md`
- `PATTERNS.md`
- `DECISIONS.md`

Do not assume they should be loaded every turn.

## Size / Shape Guidance

- `ACTIVE_CONTEXT.md` — tiny checkpoint only
- `CURRENT_WORK.md` — compact current status, blockers, next action
- `STATE.md` — queues and active context only

## Required Cleanup Behavior

After major progress or verification:

1. Remove completed items from active-state surfaces
2. Archive task history instead of appending long prose to active files
3. Update the exact next action
4. Keep only currently relevant constraints in `ACTIVE_CONTEXT.md`

## Repeated-Read Signal

If agents keep rereading the same large file for orientation, create or refresh a smaller summary/index file.
