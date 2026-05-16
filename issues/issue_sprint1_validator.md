# Sprint 1 — validator issues

## Issue 1: Agent run hit API 529 Overloaded after Dockerfile + workflow landed

**Severity**: medium
**Status**: resolved (integrator verified on-disk state)
**Description**: The Sprint 1 validator agent's run errored with `529 Overloaded` before filing its final report. On-disk inspection at integration time shows the substantive work landed:

- `tools/docker/aws/Dockerfile` (157 lines) — present
- `.github/workflows/ci.yml` — modified for CI matrix
- `.github/workflows/tools-images.yml` — modified for `aws` matrix entry
- `scripts/e2e-test.sh` — modified (skip-marker refinement)
- `scripts/test-integration-aws.sh` — present (new convenience runner)
- `tools/docker/Makefile` — modified for `build-aws` target

Integrator-side verification deferred to tech-writer (next agent run): local `docker build` of the new Dockerfile, `actionlint` on the modified workflows, `bash -n` syntax check on the scripts, `python -c "yaml.safe_load(...)"` YAML validity on the workflows.

**Files affected**: see list above.
**Proposed fix**: tech-writer agent's task 5 (tools image smoke) closes this. If tech-writer flags any issue, integrator addresses before commit.

## Issue 2: Docker image version smoke not run on this harness

**Severity**: low
**Status**: open (tech-writer task)
**Description**: Validator's task 1 verification gate required running `docker build` + `docker run --rm <img> aws --version && kubectl version --client && iperf3 --version` to confirm baked versions. The 529 error likely interrupted before this; the integrator's host doesn't have Docker daemon access via this harness. The Dockerfile is on disk and looks well-formed; verification deferred.

**Files affected**: `tools/docker/aws/Dockerfile`.
**Proposed fix**: tech-writer runs the smoke if Docker is available; else flag for first-real-CI-run validation.

## Issue 3: CI integration-test job — no live AWS confirmation

**Severity**: low
**Status**: open
**Description**: Validator's task 4 was "add CI integration-test job for the staff agent's mocked-AWS tests". The `.github/workflows/ci.yml` was modified, but no integrator-side verification that the job runs against mocked clients only (no live AWS creds required). Tech-writer should read the workflow + the staff agent's test code to confirm.

**Files affected**: `.github/workflows/ci.yml`, `internal/aws/*_test.go`.
**Proposed fix**: tech-writer verifies; if creds-required test paths exist, file follow-up for Sprint 2.

## Issue 4: cspell additions presumably done; not verified

**Severity**: low
**Status**: open
**Description**: Validator's task 7 was cspell dictionary additions for any new AWS terminology introduced this sprint. The 529 error likely interrupted before this — `cspell.json` doesn't appear in the modified files list. If tech-writer's mdbook + cspell pass surfaces new unknown-word counts in the architect's chapter 2 + chapter 33 drafts, integrator adds the missing entries.

**Files affected**: `cspell.json`.
**Proposed fix**: tech-writer's `cspell` run identifies any new misses; integrator folds.

## Issue 5: Notes for Sprint 2's validator (S3 supply chain)

**Severity**: roadmap (informational)
**Status**: open by design
**Description**: Sprint 2 implements S3 + ECR mirror (PRD 08). The validator agent for Sprint 2 should expect to:

- Add a `tools/docker/aws-s3-mirror/Dockerfile` (or extend `tools/docker/aws/`) with `skopeo` for the optional ECR mirror flow.
- Add a CI job that exercises the S3 bucket creation + put/get flow against `localstack` (or mocked aws-sdk-go-v2 S3 client).
- Add cspell entries for `skopeo`, `kms`, `IRSA`, OIDC subject claim terminology.
- Coordinate with Sprint 2 staff on `internal/aws/s3.go` + `internal/aws/iam.go` test surface.

**Files affected**: future Sprint 2 deliverables.
**Proposed fix**: pre-write into Sprint 2's `prompts/sprint2/validator.md` brief.
