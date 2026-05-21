# Archive Policy

> **Lead manages archiving. All agents read.**

## When to Archive
An item is eligible for archiving when ALL conditions are met:
1. **100% complete** — no open tasks or references
2. **3+ days old** — not actively referenced
3. **Not in active memory** — not mentioned in CURRENT_WORK.md or BACKLOG.md

## Where to Archive
| Source | Destination |
|--------|-------------|
| MEMORY.md stale entries | `memory/archive/YYYY-MM.md` |
| Completed sprint trackers | `archive/sprints/` |
| Session postmortems | `archive/sessions/` |
| Completed feature plans | `archive/features/` |
| Old planning docs | `archive/planning/` |
| Completed tasks | `.agent/tasks/done/` (TASK.md + STATUS.md only, delete work/ and reviews/) |

## How to Archive
1. Move the content (don't delete — context may be needed later)
2. Add a one-line summary in the archive index
3. Remove from the source file
4. Verify source file is under its line limit

## Failed Session Postmortems
When a session fails (bad approach, reverted work, wasted time):
1. Archive the session with explicit root cause analysis
2. Add "DO NOT" rules derived from the failure to LESSONS.md
3. Document the proposed correct fix for the next agent
4. List all files involved

## Archive Is Read-Only
Agents should **never modify** archived files. They're historical records.
Agents **may read** archived files if they need historical context.
