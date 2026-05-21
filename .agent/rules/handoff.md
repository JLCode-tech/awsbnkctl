# Handoff & Compaction Protocol

> Operational handoff/compaction procedure. For context-budget and state-hygiene policy, see canonical rules.

## Auto-Compaction (Preferred)

Modern agents compact automatically.

To survive compaction:
1. Use **TodoWrite** for in-session task continuity.
2. Keep **CURRENT_WORK.md** updated every 2-3 meaningful tasks.
3. Capture durable learnings in **MEMORY.md** (facts/gotchas/decisions).
4. Commit meaningful checkpoints.

Canonical references:
- Context budget + read-on-demand: `.agent/rules/context-budget.md`
- Active-state cleanup shape: `.agent/rules/state-hygiene.md`

## Compaction Recovery

After compaction, immediately resume by:

1. Checking TodoWrite
2. Reading `CURRENT_WORK.md`
3. Continuing the in-progress item

Lead also checks `.agent/agents/lead/STATE.md` for delegation state.

## Manual Handoff (Fallback)

Use only when auto-compaction is unavailable or hard limits are hit.

1. Stop at safe checkpoint.
2. Keep backlog `current:` set.
3. Write `handoff/HANDOFF.md` (template).
4. Update `CURRENT_WORK.md`.
5. Commit checkpoint.

## Pickup

1. Read handoff → `CURRENT_WORK.md` → `AGENTS.md`
2. Read durable references on demand
3. Run local validation commands as needed
4. Continue from next action
5. Remove handoff file after progress
