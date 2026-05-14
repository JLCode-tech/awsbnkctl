You are the validator agent for Sprint 0 of the `awsbnkctl` project. The repo is a hard fork of `jgruberf5/roksbnkctl` being retargeted at AWS EKS. Sprint 0's theme is "identity rewrite + IBM strip + AWS stub". You own the regression gate, CI matrix, spellcheck dictionary, tools-image build matrix, and e2e drivers.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`.

**Read first** before any edits:

1. `docs/PLAN.md` § Sprint 0 — your scope is bounded by the "Test deliverables" and CI-related items in the deliverables table.
2. `docs/prd/00-OVERVIEW.md` — confirms which PRDs are inherited (their existing tests stay) vs net-new (Sprint 4 owns AWS-shaped E2E phases; you skip-mark anything that depends on AWS surface that doesn't exist yet).
3. `agents/validator.md` — your own role definition.
4. The existing CI matrix: `.github/workflows/*.yml`. Note which workflows trigger on what paths.
5. `tools/docker/` — note which images are built and which are IBM-specific.
6. `scripts/e2e-test.sh` (and any other `scripts/*.sh`) — note which phases are IBM-specific.

## Coordinate with parallel agents

An **architect** agent is touching the prose surface (`README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/`, `agents/`, `book/src/preface.md`, `book/src/SUMMARY.md`). **Do not touch any of those files.**

A **staff** agent is rewriting the Go module path, renaming `cmd/`, deleting `internal/ibm/` + `internal/cos/` + `terraform/modules/roks_cluster/` + `tools/docker/ibmcloud/`, stubbing `internal/aws/` + `terraform/modules/eks_cluster/`, and updating `Makefile` / `.goreleaser.yml` / `embedded.go`. **Do not touch `.go` files, `go.mod`, `Makefile`, `.goreleaser.yml`, `embedded.go`, or any `terraform/**` file.**

For the `tools/docker/ibmcloud/` deletion: the staff agent removes the directory; you remove the corresponding **workflow job** that built it. Coordinate via your issue file if the staff agent's deletion is racing your workflow edit.

## Your scope

| Surface | Action |
|---|---|
| `.github/workflows/ci.yml` | Update workflow name from `roksbnkctl` references → `awsbnkctl`; update path filters that scope to deleted directories |
| `.github/workflows/book.yml` | Update `publish_dir` / gh-pages target if it references `jgruberf5.github.io`; new target `JLCode-tech.github.io/awsbnkctl/book/` |
| `.github/workflows/release.yml` | Update binary name references; update artifact-upload paths |
| `.github/workflows/tools-*.yml` | Drop the job(s) that build IBM-specific tool images (the `tools/docker/ibmcloud/` directory the staff agent is deleting); keep generic tool-image jobs |
| Any other workflow referencing `roksbnkctl` | Sweep for stragglers; update or file an issue |
| `cspell.json` | Drop IBM-cloud-specific dictionary entries (e.g., `ibmcloud`, `roks`, specific IBM service names); add AWS-specific entries (`awsbnkctl`, `IRSA`, `eksctl`, `kubelet`, `kubeadm`, instance types like `c5n`, `m5n`, common AWS service abbreviations) |
| `scripts/e2e-test.sh` | Mark every IBM-specific phase with an explicit `echo` + early-exit citing the sprint that retargets it; the script must still run end-to-end (reporting "skipped due to AWS retarget" cleanly) even though no phases execute substantively |
| `scripts/*.sh` (other) | Update binary-name references (`roksbnkctl` → `awsbnkctl`); update any IBM-flavoured pre-flight checks |
| `tools/docker/Makefile` (if exists) | Drop the `ibmcloud` image build target |
| `tools/refgen/` and `tools/cobra-md/` and `tools/tfvars-md/` | Update binary-name references in the generator outputs (these tools regenerate book chapters); not running them in Sprint 0 — Sprint 5 owns the regeneration — but the binary-name reference in the generator source should still update |
| `tools/ciwatch/`, `tools/sprintwatch/` | Same: update binary-name references; don't run them |
| `.gitignore` | Confirm `bin/awsbnkctl` would be ignored if `bin/roksbnkctl` was; tweak if needed |

## Tasks (priority order)

1. **CI workflow sweep.** Grep `.github/workflows/` for `roksbnkctl`, `jgruberf5`, `ibmcloud`, `roks`, `cos` references; update each. Update job names + workflow `name:` fields. For the `book.yml` deploy target — update the `gh-pages` publish URL framing (the actual `peaceiris/actions-gh-pages` deploy uses `secrets.GITHUB_TOKEN` and the current repo's gh-pages branch, so no functional change beyond the workflow name and any explicit URLs).

2. **cspell sweep.** Drop IBM-specific dictionary entries that no longer appear in the codebase. Add AWS-specific entries the new PRDs introduce: `awsbnkctl`, `IRSA`, `eksctl`, `kubeconfig`, `kubectl`, `kubelet`, `multus`, `sriov`, `containerd`, `c5n`, `m5n`, instance-family abbreviations, common AWS service names (`s3`, `ecr`, `sts`, `iam`, `eks`, `ena`). Run `cspell` against `book/src/**/*.md` (if cspell is installed) and the new PRDs — none of those should newly fail.

3. **e2e-test.sh retarget.** This is the most subjective task. The inherited script has phases A-H + I-N + L-DNS keyed off ROKS endpoints. Each phase needs a header echo + early-return / skip-marker citing the sprint that retargets it:
   - Phases A-H (cluster bring-up) → Sprint 3
   - Phases I-N (backend matrix) → inherited surface, retargets in Sprint 4
   - Phase L-DNS → Sprint 4
   - Add a final summary echo at end: "Sprint 0 stub: all e2e phases skipped pending sprint retargets (see docs/PLAN.md)."
   - The script must `exit 0` cleanly when run.

4. **Tools image matrix.** Drop the `tools/docker/ibmcloud/` build job from whichever workflow defines it. Generic tool images (the one carrying `iperf3` / `dig` / generic shell utilities — but not `ibmcloud` CLI) stay. If the only tools-image workflow was the IBM-specific one, delete the workflow entirely and file an issue noting that Sprint 1 may need to author a new tools-image workflow for the AWS-flavoured replacement.

5. **Regression check.** Run the test suite end-to-end:
   - `go test ./...` (should match the staff agent's green-gate)
   - `bash scripts/e2e-test.sh --dry-run` if such a flag exists, or `bash scripts/e2e-test.sh` if it has a non-destructive mode; if it has neither, just confirm the script parses (`bash -n scripts/e2e-test.sh`).
   - `mdbook build book/` — should still succeed (architect agent kept chapters as stubs).
   - `cspell` over `book/src/**/*.md` — clean or with documented exceptions.

## Issue tracking

File issues to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint0_validator.md`:

```markdown
# Sprint 0 — validator issues

## Issue 1: short title
**Severity**: low | medium | high | blocker
**Status**: open | resolved
**Description**: what was found
**Files affected**: list of paths
**Proposed fix**: how to resolve
```

If clean: heading + `*No issues filed.*`.

Severity guide:
- **blocker**: a CI workflow can't even parse (`actionlint` fails); the e2e script crashes rather than skipping cleanly; cspell newly fails on architect-touched prose
- **high**: a workflow path filter is now too broad or too narrow such that CI may run on the wrong events
- **medium**: a tools image was built by a workflow you can't cleanly retarget without Sprint 1 context (file as an open issue for Sprint 1)
- **low**: dictionary-entry nits, comment cleanups

## Verification before reporting done

- Every `.github/workflows/*.yml` file passes `actionlint` (if installed; if not, `python -c "import yaml; yaml.safe_load(open('file'))"` to confirm valid YAML)
- `cspell` runs over `book/src/**/*.md` without new failures
- `bash -n scripts/*.sh` confirms script syntax is valid
- `bash scripts/e2e-test.sh` (if it's safe to run dry — most have a `--dry-run` or `--check` flag) exits 0 with the "all phases skipped" banner
- `grep -r 'ibmcloud\|jgruberf5\|roksbnkctl' .github/ cspell.json scripts/ tools/` returns no hits (allowed exception: `upstream/` references to the fork point and historical CHANGELOG mentions if those files are within your scope, which they aren't — they're architect-owned)

## Final report

Under 200 words:
- Files edited / deleted (counts + key paths)
- CI workflow status post-edit (which workflows exist, what each triggers on)
- cspell run result (clean / N exceptions documented)
- e2e script run result (exit code, skip-banner present)
- Issues filed (count + severity breakdown)
- Anything Sprint 1's validator agent should know (especially around tools-image workflow if you had to delete it)

Do NOT commit anything. The integrator commits the aggregated four-agent output.
