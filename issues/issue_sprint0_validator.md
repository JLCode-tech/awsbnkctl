# Sprint 0 — validator issues

## Issue 1: cspell dictionary still carries IBM-specific entries
**Severity**: low
**Status**: open
**Description**: The IBM-specific dictionary entries (`IBMCLOUD`,
`ibmcloud`, `ROKS`, `roks`, `OpenShift`, `openshift`, `COS`, `SCC`, `oc`,
`restricted-v2`) were retained in `cspell.json` because the inherited
`book/src/` chapters (still IBM-shaped pending Sprint 5's book retarget)
contain heavy usage — dropping the entries in Sprint 0 would have
newly red-x'd cspell against architect-touched prose and tripped the
blocker-severity threshold defined in this brief. Sprint 5 (book
retarget) owns removing these entries in the same commit that rewrites
the IBM-shaped chapters.
**Files affected**: `cspell.json` (lines 162-174)
**Proposed fix** (for Sprint 5 validator): once `book/src/02-why-eks.md`,
`book/src/25-s3-supply-chain.md`, etc. land and the chapter-by-chapter
IBM references are gone, drop the IBM entries from `cspell.json` in the
same PR. Verify with `npx cspell --config cspell.json 'book/src/**/*.md'
'docs/**/*.md'` — must stay green or have new exceptions documented.

## Issue 2: tool-image workflow now single-image; Sprint 1 to add AWS replacement
**Severity**: medium
**Status**: open
**Description**: `.github/workflows/tools-images.yml`'s build matrix
was `[ibmcloud, iperf3]`. The `ibmcloud` entry was dropped in Sprint 0
because the staff agent is deleting `tools/docker/ibmcloud/`. The
matrix now only carries `iperf3` (cloud-agnostic, kept). Sprint 1 needs
to author the AWS-flavoured replacement image (working name:
`tools-aws`, carrying `aws` CLI + `kubectl` + `eksctl` for the docker
exec-backend's AWS code paths) and add it back to the matrix. The
analogous Makefile target (`build-aws`) needs adding to
`tools/docker/Makefile` at the same time.
**Files affected**: `.github/workflows/tools-images.yml`,
`tools/docker/Makefile`
**Proposed fix** (for Sprint 1 staff): land `tools/docker/aws/Dockerfile`
with `awscli` + `kubectl` + `eksctl` baked in; add `aws` to the matrix in
`tools-images.yml`; add the `build-aws` target to `tools/docker/Makefile`
mirroring the inherited `build-ibmcloud` shape (repo-root context for
the Go build stage if the image bundles awsbnkctl itself).

## Issue 3: `tools/refgen/` Go source still uses old module path; staff agent's domain
**Severity**: high
**Status**: open
**Description**: `tools/refgen/cobra-md/main.go`,
`tools/refgen/cobra-md/main_test.go`, and `tools/refgen/tfvars-md/main.go`
still import `github.com/jgruberf5/roksbnkctl/internal/cli` and embed
`roksbnkctl` strings in generated-output literals. These files are .go
source — staff agent's domain per the brief's off-limits contract —
but the staff agent's module-path rewrite focused on `cmd/awsbnkctl/`
and `internal/`; the `tools/refgen/` subdirs need the same treatment.
Same applies to `tools/ciwatch/go.mod` and `tools/sprintwatch/go.mod`
which still declare `module github.com/jgruberf5/roksbnkctl/tools/...`.
Sprint 5 owns the actual regeneration of the reference chapters via
these tools, but the import-path rewrite needs to land first or `go
build ./tools/refgen/...` will fail once the staff agent's main-module
deletion of `github.com/jgruberf5/roksbnkctl` completes.
**Files affected**:
- `tools/refgen/cobra-md/main.go` (import + comments)
- `tools/refgen/cobra-md/main_test.go` (import + expected-output assertions)
- `tools/refgen/tfvars-md/main.go` (comment + chapter-cross-ref string)
- `tools/refgen/tfvars-md/main_test.go` (`ibmcloud_api_key` /
  `roks_cluster` literals — these will need updating once the staff
  agent's `terraform/modules/eks_cluster/` lands and the variables
  rename in `terraform/variables.tf`)
- `tools/ciwatch/go.mod` (module path)
- `tools/sprintwatch/go.mod` (module path)
**Proposed fix** (carry to staff agent's Sprint 0 follow-up, or Sprint 1
staff): rerun the import-path sed pass with the glob extended to
`tools/**/*.go` and `tools/**/go.mod`. Update string literals in
`tools/refgen/**` to match the new binary name. Sprint 5's chapter
regeneration depends on these compiling.

## Issue 4: e2e-test.sh phase shape changed substantially; PRD step matrix to follow
**Severity**: low
**Status**: open
**Description**: The Sprint 0 retarget reshaped `scripts/e2e-test.sh`
into a flat skip-stub: every phase A-N + L-DNS is a one-line skip
banner with no sub-step assertions. The inherited script had ~40
numbered sub-steps (A1-A5, B1-B10, etc.); these are dropped from the
script body in Sprint 0 but the sub-step contracts will need to be
re-authored in `docs/prd/05-E2E-TEST-PLAN.md` when Sprint 4 rehydrates
the AWS-shaped driver. The sub-step contracts that survive verbatim
(workspace ops in phase E, helpers like `step` / `capture` /
`assert_contains` / `assert_matches`) need to come back when phase A
is rehydrated.
**Files affected**: `scripts/e2e-test.sh` (reshaped),
`scripts/e2e-test-backends.sh` (reshaped),
`scripts/e2e-test-full.sh` (reshaped),
`docs/prd/05-E2E-TEST-PLAN.md` (Sprint 4 architect to retarget)
**Proposed fix** (Sprint 4 architect + staff): when authoring the
AWS-shaped phase A (sanity), reintroduce the `step` / `capture` /
`assert_contains` helpers (lifted verbatim from the upstream
`jgruberf5/roksbnkctl@v1.2.1` tree's `scripts/e2e-test.sh` lines
33-128) before the first phase body. Same for phases B-H when Sprint 3
delivers `awsbnkctl up cluster`.

---

*Verification before report:*
- Every `.github/workflows/*.yml` parses as valid YAML
  (`python3 -c "import yaml; yaml.safe_load(open(...))"` — all 6 files OK).
- `bash -n scripts/*.sh` confirms all four scripts have valid syntax.
- `bash scripts/e2e-test.sh` exits 0 with the
  "Sprint 0 stub: all e2e phases skipped" banner.
- `bash scripts/e2e-test-full.sh` exits 0 with the analogous banner.
- `bash scripts/e2e-test-backends.sh` exits 0 with the analogous banner.
- `npx cspell --config cspell.json` against `book/src/**/*.md` and
  `docs/**/*.md` shows the pre-existing baseline of unknown words
  (753 across 47 files — none introduced by Sprint 0 edits; existing
  CI workflow `spellcheck.yml` runs with `continue-on-error: true` so
  these don't block).
- `go test ./...` not run by validator: `go` is not installed on this
  dev box. Staff agent's green-gate confirmation stands as the
  authoritative test-suite run.
- `mdbook build book/` not run: `mdbook` is not installed on this dev
  box. Architect agent's chapter-stub edits remain markdown-only;
  validator-scoped changes did not touch any `book/` file.
