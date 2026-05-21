# Conflict Prevention

> **All agents read. Builder follows. Reviewer checks.**

## Git
```bash
# Before work
git fetch origin && git pull origin main

# Before push
git rebase origin/main
```

**Never**: `git push --force`, `git reset --hard`

## Lock File Conflicts
```bash
git checkout --theirs package-lock.json
rm -rf node_modules && npm install
```

## Docker Compose
- Use override files for local changes
- Port ranges: Auth 3000-3099, API 3100-3199
- Validate: `docker-compose config`

## NPM
One agent installs at a time. Commit lock file immediately.

```bash
# If corrupted
rm -rf node_modules && npm cache clean --force && npm install
```

## Migrations
Use timestamps, not sequential numbers. One agent owns each table domain.

## Parallel Work in Multi-Agent Mode

In multi-agent mode, builders are scoped to their task's defined file list. Each task specifies which files it can modify. Escalate to lead if you need to touch files outside your scope.

## Emergency Reset
```bash
git stash && git checkout main && git pull
rm -rf node_modules && npm ci
docker-compose down -v && docker-compose up --build
```
