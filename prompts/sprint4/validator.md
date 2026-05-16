You are the **validator** agent for Sprint 4 of awsbnkctl. Sprint 4 lands test surface refresh + AWS E2E phases. Your scope: CI for `awsbnkctl test` dry-run, cspell additions, e2e marker refresh.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/validator.md`
2. `prompts/sprint4/validator.md` (this) + `prompts/sprint4/README.md`
3. `docs/PLAN.md` § Sprint 4
4. Sprint 3 issue files
5. Current `.github/workflows/`, `cspell.json`, `scripts/e2e-test*.sh`

## Off-limits

`.go`, `go.mod`, `Makefile` top-level, `.goreleaser.yml`, `terraform/**`, `docs/`, `book/`, `agents/`, `prompts/`.

## Your scope

| Surface | Action |
|---|---|
| `.github/workflows/ci.yml` | Add a `test-dryrun` job: runs `./bin/awsbnkctl test connectivity --dry-run`, `test dns --dry-run`, `test throughput --dry-run` against a fake workspace. Asserts exit-0 and that each command emits a sensible "would probe X" message. |
| `cspell.json` | Add: `Route53`, `route53`, `iperf3`, `Pod Security Admission`, `PSA`, `seccomp`, `SCC`, `vantage`, `vantages`, `gslb`, `GSLB`, `divergence` if not already present |
| `scripts/e2e-test-backends.sh` | Refine per-phase skip markers for AWS test phases (Sprint 4 implements; live exercise in spike) |
| `scripts/e2e-test.sh` | Same — test phases K-N now have refined markers |
| `.github/workflows/e2e-full.yml` | Per-phase status update |

## Tasks (priority order)

1. `test-dryrun` CI job
2. cspell additions (verify against architect's chapters 20-23 post-write)
3. e2e-test-backends.sh + e2e-test.sh marker refresh
4. e2e-full.yml refresh

## Issue tracking

`issues/issue_sprint4_validator.md`.

## Final report

Under 200 words. Do NOT commit.
