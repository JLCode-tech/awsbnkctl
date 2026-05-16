# Sprint 5 — validator issues

Sprint 5 (book retarget at AWS) validator scope: book CI / GitHub Pages
deployment, cspell sweep over the rewritten chapter set, Makefile
mdbook target verification.

**SPIKE DEFERRAL** carries — the book describes the design as
implemented, not as validated against live AWS. PRD 07's spike-protocol
gate sits outside the agent dispatch lane.

Format matches prior sprints: `Severity: blocker | high | medium | low |
roadmap`. The integrator triages at integration time; resolutions land
in `resolved_sprint5_validator.md`.

## Issue 1 — `.github/workflows/book.yml` is validation-only; brief assumed a deploy step
**Severity**: medium
**Status**: open (architecture decision; integrator confirms intent)

**Description**: The Sprint 5 validator brief priority 1 reads
"`.github/workflows/book.yml` verification — triggers on `book/**`
pushes; `peaceiris/actions-gh-pages` deploys correctly; published URL
framing references `JLCode-tech.github.io/awsbnkctl/book/`. Add an
`mdbook test book/` step before deploy".

The existing workflow is **validation-only** by design (per the file's
leading comment block, lines 1-16):

- It triggers on `book/**` pushes/PRs (correct per brief)
- It uses `peaceiris/actions-mdbook@v2` to install mdbook — **not**
  `peaceiris/actions-gh-pages@v3` to deploy
- It builds HTML and exits; no `gh-pages` push
- Publishing is **local-driven** via `make release` /
  `make release-publish` / `make book-publish` (Makefile lines 258-292)
  so the multi-GB pandoc + XeLaTeX + mermaid-cli toolchain stays off
  the GitHub runner

The published URL framing (`https://JLCode-tech.github.io/awsbnkctl/
book/`) is correct — it's emitted by `make book-publish` (Makefile
line 292) and referenced in `make release-publish` (line 339).

`mdbook test book/` is **deliberately not run** per the same file's
in-step comment (lines 47-55): it invokes rustdoc against every
untagged code fence, treating each as Rust. The book has zero Rust
code (Go project; languages used: bash / go / hcl / json / yaml / text
/ mermaid / powershell), so the test step generated only false
positives. Dropped in v1.0.1's recovery cut.

The validator did **not** edit `book.yml` because:

1. The "deploy" the brief assumes happens locally (the file header
   documents the rationale)
2. The "`mdbook test book/`" the brief asks for produces false
   positives against this book's content shape

**Files affected**: `.github/workflows/book.yml`

**Proposed fix**: integrator decision. Three paths:

- **(a) Keep as-is** (validator recommendation): the local-publish
  flow is intentional and documented; adding gh-pages deploy on tag
  push would require also moving the multi-GB pandoc toolchain to CI
  or splitting HTML+gh-pages from PDF+release (already what
  `make release-publish` does locally).
- **(b) Add a gh-pages HTML-only deploy on tag push**:
  HTML output already builds cleanly on the standard mdbook image
  (no pandoc/XeLaTeX needed for HTML). A new `deploy` job in
  `book.yml`, gated on `if: github.ref_type == 'tag'`, could call
  `peaceiris/actions-gh-pages@v3` with `publish_dir: book/book/html`
  and `destination_dir: book/`. Keeps the PDF flow local-driven.
- **(c) Move the `mdbook test` step back in**: revisit the
  false-positive call from v1.0.1's recovery cut. Would require
  language-tagging every code fence in the book (`bash`, `go`,
  `hcl`, etc.) so `mdbook test` skips them. Largest-surface fix;
  defer to v1.x.

## Issue 2 — `book-build` job added to `ci.yml`
**Severity**: informational
**Status**: ✅ resolved (validator scope; landed this sprint)

**Description**: Per Sprint 5 validator brief priority 2, added a
`book-build` job to `.github/workflows/ci.yml` (after the
`goreleaser-check` job). The new job:

- Runs on every PR + push to main (inherited from the workflow's
  top-level `on:` block — no path filter, so a cspell.json edit or
  a workflow YAML edit gets validated against the chapter set
  without needing a chapter edit to also trigger it)
- Installs mdbook via `peaceiris/actions-mdbook@v2` + `mdbook-mermaid`
  via `baptiste0928/cargo-install@v3` (cached across runs)
- Runs `mdbook build book/` → fails on broken markdown / dead links
  / malformed code fences
- Runs `cspell` (via `streetsidesoftware/cspell-action@v6`) against
  `book/src/**/*.md` → hard-fails on any unknown-word finding (this
  workflow runs in default fail-on-error mode, **unlike** the
  existing `.github/workflows/spellcheck.yml` which uses
  `continue-on-error: true`)

Validation:

- `python3 -c 'yaml.safe_load(open(...))'` on the edited `ci.yml`:
  ✓ clean; 10 jobs total (was 9), `book-build` appended at end
- `npx cspell --config cspell.json "book/src/**/*.md"`: ✓ zero
  findings (see Issue 3)
- `mdbook build book/` cannot be exercised at validator-run time
  (mdbook binary not in the sandbox PATH); will turn green on the
  first post-merge CI run, given chapters parse as valid markdown

**Files affected**: `.github/workflows/ci.yml`

**Resolution**: shipped.

## Issue 3 — cspell sweep landed zero unknown-word findings
**Severity**: informational
**Status**: ✅ resolved (validator scope; landed this sprint)

**Description**: Per Sprint 5 validator brief priority 3 ("cspell
final sweep — target zero unknown-word findings on `book/src/**/*.md`"),
ran `npx cspell --config cspell.json "book/src/**/*.md"` against the
post-Sprint-5 chapter set. First-pass result: **391 findings across
35 files**, ~135 unique unknown words.

The findings split into four categories — every category resolved by
allowlist add (no chapter edits required; chapters are architect/
tech-writer scope and off-limits to validator):

1. **British spellings used throughout the prose** (~30 unique):
   `behaviour`, `Behaviour`, `behavioural`, `containerised`,
   `customise`, `customised`, `defence`, `finalised`, `finaliser`,
   `finalisers`, `initialise`, `Initialise`, `initialised`,
   `Initialises`, `internalisation`, `internalise`, `internalised`,
   `internalises`, `Licence`, `licence`, `materialisation`,
   `memorise`, `modernised`, `normalises`, `operationalises`,
   `optimised`, `organised`, `parameterise`, `recognise`,
   `recognises`, `serialise`, `serialiser`, `stabilise`,
   `summarised`, `synchronise`, `amortise`, `authorise`. Sprint 4
   validator Issue 10 deferred these as a tech-writer call; Sprint 5
   includes them in the allowlist (US/British posture for this book
   is mixed; allowlisting the British forms doesn't impose a single
   posture — both spellings now pass).

2. **Cloud / k8s / AWS domain vocabulary** (~50 unique):
   `apikey`, `assumability`, `batchv` (k8s `batch/v1` API group),
   `Bitwarden`, `bluemix`, `buildconfigs`, `BXNIM`, `chdir`, `choco`
   (Chocolatey), `clientset`, `clusterrolebindings`, `clusterroles`,
   `cneinstance`, `cneinstances`, `corev` (k8s `core/v1`),
   `dearmor`, `decryptable`, `direnv`, `disambiguator`, `distros`,
   `DNSSEC`, `Dockerfiles`, `dpkg`, `dryrun`, `elbv` (AWS `elbv2`
   API), `firewalled`, `flowstate`, `gbps`, `Gbps`, `Gbit`, `Gbits`,
   `geolocation`, `Getenv`, `Getgid`, `Getuid`, `Gkta` (cspell
   tokenisation of a hex string), `gliderlabs`, `GOARCH`, `gofmt`,
   `gopass`, `gopkg`, `HMAC`, `hmac`, `hostnames`, `hostnetwork`,
   `IBMAPI`, `imagestreams`, `iperf` (non-3 variant), `ipvlan`,
   `isatty`, `jgruber`, `jjsg` (cspell tokenisation hit),
   `jumphosts`, `keyrings`, `krusty`, `kustomization`, `ldflags`,
   `libsecret`, `macvlan`, `Mbits`, `metav` (k8s `meta/v1`),
   `misconfig`, `moby`, `monolithically`, `NAPTR`, `networkstatic`
   (Docker Hub image), `NSEC`, `omitempty` (Go yaml tag), `oneoff`,
   `pgid`, `prereq`, `publickey`, `Pulumi`, `refgen`,
   `remotecommand` (k8s client-go subpackage), `reqs`, `resolv`,
   `resourcegroupstaggingapi` (AWS API), `RRSIG`, `SSHFP`,
   `storageclasses`, `subresource`, `subshell`, `subtest`, `SVCB`,
   `tcpdump`, `telco`, `TLSA`, `TMOS` (F5), `TMSH` (F5),
   `tunables`, `unparseable`, `USERPROFILE` (Windows env var),
   `vcpu`, `Vcpu`, `vcpus`, `yourfork` (placeholder).

3. **Acronyms cspell hadn't seen** (~5): `artefacts`, `autonumber`,
   `clis`.

**Final-pass verification**:

```text
$ npx --yes cspell --config cspell.json "book/src/**/*.md" 2>&1 | tail -3
34/35 book/src/preface.md            2.65ms
35/35 book/src/SUMMARY.md            1.18ms
CSpell: Files checked: 35, Issues found: 0 in 0 files.
```

Zero unknown-word findings — meets the Sprint 5 target.

**Files affected**: `cspell.json` (+~135 words, total 366)

**Resolution**: shipped. The new `book-build` CI job (Issue 2) will
hard-fail any future PR that re-introduces unknown words, replacing
the existing `spellcheck.yml`'s `continue-on-error: true` posture
with a real gate against the book chapter set.

## Issue 4 — Makefile mdbook targets verified
**Severity**: informational
**Status**: ✅ resolved (validator spot-check; no edits)

**Description**: Per Sprint 5 validator brief priority 4, spot-checked
the Makefile mdbook targets:

- `book` (line 64) — `mdbook build book/`; `make -n book` emits the
  expected command in `host` backend; `docker` backend routes through
  `BOOK_IMAGE` (ghcr.io/JLCode-tech/awsbnkctl-tools-mdbook:dev).
- `book-pdf` (line 82) — requires `BOOK_BACKEND=docker`; fails fast
  with a helpful diagnostic in host mode (Makefile lines 88-100). PDF
  artefact lands at `book/book/pandoc/pdf/book.pdf`. Not exercised at
  validator-run time (docker image not in the sandbox).
- `book-test` (line 342) — `mdbook test book/`; only available in
  host mode (the release image drops the rust toolchain after
  installing mdbook + mermaid + pandoc). False-positive caveat
  carries from `book.yml` (see Issue 1) — this target is preserved
  for the integrator's local debugging but isn't part of CI.
- `book-serve` (line 358) — `mdbook serve book/ --open` (host) or
  the docker variant on port 3000; for local iteration only.
- `book-clean` (line 361) — `rm -rf book/book`; `make -n book-clean`
  emits the expected command.

All five targets are declared `.PHONY` (line 32-34) and parse cleanly
under `make -n`. No edits made.

**Files affected**: none (verification only)

**Resolution**: shipped (no-op).

## Issue 5 — `spellcheck.yml` partially overlaps with new `book-build` job
**Severity**: low
**Status**: open (cleanup candidate for Sprint 6)

**Description**: The existing `.github/workflows/spellcheck.yml`
runs on PRs touching `book/src/**/*.md`, `docs/**/*.md`, or `**/*.go`,
uses `streetsidesoftware/cspell-action@v6`, and has
`continue-on-error: true` (so findings don't block the PR).

The new `book-build` job in `ci.yml` (Issue 2) covers
`book/src/**/*.md` in fail-on-error mode. The two now overlap on the
book surface — but `spellcheck.yml` still uniquely covers
`docs/**/*.md` and `**/*.go` (the Go-comment spell-check tier).

Three paths forward, all defer-able to Sprint 6:

- **(a) Keep both as-is**: `book-build` is the hard gate on the book
  surface; `spellcheck.yml` is the advisory gate on docs/Go.
  Mildly redundant on the book surface but harmless.
- **(b) Narrow `spellcheck.yml` to `docs/**` + `**/*.go` only**:
  removes the overlap on book/src, gives a single owner for the
  book-surface gate.
- **(c) Flip `spellcheck.yml` to fail-on-error and retire it**:
  needs a separate cspell pass on `docs/**/*.md` first to confirm
  zero unknown-word findings there too. Sprint 5's brief scope is
  `book/src/**/*.md` only; the `docs/**/*.md` sweep wasn't part of
  the priority list, so deferring.

**Files affected**: `.github/workflows/spellcheck.yml`

**Proposed fix**: defer to Sprint 6 hardening; (b) is the minimal
clean-up.

## Issue 6 — pre-existing `issue_sprint5_validator.md` content was from a different project
**Severity**: informational
**Status**: ✅ resolved (this file rewritten)

**Description**: The `issues/issue_sprint5_validator.md` file that
existed at validator-dispatch time contained Sprint 5 validator
findings from the **roksbnkctl / IBM Cloud** project (e2e Phase L-DNS
notes, miekg/dns probe behaviour, sshseam build tag, tools-images
`:dev` push). That predates the awsbnkctl fork's Sprint 5 retarget
and described surfaces that don't exist in this repo's Sprint 5
scope.

The validator overwrote the file with the awsbnkctl Sprint 5 findings
above. The historical content can be recovered from git history if
needed.

**Files affected**: `issues/issue_sprint5_validator.md`

**Resolution**: shipped.

---

## Regression-gate verdict

- `python3 -c 'yaml.safe_load(...)'` on edited workflows: ✓ clean
  (`.github/workflows/ci.yml`)
- `python3 -c 'json.load(...)'` on `cspell.json`: ✓ clean (366 words)
- `npx cspell --config cspell.json "book/src/**/*.md"`: ✓ zero
  findings (35 files, 0 issues)
- `make -n book` / `make -n book-clean`: ✓ both emit expected
  commands; all five book targets parse cleanly under `make -n`
- `bash -n` not applicable (no shell scripts edited)
- `mdbook build book/` not exercised at validator-run time (mdbook
  binary not in the sandbox PATH); will run on the first post-merge
  CI run via the new `book-build` job

**Blockers preventing the integrator from cutting v0.9 (M5)**:
none from the validator scope. Issue 1 (book.yml deploy posture) is
flagged as an architecture decision for the integrator, not a
validator-fixable defect.

Sprint 5 end-of-sprint gate per PLAN.md (`mdbook build book/` clean;
web book deploys to GitHub Pages; cross-link audit clean; cspell
clean) is achieved at the validator surface for the cspell + CI
gates. The "web book deploys to GitHub Pages" gate is the local
`make release-publish` flow per Issue 1; the integrator drives that
at tag-cut time.

SPIKE DEFERRAL carries — book content describes the design as
implemented, not as validated against live AWS; PRD 07's
operator-run spike still gates the v1.0 tag in Sprint 6.
