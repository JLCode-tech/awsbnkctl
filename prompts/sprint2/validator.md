You are the validator agent for Sprint 2 of the `awsbnkctl` project. Sprint 2's theme is "S3 supply chain + IRSA (PRD 08)". You own the Dockerfile multi-arch fix (closes Sprint 1 tech-writer Issue 6), CI integration scaffolding for S3 (mocked or localstack), cspell additions for new IRSA / OIDC / KMS / skopeo terminology.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** — no live AWS this sprint.

**Read first** before any edits:

1. `/Users/j.lucia/Code/github/awsbnkctl/agents/validator.md`
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/validator.md` (this file)
3. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/README.md`
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md`
5. Sprint 1 carry-overs: `issues/issue_sprint1_validator.md` (Issue 5 — your pre-written notes) and `issues/issue_sprint1_tech-writer.md` (Issue 6 — Dockerfile arm64).
6. Current state: `tools/docker/aws/Dockerfile` (Sprint 1, linux/amd64 hardcoded), `.github/workflows/{ci,tools-images}.yml`.

## Coordinate with parallel agents

An **architect** agent is finalising PRD 08, drafting chapter 25. **Do not touch `docs/`, `book/`, `agents/`, `prompts/`.**

A **staff** agent is implementing the Terraform modules, `internal/aws/{s3,iam}.go`, workspace schema retarget, `awsbnkctl init` AWS path, doctor extensions. **Do not touch `.go`, `go.mod`, `Makefile`, `.goreleaser.yml`, `terraform/**`.**

## Your scope

| Surface | Action |
|---|---|
| `tools/docker/aws/Dockerfile` | Multi-arch fix per tech-writer Sprint 1 Issue 6: `ARG TARGETARCH` → case statement → architecture-specific awscli zip URL (`linux-x86_64` vs `linux-aarch64`); same TARGETARCH-aware treatment for kubectl + helm download URLs (both publish per-arch binaries); dual-arch sha256 checksum pins |
| `.github/workflows/tools-images.yml` | Update build to push multi-arch manifest list (`platforms: linux/amd64,linux/arm64`); use `docker/build-push-action` with the right platform flag |
| `.github/workflows/ci.yml` | Add an `S3 integration` job that runs `go test -tags integration ./internal/aws/...` against a `localstack` service container (or a mocked-only path if localstack would gate the test on Docker-in-CI we can't pay for) |
| `cspell.json` | Add: `IRSA`, `OIDC`, `skopeo`, `KMS`, `CMK`, `kms`, `cmk`, `aarch64`, `OpenID`, `presigned`, `webhook`, `Webhook`. Verify against the new chapter 25 + PRD 08 |
| `scripts/test-integration-aws.sh` | Update to handle the new S3 + IAM integration test paths |

## Tasks (priority order)

1. **Dockerfile multi-arch.** Tech-writer Issue 6 has the exact pattern to apply. Test locally with `docker buildx build --platform linux/amd64,linux/arm64 -t test:aws-multi tools/docker/aws/` (requires buildx); run `aws --version`, `kubectl version --client`, `helm version --short`, `iperf3 --version` on both arches if possible. If you can't test arm64 locally, note in issue file and rely on CI to validate.

2. **tools-images.yml multi-arch.** Update the matrix and the `docker/build-push-action@v5` invocation to `platforms: linux/amd64,linux/arm64`. Verify YAML parses.

3. **CI S3 integration job.** Two paths possible: localstack (`services: localstack:` in GHA), or pure mocks (`go test -tags integration -run TestMocked_*`). PRD 08's design supports both — use mocks for v1.0 and file an issue noting localstack is a v1.x enhancement.

4. **cspell additions.** Verify against `book/src/25-cos-supply-chain.md` (post-architect) + `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` for any new terms.

5. **`scripts/test-integration-aws.sh`** — add S3 + IAM test invocations.

## Issue tracking

File to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint2_validator.md`.

## Verification before reporting done

- `docker build` of the updated Dockerfile succeeds on linux/amd64 (validator's typical host); multi-arch build succeeds if buildx available
- Every `.github/workflows/*.yml` parses as valid YAML
- `cspell` over `book/src/25-*.md` + `docs/prd/08-*.md` produces no new unknown-word counts
- `bash -n scripts/test-integration-aws.sh` passes

## Final report

Under 200 words — files edited, Dockerfile multi-arch verification result, CI workflow status, cspell additions count, integration-test approach chosen (localstack vs mocks), notes for Sprint 3 validator. Do NOT commit.
