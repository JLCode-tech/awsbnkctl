You are the validator agent for Sprint 3 of `awsbnkctl`. Sprint 3 ports the inherited modules + lands first end-to-end `up --dry-run`. Your scope: CI matrix for full-up dry-run integration, e2e script per-phase marker refinement, cspell additions.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/validator.md`
2. `prompts/sprint3/validator.md` (this) + `prompts/sprint3/README.md`
3. `docs/PLAN.md` § Sprint 3
4. Sprint 2 carry-over issue files
5. Current state of `.github/workflows/*.yml`, `cspell.json`, `scripts/e2e-test*.sh`

## Coordinate with parallel agents

**architect**: prose, PRDs. **staff**: Terraform port, cred/exec retarget, doctor gate.

Off-limits for you: `.go`, `go.mod`, `Makefile` top-level, `.goreleaser.yml`, `terraform/**`, `docs/`, `book/`, `agents/`, `prompts/`.

## Your scope

| Surface | Action |
|---|---|
| `.github/workflows/ci.yml` | Add a `full-up-dryrun` job: runs `./bin/awsbnkctl up --dry-run` against fake AWS creds, asserts exit-code 0 and that the plan output mentions all 7 modules (eks_cluster, cert_manager, s3_supply_chain, iam_irsa, flo, cne_instance, license, testing) |
| `scripts/e2e-test.sh` | Phase A-H markers refine: cluster phases were "Sprint 3" — now they're "Sprint 3 implements dry-run; spike validates apply"; BNK trial phases stay "Sprint 3 implements; live apply gates on spike" |
| `scripts/test-integration-aws.sh` | Add the full-up-dryrun invocation alongside the existing per-package `go test` calls |
| `cspell.json` | Add any new terminology from the ported modules (likely none beyond what's already there post-Sprint-2 — but check) |
| `.github/workflows/e2e-full.yml` | Skip-stub gate update: trigger surface still gated to JLCode-tech/awsbnkctl; body now references PRD 07 spike |

## Tasks (priority order)

1. Full-up-dryrun CI job
2. e2e-test.sh phase marker refinement
3. test-integration-aws.sh extension
4. cspell sweep against staff's module work
5. e2e-full.yml stub refresh

## Issue tracking

`issues/issue_sprint3_validator.md`.

## Final report

Under 200 words. Do NOT commit.
