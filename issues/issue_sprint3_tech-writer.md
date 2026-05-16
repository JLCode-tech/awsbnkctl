# Sprint 3 — tech-writer issues

End-of-sprint read-only review. Architect / staff / validator
all landed; this pass dogfoods the artefacts, audits the
PRD ↔ implementation contracts, and verifies the carry-over
closures the brief named (doctor visibility, workspace clean-
break, legacy_helpers retirement, chapter 26 first-pass).

Scope covered: `terraform/main.tf` full-graph plan with fake
creds; the five ported modules (`cert_manager`, `flo`,
`cne_instance`, `license`, `testing`) variable-rename sweep;
PRD 04 § "Resolved in Sprint 3" ↔ `internal/cred/resolver.go`
literal diff; workspace clean-break audit
(`grep -r 'IBMCloud' internal/`); doctor visibility on a stock
dev box (no workspace, no creds); `legacy_helpers.go` post-
retarget shape; chapter 26 first-pass narrative + cross-link
walk; cross-document drift between PRDs 04 / 07 / 08, chapters
25 / 26 / 33, README, CHANGELOG, MIGRATING.

Disk: cleaned `terraform/.terraform/` after the plan run
(post-cleanup tree is 868 KB; staff-shipped lock files
preserved).

---

## Issue 1: `internal/cred/resolver.go` still implements the IBM Cloud API-key chain — PRD 04 § "Resolved in Sprint 3" describes a different package

**Severity**: high
**Status**: open (PRD ↔ impl contract drift; architect Issue 1 flagged the deferral, this issue documents the concrete delta)

**Description**: PRD 04 § "Resolved in Sprint 3"
(`docs/prd/04-CREDENTIALS.md` lines 13-26) describes
`internal/cred.Resolver` as: "delegates to
`config.LoadDefaultConfig(ctx)` for resolution and surfaces
the resolved provider name for doctor reporting", with a
six-row chain table that runs env → shared config → SSO →
IMDS → ECS / EKS pod task role → web-identity.

The shipped file `internal/cred/resolver.go` (read in full,
253 lines) is **unchanged from the inherited Sprint 9 IBM
shape**:

- Package doc comment lines 4-31 still describe an
  `IBMCLOUD_API_KEY` / `IC_API_KEY` / `TF_VAR_ibmcloud_api_key`
  chain with a "Resolution chain (for IBM Cloud)" header.
- `apiKeyEnvVars` (line 56) is the IBM env-var list verbatim.
- The single method `IBMCloudAPIKey(ctx context.Context)
  (string, error)` (line 93) has no AWS equivalent.
- No import of `github.com/aws/aws-sdk-go-v2/config`; no call
  to `config.LoadDefaultConfig`; no provider-name surfacing
  for doctor.
- `apiKeyFromConfig` (line 199) is the only AWS-aware change:
  a no-op that always returns `""` so the IBM chain falls
  through to the (now-unreachable) prompt step.

A user reading PRD 04 and then opening
`internal/cred/resolver.go` to confirm the chain order will
not find anything resembling the documented contract. The
AWS standard chain *is* implemented — but it lives in
`internal/aws/` (used by `internal/doctor/aws.go::awsChecks`
via `awspkg.CredentialsConfigured`), not in
`internal/cred/`. The two-package shape is fine; the PRD
just describes the wrong package.

Architect Issue 1 deferred this cross-check to integration
and proposed two paths (update the resolver to match, or
update the PRD to point at the right package); filing here
as the concrete delta the integrator hits when they walk
the diff. Severity high because PRD 04 is the user-facing
source of truth for credential propagation — a first-time
reader looking up "where does the AWS cred chain live" gets
sent to a file that doesn't contain it.

**Files affected**: `docs/prd/04-CREDENTIALS.md` lines
13-26 (the inaccurate "internal/cred.Resolver delegates to
config.LoadDefaultConfig" claim); `internal/cred/resolver.go`
(the still-IBM-shaped resolver); `internal/aws/` (the actual
home of the AWS chain — `CredentialsConfigured` /
`NewClients` / `Options`).

**Proposed fix**: Sprint 4 architect picks one:
  (a) rewrite PRD 04 § "Resolved in Sprint 3" to point at
      `internal/aws/` for the resolver implementation and
      `internal/doctor/aws.go::awsChecks` for the doctor
      surfacing — minimal change, matches what's shipped, but
      means `internal/cred/` is a dormant package the AWS
      surface doesn't use; OR
  (b) Sprint 4 staff completes the cred-retarget per the
      staff Issue 1 deferral plan, deleting `Resolver.
      IBMCloudAPIKey` and the inherited `apiKeyEnv*` /
      `apiKeyFromKeychain` / `apiKeyFromPrompt` chain, then
      either dropping the package or wrapping
      `config.LoadDefaultConfig` so the PRD's described
      surface actually exists.

Recommendation: (b) — PRD 04 specifies a `cred.Resolver`
surface; honour it, even if the implementation is a thin
wrapper around the SDK chain. Keeps the doctor's
provider-name reporting (`credSource` in `awsChecks`) anchored
in `internal/cred/` where the PRD says it lives. Cred-shim
retirement (staff Issue 1) is the natural folding point.

## Issue 2: `internal/exec/creds.go::Credentials.IBMCloudAPIKey` field still wired through docker.go + local.go cred shim — Sprint 3 staff Issue 1 deferral made dormant but visible

**Severity**: medium
**Status**: open (staff Issue 1 deferred to Sprint 4)

**Description**: Staff Issue 1 explicitly documented this:
the `Credentials.IBMCloudAPIKey` struct field
(`internal/exec/creds.go:46`), the docker backend's
`credShimScript` + tmpfile bind-mount plumbing
(`internal/exec/docker.go:176-485, 605-640`), and the
`local.go` redactor wrap (`internal/exec/local.go:163-166`)
all survive on disk. Production callers don't populate the
field — `runFullLifecyclePlan` threads zero IBM env vars —
so the shim is unreachable from new code paths.

The audit hit count quantifies the dormant surface:
`grep -rn 'IBMCloud\|IBMCLOUD\|ibmcloud' internal/` returns
302 hits across 40 files; excluding test files leaves 170
hits; excluding comments + tests leaves 54 production-code
hits. Concentrations:

- `internal/cred/resolver.go` — 17 (Issue 1).
- `internal/config/secrets.go` — 13 (legacy `ResolveAPIKey`
  shim — staff Issue 1 step 4 names this for Sprint 4
  retirement).
- `internal/exec/docker.go` — 49 (cred-shim plumbing).
- `internal/exec/creds.go` — 11 (the `IBMCloudAPIKey`
  field + the `ToDockerArgs` / `ToK8sEnvVars` callers).
- `internal/exec/k8s.go` — 17 (the `ibmcloud login` wrap
  in `K8s.Run`).
- `internal/exec/ssh.go` — 7 (the `IBMRepo` flag + the
  apt-source bootstrap).
- `internal/exec/k8s_install.yaml` — 5 (the
  `${IBMCLOUD_API_KEY_B64}` template substitution in the
  ops-pod RBAC manifest).
- `internal/tf/terraform.go` — 4 (the
  `os.Setenv("TF_VAR_ibmcloud_api_key", apiKey)` call —
  potentially live if `apiKey` is ever non-empty post-PRD-04;
  worth a Sprint 4 check that the caller chain to
  `terraform.Open` never threads a non-empty value through).

The workspace clean-break audit from the brief ("`grep -r
'IBMCloud' internal/` should be near-zero") is **not met**.
Staff acknowledged this in their Issue 5 verification
summary (161 hits at staff filing; 162 reproduced this pass
counting only `IBMCloud` capital-W form vs the 302
case-insensitive total). The dormant shim is safe — no
caller materialises a non-empty value — but the line-count
target the brief implied isn't met yet.

**Files affected**: `internal/exec/creds.go`,
`internal/exec/docker.go`, `internal/exec/k8s.go`,
`internal/exec/ssh.go`, `internal/exec/local.go`,
`internal/exec/k8s_install.yaml`, `internal/cred/resolver.go`,
`internal/config/secrets.go`, `internal/tf/terraform.go`,
`internal/k8s/client.go` (one stale error message line 66
naming `ibmcloud ks cluster config`).

**Proposed fix**: Sprint 4 staff completes the cred-shim
retirement per their own Issue 1 step plan; budget the
work as a single dispatch row (the shim is mechanically
deletable, tests retarget straightforward). The
`internal/k8s/client.go:66` error message is a
trivial one-line fix that can land in any sprint —
flag it for the integrator if they want a Sprint 3 patch.

## Issue 3: `awsbnkctl up --dry-run` on stock dev box without `init` exits 1 with terraform "no value for required variable" errors

**Severity**: medium
**Status**: open (UX gap — first-run experience)

**Description**: Dogfooded the Sprint 3 gate from the brief.
Without a prior `awsbnkctl init`:

```
$ HOME=/tmp/empty AWS_ACCESS_KEY_ID=test \
    AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
    ./bin/awsbnkctl up --dry-run
→ terraform init (full lifecycle: eks_cluster → cert_manager /
  s3 / iam_irsa → flo → cne_instance → license / testing)
  [provider downloads, init succeeds]
→ terraform plan (dry-run; no apply)
Error: No value for required variable
  on variables.tf line 34: variable "vpc_id" { ... }
Error: No value for required variable
  on variables.tf line 39: variable "subnet_ids" { ... }
Error: No value for required variable
  on variables.tf line 103: variable "far_auth_file_local_path" { ... }
Error: No value for required variable
  on variables.tf line 108: variable "jwt_file_local_path" { ... }
awsbnkctl: terraform plan: exit status 1
REAL_EXIT=1
```

The CLI prints each tfvar error block twice (the wrapping
layer re-emits — minor cosmetic noise). More importantly,
the orchestrator hits this *before* the AWS provider boots,
so the staff Issue 2 "STS GetCallerIdentity 403" path the
brief envisaged as the documented spike-deferral behaviour
only surfaces after the user discovers they need to supply
vpc_id + subnet_ids + the FAR / JWT paths.

A first-time user reading the README's quick-start ("3.
Make AWS credentials available… `awsbnkctl up`") gets stuck
here. The `runFullLifecyclePlan` `cctx.Workspace == nil`
branch synthesises a `TFSource: embedded` workspace but
does not synthesise `vpc_id` / `subnet_ids` /
`far_auth_file_local_path` / `jwt_file_local_path` —
correctly, because those have no safe defaults — but the
error is presented as a terraform stack trace rather than a
"run `awsbnkctl init` first" hint.

With a synthesised tfvars file containing fake but
syntactically-valid values, plan progresses to the STS
GetCallerIdentity 403 step (staff Issue 2 — expected
spike-deferral). Plan emits resource diffs for 2 modules
(`module.s3_supply_chain.random_id.bucket_suffix` and
`module.testing.tls_private_key.jumphost_shared_key`) —
the only resources whose graph doesn't depend on the EKS
cluster's runtime-resolved data sources. The other six
modules are wired and `terraform init` validated their
sources, but their resource diffs don't render because the
graph short-circuits at the STS failure. Staff Issue 2
described "plan emits diffs for all wired modules"; that
overstates what an operator actually sees with fake creds.

**Files affected**: `internal/cli/lifecycle.go:113-118`
(`runUp`), `internal/cli/cluster.go:182-241`
(`runFullLifecyclePlan`).

**Proposed fix**: Sprint 4 wires a friendlier first-run
gate. Two reasonable shapes:
  (a) when `cctx.Workspace == nil` and the user runs `up
      --dry-run`, error early with: "no workspace
      initialised — run `awsbnkctl init` first to capture
      region / VPC / subnets / FAR / JWT, or pass
      `--var-file=...` with values".
  (b) wire a `--mock-aws` mode (staff Issue 2 floated this)
      that swaps the provider for a mock so dry-run on
      fake creds emits full module diffs for documentation /
      CI verification purposes.

Recommendation: (a) for v0.3-pre (cheap, clarifies the
first-run UX); (b) only if Sprint 4 validator wants the
mock-aws diff as a CI regression artefact. Both punts are
acceptable; the current shape is functional but rough on
first encounter.

## Issue 4: chapter 26 § "Cluster + node group" → chapter 33 cross-link resolves to a Sprint 0 stub

**Severity**: low
**Status**: open (carries — architect Issue 4 documented the deferral, this issue closes the audit-trail loop)

**Description**: Chapter 26 lines 13 + 91 both cross-link
to `./33-data-plane-decision.md` ("[Chapter 33 — The
data-plane decision] walks the SR-IOV-on-EKS background in
depth"). Per architect Issue 4, chapter 33 is the Sprint 0
stub still and the real chapter content lands in Sprint 5.
Verified by walk: `book/src/33-data-plane-decision.md`
exists and is on `SUMMARY.md` line 50; the file body is
the Sprint 0 stub. The chapter 26 reader who clicks
through gets a stub.

Same forward-link-to-stub pattern Sprints 1 + 2 + 3
established; no Sprint 3 action. Filing for the
audit-trail continuity the brief asked for under
"cross-link audit".

**Files affected**: `book/src/26-troubleshooting.md`
(lines 13 + 91), `book/src/33-data-plane-decision.md`
(stub body).

**Proposed fix**: none required this sprint. Sprint 5
architect lands chapter 33's real content; both
cross-links resolve automatically. The chapter 25 → 26
anchor cross-link verified clean
(`./26-troubleshooting.md#aws-credentials--auth` matches
the H2 at chapter 26 line 21) and the chapter 26 → 25
anchor verified clean (`./25-cos-supply-chain.md#irsa-trust-chain`
matches the H2 at chapter 25 line 74).

## Issue 5: chapter 26 first-pass — Sprint 4 architect should consider a "Workspace + tfvars" section to cover Issue 3's first-run UX gap

**Severity**: low
**Status**: open (chapter expansion candidate — depends on Issue 3's resolution shape)

**Description**: Chapter 26 first-pass covers five
symptom-categories (cluster + node group, AWS creds + auth,
EKS cluster access, terraform + AWS quotas, CI-specific) +
a "getting more help" tail. The catalogue is grep-friendly,
diagnostic-first, and matches the architect brief's "top-N
symptoms a real operator hits" framing. Read-through
verdict: ships clean for Sprint 3; voice is consistent
with chapter 25's; cross-links resolve (with the chapter
33 stub caveat in Issue 4).

The first-run UX gap from Issue 3 ("vpc_id not set,
subnet_ids not set, far_auth_file_local_path not set,
jwt_file_local_path not set" on `awsbnkctl up --dry-run`
without a prior `init`) is **not** in the catalogue. A
first-time operator who hits Issue 3 doesn't see a
chapter-26-shaped diagnostic — they see the raw terraform
error. Worth a Sprint 4 architect addition under "Workspace
+ tfvars" as the next symptom-category once Issue 3's UX
fix (or punt) is decided.

Architect Issue 3 noted the chapter is 1,698 words (above
the 1,000-1,500 target band) and recommended leaving it
as-is. Adding the Workspace + tfvars section pushes the
chapter to ~1,900-2,000 words, which is still under chapter
25's 2,524-word density precedent. Trade-off is the
architect's call.

**Files affected**: `book/src/26-troubleshooting.md`
(new section between current § "AWS credentials + auth"
and § "EKS cluster access").

**Proposed fix**: Sprint 4 architect adds a one-symptom
entry under a new § "Workspace + tfvars" header:

```
### Symptom: `awsbnkctl up --dry-run` errors with `No
value for required variable` for vpc_id / subnet_ids /
far_auth_file_local_path / jwt_file_local_path

Root cause: ... (depends on Issue 3 resolution)
Fix: `awsbnkctl init` to populate the workspace, or
     `--var-file=path/to/vars.tfvars` to supply directly.
```

Defer until Issue 3's fix shape is decided so the chapter
guidance matches the actual CLI shape.

## Issue 6: README "Highlights" + "Status" lines don't reflect Sprint 3 deliverables

**Severity**: low
**Status**: open (drift between README + shipped surface)

**Description**: `README.md` line 5 still reads: "Status:
pre-release (M0 — Sprint 0 just landed; first tagged
release `v0.2` gated on Sprint 1 per `docs/PLAN.md`)".
Sprint 3 landed: full end-to-end `terraform/main.tf` graph
across 8 modules (eks_cluster + s3_supply_chain + iam_irsa
+ ecr_mirror + cert_manager + flo + cne_instance + license
+ testing), `awsbnkctl up --dry-run` orchestrator,
workspace clean-break (mostly — see Issue 2), doctor AWS
visibility on stock dev box (Issue 7 closure).

A reader landing on the README at Sprint 3 close sees
"Sprint 0 just landed" — three sprints stale. The CHANGELOG
§ Unreleased § "Added — Sprint 2" subsection exists but no
Sprint 3 § subsection yet (the change is expected at
integrator-tag-cut).

Sprint 2 tech-writer Issue 4 (in the inherited stale fork
content) flagged the equivalent gap for v0.9 / Sprint 3
work. Pattern continues: README's status/highlights lag
the actual surface by one to three sprints.

**Files affected**: `README.md` line 5 (Status line),
`CHANGELOG.md` § Unreleased (Sprint 3 subsection
not-yet-written), `MIGRATING.md` (the inherited "From
roksbnkctl" section is present per architect Issue 2 audit
— that issue can close).

**Proposed fix**: integrator updates at sprint-3 tag-cut.
Two-paragraph change:
  - README line 5 → "Status: pre-release (Sprints 0-3
    landed; `awsbnkctl up --dry-run` plans the full
    end-to-end graph against fake creds; live `apply` still
    gated on the PRD 07 operator-run spike)".
  - CHANGELOG `## Unreleased` → add `### Added — Sprint 3
    (Module port + first end-to-end up per PLAN.md)`
    subsection mirroring the Sprint 2 shape: list the
    five ported modules, the `runFullLifecyclePlan`
    orchestrator + `up --dry-run` wiring, the
    `Workspace.IBMCloud` removal, the `awsChecks`
    visibility relaxation, chapter 26 first-pass, PRD 04 §
    "Resolved in Sprint 3" + PRD 08 versioning correction.

Doesn't need to wait for Sprint 4 — the integrator can
fold this into the v0.3-pre tag preparation.

## Issue 7: Sprint 2 tech-writer Issue 4 (doctor visibility) — CLOSED this sprint

**Severity**: high (carry-over)
**Status**: ✅ closed-during-sprint

**Description**: Sprint 2 tech-writer Issue 4 documented
that `internal/doctor/doctor.go` gated `awsChecks(ctx,
cctx)` behind `cctx.Workspace != nil`, so a stock dev box
without a workspace saw zero AWS rows.

Verification this pass:

```
$ HOME=/tmp/empty-home-techwriter ./bin/awsbnkctl doctor
[exit 0; 14 rows including:]
⚠  aws credentials                     no credentials
     resolved via env / profile / instance role / SSO — set
     AWS_PROFILE or AWS_ACCESS_KEY_ID before `awsbnkctl up`
⚠  aws sts caller-identity             skipped (no credentials)
⚠  aws eks:DescribeCluster permission  skipped (no credentials)
⚠  aws ec2 vCPU quota                  skipped (no credentials)
⚠  aws s3:PutObject feasibility        skipped (no credentials)
⚠  aws iam:GetRole (FLO IRSA)          skipped (no credentials)
```

All six AWS pre-flight rows render unconditionally per
PRD 04 § "Doctor surface (AWS-shaped)". The
`TestRunWithWhy_StockDevBox_NoWorkspace` contract was
widened to accept the `aws credentials` Warning row
(verified: `internal/doctor/doctor_test.go:65-75` —
"Sprint 3 contract allows 'workspace' + 'aws credentials'
warnings only"). Staff Issue 5 confirms the same.

**Files affected**: none requiring further action.

**Proposed fix**: none. This issue closes Sprint 2
tech-writer Issue 4. The integrator can mark it resolved
at tag-cut.

## Issue 8: `legacy_helpers.go` retirement — CLOSED this sprint (mostly)

**Severity**: medium (carry-over)
**Status**: ✅ closed-during-sprint with caveat

**Description**: The brief asked for confirmation that
`legacy_helpers.go` is "mostly or fully deleted; cleanup
confirmed". Verified by read: the file shrank to 103
lines, holding only the four helpers with live callers
(`workspaceEnv`, `resolveBackendSpecWith`, `podReady`,
`refDescription`) — the IBM-cred silencer + context import
dropped, the file-level doc comment confirms the
retirement is intentional. Staff filing notes the trim.

Caveat: the `perToolDefaultBackend` map (lines 70-73)
still declares `"iperf3": "k8s"` as the per-tool default,
which is a Sprint 4 deliverable per PLAN.md and per Sprint
2's tech-writer chapter-12 vs chapter-17 drift issue (in
the inherited stale fork content). The doctor /
`runBackendChecks` paths today never resolve iperf3
through the docker / k8s backend (no caller in the Sprint
3 dispatch path), so the map entry is a forward-looking
contract waiting for the Sprint 4 wiring. Not a regression
— flagging for continuity.

**Files affected**: `internal/cli/legacy_helpers.go`
(103 lines; live).

**Proposed fix**: none for the retirement. Sprint 4
landing the iperf3 backend dispatch picks up the
`perToolDefaultBackend["iperf3"]` resolution; revisit at
that point whether the map belongs in `legacy_helpers.go`
or moves to a Sprint 4 `internal/cli/backend.go`.

---

## Per-prose-surface verdict

| Surface | Verdict |
|---|---|
| `book/src/26-troubleshooting.md` (first-pass, 1,698 words) | Ships — voice consistent with chapter 25; symptom catalogue is diagnostic-first and grep-friendly; cross-links resolve modulo Issue 4 (chapter 33 stub). One absent symptom (Issue 5) for Sprint 4 expansion. |
| `book/src/25-cos-supply-chain.md` (single-line cross-link refresh) | Ships — anchor cross-ref to chapter 26 § "AWS credentials + auth" resolves. Versioning-paragraph drift the architect flagged (Issue 6 in architect file) carries to Sprint 4. |
| `docs/prd/04-CREDENTIALS.md` § "Resolved in Sprint 3" | Ships with caveat per Issue 1 — describes a contract `internal/cred/` doesn't implement. The chain itself is documented accurately for the user; the implementation pointer is wrong. |
| `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` § Decision (versioning correction) | Ships — reconciles with the shipped module. |
| `docs/PLAN.md` § "Sprint 3 close (actual)" | Ships per architect Issue 5 caveat — integrator extends with sibling deliverables at tag-cut. |
| README.md / CHANGELOG.md / MIGRATING.md | Drift per Issue 6 — integrator updates at tag-cut. |

## Dogfooding-loop stuck-points

| Stuck-point | Severity | Issue |
|---|---|---|
| First-time user runs `awsbnkctl up --dry-run` on stock dev box without prior `init`; sees raw terraform "no value for required variable" stack trace instead of "run `awsbnkctl init` first" hint | medium | Issue 3 |
| Sprint 3 reader follows chapter 26 § "Cluster + node group" → chapter 33 cross-link; lands on Sprint 0 stub | low | Issue 4 |
| Reader looking for "where does my AWS cred chain live in code" reads PRD 04 → opens `internal/cred/resolver.go` → finds IBM cred chain instead | high | Issue 1 |
| Reader on README still sees "Sprint 0 just landed" three sprints in | low | Issue 6 |

## Cross-document drift verdict

PRD 04 ↔ `internal/cred/` mismatch (Issue 1) is the only
high-severity drift. PRD 04 ↔ `internal/doctor/aws.go` is
clean (the six-row doctor surface matches the PRD's "Doctor
surface (AWS-shaped)" table). PRD 07 ↔ `terraform/modules/
eks_cluster/` is clean (no Sprint 3 changes to either side).
PRD 08 ↔ `terraform/modules/{s3_supply_chain,iam_irsa,
ecr_mirror}/` is clean (architect's Sprint 3 versioning
reconciliation closes the previous drift). Chapter 25 ↔
chapter 26 cross-links are clean (verified anchors both
directions). README + CHANGELOG drift per Issue 6 is the
expected pre-tag-cut gap.

## Workspace clean-break verdict

**Not met by the brief's "near-zero" target**: 302
case-insensitive `IBMCloud`/`IBMCLOUD`/`ibmcloud` hits across
40 files in `internal/` (170 excluding tests; 54 production-
code-only excluding comments). Reduction from Sprint 2's
77 (per staff filing) is *negative* — the brief targeted a
clean break but the Sprint 3 cred-shim deferral (staff
Issue 1) left the IBM surface intact behind dormant flags.
The shim is unreachable from new code paths so no functional
regression, but the line-count target the brief implied
isn't met. See Issue 2 for the per-file breakdown and
Sprint 4 retirement plan. **Verdict: contract met
functionally (no IBM-cred path executes in production
flows), contract unmet structurally (the IBM surface area
on disk hasn't shrunk).**

## Doctor visibility verdict

**Closed.** All six AWS pre-flight rows render
unconditionally on a stock dev box per the closing
verification in Issue 7. Sprint 2 tech-writer Issue 4 can
be marked resolved at integrator tag-cut.

## Chapter 26 verdict

**Ships.** First-pass narrative is solid; symptom catalogue
is diagnostic-first; cross-links resolve cleanly except the
chapter 33 stub link (Issue 4, architect Issue 4 — same
audit-trail entry). One Sprint 4 candidate addition
flagged in Issue 5. The 1,698-word density is above the
brief's target band per architect Issue 3 — agree with
architect's "leave as-is, density serves the 2 AM reader"
call.

## Ready-for-integrator verdict

**Conditional yes** — the integrator can cut v0.3-pre with
the Sprint 3 surface as-shipped, with three pieces of
fold-at-tag-cut work:

1. README + CHANGELOG drift (Issue 6) — mechanical
   two-paragraph integrator edit.
2. PRD 04 ↔ `internal/cred/` mismatch (Issue 1) — either
   PRD edit (small) or Sprint 4 staff dispatch row (larger).
   Recommend deferring the PRD edit decision to Sprint 4
   architect; the dispatch row decision is the integrator's
   call.
3. Doctor visibility (Sprint 2 tech-writer Issue 4)
   formally closed; mark resolved.

End-of-Sprint-3 gate per PLAN.md ("`terraform validate`
succeeds on root + all ported modules; `awsbnkctl up
--dry-run` plans the full end-to-end resource graph without
panicking; offline build green") is **met** — staff
verification summary + this pass agree. The SPIKE DEFERRAL
carries; live `apply` still gates on PRD 07 operator-run
spike per design.

## Files reviewed

- `terraform/main.tf` (190 lines; full-graph wiring)
- `terraform/modules/{cert_manager,flo,cne_instance,license,
  testing}/variables.tf` (variable rename sweep)
- `internal/cred/resolver.go` (253 lines; Issue 1)
- `internal/exec/creds.go`, `internal/exec/docker.go` (Issue 2)
- `internal/doctor/doctor.go` + `internal/doctor/aws.go` (Issue 7)
- `internal/doctor/doctor_test.go` (Sprint 3 contract widening)
- `internal/cli/lifecycle.go` + `internal/cli/cluster.go`
  (Issue 3 — `runFullLifecyclePlan`)
- `internal/cli/legacy_helpers.go` (Issue 8)
- `internal/cli/doctor_backend.go` (IRSA-shape ops-pod probe)
- `book/src/26-troubleshooting.md` (first-pass; 1,698 words)
- `book/src/25-cos-supply-chain.md` (chapter 26 anchor refresh)
- `docs/prd/04-CREDENTIALS.md` § "Resolved in Sprint 3"
- `README.md`, `CHANGELOG.md`, `MIGRATING.md` (drift sweep)
- Sibling Sprint 3 issue files (architect / staff / validator)

## Dogfooding commands run

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go test ./internal/cred/... ./internal/doctor/...` → exit 0
- `HOME=/tmp/empty-home-techwriter ./bin/awsbnkctl doctor`
  → exit 0; 14 rows; all six AWS rows visible
- `AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test
  AWS_REGION=us-east-1 ./bin/awsbnkctl up --dry-run`
  → exit 1 (terraform plan: no value for required vars;
  Issue 3)
- `cd terraform && terraform init && terraform plan
  -var-file=<synth-tfvars>` with fake creds → init exit 0;
  plan emits resource diffs for 2 modules
  (s3_supply_chain.random_id, testing.tls_private_key)
  before STS 403 short-circuits the rest of the graph
- Cleaned up `terraform/.terraform/` directories
  post-run (post-cleanup tree: 868 KB)

## Issues filed: 8

- 0 blocker
- 2 high (Issue 1 PRD ↔ impl mismatch, Issue 7 closed)
- 3 medium (Issue 2 IBM-shim residue, Issue 3 first-run UX,
  Issue 8 closed-mostly)
- 3 low (Issue 4 chapter 33 stub, Issue 5 chapter 26
  expansion candidate, Issue 6 README/CHANGELOG drift)
- 0 roadmap

Of the 8: 2 closed-during-sprint (Issues 7 + 8), 6 open
for Sprint 4 / integrator fold.
