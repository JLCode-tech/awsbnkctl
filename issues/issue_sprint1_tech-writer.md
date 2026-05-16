# Sprint 1 — tech-writer issues

End-of-sprint read-only review. Covered: PRD 07 ↔ Terraform module
reconciliation, chapter 2 (`book/src/02-why-roks.md` — filename retained
per architect Issue 3) + chapter 33 read-through, build + dry-run + go
test + go vet + terraform validate dogfood, terraform fmt sweep,
cross-link audit, cspell verification on new prose, README/CHANGELOG
alignment, tools image docker build + smoke. Sibling Sprint 1 issue
files cross-referenced: architect (7), staff (6), validator (5).

PRD 07 ↔ implementation alignment: **yes-with-deltas** (one cosmetic
fmt nit, one architect-flagged naming-default decision still
open — see Issues 2 and 3 below; PRD-inputs/outputs ↔ Terraform shape
otherwise matches). Spike deferral: **yes** (PRD 07's "Resolved in
spike" placeholder is intentionally operator-fillable; not a Sprint 1
gap). Ready for integrator commit: **yes-with-followups** (no blockers;
six low/medium findings listed below for integrator triage or Sprint 2
fold).

---

## Issue 1: `awsbnkctl doctor` AWS checks invisible on a fresh dev box (workspace-gated)

**Severity**: medium
**Status**: open
**Description**: `internal/doctor/doctor.go:130` gates the call to
`awsChecks(ctx, cctx)` on `cctx.Workspace != nil`. On a fresh dev box
without `awsbnkctl init` run, the workspace is nil and the three new
Sprint 1 AWS pre-flight rows (`aws credentials`, `aws sts
caller-identity`, `aws eks:DescribeCluster permission`) never render
— even with `AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test
AWS_REGION=us-east-1` exported. Verified by running:

```
$ unset AWS_*; ./bin/awsbnkctl doctor       # 8 rows, no AWS lines
$ AWS_ACCESS_KEY_ID=test ... ./bin/awsbnkctl doctor          # same 8 rows
$ AWS_ACCESS_KEY_ID=test ... ./bin/awsbnkctl --workspace test doctor   # same
```

In every case the AWS rows are absent. The tech-writer brief's
"Build dogfood" gate explicitly expected `doctor` to *"reports
AWS-shaped checks"* on a fresh dev box, and the "Dry-run dogfood" gate
expected the no-creds case to "report `aws credentials` as warning,
`aws sts caller-identity` as skipped, etc." Neither happens, because
the new check block sits behind the workspace gate.

The doctor logic before the gate is intentional (workspace-specific
region lookup lives inside `awsRegionFromContext`), but the AWS-side
checks themselves work fine on an empty `Context` — `CredentialsConfigured`
and `NewClients` both fall back cleanly to the SDK's default chain.
Sprint 1 staff Issue 3 already notes the field is read off the
IBM-shaped struct via `cctx.Workspace.IBMCloud.Region`; that fallback
masks the visibility issue rather than fixing it.

**Files affected**: `internal/doctor/doctor.go:122-138`,
`internal/doctor/aws.go:147-154`.

**Proposed fix**: Move the `awsChecks(ctx, cctx)` call outside the
`cctx.Workspace != nil` block, and let `awsRegionFromContext`
nil-check `cctx.Workspace` (it already does, mostly — but the call
site never reaches it without a workspace). One-line refactor;
preserves the existing region-from-workspace path when a workspace
exists, surfaces the warning/skipped rows when it doesn't. Without
this, a fresh-dev-box user has no signal that AWS credentials are
missing until `awsbnkctl up cluster --dry-run` errors deep in
terraform.

## Issue 2: `intel.com/sriov` vs `intel.com/intel_sriov_netdevice` naming drift between PLAN.md and PRD 07 / module / chapters — architect Issue 2 unresolved

**Severity**: medium
**Status**: open (re-filed from architect Issue 2)

**Description**: Sprint 1 architect's Issue 2 flagged that `docs/PLAN.md:145`
says VFs should appear as `intel.com/intel_sriov_netdevice` while
PRD 07 (5 places), the eks_cluster module
(`terraform/modules/eks_cluster/variables.tf:73,75`,
`sriov.tf:13,23`), the root TF (`terraform/variables.tf:81,83`), and
both chapters consistently use `intel.com/sriov`. Verified by
`grep -nE 'intel_sriov_netdevice|intel\.com/sriov' ...`:

- PLAN.md: 1 occurrence (`intel.com/intel_sriov_netdevice`)
- everything else: `intel.com/sriov` throughout

The drift remains in place after staff completed the Sprint 1
module. The architect deferred the pick-one decision to the
integrator. Either is *defensible* (upstream sriov-network-device-plugin
ships the longer form as its example default; the shorter form
matches what awsbnkctl ships and what every doc except PLAN.md
expects). What's wrong is the *cross-document disagreement* —
a reader who compares PLAN.md's spike-step to PRD 07's spike-protocol
day-2 step will be confused on which key to look for.

**Files affected**: `docs/PLAN.md:145`.

**Proposed fix**: Align PLAN.md with the implementation. The simplest
edit is to change PLAN.md line 145 from
`intel.com/intel_sriov_netdevice` to `intel.com/sriov` (matching the
module default and PRD 07). Alternative: rename the module default
to match upstream's longer form, but that touches 5+ files for what
is effectively a cosmetic convention pick. Integrator decision; the
single-line PLAN.md edit is the lighter touch.

## Issue 3: `terraform fmt -check` fails on `terraform/modules/eks_cluster/main.tf` — two alignment nits

**Severity**: low
**Status**: open

**Description**: Running `terraform fmt -check -recursive` against
`terraform/modules/eks_cluster/` exits with status 3 and flags
`main.tf`. The diff `terraform fmt -diff` shows two cosmetic
issues:

```
- "awsbnkctl.io/role"      = "sriov-data-plane"
- "awsbnkctl.io/prd"       = "07"
+ "awsbnkctl.io/role" = "sriov-data-plane"
+ "awsbnkctl.io/prd"  = "07"

- sriov       = var.enable_sriov ? "on" : "off"
+ sriov        = var.enable_sriov ? "on" : "off"
```

Cosmetic only — `terraform validate` succeeds. But a CI fmt-gate (if
the validator agent's Sprint 1 CI matrix or any future workflow runs
`terraform fmt -check`) would fail the build. Worth a one-touch
`terraform fmt` pass at integration time.

**Files affected**: `terraform/modules/eks_cluster/main.tf:157-158,188`.

**Proposed fix**: integrator runs `terraform -chdir=terraform/modules/eks_cluster fmt -recursive`
before commit. Single file, two trivial diffs.

## Issue 4: cspell.json missing entries for the new SR-IOV / kernel terminology in chapter 33 — validator Issue 4 confirmed open

**Severity**: medium
**Status**: open (re-filed from validator Issue 4)

**Description**: Validator's Sprint 1 Issue 4 worried the cspell
additions may not have landed because the validator agent 529'd
before reaching that task. Re-verified by running `cspell` against
the two new chapters. Some AWS terms (`multus`, `Multus`, `sriov`,
`SRIOV`, `c5n`, `m5n`, `IRSA`, `OIDC`, `Karpenter`, `ENA`, etc.)
*did* land. But the following terms used in `book/src/33-data-plane-decision.md`
and `book/src/02-why-roks.md` are not in cspell.json:

- `iommu`, `IOMMU` — kernel parameter name (used 5× in chapter 33,
  also in PRD 07)
- `VFIO` — kernel SR-IOV machinery (chapter 33 × 2)
- `Mellanox` — vendor name (chapter 33 × 3)
- `libfabric` — EFA's API (chapter 33)
- `Microkernel` — describes TMM (chapter 33)
- `userspace` — kernel/userspace boundary (chapter 33 × 2)
- `Virtualisation` — full name of SR-IOV (chapter 33)
- `schedulable` — used in both chapters (× 2 in 33)
- `dpubnkctl` — sibling fork name (chapter 33)
- `snetworkplumbingwg` — partial match from upstream org URL
  (`k8snetworkplumbingwg`)
- `xlarge` — c5n.4xlarge etc. (chapter 33 × 4, chapter 2 × 1)
- `Forkable` — chapter 2

`.github/workflows/spellcheck.yml` runs cspell on `book/src/**/*.md`
and `docs/**/*.md` on every push touching those paths; the next push
that touches the book will fail spellcheck. Most of these are
load-bearing technical terms that should be in cspell.json (not
ignored).

**Files affected**: `cspell.json` (words list).

**Proposed fix**: Add the 11 entries above to `cspell.json`'s `words`
array. Alphabetical insertion is the project's convention. One-line
patch; integrator-fold or Sprint 2 validator picks up.

## Issue 5: `awsbnkctl up cluster --help` does not document the `--workspace` flag (it's a global flag, but absent from the per-command help body)

**Severity**: low
**Status**: open

**Description**: The brief's Build dogfood gate expected
`./bin/awsbnkctl up cluster --help` to "document `--dry-run` +
`--workspace`". `--dry-run` shows up correctly under "Flags";
`--workspace` only shows up under "Global Flags" (correctly, since
it's a persistent flag set at the root). Cobra's default rendering
puts global flags below per-command flags, which the help output
does. So technically the gate is met (the flag is documented). But
the per-command Flags section alone doesn't surface it, and a reader
reading top-to-bottom may stop before the Global Flags section. The
relevant Long-description blob for `upClusterCmd` (cluster.go:39-46)
could mention `--workspace` explicitly so the reader doesn't miss it
on a quick scan. Filing as low because the flag *does* work; it's a
visibility nit.

**Files affected**: `internal/cli/cluster.go:39-46` (Long blob),
optionally cluster.go:50-57 (down cluster's Long blob).

**Proposed fix**: append one sentence to each Long block: "Pass
`--workspace <name>` to target a specific workspace; defaults to
`default` and synthesises an empty workspace if none exists yet
(suitable for first-run dry-run before `awsbnkctl init`)." Or punt
to Sprint 3 when `awsbnkctl init` lands and the workspace story
becomes load-bearing.

## Issue 6: `tools/docker/aws/` image is linux/amd64-only; `aws --version` fails on darwin_arm64 host runs (rosetta error)

**Severity**: low
**Status**: open

**Description**: `docker build tools/docker/aws/` succeeded cleanly
on this host (linux/amd64 + linux/arm64 manifest list emitted —
multi-arch). But `docker run --rm <img> aws --version` on darwin_arm64
host fails with `rosetta error: failed to open elf at
/lib64/ld-linux-x86-64.so.2`, because the Dockerfile downloads the
linux-x86_64-only awscli v2 zip
(`awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip` at line 60) regardless
of the build platform. `kubectl`, `helm`, and `iperf3` work fine
under emulation (single-platform single-arch download URLs hit the
linux/amd64 manifest entry). The validator's task brief asked for
`docker run --rm <img> aws --version` to report v2.x; on a linux
host this would work, on darwin_arm64 (this tech-writer's host) it
doesn't.

The Sprint 1 CI matrix runs on `ubuntu-latest` (linux/amd64) per
`.github/workflows/tools-images.yml:30-44` so the smoke succeeds in
CI. The user-facing impact is that an awsbnkctl developer on Apple
Silicon trying to test the docker backend locally hits the rosetta
error. Filing as low because the CI path works and the docker
backend is Sprint 4 anyway, but worth tracking for the next pass at
the Dockerfile.

Smoke results from this host (linux/arm64 manifest under rosetta):

| tool | version reported | status |
|---|---|---|
| `aws --version` | rosetta error | fail (host-arch limitation) |
| `kubectl version --client` | `v1.30.5` | OK (matches PRD 07 cluster_version default) |
| `helm version --short` | `v3.15.4+gfa9efb0` | OK |
| `iperf3 --version` | `iperf 3.16` | OK (3.x — matches brief) |

**Files affected**: `tools/docker/aws/Dockerfile:36,60`
(AWSCLI_VERSION + hardcoded x86_64 zip URL).

**Proposed fix**: detect `TARGETARCH` from the `--platform` build arg
and pick the matching awscli v2 zip (the bundle is published for
both `linux-x86_64` and `linux-aarch64`). The pattern is:

```dockerfile
ARG TARGETARCH
RUN case "$TARGETARCH" in \
      amd64) ARCH=x86_64 ;; \
      arm64) ARCH=aarch64 ;; \
    esac; \
    curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${ARCH}-${AWSCLI_VERSION}.zip" ...
```

The kubectl + helm download URLs already have amd64-specific paths;
those would need the same TARGETARCH-aware treatment. Pinned sha256
checksums then need both arches recorded. Probably a half-day pass
in a Sprint 2 or 3 validator fold rather than an integrator-side
patch this sprint.

---

*Total filed: 6 issues — 0 blocker, 0 high, 3 medium (doctor
visibility gate, PLAN.md naming drift, cspell missing terms), 3 low
(terraform fmt nit, up cluster help cosmetic, Dockerfile arm64
gap). PRD 07 ↔ Terraform module reconciliation confirms staff
Issue 5 (architect Issue 1 cross-ref): the 12 PRD inputs all match
the module's `variables.tf` defaults + types; the 7 PRD outputs all
wire through `outputs.tf` to either `module.eks.*` or
`null_resource.cluster_ready.id` as PRD 07 contracts. Spike deferral
verdict: as designed; PRD 07's "Resolved-in-spike" section is
expected operator output, not a Sprint 1 gap.*
