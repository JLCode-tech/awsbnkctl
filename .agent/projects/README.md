# Project Decomposition Protocol

> How large projects are broken down into manageable, executable tasks.

## Hierarchy

```
Project (large, multi-week)     — Human defines vision and goals
  └── Phase (days)              — Lead + Architect break down
       └── Module (hours)       — Lead + Architect specify
            └── Task (minutes)  — Builder executes, Reviewer checks, Lead verifies
```

## Directory Structure

```
.agent/projects/{project-id}/
├── PROJECT.md          # Vision, goals, constraints (human writes or approves)
├── PLAN.md             # Phases + modules (lead + architect produce)
├── PROGRESS.md         # Roll-up status (lead updates after each task)
└── phases/
    └── {phase-id}/
        ├── PHASE.md    # Phase scope, dependencies, module list
        └── modules/
            └── {module-id}/
                └── MODULE.md  # Detailed spec → generates tasks
```

## Decomposition Loop

The lead does NOT hand a project directly to a builder. Decomposition happens first:

```
Human defines Project
    ↓
Lead drafts Phase breakdown → Architect reviews → [loop until coherent]
    ↓
Lead drafts Module specs per Phase → Architect reviews → [loop]
    ↓
Lead creates Tasks from Module → Tasks enter the build/review/verify cycle
```

The architect validates each level:
- Are dependencies explicit and acyclic?
- Is scope per task achievable within a single builder session?
- Is anything missing (migrations, tests, error handling, config)?
- Are acceptance criteria testable and unambiguous?

## PROJECT.md Template

```markdown
# Project: {project-name}

Created: {timestamp}
Status: {planning|active|complete}

## Vision
{What are we building and why? 2-3 sentences.}

## Goals
1. {Goal 1}
2. {Goal 2}

## Constraints
- {Timeline, technology restrictions, compatibility requirements}

## Out of Scope
- {What we are explicitly NOT building}
```

## PLAN.md Template

```markdown
# Plan: {project-name}

Created: {timestamp}
Last-Updated: {timestamp}
Architect-Review: {pending|approved}

## Phases

### Phase 1: {name}
- **Goal**: {what this phase delivers}
- **Duration**: {estimate}
- **Depends-On**: nothing
- **Modules**:
  - {module-1}: {one-line description}
  - {module-2}: {one-line description}

### Phase 2: {name}
- **Goal**: {what this phase delivers}
- **Depends-On**: Phase 1
- **Modules**:
  - {module-3}: {one-line description}
```

## MODULE.md Template

```markdown
# Module: {module-name}

Phase: {phase-id}
Status: {planning|active|complete}

## Purpose
{What this module delivers. 1-2 sentences.}

## Files/Directories
- `src/auth/` — all auth-related code
- `src/auth/__tests__/` — auth tests

## Dependencies
- Requires: {other modules that must complete first}
- Feeds-Into: {modules that depend on this one}

## Tasks (generated from this spec)
- [ ] task-001: {description}
- [ ] task-002: {description}
- [ ] task-003: {description}

## Technical Notes
{Implementation guidance, API contracts, data models}
```

## PROGRESS.md Template

```markdown
# Progress: {project-name}

Last-Updated: {timestamp}

## Phase 1: {name}
- [x] Module: {name} ({N} tasks, all done)
- [ ] Module: {name} ({done}/{total} tasks, {N} in progress)

## Phase 2: {name}
- [ ] Module: {name} (not started)

Overall: {done}/{total} tasks | Phase 1: {X}% | Phase 2: {Y}%
```

Lead updates this after each task verification pass.
