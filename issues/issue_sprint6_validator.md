# Sprint 6 — validator issues

Sprint 6 (final pre-`v1.0` sprint, hardening + release prep) validator
scope: `security-audit` CI job (gosec + govulncheck + secret-scan),
`release.yml` spec-only validation, book PDF build verification, final
cspell sweep across `book/src/**/*.md` + `docs/**/*.md`, `e2e-full.yml`
final stub-banner refresh.

**SPIKE DEFERRAL** carries — the v1.0 cut still gates on the operator-
run PRD 07 spike (live EKS 1.30 + SR-IOV CNI bring-up). The Sprint 6
release is **structurally complete** at v0.9-rc1; anyone with operator-
run spike validation can cut v1.0 immediately.

Format matches prior sprints: `Severity: blocker | high | medium | low |
roadmap | informational`. The integrator triages at integration time;
resolutions land in `resolved_sprint6_validator.md`.

## Issue 1 — `security-audit` job landed in `ci.yml`
**Severity**: informational
**Status**: ✅ resolved (validator scope; landed this sprint)

**Description**: Per Sprint 6 validator brief priority 1, added a
`security-audit` job to `.github/workflows/ci.yml` (after the
`book-build` job). The new job runs on every PR + push to main (inherits
the workflow's top-level `on:` block — no path filter, so a Go-source
edit or a dependency bump immediately triggers a full re-scan):

- **gosec** — uses `securego/gosec@master`, `args: ./...`. Default
  ruleset (G104 unchecked errors, G304 file-inclusion, G306 file perms,
  G402 InsecureSkipVerify, G404 weak RNG, ...).
- **govulncheck** — uses `golang/govulncheck-action@v1` with
  `go-version-file: go.mod` + `check-latest: true`. Reachability-aware:
  reports only vulns the binary actually exercises.
- **gitleaks** — uses `gitleaks/gitleaks-action@v2`. Requires
  `fetch-depth: 0` at the checkout step (configured) so the scan can
  walk full git history rather than only the worktree. Free for public
  repos; no license token required.

All three steps run with the default `continue-on-error: false` posture:
any finding fails the job, blocks the PR, and surfaces in the Actions
tab as a red-x check. Matches the PLAN.md § Sprint 6 end-of-sprint gate
("gosec ./... clean; secrets scan clean").

Validation:

- `python3 -c 'yaml.safe_load(open(...))'` on edited `ci.yml`: ✓ clean
  (10 jobs → 11 jobs, `security-audit` appended last)
- `actionlint` (installed via `go install
  github.com/rhysd/actionlint/cmd/actionlint@latest`): ✓ clean across
  all 6 workflow files
- The actual scan can't be exercised at validator-run time (gosec /
  govulncheck / gitleaks binaries not on sandbox PATH); turns green on
  the first post-merge CI run, given the code under `./...` doesn't
  carry an existing finding the integrator should triage separately

**Files affected**: `.github/workflows/ci.yml` (+`security-audit` job)

**Resolution**: shipped.

## Issue 2 — `release.yml` workflow as written does NOT attach the book PDF (by design)
**Severity**: informational (architecture decision; matches the
                              integrator's documented local-publish flow)
**Status**: filed; integrator review

**Description**: The Sprint 6 validator brief priority 2 reads
"`release.yml` validation — verify on tag push: goreleaser builds 6
archives, generates checksums, attaches book PDF, publishes to GitHub
Releases". Spec-only check (the brief explicitly says "don't trigger").

Three of the four are met by the workflow as written:

- **6 archives**: `.goreleaser.yml` `builds:` block declares `goos:
  [linux, darwin, windows]` × `goarch: [amd64, arm64]` = 6
  cross-compiled binaries; `archives:` block uses a single
  `name_template` so each binary lands in `dist/` as a tar.gz (zip on
  Windows via `format_overrides`). ✓
- **checksums.txt**: `.goreleaser.yml` `checksum:` block emits a
  `checksums.txt` asset attached to the GitHub Release. ✓
- **publish to GitHub Releases**: the goreleaser action `args: release
  --clean` step under `permissions: contents: write` does this. ✓

**The PDF attach is NOT done by this workflow**, deliberately. The
`.goreleaser.yml` `release:` block carries an explicit comment
(.goreleaser.yml lines 96-109):

> The PDF book artifact is NOT attached by goreleaser. The CI runner
> for release.yml doesn't have pandoc/XeLaTeX/mermaid-cli installed
> (deliberately — pulling the multi-GB tools/docker/mdbook image on
> every tag-push is wasteful when the integrator already has it built
> locally). Instead, after this workflow finishes, the integrator runs:
>
>   make release-publish VERSION=vX.Y.Z
>
> from their machine, which uploads ./book/book/pandoc/pdf/book.pdf to
> this Release as awsbnkctl-book-vX.Y.Z.pdf via `gh release upload`.
>
> Earlier versions of this config had `extra_files` pointing at the PDF.
> That turned out to fail-stop the release (not warn-and-continue as
> the comment claimed); removed in v1.0.1's recovery cut.

This matches the same architecture decision Sprint 5 validator Issue 1
flagged for `book.yml` (HTML publishing is local-driven via
`make book-publish`; PDF publishing is local-driven via
`make release-publish`). The multi-GB pandoc + XeLaTeX + mermaid-cli
toolchain stays off the GitHub runner.

The validator did **not** edit `release.yml` (or `.goreleaser.yml` —
off-limits) to add a PDF-attach step because:

1. The PDF flow is local-driven by design (the goreleaser config header
   documents the rationale)
2. The `release:` body's footer text already references the PDF asset
   name (`awsbnkctl-book-{{ .Tag }}.pdf`) — so the Release page tells
   users what file to look for, and `make release-publish` lands it
   immediately after the goreleaser run

**Files affected**: `.github/workflows/release.yml` (verification only),
`.goreleaser.yml` (off-limits; spec-checked)

**Proposed fix**: integrator decision. Two paths:

- **(a) Keep as-is** (validator recommendation): the local-publish flow
  is intentional and documented; the integrator's
  `make release-publish` step is the v1.0 sign-off ritual. Update the
  brief's "attaches book PDF" wording to "release.yml publishes 6
  archives + checksums; `make release-publish` attaches the PDF as a
  separate post-workflow step".
- **(b) Add PDF attach back to `release.yml`**: requires adding pandoc
  + XeLaTeX + mermaid-cli + Chromium to the runner (or pulling the
  ~1.2 GB `tools/docker/mdbook` image) — the v1.0.1 recovery cut
  evidence indicates this fail-stops the workflow. Defer to v1.x and
  use a self-hosted runner if there's appetite to move the toolchain
  into CI.

## Issue 3 — Book PDF build not exercisable in validator sandbox; spec checks green
**Severity**: low (sandbox artefact; integrator can exercise locally)
**Status**: filed; integrator verification step

**Description**: Per Sprint 6 validator brief priority 3 ("Book PDF
build verification — `make book-pdf` (or equivalent) produces a PDF
with Mermaid diagrams pre-baked; ship via release.yml"). The validator
could not actually run `make book-pdf` because:

- `make book-pdf` (host backend, the default) refuses with a helpful
  diagnostic — pandoc/LaTeX/mermaid-cli aren't on PATH (correct
  behaviour per Makefile lines 88-100).
- `make book-pdf BOOK_BACKEND=docker` requires the
  `ghcr.io/JLCode-tech/awsbnkctl-tools-mdbook:dev` image. Docker IS
  available on the sandbox host (`docker info` succeeds), but the image
  isn't pulled locally and the build is ~1.2 GB / ~10 min cold —
  outside the validator's iteration loop.

Spec checks (what the validator CAN verify):

- `make -n book-pdf` (default host) emits the expected
  bail-with-diagnostic flow.
- `make -n book-pdf BOOK_BACKEND=docker` emits the expected `docker
  run ... mdbook build` invocation routing through the GHCR image.
- `tools/docker/mdbook/Dockerfile` declares all expected toolchain
  layers: `mdbook + mdbook-mermaid + mdbook-pandoc + pandoc + texlive
  + @mermaid-js/mermaid-cli + Chromium`. The Mermaid pre-bake is
  invoked via the Lua filter at `/opt/render-mermaid.lua` referenced
  from `book/book.toml` (per the Dockerfile header).
- `book/book.toml`'s `[output.pandoc.profile.pdf]` block defines the
  PDF profile; mdbook-pandoc walks the rendered markdown and pandoc
  invokes XeLaTeX (per the Makefile commentary at lines 75-80).
- The published PDF lands at `book/book/pandoc/pdf/book.pdf`
  (Makefile line 85). `make release-publish` uploads it under the
  asset name `awsbnkctl-book-vX.Y.Z.pdf` (.goreleaser.yml `release:`
  footer + Makefile line 317).

The Sprint 5 validator also verified all five Makefile mdbook targets
parse cleanly under `make -n` (issue_sprint5_validator.md Issue 4).
Sprint 6 carries that forward; no new edits to Makefile targets.

**Files affected**: none (verification only; Makefile is off-limits)

**Proposed fix**: integrator runs `make -C tools/docker build-mdbook`
followed by `make book-pdf BOOK_BACKEND=docker` once at the v1.0 cut
point and confirms a non-zero-byte
`book/book/pandoc/pdf/book.pdf` lands. Then `make release-publish
VERSION=v1.0.0` after the goreleaser run.

## Issue 4 — Final cspell sweep landed zero unknown-word findings (book/src + docs)
**Severity**: informational
**Status**: ✅ resolved (validator scope; landed this sprint)

**Description**: Per Sprint 6 validator brief priority 4 ("Final cspell
sweep — zero findings on book/src/**/*.md + docs/**/*.md"), ran
`npx cspell --config cspell.json "book/src/**/*.md" "docs/**/*.md"`
against the post-Sprint-5 chapter set plus the `docs/` tree (PRDs +
PLAN + SHAKEOUT + E2E_TEST + PRD.md).

First-pass result: **84 findings across 21 files, 48 unique unknown
words**. The findings split into four categories — every category
resolved by allowlist add (no chapter / docs edits; both surfaces are
architect / tech-writer scope and off-limits to validator):

1. **AWS / Go / k8s domain vocabulary** (~30 unique):
   `AKIA`, `AKIAIOSFODNN` (canonical AWS access-key prefix used in
   examples), `apimachinery`, `AWSAPI`, `awscli`, `awscliv` (cspell
   tokenisation of `awscliv2`), `bnkfun` (workspace-name placeholder),
   `Bubbletea` (charmbracelet TUI lib), `callsites`, `cctx` (context
   alias), `Errorf`, `finalizer`, `genericclioptions` (kubectl helper
   package), `godoc`, `gosec`, `healthz`, `hostkeys`, `iamidentityv`
   (cspell tokenisation of `iamidentityv1`), `jumpbox`, `knownhosts`,
   `kubernetesserviceapiv` (cspell tokenisation of
   `kubernetesserviceapiv1`), `portforward`, `regen`, `remotecommand`,
   `tfexec`, `tfws`, `validatable`, `zerolog`.

2. **British spellings the Sprint 5 sweep didn't catch** (~10):
   `fulfilment`, `licences`, `materialises`, `maximise`, `misroute`,
   `organisations`, `parameterised`, `Productisation`, `productise`,
   `productising`, `standardise`, `Synthesises`.

3. **Project-internal placeholders + acronyms** (~5): `Gruber`,
   `kwallet`, `subverb`, `subverbs` (carried over from architect Issue
   2's "Available in v1.x" annotations in chapters 8 / 9 / 11),
   `testdata`, `donatable`, `retargets`.

4. **Edge cases** (~3): `bursty`, `emptively` (cspell tokenisation of
   `pre-emptively`), `nsid`.

**Final-pass verification**:

```text
$ npx --yes cspell --config cspell.json --no-progress \
    "book/src/**/*.md" "docs/**/*.md" 2>&1 | tail -1
CSpell: Files checked: 48, Issues found: 0 in 0 files.
```

Zero unknown-word findings across both surfaces — meets the Sprint 6
target.

**Files affected**: `cspell.json` (+~48 words; total ~414)

**Resolution**: shipped. The `book-build` CI job (Sprint 5 Issue 2) was
extended to also cover `docs/**/*.md` so the same hard-fail gate covers
the docs surface from now on. The legacy `spellcheck.yml` workflow
remains the advisory (continue-on-error) gate for `**/*.go` Go-comment
spell-check; Sprint 5 validator Issue 5 left the cleanup pass on
`spellcheck.yml` as a Sprint 6 candidate — see Issue 6 below for the
status update.

## Issue 5 — `e2e-full.yml` skip-banner refreshed for Sprint 6 / v1.0 framing
**Severity**: informational
**Status**: ✅ resolved (validator scope; landed this sprint)

**Description**: Per Sprint 6 validator brief priority 5
("e2e-full.yml final stub gate update — references PRD 07 spike +
Sprint 4 AWS phases"), refreshed the file header + the
`Sprint 4 skip banner` step inside `.github/workflows/e2e-full.yml`:

- Header text — updated from "Sprint 4 status" to "Sprint 6 status
  (final pre-v1.0 sprint)"; the bullet list of dry-run jobs now
  includes the Sprint 6 `security-audit` job alongside the Sprint 3
  `full-up-dryrun` + Sprint 4 `test-dryrun` jobs; closing paragraph
  reframes "Sprint 6 cuts v1.0" to "Sprint 6 is the final pre-v1.0
  sprint; release is structurally complete at v0.9-rc1; v1.0 cut
  gates on the operator-run PRD 07 spike".
- `name:` — "Full E2E (Sprint 4 stub ...)" → "Full E2E (Sprint 6 stub
  — v1.0 cut gates on PRD 07 spike)".
- Step name + echoed banner — "Sprint 4 skip banner" → "Sprint 6
  skip banner (final pre-v1.0 sprint)"; per-phase status block
  appended two lines for the Sprint 6 deliverables: "Security audit
  (offline)" + "Book PDF artefact (release-time)"; "Full v1.0
  sign-off" line reframed from "Sprint 6 (gated on spike)" to "v1.0
  cut (gated on PRD 07 spike)".

The trigger surface (`workflow_dispatch` with `cluster_region` input +
`teardown_on_success` boolean; `push: branches: ['release/**']`) is
preserved verbatim — downstream automation (release-branch CI, the
integrator's babysit loop) keeps parsing the contract. The body still
exits-0 (skip-stub posture); the live-apply transition lands together
with the v1.0 cut.

Validation:

- `python3 -c 'yaml.safe_load(...)'` on edited `e2e-full.yml`:
  ✓ clean
- `actionlint`: ✓ clean

**Files affected**: `.github/workflows/e2e-full.yml`

**Resolution**: shipped.

## Issue 6 — `spellcheck.yml` overlap with `book-build` job (carry-over)
**Severity**: low (cleanup candidate; non-blocking)
**Status**: open (deferred to v1.x polish; Sprint 5 Issue 5 carry-over)

**Description**: Sprint 5 validator Issue 5 flagged that the existing
`.github/workflows/spellcheck.yml` workflow (continue-on-error
advisory gate over `book/src/**/*.md` + `docs/**/*.md` + `**/*.go`)
partially overlaps with the new `book-build` job in `ci.yml`
(fail-on-error hard gate over `book/src/**/*.md`).

Sprint 6 extended `book-build` to also cover `docs/**/*.md` (this
sprint's Issue 4). That widens the overlap: now both jobs gate on
both book + docs surfaces, but `book-build` is fail-on-error and
`spellcheck.yml` is advisory. The advisory tier still uniquely
covers `**/*.go` (the Go-comment spell-check tier), so it can't be
retired outright without first running a separate cspell pass on
`**/*.go` to confirm zero unknown-word findings there too.

Three paths, all defer-able beyond Sprint 6:

- **(a) Keep both as-is**: `book-build` is the hard gate on book + docs;
  `spellcheck.yml` remains the advisory gate, primarily useful for the
  Go-comment surface. Mildly redundant but harmless.
- **(b) Narrow `spellcheck.yml` to `**/*.go` only**: removes the
  full overlap on book + docs; gives a single owner for each surface.
  Minimal, recommended for v1.x polish.
- **(c) Flip `spellcheck.yml` to fail-on-error and retire it**: needs
  a separate cspell pass on `**/*.go` first. Sprint 6 brief scope is
  `book/src/**/*.md` + `docs/**/*.md` only; the Go-comment sweep
  wasn't part of the priority list, so deferring.

**Files affected**: `.github/workflows/spellcheck.yml`

**Proposed fix**: defer to v1.x polish; (b) is the minimal clean-up.
Track as a v1.x roadmap item.

## Issue 7 — Stale `issue_sprint6_validator.md` content overwritten
**Severity**: informational
**Status**: ✅ resolved (this file rewritten)

**Description**: The `issues/issue_sprint6_validator.md` that existed
at validator-dispatch time contained Sprint 6 validator findings from
the **roksbnkctl / IBM Cloud** project — scripts/e2e-test-backends.sh
Phase I/N notes, IBMCLOUD_API_KEY secret references, `wait -t` Bash
5.2 RFE, repo-unreachable APT mutation notes. That predates the
awsbnkctl fork's Sprint 6 retarget and described surfaces that don't
exist in this repo's Sprint 6 scope.

The validator overwrote the file with the awsbnkctl Sprint 6 findings
above. The historical content can be recovered from git history if
needed.

**Files affected**: `issues/issue_sprint6_validator.md`

**Resolution**: shipped.

---

## Regression-gate verdict

- `python3 -c 'yaml.safe_load(...)'` on all 6 workflow files: ✓ clean
  (`book.yml`, `ci.yml`, `e2e-full.yml`, `release.yml`,
  `spellcheck.yml`, `tools-images.yml`)
- `actionlint` (v1.7.x, installed for this sprint via `go install
  github.com/rhysd/actionlint/cmd/actionlint@latest`): ✓ clean across
  all 6 workflow files
- `python3 -c 'json.load(...)'` on `cspell.json`: ✓ clean
- `npx cspell --config cspell.json "book/src/**/*.md" "docs/**/*.md"`:
  ✓ zero findings (48 files, 0 issues)
- `make -n book-pdf` (host + docker backends): ✓ both emit expected
  commands
- `bash -n` not applicable (no shell scripts edited)
- `gosec ./...` / `govulncheck ./...` / `gitleaks detect`: not
  exercised at validator-run time (binaries not on sandbox PATH);
  turns green on the first post-merge CI run via the new
  `security-audit` job, assuming the code under `./...` doesn't carry
  an existing finding the staff agent's Sprint 6 `gosec ./...` local
  run (per their brief: "runs `gosec ./...` and folds findings") has
  already triaged

**Blockers preventing the integrator from cutting v0.9-rc1**: none from
the validator scope. Issue 2 (`release.yml` PDF-attach posture) is
flagged as an architecture decision for the integrator, not a
validator-fixable defect — the local `make release-publish` flow ships
the PDF in lockstep with the goreleaser run. Issue 3 (book PDF docker
build) and Issue 6 (`spellcheck.yml` overlap) are non-blockers for the
v0.9-rc1 cut.

Sprint 6 end-of-sprint gate per PLAN.md (`awsbnkctl doctor`
green-by-default; `gosec ./...` clean; secrets scan clean; goreleaser
build succeeds across linux/macOS/windows × amd64/arm64; book PDF
generates; `MIGRATING.md` is the final word for migrators) is achieved
at the validator surface for the CI gate + cspell + e2e-full skip-stub.
The `awsbnkctl doctor` green + `gosec` clean + `secrets scan` clean +
`book PDF generates` + `MIGRATING.md final` items land in the staff +
architect surfaces; the validator's CI gate (`security-audit` job) is
the regression backstop that catches any future drift on the gosec /
govulncheck / gitleaks dimensions.

SPIKE DEFERRAL carries — v1.0 first-tag still gates on the operator-run
PRD 07 spike. The Sprint 6 release is **structurally complete** at
v0.9-rc1 candidate; anyone with operator-run spike validation can cut
v1.0 immediately following the integrator's
`make release-publish VERSION=v1.0.0` step.
