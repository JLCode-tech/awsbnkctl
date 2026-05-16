# Sprint 4 — tech writer issues

Read-only sprint-close review covering: PRD 04 wording reconciliation
(closes Sprint 3 tech-writer Issue 1, HIGH), chapters 20-23 first-time-
reader pass, test-surface dogfood (4 commands), first-run UX fix
(closes Sprint 3 tech-writer Issue 3, medium), PSA spec-check on the
iperf3 Pod, build-green dogfood, cross-link audit. **SPIKE DEFERRAL**
carries — no live AWS.

Sibling deliverables reviewed in full:
- `issues/issue_sprint4_architect.md` (4 low issues; PRD 04 §"Where the
  AWS chain lives in the tree" lands; chapters 20-23 ship in band)
- `issues/issue_sprint4_staff.md` (1 resolved-during-sprint, 3 carry-
  overs; test --dry-run on three verbs; first-run UX fix in
  `runFullLifecyclePlan`; PSA Pod spec verified; Service Quotas behind
  `AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1`)
- `issues/issue_sprint4_validator.md` (15 issues across two passes;
  `test-dryrun` CI job aligned to actual workspace schema; cspell +6;
  e2e marker refresh across two scripts + one workflow)

---

## Issue 1: chapter 22 § "Tuning knobs" image tag uses the awsbnkctl-tools-iperf3 form but `Iperf3DefaultImage` is still `networkstatic/iperf3:latest`

**Severity**: medium
**Status**: open

**Description**: Chapter 22 § "Tuning knobs in workspace config"
(lines 113-121) shows the workspace-config knob as:

```yaml
test:
  throughput:
    image: ghcr.io/JLCode-tech/awsbnkctl-tools-iperf3:v0.7.0
```

The dry-run plan output prints the **actual** server image and confirms
the drift:

```
$ ./bin/awsbnkctl test throughput --dry-run --workspace test
## test throughput — dry-run plan (workspace=test)
  …
  targets (1):
    - iperf3 server image: networkstatic/iperf3:latest
```

`internal/k8s/iperf3.go:22` pins `Iperf3DefaultImage =
"networkstatic/iperf3:latest"`. A first-time reader who follows
chapter 22 § "EKS 1.25+ Pod Security Admission compliance" (lines 53-82)
will read that the **bundled** `awsbnkctl-tools-iperf3` image declares
`USER 1000` and therefore satisfies PSA `restricted` without further
config, then read § "Tuning knobs" and see that same bundled image as
the suggested workspace config — and the next paragraph in § "PSA
compliance" warns:

> Stock images that run as root (the unbundled
> `networkstatic/iperf3:latest`) will fail PSA admission

…which is **exactly** the default the binary uses when the workspace
doesn't override it. This is the same forward-statement that the
architect's Issue 1 flags for the version pin, but extends to the image
**name** itself: chapter 22 documents the bundled image as the default
end-state; the binary's default is the unbundled stock image. Until the
`awsbnkctl-tools-iperf3` image actually publishes to GHCR (Sprint 6
hardening, per architect's Issue 1), the chapter's worked example reads
the bundled image as canonical and the dry-run plan contradicts it.

The discrepancy is harmless at the dry-run tier (no pod is created), but
on a live `--mode east-west` run against an EKS cluster with PSA
`restricted` enforced on `awsbnkctl-test`, the default
`networkstatic/iperf3:latest` will get rejected at admission with the
exact PodSecurity error chapter 22 § PSA compliance documents — a
classic "documented failure mode that the default config triggers"
trap.

**Files affected**: `internal/k8s/iperf3.go:22`
(`Iperf3DefaultImage` constant); `book/src/22-throughput-testing.md`
§ "Tuning knobs in workspace config" (lines 113-121) +
§ "PSA compliance" (line 73). Cross-cuts architect's Issue 1.

**Proposed fix**: two reasonable shapes:
  (a) Sprint 5 staff flips `Iperf3DefaultImage` to the bundled
      `ghcr.io/JLCode-tech/awsbnkctl-tools-iperf3:<v>` form **after**
      the image has been published at v0.7.0 (Sprint 6 hardening lane);
      chapter 22 stays as-is.
  (b) Until (a) lands, chapter 22 adds a one-line callout right after
      the PSA compliance § warning that says "the default image
      (`networkstatic/iperf3:latest`) **does** run as root and will fail
      PSA `restricted` admission on EKS 1.25+; set
      `test.throughput.image` to the bundled image (or any non-root
      iperf3 image) before running `--mode east-west` against an EKS
      cluster".

Either resolves the dogfooding stuck-point. (a) is the cleaner shape
once the image publishes; (b) closes the gap immediately. Defer to the
integrator's call on sprint ordering.

## Issue 2: `up --dry-run` first-run UX fix is friendly but exits non-zero — verify intended UX

**Severity**: low (verification-only)
**Status**: open (informational)

**Description**: Sprint 3 tech-writer Issue 3 asked staff to fold a
friendly init-needed message into `runFullLifecyclePlan` when the
workspace's tfvars haven't been written yet. Verified:

```
$ HOME=/tmp/empty-home AWS_ACCESS_KEY_ID=test \
    AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
    ./bin/awsbnkctl up --dry-run
Error: workspace "default" is not initialised — run `awsbnkctl init
-w default` first to capture region / VPC / subnets / FAR archive /
JWT, or pass --var-file=<path> to supply values directly
awsbnkctl: workspace "default" is not initialised — run `awsbnkctl init
-w default` first to capture region / VPC / subnets / FAR archive /
JWT, or pass --var-file=<path> to supply values directly
$ echo $?
1
```

The message is friendly, concrete, names the right `init` invocation,
mentions the escape-hatch (`--var-file=`), and short-circuits **before**
terraform boots (no `terraform plan` noise; the staff issue confirms
this lands in `cli/cluster.go::translatePlanError`). All of that closes
Sprint 3 tech-writer Issue 3.

One verification-only observation: exit code is `1`, not `0`. For a
`--dry-run` invocation, `0` would also be defensible — "this would
need init first" is a plan-tier outcome, not a failure. The current
shape (`1`) treats "not initialised" as an error condition, which has
its own logic ("CI should fail loudly if its workspace isn't set up").
Both are reasonable; flagging here only to confirm the integrator is
aware of the choice. Not a blocker.

The friendly message itself prints twice (`Error: …` then
`awsbnkctl: …`) — that's the standard cobra error path layered on
top of the binary's own stderr error wrapper. Same shape every other
error path uses, so consistent.

**Files affected**: `internal/cli/cluster.go::translatePlanError`
(staff's Sprint 4 deliverable; verified working as advertised).

**Proposed fix**: none required. If the integrator wants the `--dry-run`
path to exit `0` for "not initialised yet" instead of `1`, that's a
one-line change in `runFullLifecyclePlan`'s error handling — but the
current behavior is consistent with `terraform plan` returning `2` for
"changes pending", `1` for "init required", `0` for "no changes". I'd
keep it as-is.

## Issue 3: `awsbnkctl test --help` lists four subcommands but the brief enumerates three (connectivity, dns, throughput) + `all`

**Severity**: low (cosmetic; no user-visible impact)
**Status**: open

**Description**: Dogfood result for `awsbnkctl test --help`:

```
Available Commands:
  connectivity HTTP/HTTPS reachability against configured hosts
  dns          DNS resolution probe (single-vantage, GSLB-compare, or workspace-driven)
  list         List available test suites
  throughput   iperf3 throughput; deploys server pod automatically (v1.x)
```

Four subcommands: `connectivity`, `dns`, `list`, `throughput`. The
parent help block describes `all` as "the default if no suite is
specified" (i.e., `awsbnkctl test` with no positional arg dispatches
`all`), and `list` is the subcommand the staff issue catalogues as
"the four subcommands". Two reasonable framings:

- Today: bare `awsbnkctl test` → all suites; `awsbnkctl test list` →
  enumerate; per-suite subcommands → individual suites. There is no
  literal `awsbnkctl test all` subcommand registered; the prose framing
  in `--help` ("`all` — run all of the above (default if no suite is
  specified)") implies one exists.
- Chapter 20 § "Running connectivity inside `awsbnkctl test all`"
  (lines 106-126) explicitly invokes `awsbnkctl test all`. This works
  today because positional-arg dispatch sends bare `test all` through
  the bare-`test` path, but the conceptual model is "implicit default,
  not a named subcommand".

This is a documentation-shape issue, not a behavior issue. The reader
who runs `awsbnkctl test all --help` will get the parent `test`
help text (not a dedicated `all`-subcommand help), and the reader who
runs `awsbnkctl test list` will see the four-subcommand enumeration
that omits `all`. Both behaviours are explainable, neither is wrong.

**Files affected**: `book/src/20-connectivity-testing.md` § "Running
connectivity inside `awsbnkctl test all`" (lines 106-126);
`internal/cli/test.go::testCmd` long description.

**Proposed fix**: Sprint 5 architect (book retarget pass) adds a
one-line clarification to chapter 20 noting that `awsbnkctl test all`
is the bare-`test` invocation (no subcommand registered), or registers
an explicit `all` subcommand if the conceptual clarity is worth the
indirection. Cosmetic; no Sprint 4 blocker.

## Issue 4: chapter 23 still tags PRD 04 wording-fix to chapter 26's troubleshooting cross-link but chapter 26's "AWS LoadBalancer" sub-section isn't named verbatim

**Severity**: low
**Status**: open

**Description**: Chapter 20 § "AWS LoadBalancer shapes the suite
recognises" (line 47) cross-links:

> For diagnosing the failure mode when one *doesn't* answer, see
> [Chapter 26 § AWS LoadBalancer](./26-troubleshooting.md)…

Chapter 26's actual section headings (sampled): "Cluster + node group",
"AWS credentials + auth", "S3 supply-chain", "EKS kubeconfig",
"Orphan resources", "CI provider-cache". No literal `§ AWS
LoadBalancer` heading exists yet; the LoadBalancer failure shapes are
sprinkled across the "Cluster + node group" and "AWS credentials"
sections.

Same for chapter 21 § "Worked example" (line 176):

> Three common follow-up failure modes are catalogued in
> [Chapter 26 § DNS](./26-troubleshooting.md).

No `§ DNS` heading in chapter 26 either. Sprint 5 architect's book-
retarget pass naturally folds these in; the cross-links are forward-
statement to that work. The chapter 26 catalog grows sprint-by-sprint
per its own preamble ("expect the catalogue to grow alongside
Sprint 4's test-surface work and Sprint 6's hardening pass").

Reader experience: clicking the link lands in chapter 26 (which exists,
which has real content), but the in-page anchor `#aws-loadbalancer` or
`#dns` doesn't resolve to a specific heading — the reader scrolls to
find the relevant section. Stuck-point severity: low. A first-time
reader looking for "why didn't my NLB answer" still gets to the right
chapter; the friction is one scroll, not a dead-end.

**Files affected**: `book/src/26-troubleshooting.md` (missing sub-
sections); `book/src/20-connectivity-testing.md` line 47;
`book/src/21-dns-testing-gslb.md` line 176.

**Proposed fix**: Sprint 5 architect adds `### AWS LoadBalancer` and
`### DNS` sub-sections to chapter 26 (or whatever sub-section naming
convention the chapter settles on) and the cross-link anchors resolve
naturally. No Sprint 4 blocker; folding into Sprint 5's natural revisit
is the right shape.

---

## Per-prose-surface verdict

| Surface | Verdict |
|---|---|
| `docs/prd/04-CREDENTIALS.md` § "Where the AWS chain lives in the tree" | **Ships.** Closes Sprint 3 tech-writer Issue 1 (HIGH) end-to-end. Function names (`NewClients`, `CredentialsConfigured`, `HasEnvCredentials`, `Clients.CallerIdentity`) all verified verbatim against `internal/aws/{client,sts}.go`. `internal/cred/` correctly named as deprecated-for-back-compat-only. |
| `book/src/20-connectivity-testing.md` (~1,500 words) | **Ships.** Reads cleanly as first-time reader; NLB/ALB shape recognition is concrete; failure-mode table is diagnostic-first. Chapter 26 cross-link missing sub-anchor (Issue 4). |
| `book/src/21-dns-testing-gslb.md` (~1,800 words) | **Ships.** Three-vantage diagram + JSON schemas + worked us-west-2/us-east-1/eu-west-1 example all land. Worth its slightly-above-band word count. Chapter 26 cross-link missing sub-anchor (Issue 4). |
| `book/src/22-throughput-testing.md` (~1,500 words) | **Ships with caveat.** PSA contract documented correctly; c5n.4xlarge baselines concrete; tuning knobs schema clear. Issue 1 (bundled-image-vs-default-image drift) is the medium-severity stuck-point a live east-west reader would hit. |
| `book/src/23-e2e-test-plan.md` (~2,200 words) | **Ships.** Phase-letter system + dry-run/live-tier split + cost-and-time table all useful. Estimates pending validation (architect's Issue 3) — cosmetic. |
| `docs/PLAN.md` § Sprint 4 close (architect surface) | **Ships.** Mirrors Sprint 3 shape; integrator extends with sibling surfaces at tag-cut per the maintenance contract. |

## Dogfooding-loop stuck-points

**Total: 2** (1 medium, 1 low).

- **Medium (Issue 1)** — chapter 22's "set this in your workspace
  config" image tag (`awsbnkctl-tools-iperf3`) reads as the canonical
  default, but the binary's `Iperf3DefaultImage` is the stock
  `networkstatic/iperf3:latest` that chapter 22 § PSA compliance
  itself warns against. A live `--mode east-west` runner who skips
  setting `test.throughput.image` will hit the documented PSA
  admission rejection.
- **Low (Issue 3)** — `awsbnkctl test --help` enumerates four
  subcommands and prose says "five" implicitly (including `all`).
  Conceptual confusion only; no behavior break.

## Dogfood results (exit codes)

| Command | Exit | Notes |
|---|---|---|
| `./bin/awsbnkctl test --help` | **0** | Renders four subcommands cleanly + persistent `--dry-run` + `--insecure` flags |
| `HOME=/tmp/tw4home ./bin/awsbnkctl test connectivity --dry-run -w test` | **0** | Plan resolved; 1 target enumerated |
| `HOME=/tmp/tw4home ./bin/awsbnkctl test dns --dry-run -w test` | **0** | Plan resolved; 1 target enumerated |
| `HOME=/tmp/tw4home ./bin/awsbnkctl test throughput --dry-run -w test` | **0** | Plan resolved; mode=north-south, backend=k8s; iperf3 image visible in plan output (surfaces Issue 1) |
| `HOME=/tmp/empty-home AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 ./bin/awsbnkctl up --dry-run` | **1** | Friendly init-needed message; no terraform spew; closes Sprint 3 tech-writer Issue 3 |
| `make build` | **0** | Single `go build` line; binary at `bin/awsbnkctl` |
| `go test ./...` | **0** | All packages pass (aws, cli, config, cred, doctor, exec, k8s, remote, test, tf, refgen) |

## PSA compliance spec-check

`internal/k8s/iperf3.go::BuildIperf3Pod` (lines 95-132) verified to
include the four PSA-`restricted`-required fields:

- **Pod-level** `RunAsNonRoot: true` (line 110), `RunAsUser: 1000`
  (line 111), `SeccompProfile.Type: RuntimeDefault` (lines 112-114)
- **Container-level** `AllowPrivilegeEscalation: false` (line 122),
  `RunAsNonRoot: true` (line 123), `Capabilities.Drop: [ALL]`
  (lines 124-126)

The function's docstring (lines 71-94) explicitly cites PRD 03 §iperf3
+ § OpenShift SCC + Sprint 4 staff brief § "throughput.go PSA
compliance" as the contract source. Closes the brief's PSA gate at the
Pod surface. The client-side Job spec built by the k8s execution
backend (`internal/exec/k8s.go`) is **not** equivalently pinned —
that's staff's own Issue 3 (medium carry-over to Sprint 5), unchanged.

## Sprint 3 carry-overs closure verdict

| Carry-over | Status |
|---|---|
| **Sprint 3 tech-writer Issue 1 (HIGH)** — PRD 04 wording drift | **Closed.** Sprint 4 architect's § "Where the AWS chain lives in the tree" describes the actual two-package split verbatim; function names verified against shipped code. |
| **Sprint 3 tech-writer Issue 3 (medium)** — `up --dry-run` first-run UX gap | **Closed.** Staff's `translatePlanError` fold returns the friendly init-needed message before terraform boots; verified by dogfood (HOME=/tmp/empty-home invocation above). |
| **Sprint 3 tech-writer Issue 2 (medium)** — 302 IBM-residue hits | **Carried.** PLAN.md Sprint 4 close + architect's Sprint 4 close section both name this as Sprint 5 work; staff Issue 1 in Sprint 5 catalog has the per-file breakdown. Not a Sprint 4 regression. |

## Cross-document drift verdict

- **PRD 04 ↔ `internal/aws/`** — clean. Every function named in
  PRD 04 § "Where the AWS chain lives in the tree" exists at the
  documented path with the documented signature.
- **PRD 04 ↔ `internal/cred/resolver.go`** — clean. PRD documents the
  package as deprecated; code matches (no production caller threads
  non-empty value; Sprint 5 deletion target).
- **PRD 05 ↔ chapter 23** — clean. Chapter 23 reframes PRD 05's phase
  letters for the AWS retarget; cost-and-time table matches PRD 05's
  budget orders of magnitude.
- **Chapter 22 ↔ `internal/k8s/iperf3.go`** — drift. Issue 1 — chapter
  documents the bundled image as canonical; code defaults to the stock
  `networkstatic/iperf3:latest` that the same chapter's PSA section
  flags as failure-mode-triggering.
- **Chapters 20-23 internal cross-links** — resolve cleanly (all linked
  chapters exist; sampled 12, 17, 19, 26, 33). Chapter 26 sub-section
  anchors missing (Issue 4) — folds into Sprint 5 book retarget.
- **CHANGELOG / README** — Sprint 4 entry not yet appended to either
  (integrator's responsibility at tag-cut per the maintenance contract
  at PLAN.md bottom; consistent with prior sprint pattern).

## Gate-criteria audit (PLAN.md Sprint 4 end-of-sprint gate)

| Gate criterion | Status |
|---|---|
| `awsbnkctl test {dns,connectivity,throughput}` runs against a workspace config (mocked end-to-end) | **Met.** All three dry-run subcommands exit 0; validator's `test-dryrun` CI job exercises the same surface; staff plumbed workspace region + cluster outputs into the plan path. |
| Live AWS validation in operator-run spike | **TBD-by-spike-operator.** SPIKE DEFERRAL carries; PRD 07 § spike protocol still gates v0.2. |
| `awsbnkctl doctor` covers all six AWS rows | **Met from Sprint 3.** Sprint 4 staff added the optional Service Quotas check (`AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1`); off by default. |
| AWS-flavoured E2E phases pass against mocked fixtures | **Met.** Validator's e2e marker refresh + DRY_RUN smoke green on both scripts. |
| `internal/k8s/iperf3.go` Job spec includes runAsNonRoot, seccompProfile, capabilities.drop ALL | **Met.** Verified verbatim. |
| Build green (`make build`, `go test ./...`) | **Met.** Both exit 0. |

## Issues filed: 4

- 0 blocker
- 0 high
- 1 medium (Issue 1 — image-default ↔ chapter-22-canonical drift)
- 3 low (Issues 2, 3, 4 — exit-code verification, `test all`
  cosmetic, cross-link sub-anchors)
- 0 roadmap

## Release-readiness verdict

**Ready for integrator.** Sprint 4 does not cut a tag per PLAN.md
("The integrator commits. Sprint 4 does not cut a tag."). All gate
criteria are met at the **dry-run / mocked-fixture tier**; live-tier
gate carries on the operator-run spike per the SPIKE DEFERRAL contract.
Sprint 3 carry-overs (tech-writer Issues 1 and 3) are closed cleanly.
The single medium-severity finding (Issue 1) is a docs/code drift the
integrator can either patch at commit time (chapter 22 callout) or fold
into Sprint 5 (`Iperf3DefaultImage` flip after image publish). No
blockers.
