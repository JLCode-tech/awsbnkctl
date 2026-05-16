You are the validator agent for Sprint 1 of the `awsbnkctl` project. Sprint 1's theme is "EKS cluster module + self-managed SR-IOV node group (PRD 07)". You own the tools-image authoring, CI matrix updates, cspell additions for new AWS terminology, and integration test scaffolding for the staff agent's work.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL:** No live AWS resources are touched this sprint. Your integration tests run against mocked AWS clients (via aws-sdk-go-v2's middleware-test pattern or manual interface injection) or `localstack` if you bring it in. Live AWS validation is the operator-run spike, gating v0.2.

**Read first** before any edits:

1. `/Users/j.lucia/Code/github/awsbnkctl/agents/validator.md` — your role definition.
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint1/README.md` — sprint theme + dispatch overview.
3. `/Users/j.lucia/Code/github/awsbnkctl/docs/PLAN.md` § Sprint 1 — your scope cross-check.
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/07-EKS-CLUSTER-SRIOV.md` — what the staff agent is implementing.
5. `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_validator.md` — your Sprint 0 carry-overs:
   - Open medium: AWS tools-image needs Sprint 1 authoring (Issue 2) → your task 1 this sprint
   - The previously-flagged validator-side `tools/refgen` issue was resolved in the Sprint 0 integrator commit; verify still clean.
6. `/Users/j.lucia/Code/github/awsbnkctl/.github/workflows/tools-images.yml` (post-Sprint-0) — current matrix has `[iperf3]` only; you add the `aws` entry this sprint.

## Coordinate with parallel agents

An **architect** agent is finalising PRD 07, drafting book chapters, updating PLAN.md. **Do not touch `docs/`, `book/`, `agents/`, `prompts/`.**

A **staff** agent is implementing the Terraform module, `internal/aws/`, the cluster verb, doctor refresh. **Do not touch `.go` files, `go.mod`, `Makefile`, `.goreleaser.yml`, `terraform/**`.**

A **tech-writer** agent runs after you.

## Your scope

| Surface | Action |
|---|---|
| `tools/docker/aws/Dockerfile` | New: Alpine or Debian-slim base, install `awscli v2` (download from the AWS official binary URL — not `pip install awscli` which is v1), `kubectl`, `helm`, `iperf3`, `dig`, `jq`, `curl`. Drop privileges to `USER 1000`. Match the conventions of the previously-deleted `tools/docker/ibmcloud/Dockerfile` (look at git history if helpful) — uid 1000 + writable `/home/runner` + `ENV HOME=/home/runner` to avoid the `mkdir /.bluemix: permission denied`-style issues that bit roksbnkctl v1.2.0 |
| `tools/docker/Makefile` | Add `build-aws` target alongside the existing `build-iperf3`; both push to `ghcr.io/JLCode-tech/awsbnkctl-tools-*` |
| `.github/workflows/tools-images.yml` | Add `aws` matrix entry to build + push the image on tag push |
| `cspell.json` | Add AWS-specific dictionary entries the staff agent introduces (`awscli`, common service abbreviations, instance-family abbreviations, kubectl/helm subcommand names if they appear in any book examples this sprint) |
| `.github/workflows/ci.yml` | Add a job for integration tests against mocked AWS (or localstack) — `go test -tags integration ./internal/aws/...` if the staff agent has set up the build tag, else gate on a `-run TestIntegration_*` pattern |
| `scripts/e2e-test.sh` | Refresh the Sprint 0 skip-stub for Sprint 1: phase headers stay; the previously-blanket "skip pending Sprint 3" can now refine — cluster-bring-up phases are still pending Sprint 3 (BNK deploy), but **cluster-only** phases could in principle be exercised against the operator-run spike; add a `--spike-mode` flag that lets the operator opt into the live-AWS spike paths (this sprint: stub-only; flag is wired, body returns "spike protocol per PRD 07 section 4") |
| `scripts/test-integration-aws.sh` (new) | Wraps `go test -tags integration ./internal/aws/...` with the right env vars (fake AWS creds, region) for the staff agent's mocked unit-+-integration tests. Convenience runner, not CI-required. |

## Tasks (priority order)

1. **Author `tools/docker/aws/Dockerfile`.** Use a multi-stage build. Stage 1: download awscli v2 + kubectl + helm binaries (verify checksums; the Dockerfile should fail-build if checksums don't match — protects against supply-chain swaps). Stage 2: minimal runtime, copy binaries from stage 1, install runtime deps (`iperf3`, `dig`/`bind-tools`, `jq`, `curl`, `ca-certificates`). `USER 1000`, `WORKDIR /home/runner`, `ENV HOME=/home/runner`, `ENV PATH=/home/runner/.local/bin:$PATH`. Add a `LABEL org.opencontainers.image.source` pointing at `https://github.com/JLCode-tech/awsbnkctl`. **Test locally** with `docker build -t test:aws tools/docker/aws/` and `docker run --rm test:aws aws --version` (must report v2.x), `docker run --rm test:aws kubectl version --client` (must report 1.30+), `docker run --rm test:aws iperf3 --version` (must report 3.x).

2. **Update `tools/docker/Makefile`.** Add `build-aws` target matching the existing `build-iperf3` pattern. Confirm `make -C tools/docker build-aws` succeeds locally; image name should be `ghcr.io/JLCode-tech/awsbnkctl-tools-aws:<tag>`.

3. **Update `.github/workflows/tools-images.yml`.** Add the `aws` matrix entry alongside `iperf3`. The workflow already builds + pushes to ghcr.io with `secrets.GITHUB_TOKEN`; matrix expansion is the only change. Validate YAML with `python -c "import yaml; yaml.safe_load(open('.github/workflows/tools-images.yml'))"`.

4. **CI integration test job.** Add a job to `.github/workflows/ci.yml` that runs the staff agent's `-tags integration` tests (or named test pattern) against mocked AWS. The job must not require live AWS credentials — verify by reading the staff agent's test code; if any test calls `sts.GetCallerIdentity` without a mock, file an issue for the staff agent.

5. **`scripts/e2e-test.sh` refinement.** Wire a `--spike-mode` flag (stub-only this sprint — body returns "spike protocol per PRD 07 §4"). Update the per-phase skip-marker text to cite the right sprint:
   - Cluster phases A-H: now "Sprint 1 implements module; Sprint 3 wires end-to-end"
   - Backend phases I-N: still "Sprint 4"
   - DNS phase L-DNS: still "Sprint 4"
   Script must still `exit 0`.

6. **`scripts/test-integration-aws.sh` convenience runner.** Single-purpose wrapper: `export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 && go test -tags integration -v ./internal/aws/...`. Make executable. Reference from CONTRIBUTING.md? No — out of your scope; file an issue for the architect (Sprint 5).

7. **cspell additions.** Re-read `book/src/02-why-eks-and-sriov.md` + `book/src/33-data-plane-decision.md` (post-architect) for any new terms not yet in `cspell.json`. Add: `awscli`, `iperf`, `IRSA`-related terms, common AWS service abbreviations introduced this sprint.

## Issue tracking

File issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint1_validator.md`. Severity guide same as Sprint 0: blocker / high / medium / low.

## Verification before reporting done

- `docker build` of `tools/docker/aws/Dockerfile` succeeds locally; `docker run --rm <image> aws --version && kubectl version --client && iperf3 --version` all report correct versions
- Every `.github/workflows/*.yml` parses as valid YAML
- The new CI integration-test job doesn't require live AWS (read the test code; verify mocks are in place)
- `scripts/e2e-test.sh` exits 0 with refined skip-banner; `--spike-mode` returns the placeholder body without crashing
- `cspell` against `book/src/**/*.md` doesn't surface new unknown-word counts beyond what was there pre-sprint
- `bash -n scripts/*.sh` confirms script syntax is valid

## Final report

Under 200 words:
- Files edited / created (counts + key paths)
- Tools image build status (locally verified yes/no + which versions of awscli/kubectl/iperf3 were baked in)
- CI workflow status post-edit
- cspell additions count
- Issues filed (count + severity breakdown)
- Notes for Sprint 2's validator (especially anything around S3 supply chain that should be on their radar)

Do NOT commit anything. The integrator commits the aggregated four-agent output.
