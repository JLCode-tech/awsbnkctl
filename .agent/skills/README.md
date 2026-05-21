# Agent Skills

Reusable agent behaviors, lifted from the [mattpocock/skills](https://github.com/mattpocock/mattpocock-skills) pattern and adapted for MAF.

Skills live here as source files. Run `bin/maf-skills.sh sync` to deploy them to `.claude/skills/` where Claude Code picks them up.

## Buckets

| Bucket | Skills |
|--------|--------|
| `engineering/` | diagnose, tdd, to-issues, to-prd, triage, grill-with-docs, zoom-out, improve-codebase-architecture, setup-maf-skills |
| `productivity/` | caveman, grill-me, write-a-skill |
| `misc/` | (empty — reserved for repo-specific skills) |

## Frontmatter

Each `SKILL.md` uses this frontmatter:

```yaml
---
name: skill-name
description: Brief description. Use when [triggers].
user-only-trigger: true   # only on skills that must be invoked by the user, not auto-triggered
---
```

`user-only-trigger: true` is translated to `disable-model-invocation: true` by `bin/maf-skills.sh sync` when deploying to `.claude/skills/`.

## CLI

```
bin/maf-skills.sh check   # validate all SKILL.md files (run before sync)
bin/maf-skills.sh sync    # deploy to .claude/skills/ (idempotent)
bin/maf-skills.sh list    # show all skills with metadata
```

## Adding a new skill

1. Create `.agent/skills/{bucket}/{skill-name}/SKILL.md`
2. Add `name:` and `description:` (description must include "Use when")
3. Run `bin/maf-skills.sh check` — must exit 0
4. Run `bin/maf-skills.sh sync`
