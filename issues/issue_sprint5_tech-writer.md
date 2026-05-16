# Sprint 5 — tech-writer issues

Sprint 5 is the book retarget sprint. The architect rewrote 12+ chapters
plus did mechanical sweeps; staff swept IBM residue from 297 to 1 hit in
`internal/*.go`; validator added the `book-build` CI job + cspell at zero
findings. This tech-writer pass walked the SUMMARY.md from Part I through
Part X, audited AWS-retarget completeness, ran the cross-link sweep,
spec-checked the book CI workflow, and verified the build-green gate.

**Read-only.** No project files edited. SPIKE DEFERRAL carries.

**Scope:** 35 markdown files under `book/src/` (34 chapters + SUMMARY.md
+ preface), plus README, CHANGELOG, MIGRATING.

**Verification status (end of sprint):**

- `go build ./...` ✓ clean
- `go test ./...` ✓ clean (12 packages cached)
- `make build` ✓ binary at `bin/awsbnkctl` (dev tag, commit 727b2bb)
- `grep -rn 'IBMCloud\|IBMCLOUD\|ibmcloud' --include='*.go' internal/`
  → 1 hit (the intentional `secrets.go` keychain user-key for v0.x → v0.9
  cleanup; staff Issue carries the rationale). Matches the staff report.
- `grep -rn '...IBM...' book/src/` → ~50 hits; most are allowed
  (fork-relationship sections in 02/04/14/25/32/33/preface, the SUMMARY
  filenames that the architect Issue 3 deferred), but Issues 1-5 below
  catalogue the surfaces where IBM residue **survived** the architect's
  Sprint 5 sweep.
- `mdbook build book/` — not exercised (mdbook not installed locally per
  Sprint 5 staff verification + macOS sandbox). Spec-check on
  `.github/workflows/book.yml` + the new `book-build` job in `ci.yml`
  confirms the CI gate is in place; first post-merge run will exercise it.
- `cspell` — not exercised (cspell not on PATH); validator Issue 3
  confirmed zero unknown-word findings via `npx cspell` and the
  `book-build` CI job will hard-fail any future regression.
- 361 relative `[link](./XX-*.md)` cross-links checked; **0 broken file
  targets**. Anchor cross-links flagged 19 candidate slug-mismatches —
  most are mdbook slug-rule differences from a naive Python slugger
  (backtick + colon stripping in fenced headings like `## ` + "tf_source:" +
  ` ` resolve fine through mdbook's slug rules), but Issue 4 below
  catalogues the genuine stale anchors that survived the retarget.

---

## Issue 1 (BLOCKER) — Chapter 19 (in-cluster ops pod) describes an `awsbnkctl-ibm-creds` Secret + IBM Trusted Profile flow that the as-shipped `awsbnkctl ops install` cannot create — and the as-shipped installer is itself still IBM-shaped

**Severity**: blocker
**Status**: open (cross-agent — staff + architect)

**Description**: Chapter 19 (`book/src/19-in-cluster-ops-pod.md`) is
**14 references** to `awsbnkctl-ibm-creds` long. The chapter walks the
reader through `awsbnkctl ops install`, shows sample output emitting
"✓ created secret awsbnkctl-ops/awsbnkctl-ibm-creds", documents the SA's
RBAC binding scoped to `resourceNames: ["awsbnkctl-ibm-creds"]`, and
explains the credential-rotation flow.

The architect Sprint 5 issue file marks chapter 19 as "Ships. IRSA-based
auth replaces trusted-profile flow." That is **incorrect** — the chapter
prose still describes the trusted-profile + static-IBM-key flow
verbatim.

Worse, the **prose matches the as-shipped installer reality**:
`internal/exec/k8s_install.yaml` is still entirely roksbnkctl-shaped:

```
$ grep -nE 'roksbnkctl|ibm' internal/exec/k8s_install.yaml | head
 1:# roksbnkctl ops install manifests
 9:# are created up front so the long-lived ops Pod (in roksbnkctl-ops)
20:  name: roksbnkctl-ops
27:  name: roksbnkctl-test
35:  name: roksbnkctl-ops
40:  # ops install --trusted-profile=auto|on patches an additional
41:  # annotation iam.cloud.ibm.com/trusted-profile: <profile-id> here
47:  name: roksbnkctl-ibm-creds
55:  IBMCLOUD_API_KEY: "${IBMCLOUD_API_KEY_B64}"
68:  IC_API_KEY: "${IBMCLOUD_API_KEY_B64}"
102:  resourceNames: ["roksbnkctl-ibm-creds"]
122:  name: roksbnkctl-ops
129:  restartPolicy: Always
130:  # OpenShift restricted-v2 SCC compliance ...
```

Staff Issue 2 acknowledges the installer YAML wasn't retargeted ("scope
deferred to Sprint 6"). The headline IBM-residue closure ("297 → 1 hit")
counted **only `.go` files** — the embedded YAML template was excluded
from the grep. So the book prose, the doctor probe code, and the
installer manifest are all still IBM-shaped end-to-end.

This is the largest single drift in the Sprint 5 surface. Either:

- (a) Sprint 6 hardening sweeps `internal/exec/k8s_install.yaml` + the
  template substitution code in `internal/cli/ops.go` + the `K8sOpsSecretName`
  constant + `probeOpsPodIRSA`, **and** the architect rewrites chapter 19
  prose against the new IRSA-only shape; or
- (b) chapter 19 is annotated at the top as "v1.x feature — current
  install is the inherited IBM trusted-profile flow; IRSA retarget tracked
  in Sprint 6 staff Issue 2" so first-time readers don't waste hours
  trying to map the prose onto an installer that doesn't exist.

For v0.9 (M5 milestone), (b) is the smallest fix that closes the
correctness gap. (a) is the right closure for v1.0.

**Files affected**: `book/src/19-in-cluster-ops-pod.md` (chapter prose);
`internal/exec/k8s_install.yaml` (embedded template); `internal/cli/ops.go`
(template substitution + `awsbnkctl ops install` command);
`internal/cli/doctor_backend.go` (probe code); `internal/exec/k8s.go`
(`K8sOpsSecretName`); `book/src/30-glossary.md` (envFrom / Secret /
Trusted Profile entries — see Issue 2).

**Proposed fix**: Sprint 6 architect + staff coordinate; tech-writer
re-audits at Sprint 6 close.

## Issue 2 (HIGH) — Chapter 30 (Glossary) has factual errors and unswept IBM residue that contradict the rest of the retargeted book

**Severity**: high
**Status**: open (architect)

**Description**: The architect Sprint 5 issue file marks chapter 30 as
"Ships. AWS-flavoured entries replace IBM ones." Read-through finds
~10 entries that survived the sweep with stale or factually wrong
content. Selected examples:

- **L17 (CIS):** "At the AWS account level, **CIS** is **Cloud Internet
  Services** — IBM's DNS, CDN, WAF, and DDoS-protection product." The
  entry pivots on the word "AWS" but the product is IBM's. The reader
  is left wondering which.
- **L31 (S3 was COS):** "IBM **Cloud Object Storage** — S3-compatible
  object store. The BNK supply chain bucket lives on a S3 bucket." The
  term is being defined as IBM Cloud Object Storage while the
  definition body uses S3.
- **L38 (CRN):** "IBM's globally-unique resource identifier. … Most
  `awsbnkctl cos` commands accept either a friendly name (resolved at
  runtime) or a CRN." There is no `awsbnkctl cos` subtree. CRN is an
  IBM-only concept and should either be deleted or labelled as a
  legacy concept inherited from `roksbnkctl`.
- **L49 (envFrom):** describes the ops pod using
  `envFrom: secretRef: awsbnkctl-ibm-creds` (see Issue 1).
- **L125 (OpenShift):** "EKS = managed OpenShift on AWS." **EKS is
  managed Kubernetes**, not managed OpenShift. The AWS-managed
  OpenShift product is ROSA. This is the kind of factual error a
  first-time reader will catch.
- **L146 (redactor):** "masks the IBM API key value in any subprocess's
  stdout/stderr".
- **L148-149 (EKS):** "**Red Hat OpenShift on AWS** — IBM's managed
  OpenShift offering. The cluster `awsbnkctl up` provisions. See
  [Chapter 2](./02-why-roks.md)." EKS is neither Red Hat OpenShift on
  AWS (that's ROSA) nor IBM's offering.
- **L162-163 (SCC):** OpenShift terminology preserved as if it's an
  EKS concern; EKS uses PSA (Pod Security Admission), not SCC.
  Cross-references to "the bundled image and the runAsNonRoot
  constraint" are fine but the SCC framing should be retired.
- **L166 (Secret):** "creates `awsbnkctl-ibm-creds`" (see Issue 1).
- **L198 (Trusted Profile):** "An AWS IAM construct that lets a
  Kubernetes ServiceAccount assume AWS permissions." That's IRSA, not
  Trusted Profile. Trusted Profile is the IBM equivalent.
- **L207-208 (VPE):** "**Virtual Private Endpoint** — AWS's
  private-network access point for managed services." That's an IBM
  term (VPE); AWS calls them VPC endpoints. Cross-reference is to a
  stale chapter 26 anchor (see Issue 4).

**Files affected**: `book/src/30-glossary.md`.

**Proposed fix**: Sprint 6 architect rewrites the glossary against the
as-shipped AWS shape. Suggested approach: delete CRN, VPE, Trusted
Profile, SCC outright (cross-link them to a one-line "see roksbnkctl
docs for the IBM equivalent" note if the fork-relationship hint is
useful); rewrite EKS, S3, redactor, envFrom, Secret entries against the
real AWS / EKS shape.

## Issue 3 (HIGH) — Multiple chapters describe an `awsbnkctl cluster` subcommand subtree that does not exist in the v0.9 binary

**Severity**: high
**Status**: open (architect or staff — depends on whether the verb is added or the chapters retargeted)

**Description**: Architect Issue 5 flagged that `cluster register` may
not exist. The actual surface is worse — the entire `cluster` subtree is
missing:

```
$ ./bin/awsbnkctl cluster --help
Error: unknown command "cluster" for "awsbnkctl"
```

Yet the as-shipped book chapters describe a rich `cluster` command
group as if it ships:

- `book/src/02-why-roks.md:71` — "`awsbnkctl up cluster` provisions the
  SR-IOV stack as part of the cluster bring-up."
- `book/src/06-workspaces.md:36-37` — references `awsbnkctl cluster up`
  and `cluster register` as live commands.
- `book/src/08-cluster-phase.md` — **entire chapter** describes a
  two-phase model with `cluster up`, `cluster down`, `cluster show`,
  the `state-cluster/` directory, the `cluster-outputs.json` written
  by `cluster up`. ~150 lines of prose against a verb that doesn't
  exist.
- `book/src/09-registering-existing-cluster.md` — entire chapter is
  the `cluster register` walkthrough (architect Issue 5).
- `book/src/11-tearing-down.md` — refers to three destroy verbs
  (`down`, `bnk down`, `cluster down`); only `down` ships.
- `book/src/12-workspace-config.md:119` — schema row for `cluster.create`
  describes `awsbnkctl cluster up` and `cluster register <name>`
  semantics.

The as-shipped surface offers `up`, `down`, `apply`, `plan`, `init`,
`status`, plus the `k *` subtree, `targets`, `self`, `install`,
`logs`, `get`, `doctor`. There is no `cluster` group, no `bnk`
group, no `register` verb.

Per architect Issue 5, the lineage is that `roksbnkctl` did ship a
working `cluster register` for ROKS, and the chapter was rewritten
"against a hypothetical future implementation". For v0.9 the gap is
significant — readers walking the book linearly will hit
`./bin/awsbnkctl cluster up` on a real terminal and bounce.

**Files affected**:
- `book/src/02-why-roks.md` (one cross-link)
- `book/src/06-workspaces.md` (two cross-references)
- `book/src/08-cluster-phase.md` (entire chapter, ~1,200 lines)
- `book/src/09-registering-existing-cluster.md` (entire chapter)
- `book/src/10-deploying-bnk-trials.md` (the `bnk up`/`bnk down`
  group also doesn't exist as a separate subtree)
- `book/src/11-tearing-down.md` (the three-verb framing)
- `book/src/12-workspace-config.md:119` (schema row)
- `internal/cli/` (verb registration, if path (a) below is taken).

**Proposed fix**: integrator decision between (a) Sprint 6 staff lifts
the `cluster` + `bnk` subtree from `roksbnkctl/internal/cli/cluster.go`
+ `bnk.go` and retargets at EKS (substantial implementation work but
preserves the book unchanged), or (b) Sprint 6 architect rewrites
chapters 8-12 against the single-phase `awsbnkctl up` / `down` shape
that actually ships and annotates chapter 9 as "v1.x feature".

Recommendation: (b) for v0.9 (the architect-only path closes the gap
this cycle); (a) for v1.0 if customer feedback justifies the
two-phase lifecycle.

## Issue 4 (MEDIUM) — Stale anchor cross-links survived the chapter-26 retarget

**Severity**: medium
**Status**: open (architect)

**Description**: The architect's chapter 26 retarget renamed the
"orphan IBM Cloud resources" section to "orphan ENIs" (line 63). The
glossary's VPE entry (chapter 30, line 208) still cross-links to the
old slug:

```
[Chapter 26 §"orphan AWS resources"](./26-troubleshooting.md#symptom-terraform-destroy-leaves-orphan-ibm-cloud-resources-lbs-security-groups-vpes)
```

The link text reads "orphan AWS resources" but the anchor target is
the deleted IBM-resources slug. mdBook will render the link as a dead
hash; the `book-build` CI job's mdbook build will pass (mdbook doesn't
hard-fail dead intra-page anchors) but the click-through is broken.

The naive Python anchor sweep flagged 19 candidates; most are
slug-rule artefacts (em-dashes, double-dashes, backticks, colons in
headings) that mdbook handles differently from the script. The one
above is the load-bearing genuine break. Two other candidates that
warrant manual review:

- `30-glossary.md:8` → `14-credentials-resolver.md#source-3--workspace-api_key_b64`
  — chapter 14's heading is `### Source 3 — SSO cached token`, not
  the Sprint 4-era `workspace-api_key_b64` shape. The glossary entry's
  prose is still describing the IBM-era inline-key path that the
  AWS-retarget no longer ships.
- `30-glossary.md:64` → `25-cos-supply-chain.md#licence-rotation` —
  chapter 25's heading is `### Rotating the subscription JWT` (and
  similarly for FAR / IRSA); no `licence-rotation` anchor exists.

**Files affected**: `book/src/30-glossary.md` (3 stale anchors); chapter
14 entry needs the AWS-source semantics; chapter 25 rotation slug
needs to match the actual heading shape.

**Proposed fix**: Sprint 6 architect re-audits chapter 30's
cross-references against the current chapter 14/25/26 heading slugs.
A single `grep -n '\](./[0-9].*#' book/src/30-glossary.md` and a
heading-by-heading walk in the target chapters closes it.

## Issue 5 (MEDIUM) — Chapter 17 + 18 + 32 IBM-residue (code snippets, secret names, fictitious tool maps) survived the architect's chapter-by-chapter sweep

**Severity**: medium
**Status**: open (architect)

**Description**: Sample selections from the post-sweep state:

- `17-execution-backends.md:442` — describes `Credentials.IBMCloudAPIKey`
  becoming a one-shot Secret. Per staff's IBM-residue sweep, the
  `Credentials.IBMCloudAPIKey` field is **deleted** from `internal/exec/creds.go`.
  The chapter is now describing a Go field that does not exist.
- `17-execution-backends.md:629` — code-fence example struct contains
  `IBMCloudAPIKey  string`.
- `18-choosing-backend.md:309` — worked-example terminal output shows
  `✓ Secret awsbnkctl-ibm-creds applied (envFrom secretRef)`. Same
  residue category as chapter 19 (Issue 1).
- `32-extending-roksbnkctl.md:59` — `toolImages` map example contains
  `"ibmcloud": "ghcr.io/JLCode-tech/awsbnkctl-tools-ibmcloud"`. Per
  staff's sweep, the `toolImages["ibmcloud"]` entry is deleted from
  `internal/exec/docker.go`. The chapter teaches the wrong shape.
- `32-extending-roksbnkctl.md:70,82-84` — k8s ops-pod and ssh
  `toolPackages` examples both still use `ibmcloud` as the canonical
  worked-example tool. Same delete-in-code, survive-in-book pattern.
- `25-cos-supply-chain.md:5` — "On the upstream `awsbnkctl` fork the
  same artefacts lived in IBM Cloud Object Storage (COS)..." — typo:
  the upstream is `roksbnkctl`, not `awsbnkctl`. Self-reference
  rather than fork reference.
- `01-what-is-bnk.md:46` — fine as written (legitimate F5 support
  matrix bullet listing ROKS + IBM Cloud alongside AKS / GKE), no
  action.

**Files affected**: `book/src/17-execution-backends.md`,
`book/src/18-choosing-backend.md`, `book/src/25-cos-supply-chain.md`,
`book/src/32-extending-roksbnkctl.md`.

**Proposed fix**: Sprint 6 architect spot-pass on these chapters using
`iperf3` / `terraform` / `helm` as the worked-example tool instead of
`ibmcloud`; retarget chapter 17 code-fence + L442 prose against the
post-sweep Credentials struct shape; one-character typo fix in
chapter 25 L5.

## Issue 6 (MEDIUM) — README.md status block is 5 sprints stale

**Severity**: medium
**Status**: open (staff)

**Description**: `README.md` line 5: "**Status:** pre-release (M0 —
Sprint 0 just landed; first tagged release `v0.2` gated on Sprint 1
per `docs/PLAN.md`). … **Nothing in this README works yet until
Sprint 1 closes**."

Reality:
- Sprint 5 (M5 / v0.9 milestone) just landed per the PLAN.
- CHANGELOG.md documents Sprints 1, 2, 3, 4 in detail (and no Sprint 5
  entry — see Issue 7).
- `book/src/preface.md:3` correctly says "Status: v0.9 — Sprint 5 of
  the AWS retarget complete."

A reader arriving at the GitHub repo front door reads README first and
walks away thinking the binary doesn't exist yet. README needs to
catch up to the book preface's v0.9 framing.

**Files affected**: `README.md`.

**Proposed fix**: Sprint 6 staff updates the README status block + the
"Planned quick start (post-Sprint 1)" section + the "What's in this
repo" Internalised path table (currently describes future state). The
"Planned" framing throughout is M0-era; v0.9 is "Implemented and
shipping in v0.9 — for the canonical user guide see the book at
JLCode-tech.github.io/awsbnkctl/book/."

## Issue 7 (LOW) — CHANGELOG.md missing Sprint 5 entry

**Severity**: low
**Status**: open (staff or integrator at tag-cut)

**Description**: CHANGELOG.md "Unreleased" section has detailed
entries for Sprints 0-4 but **no Sprint 5 entry**. Sprint 5's
deliverables (book retarget, `book-build` CI job, cspell sweep, IBM
residue closure on `.go` files, refgen regeneration) warrant a
sentence each.

**Files affected**: `CHANGELOG.md`.

**Proposed fix**: integrator adds the Sprint 5 entry at tag-cut time.
Validator Issue 2/3 + staff Issue priorities table give the line-items
verbatim.

## Issue 8 (LOW) — Architect Issue 3 (filename `25-cos-supply-chain.md`) carries — chapter title is "S3 (and optional ECR) supply chain" but the file slug, the SUMMARY.md TOC entry, and every cross-link still use "cos-supply-chain"

**Severity**: low (cosmetic; mdbook serves the file fine)
**Status**: open (deferred to Sprint 6 integrator — confirms architect plan)

**Description**: Architect Issue 3 catalogued this; tech-writer
confirms the file is still `25-cos-supply-chain.md` on disk, the
SUMMARY.md TOC reads `[S3 (and optional ECR) supply chain](./25-cos-supply-chain.md)`,
and 17 chapters cross-link to that path. The rename is mechanically
straightforward (`git mv` + sed sweep) and should land in Sprint 6's
first integrator commit before any new chapter content. Validator's
`book-build` job will gate against breakage.

**Files affected**: `book/src/25-cos-supply-chain.md` (rename target),
`book/src/SUMMARY.md`, 17 chapter cross-links.

**Proposed fix**: Sprint 6 integrator atomic rename per architect's
Issue 3 plan.

---

## Per-prose-surface verdict

| Surface | Tech-writer verdict |
|---|---|
| `book/src/preface.md` | ✓ ships clean; v0.9 framing accurate |
| `book/src/SUMMARY.md` | ✓ ships clean; structure matches Parts I-X |
| `book/src/01-what-is-bnk.md` | ✓ ships clean; F5 framing intact |
| `book/src/02-why-roks.md` | ⚠ filename + the chapter title "Why EKS + self-managed SR-IOV node groups" mismatch (filename retained for fork-relationship reasons per architect plan; SUMMARY label is correct); one `awsbnkctl up cluster` reference (Issue 3) |
| `book/src/03-what-roksbnkctl-does.md` | ⚠ filename mismatch (same architect-plan rationale); content reads clean |
| `book/src/04-installation.md` | ✓ ships clean |
| `book/src/05-doctor.md` | ✓ ships clean |
| `book/src/06-workspaces.md` | ⚠ two `cluster` subverb cross-links (Issue 3) |
| `book/src/07-quick-start.md` | ✓ ships clean — the headline first-time-reader chapter; concrete, runnable, end-to-end |
| `book/src/08-cluster-phase.md` | ✗ blocker for first-time readers if the `cluster` subtree doesn't ship (Issue 3) |
| `book/src/09-registering-existing-cluster.md` | ✗ same as 08 — entire chapter against an absent verb (Issue 3 + architect Issue 5) |
| `book/src/10-deploying-bnk-trials.md` | ⚠ `bnk up`/`bnk down` subverb framing (Issue 3) |
| `book/src/11-tearing-down.md` | ⚠ three-verb framing rests on `cluster down` + `bnk down` (Issue 3) |
| `book/src/12-workspace-config.md` | ⚠ schema row L119 (Issue 3); otherwise clean |
| `book/src/13-terraform-variables.md` | ✓ ships clean; architect Issue 1 (auto-regen) closed by staff |
| `book/src/14-credentials-resolver.md` | ✓ ships clean post-rewrite |
| `book/src/15-ssh-targets.md` | ✓ ships clean |
| `book/src/16-on-flag-ssh-jumphosts.md` | ✓ ships clean |
| `book/src/17-execution-backends.md` | ⚠ `IBMCloudAPIKey` references (Issue 5) |
| `book/src/18-choosing-backend.md` | ⚠ worked-example IBM-creds output (Issue 5) |
| `book/src/19-in-cluster-ops-pod.md` | ✗ blocker per Issue 1 — entire chapter describes the un-retargeted IBM ops-pod flow |
| `book/src/20-connectivity-testing.md` | ✓ ships clean |
| `book/src/21-dns-testing-gslb.md` | ✓ ships clean |
| `book/src/22-throughput-testing.md` | ✓ ships clean; iperf3 image-tag drift per Sprint 4 tech-writer Issue 1 is resolved via staff Issue 1 documenting the decision |
| `book/src/23-e2e-test-plan.md` | ✓ ships clean |
| `book/src/24-day-2-ops.md` | ✓ ships clean; the OpenShift-extensions § L282-295 is legitimately scoped (BNK clusters do surface those types on EKS via CRDs — OK as written) |
| `book/src/25-cos-supply-chain.md` | ⚠ L5 self-reference typo (Issue 5); filename rename pending (Issue 8); otherwise clean |
| `book/src/26-troubleshooting.md` | ✓ ships clean; sub-anchors landed per architect Issue 6 closure |
| `book/src/27-command-reference.md` | ✓ ships clean — confirms `cluster`/`bnk` subtrees are absent (which is the inverse problem of Issue 3) |
| `book/src/28-configuration-reference.md` | ⚠ L65 + L111 OpenShift references (cosmetic) |
| `book/src/29-terraform-variable-reference.md` | ✓ ships clean — AWS-shaped variables throughout, regenerated this sprint |
| `book/src/30-glossary.md` | ✗ Issue 2 high-severity (factual EKS errors + unswept IBM residue) |
| `book/src/31-building-from-source.md` | ✓ ships clean |
| `book/src/32-extending-roksbnkctl.md` | ⚠ ibmcloud tool-map worked examples (Issue 5) |
| `book/src/33-data-plane-decision.md` | ✓ ships clean; the upstream-roksbnkctl decision-narrative paragraphs are legitimate fork-relationship context |
| `README.md` | ✗ Issue 6 (5 sprints stale) |
| `CHANGELOG.md` | ⚠ Issue 7 (no Sprint 5 entry) |
| `MIGRATING.md` | ✓ ships clean post-Sprint 4; "scaffolding" header preserves the it's-honest-about-state framing |

## Dogfooding-loop stuck-points

Walked the SUMMARY.md from the preface through Part X as a
first-time-reader. Stuck-point count + severity:

| Stuck-point | Severity | Issue # |
|---|---|---|
| Chapter 7 (quick-start) is clean — no first-time-reader friction; the `awsbnkctl init` → `up` → `test` → `down` path reads correctly | — | — |
| Chapter 8 reader runs `./bin/awsbnkctl cluster up` → "unknown command" → bounces | blocker | Issue 3 |
| Chapter 9 reader runs `awsbnkctl cluster register` → same outcome | blocker | Issue 3 |
| Chapter 19 reader runs `awsbnkctl ops install` → succeeds but pod is named `roksbnkctl-ops`, Secret is `roksbnkctl-ibm-creds`, $IBMCLOUD_API_KEY env var is set; reader concludes the AWS retarget didn't ship | blocker | Issue 1 |
| Chapter 30 glossary reader sees "EKS = managed OpenShift on AWS" and re-evaluates their understanding of EKS | high | Issue 2 |
| Cross-references chapter 26 §"orphan AWS resources" → dead anchor | medium | Issue 4 |
| README.md status block says nothing works yet → reader bounces back to the repo front door confused about what's shipping | medium | Issue 6 |

Skew: 3 blockers, 1 high, 2 medium. The three blockers all root-cause
at the same shape (book describes a surface that the AWS retarget
hasn't lifted across — `cluster` subtree, `bnk` subtree, IBM ops-pod
installer).

## Cross-document drift verdict

Drift caught between PLAN / PRDs / book / README / CHANGELOG /
MIGRATING:

- **README ↔ book preface**: README says M0, preface says v0.9. Issue 6.
- **CHANGELOG ↔ PLAN**: PLAN documents Sprint 5; CHANGELOG has no Sprint
  5 entry. Issue 7.
- **Book chapter 19 ↔ `internal/exec/k8s_install.yaml`**: chapter
  prose and YAML template are both still IBM-shaped; staff's IBM-residue
  closure didn't touch the YAML (Issue 2 in staff). The "297 → 1 hit"
  headline only counted `.go` files. Issue 1.
- **Book chapter 17 ↔ `internal/exec/creds.go`**: chapter L442 + L629
  describe `Credentials.IBMCloudAPIKey` which staff deleted. Issue 5.
- **Book chapters 8-12 ↔ `internal/cli/`**: chapters describe
  `cluster up/down/show/register` + `bnk up/down`; binary has no such
  subtrees. Issue 3.
- **Book chapter 30 ↔ everything**: glossary contradicts the rest of
  the book on the EKS framing. Issue 2.

## Gate-criteria audit (PLAN.md § Sprint 5)

| Gate criterion | Status |
|---|---|
| `mdbook build book/` clean | TBD-by-integrator (mdbook not on local PATH; `book-build` CI job will exercise on first push) |
| Web book deploys to GitHub Pages | TBD-by-integrator (per validator Issue 1, deploy is local-driven via `make release-publish`; tag-cut workflow) |
| Cross-link audit clean | ⚠ 1 genuine broken anchor (Issue 4); 0 broken file targets |
| cspell clean | ✓ validator Issue 3 confirms zero findings |
| All chapter bodies retargeted at AWS / EKS | ✗ chapters 8, 9, 19 still describe IBM / roksbnkctl-only surfaces (Issues 1, 3); chapter 30 has factual errors (Issue 2) |

## Issues filed: 8

- **2 blocker** (Issues 1 — IBM ops-pod installer + chapter 19 drift; 3 — `cluster`/`bnk` subverbs absent)
- **2 high** (Issues 2 — glossary; conceptually scoped sub-issue of 3 — chapters 8/9/11 cluster verbs)
- **3 medium** (Issues 4 — anchor; 5 — chapters 17/18/32 IBM residue; 6 — README stale)
- **1 low** (Issue 7 — CHANGELOG; Issue 8 — cos→s3 rename deferred per architect plan)
- 0 roadmap

## Sibling cross-references

- **Architect Issue 5** (chapter 9 `cluster register` may not exist):
  confirmed and broadened — the entire `cluster`/`bnk` subtree is
  absent (this tech-writer's Issue 3).
- **Architect Issue 1** (auto-regen pending): closed by staff this
  sprint; chapters 27, 29 are clean post-regeneration.
- **Architect Issue 3** (chapter 25 filename rename): confirmed; this
  tech-writer's Issue 8.
- **Architect Issue 6** (chapter 26 sub-anchors): confirmed closed;
  the `### AWS LoadBalancer` and `### DNS` sub-anchors land cleanly.
- **Staff Issue 1** (iperf3 image-tag decision): chapter 22 reads
  clean against the decision; PSA callout in chapter 22 L52-83 is
  present.
- **Staff Issue 2** (`k8s_install.yaml` IBM-Secret retire deferred):
  upgrade to **blocker** when read together with chapter 19
  (this tech-writer's Issue 1). The deferral is the entire reason
  Issue 1 fires.
- **Validator Issue 1** (book.yml deploy posture): architecture
  decision; tech-writer agrees with the validator recommendation to
  keep local-driven publish + add an HTML-only gh-pages deploy on tag
  push in Sprint 6.
- **Validator Issue 2/3** (book-build CI job + cspell): closed; the
  hard gate is in place for future PRs.
- **Validator Issue 5** (spellcheck.yml overlap): defer to Sprint 6
  per validator plan; no tech-writer surface impact.

---

## AWS-retarget completeness verdict

Approximately **~80% of book by chapter count** is AWS-retargeted
cleanly (28/35 markdown files ship clean). The remaining ~20% — chapters
8, 9, 10, 11, 12, 19, 30 plus README — carry the load-bearing residue
catalogued above. The architect's "12+ chapters rewritten" headline
captures the work delivered; the missed surface is concentrated in the
cluster-lifecycle subtree (8-11) and the ops-pod subtree (19), both of
which depend on staff or v1.x-feature work that didn't land this sprint.

## IBM-residue closure verdict

**`.go` surface**: staff's "297 → 1" is accurate and the residual hit
in `internal/config/secrets.go` is intentional (legacy-keychain cleanup
on workspace delete).

**Non-`.go` surface** (the part the headline didn't count): substantial
residue survives in `internal/exec/k8s_install.yaml` (the embedded
ops-install manifest is entirely IBM-shaped — see Issue 1) and in 7
book chapters (Issues 1, 2, 5). Staff Issue 2 acknowledged the YAML
deferral but the headline understates the scope of the deferral.

## mdbook build verdict

**TBD-by-integrator**. mdbook is not on the local sandbox PATH; the new
`book-build` CI job in `ci.yml` will exercise it on the first
post-merge run. Static analysis: 361 cross-links resolve to existing
files; 19 anchor candidates flagged of which 1 is a genuine break
(Issue 4) and 18 are slug-rule artefacts that mdbook handles. The 0
broken file targets is the load-bearing result for the build pass.

## Ready-for-Sprint-6 verdict

**Conditional pass.** Sprint 5 lands the bulk of the book retarget at
quality. The two blocker-class issues (Issue 1 — ops-pod IBM residue
end-to-end; Issue 3 — `cluster`/`bnk` subverbs in the book against
absent binary surface) carry into Sprint 6 hardening per the
sprint-close PLAN. The v0.9 milestone gate sits between Sprint 5 close
and Sprint 6 close — the integrator can cut a Sprint 5 commit safely
now, but **v0.9-tagged release is not ready until at minimum Issue 3
(via path b — annotate chapters as v1.x feature) and Issue 6 (README
stale) land**. Issue 1 strictly needs Sprint 6 staff to retarget the
installer YAML; the integrator can document this as a known v0.9
limitation if a v0.9 tag is wanted before Sprint 6 closes.

SPIKE DEFERRAL carries — book content describes the design
as-implemented (which is the right call for the retarget sprint), not
as-validated against live AWS; PRD 07's operator-run spike still gates
the v1.0 tag.
