# Sprint 6 — tech-writer issues (final pre-v1.0 sprint)

Final read-only pass at sprint close. This is the **v1.0-readiness preview** that goes into the v0.9-rc1 release notes.

**Read-only.** No project files edited. **SPIKE DEFERRAL** carries — v0.2 first tag and v1.0 final still gate on operator-run PRD 07 spike (live EKS 1.30 + SR-IOV CNI bring-up).

> **File-overwrite note.** The `issue_sprint6_tech-writer.md` that existed at tech-writer-dispatch time contained Sprint 6 tech-writer findings from the **roksbnkctl / IBM Cloud** upstream project (EDNS Dockerfile, `tools/docker/ibmcloud` carry-overs, IBM-Cloud Phase I/M/N notes). That predates the awsbnkctl fork and described surfaces that do not exist in this repo. The validator did the same overwrite on its sibling file (Issue 7 in `issue_sprint6_validator.md`); this file follows the same path. The historical content is recoverable from git history.

**Scope walked end-to-end:**

- 34 chapters under `book/src/` (Parts I-X) + `SUMMARY.md` + `preface.md`
- `README.md` (refreshed sprint-count framing)
- `CHANGELOG.md` (entries Sprint 0-5; **Sprint 6 entry missing**)
- `MIGRATING.md`
- `internal/exec/k8s_install.yaml` (staff IRSA retarget verification)
- All 6 workflow files under `.github/workflows/`
- Sibling Sprint 6 issue files (architect / staff / validator)
- Sprint 5 tech-writer issue file (closure verification)

**Verification (Sprint 6 close):**

- `go build ./...` ✓ clean
- `go test ./...` ✓ 10 packages ok / 0 failures (cached)
- `terraform validate` on **all 8 modules** (`cert_manager`, `cne_instance`, `ecr_mirror`, `eks_cluster`, `flo`, `iam_irsa`, `license`, `s3_supply_chain`, `testing`) → ✓ all "Success! The configuration is valid." Root tree requires `terraform init` first (expected; modules not pre-resolved on a clean clone).
- `grep -iE 'ibm|roks|cos|ibmcloud' internal/exec/k8s_install.yaml` → **0 hits** (Sprint 5 BLOCKER Issue 1 closed end-to-end on the YAML side; ServiceAccount carries `eks.amazonaws.com/role-arn: ${OPS_IRSA_ROLE_ARN}` annotation; standalone IBM-creds Secret deleted; rbac scoped to `awsbnkctl-ops` / `awsbnkctl-test`).
- Sprint 5 BLOCKER Issue 1 **prose** side: chapter 19 annotated at top with "Available in v1.x" banner per architect path-(b) closure; the architect's chapter-banner annotation closes the first-time-reader bounce. However, the body of chapter 19 (lines 166-425) **still reads as the IBM trusted-profile + `awsbnkctl-ibm-creds` Secret narrative verbatim** — see Issue 2 below.
- Chapters 8 / 9 / 11 carry the v1.x banners listing absent subverbs explicitly (Sprint 5 Issue 2 architect closure verified).
- Chapters 17 / 18 / 32 are clean of IBM/ROKS/COS residue (Sprint 5 Issue 4 closure verified).
- `.github/workflows/ci.yml` carries the **`security-audit`** job (gosec + govulncheck + gitleaks) added by validator this sprint; `book-build` job extended to also cover `docs/**/*.md`; `release.yml`, `book.yml`, `e2e-full.yml`, `spellcheck.yml`, `tools-images.yml` all spec-clean.

---

## Issue 1 (BLOCKER) — CHANGELOG.md has no Sprint 6 entry; the v0.9-rc1 release notes have nothing to cite

**Severity**: blocker (release-gate)
**Status**: open (integrator pre-tag)

**Description**: The Sprint 6 architect brief noted CHANGELOG.md as "the integrator adds Sprint 6 at commit time." At sprint close the file's "Unreleased" section opens directly with the Sprint 5 entry — there is no Sprint 6 entry covering the staff IRSA YAML retarget, the govulncheck `golang.org/x/net@v0.53.0` bump, the goreleaser snapshot validation, the gosec audit posture, the new `security-audit` CI job, the cspell `docs/**/*.md` extension, the `e2e-full.yml` Sprint 6 banner refresh, the chapter 8/9/11 v1.x annotations, the chapter 17/18/19/32 IBM-residue secondary sweep, the glossary cleanup, the PLAN.md "What's deferred to post-v1.0" appendix, or the README sprint-count refresh.

The Sprint 5 entry also still carries a "Carried into Sprint 6 (blockers per tech-writer review)" subsection — by sprint close that list either lands as resolved items in the new Sprint 6 entry or rolls into the deferred-work appendix.

**Files affected**: `CHANGELOG.md` (integrator scope at commit time).

**Proposed fix**: integrator authors the Sprint 6 entry as part of the v0.9-rc1 tag-cut commit. Without it, the v0.9-rc1 release notes have no per-sprint summary of what's in the candidate.

## Issue 2 (BLOCKER) — Chapter 19 body (lines 166-425) is still the IBM trusted-profile narrative verbatim; only the top-of-chapter banner names the v1.x framing

**Severity**: blocker (correctness drift; first-time reader who skips the banner is misled end-to-end)
**Status**: open (architect — path (a) closure that was deferred from Sprint 5)

**Description**: The architect's Sprint 6 chapter-19 closure is the path-(b) annotation banner at the top (lines 3-5) explicitly framing the chapter as describing the inherited shape pending the v1.x IRSA retarget. That banner is the smallest-correct closure for the first-time-reader-bounce problem.

But the **chapter body**, from §"Trusted-profile flow (v1.2+)" (line 166) onwards, still reads verbatim as the IBM trusted-profile + `awsbnkctl-ibm-creds` + `IBMCLOUD_API_KEY` walkthrough. ~260 lines describe `--trusted-profile=auto|on|off`, the IBM IAM perm probe (`iam-identity`), the v1.2.0 partial-closure admonition referencing "Sprint 10", the `awsbnkctl.io/trusted-profile-managed` annotation, the `--apikey "$AWS_ACCESS_KEY_ID"` in-pod wrap, the Secret-rendered-with-empty-data manifest. The prose is internally inconsistent: the banner says "do not run `awsbnkctl ops install` against a real EKS cluster" while the body walks the reader through `awsbnkctl ops install --trusted-profile=auto` as if it were a supported v1.2 surface.

A reader who lands on chapter 19 from a SUMMARY.md TOC click and scrolls past the banner (banners are easy to skip) reads ~260 lines of v1.2-shape IBM trusted-profile guidance and walks away with a fundamentally wrong mental model of how IRSA works on AWS.

The staff-side YAML retarget (`internal/exec/k8s_install.yaml`) is clean. The architect annotation banner is in place. The body rewrite is the missing piece — either gate the body sections behind explicit "(v1.x — current installer is still inherited shape)" subheadings, or rewrite the body against the as-shipped IRSA design (the YAML is now structurally correct; the prose can describe it truthfully).

**Files affected**: `book/src/19-in-cluster-ops-pod.md` (lines 166-425 — the §"Trusted-profile flow (v1.2+)" subtree).

**Proposed fix**: post-Sprint-6 architect pass (or v1.0 finalisation pass): rewrite the §"Trusted-profile flow" subtree against the IRSA shape the YAML now ships. Sprint 6 architect's deferral here is defensible (the body rewrite is ~3 hours of architect work and the brief gave priority to the secondary-residue sweep + glossary cleanup + chapter-banner closures), but the body-rewrite gap means chapter 19 is **not** v1.0-ready end-to-end.

## Issue 3 (BLOCKER) — Chapter 13 (Terraform variables) was missed entirely by the Sprint 5 mechanical sweep and the Sprint 6 secondary sweep; it still describes ROKS / OpenShift / IBM Cloud verbatim

**Severity**: blocker (correctness drift; chapter 13 is a Part-IV reference surface, hit on every "how do I tune <variable>" query)
**Status**: open (architect)

**Description**: `book/src/13-terraform-variables.md` carries **10 hits** of unswept IBM-Cloud / ROKS / OpenShift residue. The Sprint 5 architect's mechanical sweep was lexical (`roksbnkctl`→`awsbnkctl` / `ROKS`→`EKS` / `COS`→`S3`) and did not cover the variable names because the chapter uses `roks_workers_per_zone` / `roks_cluster` / `openshift_cluster_name` / `openshift_cluster_version` — variable names the sweep had no rules for.

Concrete examples (line numbers from current file):

- L53: `| openshift_cluster_name | tf-openshift-cluster | Cluster name. ... |`
- L54: `| roks_workers_per_zone | 1 | Worker nodes per AZ. 2 ⇒ 6 workers in a 3-AZ MZR region. |` ("MZR" is an IBM Cloud Multi-Zone Region term)
- L55: `| create_roks_cluster | true | Set false to adopt an existing cluster. Pair with roks_cluster_id_or_name. |`
- L56: `| openshift_cluster_version | "4.18" | OpenShift minor. Quote it — YAML/HCL parses 4.18 as float otherwise. |`
- L116: `S3 HMAC keys — auto-generated by the roks_cluster module via the S3 service-credentials resource`
- L129-131: worked-example tfvars block uses `roks_workers_per_zone = 6 / roks_min_worker_vcpu_count = 32 / roks_min_worker_memory_gb = 128`
- L158: "Cluster identity, region, **OpenShift version**, worker count"

The actual variable names in `terraform/variables.tf` (per the chapter 29 auto-generated reference) are `eks_cluster_name`, not `openshift_cluster_name`; the EKS module has no `openshift_cluster_version`; the chapter 13 worked example tells the reader to drop tfvars that don't exist in the v0.9 HCL. A user following chapter 13 verbatim against the as-shipped `terraform/variables.tf` gets `Error: An input variable with the name "roks_workers_per_zone" has not been declared` on the first `terraform plan`.

**Files affected**: `book/src/13-terraform-variables.md` (~10 surface edits + the worked-example tfvars block rewrite).

**Proposed fix**: post-Sprint-6 architect pass: rewrite chapter 13 against the actual `terraform/variables.tf` surface (cross-check with chapter 29 auto-gen). The chapter is otherwise structurally sound (the layering rule + `--var-file` semantics + AWS_ACCESS_KEY_ID exception are all generic); it's the variable-name surface and the worked example that need the IBM→AWS retarget.

## Issue 4 (HIGH) — Chapter 28 (Configuration reference) carries an unretargeted `cos:` block, IBM-shaped field descriptions, and `openshift_version` field

**Severity**: high (Part VIII reference chapter; users hit it on every "what fields does config.yaml support" lookup)
**Status**: open (architect)

**Description**: `book/src/28-configuration-reference.md` carries the largest cluster of OpenShift / COS residue outside chapter 19 (11 OpenShift hits, 9 `cos`-named field hits). The Sprint 5 chapter-28 mechanical sweep retargeted some prose (the chapter has "S3" wording around the `cos:` block) but missed the structural fields:

- L28: `cos:             # optional; supply-chain auto-upload` in the config.yaml example block
- L148: `## cos:` block — heading still uses the IBM-Cloud Object Storage name
- L151: full `cos:` YAML block + L161-165 field-by-field table calling out IBM CRN semantics
- L231: `| cluster.openshift_version | string | 4.18 | OpenShift minor version. |`
- L247-250: field-by-field table rows for `cos.instance` / `cos.bucket` / `cos.upload[].source` / `cos.upload[].key`
- L270: behaviour table row: `| cluster.openshift_version | Empty string passed to upstream HCL; the module picks the current default. |`
- L278: `| cos | Block omitted ⇒ no pre-flight uploads; FLO reads whatever's already in the configured bucket. |`

The actual `Workspace` struct in `internal/config/workspace.go` uses an `s3:` / `aws.supply_chain:` shape; there is no `cluster.openshift_version` field. A user who lands on chapter 28 reads ~40 lines of guidance on a `cos:` block that the as-shipped binary rejects.

**Files affected**: `book/src/28-configuration-reference.md` (~20 surface edits + the `cos:` block rewrite per the actual schema).

**Proposed fix**: post-Sprint-6 architect pass: cross-check chapter 28 against `internal/config/workspace.go::Workspace` and the chapter 27 auto-generated cobra-md reference. Rewrite the `cos:` block, drop the `openshift_version` field row, update the field-by-field table to match the actual schema.

## Issue 5 (HIGH) — MIGRATING.md is still scaffold-shape ("no shipped release yet") despite being the v0.9-rc1 / v1.0-candidate migration surface

**Severity**: high (PLAN.md Sprint 6 end-of-sprint gate explicitly calls out "MIGRATING.md is the final word for migrators"; status banner says "scaffolding")
**Status**: open (architect or staff)

**Description**: `MIGRATING.md` opens with a "**Status:** scaffolding. awsbnkctl has no shipped release yet — the sections below describe the migration *target* so the implementation knows what to honour. Each section will be tightened as the corresponding sprint lands." That status framing was correct at Sprint 0; by Sprint 6 close the binary works end-to-end (modulo the operator-run spike) and the file is shipping as part of the v0.9-rc1 candidate cut.

Concrete gaps versus the as-shipped surface:

- L46: `Per-version migration notes will land here as releases are cut. Until v0.2 ships (gated on Sprint 1 — see docs/PLAN.md), there is nothing to migrate between.` The sprint count is now wrong (Sprint 6 closed, not Sprint 1); the v0.9-rc1 candidate is the first migration target.
- L52-57: chapter cross-references describe Chapter 6/12/13/14/17 as "will document the underlying mechanics referenced above once they are retargeted at AWS" — those chapters are retargeted (Sprint 5), so the cross-references should be tightened to "see Chapter X for the mechanics" rather than "will document … once retargeted".
- L58: "Until then, the equivalent chapters in the [roksbnkctl book] describe the same mechanics for the shared surface" — replace with a direct link to the published awsbnkctl book section.
- "From manual EKS + BNK deployment" section (L9-22) needs a paragraph explicitly naming the as-shipped v0.9 limitations (no `cluster *` / `bnk *` subverbs; ops-pod prose lags the YAML; operator-run spike still required for the SR-IOV node-group VF advertisement).

PLAN.md § Sprint 6 lists "MIGRATING.md is the final word for migrators" as an end-of-sprint gate. Sprint 6 close ships the file as scaffold-shape; that gate is not met.

**Files affected**: `MIGRATING.md` (status banner + per-version section + cross-reference tightening + v0.9-rc1-specific migration notes).

**Proposed fix**: post-Sprint-6 architect pass (or v1.0 finalisation pass): rewrite the status banner against the v0.9-rc1 candidate cut; add a "## v0.9-rc1" subsection under "Between awsbnkctl versions" naming the as-shipped surface + known v1.x gaps; tighten the chapter cross-references to point at the now-retargeted awsbnkctl chapters directly.

## Issue 6 (HIGH) — Chapter 30 Glossary closure (Sprint 5 Issue 3) is partial — some entries correct, several still carry OpenShift/IBM framing or stale cross-references

**Severity**: high (glossary is the lookup surface every new term references; factual errors here propagate)
**Status**: open (architect)

**Description**: Sprint 5 tech-writer Issue 2 (HIGH) catalogued ~10 glossary entries that survived the architect Sprint 5 sweep with stale or factually wrong content. The Sprint 6 architect's glossary rewrite closed the most-egregious ones — CIS entry now disambiguates IBM's product as unrelated to BNK on EKS; the S3 definition is now correct; the EKS entry correctly identifies EKS as managed Kubernetes (not managed OpenShift); the redactor entry now says "masks AWS credential values"; the OpenShift entry now correctly identifies ROSA.

Remaining drift the Sprint 6 architect pass did not catch:

- **L46 "envFrom"** describes `envFrom: secretRef: awsbnkctl-aws-creds` — but the as-shipped k8s_install.yaml (per Sprint 6 staff verification) **has no envFrom field at all** (IRSA injection replaces it entirely; the manifest carries only `env: HOME=/tmp`). The glossary describes a Secret-projection mechanism that doesn't exist in v0.9.
- **L145 "restricted (PSA profile) / restricted-v2 (SCC)"** and **L168 "SCC (legacy)"** preserve OpenShift SCC framing as a co-equal admission profile. EKS uses PSA only; SCC mentions should be relegated to a one-line "(inherited from roksbnkctl framing; not enforced on EKS)" parenthetical rather than full entries.
- **L171 "Secret (k8s)"** says `awsbnkctl-aws-creds` Secret is created at `ops install` time — but the as-shipped YAML deletes the standalone Secret entirely (the per-Job ephemeral Secret in `awsbnkctl-test` is the only Secret the ops surface creates).
- **L19 "ClusterRole"** references the `awsbnkctl-test` namespace verbatim without cross-linking chapter 19 — readers seeing the namespace name land on chapter 19's v1.x banner and get confused about whether the namespace is real or roadmap.

**Files affected**: `book/src/30-glossary.md` (~5 surface edits, none structural).

**Proposed fix**: post-Sprint-6 architect pass: cross-check glossary entries against `internal/exec/k8s_install.yaml` as actually shipped; collapse PSA / SCC entries; drop the `awsbnkctl-aws-creds` envFrom references (or annotate as v1.x); tighten the ClusterRole cross-link.

## Issue 7 (MEDIUM) — README "Status" banner says "Sprint 6 complete; v0.9-rc ready" but the "Quick start" section uses `go install ...@latest` which won't work until a tag is cut

**Severity**: medium (first-time-reader stuck-point — the binary install path doesn't work end-to-end at v0.9-rc1 cut time)
**Status**: open (architect or integrator pre-tag)

**Description**: README §"Quick start" (L18-37) opens with `go install github.com/JLCode-tech/awsbnkctl/cmd/awsbnkctl@latest`. At v0.9-rc1 candidate (pre-tag) `@latest` resolves to the most-recent tag — which is `v0.0.0` or the upstream fork-point reference, neither of which is a working AWS-shape build. A first-time reader copy-pasting the quick-start runs into either "no matching versions" or installs a stale fork-point binary that doesn't have the AWS surface.

The status banner correctly frames the project as "Sprint 6 complete; v0.9-rc ready; v1.0 awaits spike" but the quick-start should either (a) explicitly note "available once v0.9-rc1 tag is cut" with a placeholder version (`@v0.9.0-rc1`) and a `git clone` + `make build` fallback for pre-tag dogfood readers; or (b) defer until the integrator cuts the tag and the workflow lands the binary at goreleaser's release page.

**Files affected**: `README.md` § Quick start (L18-37).

**Proposed fix**: integrator at v0.9-rc1 tag-cut: replace `@latest` with `@v0.9.0-rc1` (or the actual tag chosen) and add a "Build from source (pre-tag readers)" sub-block with `git clone … && cd awsbnkctl && make build`.

## Issue 8 (LOW) — Chapter 25 filename rename (`25-cos-supply-chain.md` → `25-s3-supply-chain.md`) still pending

**Severity**: low (cosmetic; mdbook serves the file fine; 17 cross-links across the book reference the IBM-era slug)
**Status**: open (integrator at sprint close — same surface as Sprint 6 architect Issue 2)

**Description**: Already filed by Sprint 6 architect (Issue 2); included here for tech-writer cross-reference. The rename is an atomic `git mv` + sed-sweep across the 17 chapters referencing the old slug; chapter content has already been retargeted to "S3 (and optional ECR) supply chain" so only the slug carries the legacy name. The `book-build` CI job will catch any broken cross-link.

**Files affected**: `book/src/25-cos-supply-chain.md` → `book/src/25-s3-supply-chain.md`; `book/src/SUMMARY.md`; 17 cross-linking chapters.

**Proposed fix**: integrator pre-commit: `git mv book/src/25-cos-supply-chain.md book/src/25-s3-supply-chain.md && grep -rln 25-cos-supply-chain.md book/src/ | xargs sed -i '' 's|25-cos-supply-chain.md|25-s3-supply-chain.md|g'`.

## Issue 9 (LOW) — `release.yml` PDF-attach posture is correct-as-architected but the chain (`release.yml` → operator `make release-publish`) is not visible to a first-time release-cutter

**Severity**: low (process surface; the validator's Sprint 6 Issue 2 documents the architecture rationale)
**Status**: open (post-Sprint-6 documentation pass)

**Description**: Sprint 6 validator's Issue 2 caught that `release.yml` deliberately does not attach the book PDF (the multi-GB pandoc + XeLaTeX + mermaid-cli toolchain is local-driven; the integrator runs `make release-publish VERSION=vX.Y.Z` post-workflow to upload the PDF). This is the right design decision but the chain is invisible to a first-time release-cutter — `release.yml` succeeds, the GitHub Release page exists, the PDF attachment is missing, and there is no in-release-workflow signal telling the cutter "now run `make release-publish` locally to land the PDF".

The `.goreleaser.yml` carries the comment; the `release.yml` workflow does not. A future operator who cuts v1.0 by tag-push and doesn't read `.goreleaser.yml`'s release block ships v1.0 without the PDF asset.

**Files affected**: `.github/workflows/release.yml` (final echo step), `CONTRIBUTING.md` (release-cutting section), `docs/PLAN.md` (release-cutting checklist).

**Proposed fix**: post-Sprint-6 staff or validator pass: add a final `echo` step to `release.yml`'s `goreleaser` job naming the `make release-publish` next-action; add a "Cutting a release" section to `CONTRIBUTING.md` that lists the two-step ritual (tag-push → workflow → `make release-publish`); cross-link from PLAN.md.

---

## Per-prose-surface verdict

| Surface | Verdict |
|---|---|
| **Preface** | Clean. Status banner correctly frames v0.9 + Sprint 5 retarget. The "fork-relationship paragraph in Chapter 32 is the only place that intentionally references the upstream roksbnkctl codebase" framing is now slightly wrong (chapter 19 banner intentionally references it too; chapters 13 / 28 unintentionally do) — minor banner-text drift, not a stuck-point. |
| **SUMMARY.md** | Clean. All Parts I-X resolve; TOC labels read as AWS-shape. Legacy slugs deferred to post-v1.0 housekeeping per Sprint 6 architect Issues 2 + 3. |
| **Chapters 1-7 (Parts I-II)** | Clean. Read smoothly as a first-time reader. Quick-start (chapter 7) walks correctly through `init → up → test → down` against the AWS shape. |
| **Chapters 8-11 (Part III — cluster lifecycle)** | Banners correctly frame as v1.x; v0.9 readers redirect to quick start + chapter 10. Sprint 5 Issue 2 closed end-to-end. |
| **Chapter 12 (workspace config)** | Clean prose, but a few `cos:` cross-references (L149/369) drift — minor. |
| **Chapter 13 (Terraform variables)** | **BLOCKER (Issue 3).** Still describes ROKS / OpenShift / IBM-Cloud variable surface verbatim. Worked example doesn't match `terraform/variables.tf`. |
| **Chapter 14 (Credentials resolver)** | Clean. Correctly describes AWS standard chain + IRSA in-cluster shape. |
| **Chapters 15-18 (Parts IV-V)** | Clean. Chapters 17/18 verified IBM-residue-free per Sprint 5 Issue 4 closure. |
| **Chapter 19 (ops pod)** | **BLOCKER (Issue 2).** Top banner closes the bounce; body is still ~260 lines of inherited IBM trusted-profile narrative including ghost-Sprint-10 references and `IBMCLOUD_API_KEY` env-var prose. |
| **Chapters 20-23 (Part VI — testing)** | Clean. Sprint 4 prose stands up at sprint close. |
| **Chapters 24-26 (Part VII — operations)** | Clean. Chapter 25 carries the slug-rename Issue 8 but content is correctly S3-shape. Chapter 26 troubleshooting walks the IRSA trust chain correctly. |
| **Chapter 27 (Command reference)** | Clean. Auto-generated from cobra-md per Sprint 5. |
| **Chapter 28 (Configuration reference)** | **HIGH (Issue 4).** Still carries `cos:` block + `openshift_version` field. |
| **Chapter 29 (Terraform variable reference)** | Clean (auto-generated from tfvars-md per Sprint 5). The "roks_*" / "trusted_profile" mentions are all "replaces …" migration annotations — correct prose, not stale residue. |
| **Chapter 30 (Glossary)** | **HIGH (Issue 6).** Sprint 6 architect closed most Sprint 5 Issue 3 items but ~5 entries drift on PSA/SCC framing + envFrom + Secret naming. |
| **Chapters 31-33 (Parts IX-X)** | Clean. Chapter 32 fork-relationship section reads correctly; chapter 33 SR-IOV decision-record is accurate. |
| **README.md** | Status banner reflects Sprint 6 complete + v0.9-rc ready + v1.0 awaits spike — correct per architect refresh. Quick-start has the `@latest` issue (Issue 7). |
| **CHANGELOG.md** | **BLOCKER (Issue 1).** No Sprint 6 entry. |
| **MIGRATING.md** | **HIGH (Issue 5).** Still scaffold-shape. |
| **k8s_install.yaml** | Clean. Staff IRSA retarget verified — zero IBM/ROKS/COS hits. |
| **CI workflows (6 files)** | Clean. `security-audit` job lands; `book-build` extended to `docs/**/*.md`; `e2e-full.yml` banner refreshed; `release.yml`/`book.yml` posture documented per validator Issue 2. |

---

## Dogfooding-loop stuck-points

Walked the README quick-start → first cluster scenario as a first-time reader:

1. **Stuck-point at install (Issue 7, MEDIUM):** `go install …@latest` against a pre-tag candidate doesn't yield the AWS-shape binary. First-time reader bounces.
2. **Stuck-point at "what variables can I tune" (Issue 3, BLOCKER):** new reader landing on chapter 13 from the quick-start's `awsbnkctl init` worked example sees `roks_workers_per_zone` and tries it; gets `unknown variable`. Real-user-give-up severity.
3. **Stuck-point at "what fields does config.yaml support" (Issue 4, HIGH):** new reader landing on chapter 28 sees `cos:` block; tries to add `cos.bucket` to their `config.yaml`; gets `unknown field cos`.
4. **Stuck-point at "how do I install the ops pod" (Issue 2, BLOCKER):** reader scrolls past chapter 19 banner, follows the `awsbnkctl ops install --trusted-profile=auto` walkthrough, gets `unknown flag --trusted-profile` (the v0.9 binary has no such flag) — and the chapter still references "Sprint 10" as the closure milestone for a chapter that's part of a v1.0-candidate book.

Severity skew: **2 blocker stuck-points + 2 high stuck-points** at the reader-actually-bounces level.

## Cross-document drift verdict

PLAN.md (per architect Sprint 6 Issue 6 — "What's deferred to post-v1.0" appendix) is the canonical post-v1.0 list. CHANGELOG is missing the Sprint 6 entry (Issue 1). MIGRATING is scaffold-shape (Issue 5). README quick-start uses `@latest` against a pre-tag candidate (Issue 7). The four documents tell **inconsistent** stories about the v0.9-rc1 candidate — PLAN says "Sprint 6 complete; v1.0 awaits spike", README says "Sprint 6 complete; v0.9-rc ready", CHANGELOG says nothing about Sprint 6, MIGRATING says "no shipped release yet". Drift caught: yes; load-bearing.

## Gate-criteria audit (Sprint 6 close per the brief)

| Gate | Status |
|---|---|
| Sprint 5 BLOCKER Issue 1 (chapter 19 + k8s_install.yaml) | YAML side: ✓ closed (staff verified). Prose side: partial — banner closes the bounce; body (Issue 2) still drifts. |
| Sprint 5 BLOCKER Issue 2 (chapters 8/9/11 "Available in v1.x") | ✓ closed (architect-verified; tech-writer-verified). |
| Sprint 5 HIGH Issue 3 (glossary correctness) | Partial — most-egregious items closed; ~5 entries drift (Issue 6 above). |
| Sprint 5 HIGH Issue 4 (chapters 17/18/19/32 IBM-residue) | ✓ closed for 17/18/32. Chapter 19 covered by banner per Issue 1 closure path. |
| README sprint-count refresh | ✓ closed (architect-verified). Quick-start path has separate Issue 7. |
| CHANGELOG comprehensive (Sprint 0-6 entries) | ✗ open (Issue 1 above). |
| MIGRATING is the final word for migrators | ✗ open (Issue 5 above). |
| goreleaser snapshot produces 6 binary archives | ✓ closed (staff verified). |
| Security audit (gosec / govulncheck / secrets scan) | ✓ closed (staff verified; validator landed the `security-audit` CI job). |
| `make build`, `go test ./...`, `terraform validate` | ✓ closed (tech-writer-verified). |
| CI matrix end-state (security-audit + release + book-build) | ✓ closed (validator-verified + tech-writer-verified). |

## Issues filed: 9

- **3 blocker** (Issue 1 CHANGELOG; Issue 2 chapter 19 body; Issue 3 chapter 13)
- **3 high** (Issue 4 chapter 28; Issue 5 MIGRATING; Issue 6 glossary residue)
- **1 medium** (Issue 7 README `@latest`)
- **2 low** (Issue 8 chapter 25 slug; Issue 9 release.yml PDF-attach visibility)

## Sibling cross-references

- **architect Sprint 6 Issue 2** (chapter 25 rename) — same surface as my Issue 8.
- **architect Sprint 6 Issues 4 + 5** (chapter banners spot-check + chapter 18 decision-tree v0.9 caveat) — correctly out of scope for v0.9-rc1.
- **staff Sprint 6** — YAML retarget closed end-to-end; chapter 19 body rewrite is the unfinished half (architect scope, not staff).
- **validator Sprint 6 Issue 2** (release.yml PDF-attach posture) — same surface as my Issue 9.
- **Sprint 5 tech-writer Issues 1-4** — closures verified in this pass; partial-closure detail in Issues 2 / 6 above.

---

## Release-readiness verdict (v1.0-readiness preview for v0.9-rc1 release notes)

**Verdict: yes-with-spike-pending — AND with 3 blocker-class doc gaps to close pre-tag.**

The **code surface** is v0.9-rc1-ready: build clean, tests pass, all 8 terraform modules validate, `internal/exec/k8s_install.yaml` ships the correct IRSA shape, security audit posture documented and CI-gated, goreleaser produces the 6-binary matrix. The **operator-run spike (PRD 07)** carries as the gate to v1.0 final per the deferred-work appendix.

The **docs surface** is NOT v0.9-rc1-ready end-to-end. Three blocker-class gaps:

1. CHANGELOG.md has no Sprint 6 entry (Issue 1; integrator at tag-cut)
2. Chapter 19 body still walks the IBM trusted-profile narrative verbatim despite the v1.x banner (Issue 2; architect post-Sprint-6)
3. Chapter 13 still describes ROKS / OpenShift / IBM-Cloud variable surface (Issue 3; architect post-Sprint-6 — first-time reader following the quick-start hits this within 10 minutes)

Three additional high-severity drift items (Issues 4-6) plus the README quick-start `@latest` issue (Issue 7) follow on the heels of the blockers.

**Recommended integrator path before cutting v0.9-rc1:** land Issue 1 (CHANGELOG) at tag-cut; defer Issues 2-7 to a v0.9.x architect cycle that ships before v1.0 final. The binary surface is structurally ready; the documentation surface needs one more architect pass to clear chapters 13 / 19 (body) / 28 plus MIGRATING.md before v1.0 final.

SPIKE DEFERRAL carries — v1.0 first-tag still gates on operator-run PRD 07 spike (live EKS 1.30 + SR-IOV CNI VF advertisement on `c5n.4xlarge`). The Sprint 6 release is **structurally complete** at v0.9-rc1; anyone with operator-run spike validation can cut v1.0 immediately once the three blocker doc gaps above are folded.
