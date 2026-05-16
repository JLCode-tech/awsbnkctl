# Sprint 2 — tech writer issues

End-of-sprint read-only review. Covered: PRD 08 ↔ Terraform module reconciliation (`s3_supply_chain`, `iam_irsa`, `ecr_mirror`); chapter 25 first-time-reader pass; `make build` + `init --help` + `init --dry-run` + `doctor` + `terraform validate` dogfood loop; workspace retarget audit (`grep -r 'IBMCloud' internal/`); cross-link audit (PRD 08 ↔ chapter 25 ↔ README ↔ MIGRATING ↔ CHANGELOG); Sprint 1 carry-over closure verification. Sibling Sprint 2 issue files cross-referenced: architect (9 issues; 1 medium / 7 low / 1 resolved), staff (7 issues; 1 low / 5 low+roadmap / 1 resolved), validator (10 issues; 5 low / 5 roadmap).

Build dogfood results — all green: `make build` exit 0; `terraform validate` exit 0 on each of `s3_supply_chain`, `iam_irsa`, `ecr_mirror`; `init --dry-run` exit 0; `init --help` exit 0; `doctor` exit 0 with the five new AWS rows (credentials / sts caller-identity / eks:DescribeCluster / ec2 vCPU / s3:PutObject / iam:GetRole FLO IRSA) visible **only when a workspace is initialised**.

`docker buildx` available on this host but the actual multi-arch image build was not re-run — validator already executed `docker buildx build --platform linux/amd64,linux/arm64 tools/docker/aws/` end-to-end (validator Issue 3, exit 0) and the Dockerfile read-through cross-checks clean against PRD 08's tools-image references. No need to re-burn 15 minutes.

PRD 08 ↔ implementation alignment: **yes-with-deltas** (six variable-shape adds beyond the PRD's tables, plus two PRD-side doc claims that don't match the landed Terraform — see Issues 1, 2, 3 below). Workspace retarget: **back-compat alias chosen** (matches staff Sprint 2 Issue 2); `grep -r 'IBMCloud' internal/` returns 77 hits, well above the brief's "<=1 for alias" or "0 for clean break" expectation — see Issue 5. Sprint 1 carry-overs: **tech-writer Issue 1 NOT closed** — see Issue 4 (blocker for the integrator's release-gate audit). Staff Issue 2 (vCPU quota) **partially closed** — row landed, live quota deferred (matches staff Sprint 2 Issue 5).

---

## Issue 1: PRD 08 ↔ `iam_irsa` module inputs — PRD omits `cluster_name` + `role_name_override` + `tags`; staff added all three

**Severity**: low
**Status**: open

**Description**: PRD 08 § "Terraform module: `iam_irsa`" Inputs table lists 7 variables: `region`, `oidc_provider_arn`, `cluster_oidc_issuer_url`, `flo_namespace`, `flo_service_account_name`, `s3_bucket_arn`, `kms_key_arn`. Staff's landed `terraform/modules/iam_irsa/variables.tf` declares 9: the PRD's 7 plus `cluster_name` (required, threaded into the role name via `awsbnkctl-<cluster>-flo-supply-chain-reader`) and two optional adds (`role_name_override` + `tags`). The `cluster_name` add is load-bearing — without it the role name pattern PRD 08 implies (and that `internal/aws/iam.go` `IRSARoleNameForCluster` hardcodes) can't be rendered, so it's the right call. PRD 08's Inputs table is missing it.

The same shape holds for `s3_supply_chain`: PRD lists 6 inputs (`region`, `workspace_name`, `kms_key_arn`, `far_auth_file_local_path`, `jwt_file_local_path`, `bucket_name_override`); module declares 9 (the 6 plus `far_auth_object_key`, `jwt_object_key`, `tags`). The object-key overrides are sensible (operators in regulated environments name their objects per scheme); PRD silent.

**Files affected**: `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` § "Terraform module: `iam_irsa`" Inputs table (lines ~157-167); same § "Terraform module: `s3_supply_chain`" Inputs table (lines ~134-145); `terraform/modules/iam_irsa/variables.tf`; `terraform/modules/s3_supply_chain/variables.tf`.

**Proposed fix**: Sprint 3 architect updates PRD 08's two Inputs tables to mention the landed extras. The PRD is the user-visible contract; module shape is what the operator actually edits in `terraform.tfvars`. Either path (PRD adds rows, or staff removes the extras) is fine — recommendation is PRD-adds because `cluster_name` is required and `tags` is universally useful. Pure docs fold; no code change.

## Issue 2: PRD 08 + chapter 25 claim bucket versioning is "off by default"; staff module forces `versioning_configuration.status = "Enabled"` unconditionally

**Severity**: medium
**Status**: open

**Description**: Chapter 25 line 72: *"Versioning. Off by default. The artefacts are operator-rotated (see Day-2 ops) and the rotation flow overwrites the existing keys; bucket versioning would accumulate stale-but-still-decryptable copies. Operators who want a rotation audit trail enable versioning via `var.enable_bucket_versioning = true`."* PRD 08 doesn't address versioning explicitly but the chapter's claim is the operator-facing contract.

Staff's `terraform/modules/s3_supply_chain/main.tf:92-97` ships:

```hcl
resource "aws_s3_bucket_versioning" "supply_chain" {
  bucket = aws_s3_bucket.supply_chain.id
  versioning_configuration {
    status = "Enabled"
  }
}
```

— versioning **on**, no `var.enable_bucket_versioning` variable exists, no opt-out. The chapter's stated default is wrong; an operator who reads the chapter expecting overwrite-only semantics will get accumulated versions and an unexpected storage-cost line on the bill.

Either path closes the drift: (a) staff adds the variable and gates the resource on `count = var.enable_bucket_versioning ? 1 : 0`, or (b) the architect rewrites chapter 25's Versioning paragraph to say "Versioning **on** by default; the JWT and FAR archive are sensitive enough that an audit trail of overwrites is worth the marginal storage cost. Operators in cost-sensitive scenarios disable via [TODO once variable lands]." Option (b) is the smaller change and arguably the better posture for a supply-chain bucket; recommendation is the doc-side fix unless staff feels strongly about adding the variable.

**Files affected**: `book/src/25-cos-supply-chain.md:72`; `terraform/modules/s3_supply_chain/main.tf:92-97`; `terraform/modules/s3_supply_chain/variables.tf` (would gain `enable_bucket_versioning` under option a).

**Proposed fix**: Sprint 3 architect rewrites the Versioning paragraph in chapter 25 to match the as-shipped "on by default" posture (or staff adds the gate variable; pick one).

## Issue 3: PRD 08 + doctor row name mismatch — PRD says `aws iam:GetOpenIDConnectProvider permission`; doctor renders `aws iam:GetRole (FLO IRSA)`

**Severity**: low
**Status**: open

**Description**: PRD 08 § "CLI surface" § "`awsbnkctl doctor`" promises two new rows: `aws s3:PutObject permission (uses a probe key under a workspace prefix)` and `aws iam:GetOpenIDConnectProvider permission probe`. Staff's `internal/doctor/aws.go` ships row names:

- `aws s3:PutObject feasibility` (uses HeadBucket, not PutObject — the rename to "feasibility" reflects that the probe is structural, not actual upload).
- `aws iam:GetRole (FLO IRSA)` — uses `iam:GetRole` against the FLO IRSA role name, NOT `iam:GetOpenIDConnectProvider` against the OIDC provider ARN as PRD 08 promises.

Both reframes are defensible — PutObject would require a probe key + cleanup; GetOpenIDConnectProvider needs the OIDC ARN which isn't available until `eks_cluster` applies. `iam:GetRole` against the predicted role name (via `IRSARoleNameForCluster`) is structurally sound. But the PRD claim and the actual binary surface should match; a reader cross-referencing PRD 08 §"CLI surface" with `awsbnkctl doctor` output will be briefly confused.

`internal/aws/iam.go:18` already exposes both `GetOpenIDConnectProvider` and `GetRole` methods — the helper surface is correct; only the doctor's choice of which to call differs from the PRD.

**Files affected**: `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` § "CLI surface" §"`awsbnkctl doctor`" (line ~194); `internal/doctor/aws.go` (the row names + the doctor § comment at lines 27-32).

**Proposed fix**: Sprint 3 architect updates PRD 08 to read: `aws s3:PutObject feasibility` (HeadBucket probe — distinguishes "bucket not created" from "AccessDenied"); `aws iam:GetRole (FLO IRSA role)` (predicts the role name from the cluster name via the iam_irsa naming convention; NoSuchEntity is the pre-apply happy path). The current doctor row implementation is sound; only the PRD wording lags.

## Issue 4: Sprint 1 tech-writer Issue 1 (doctor AWS-checks visibility on fresh dev box) NOT closed by Sprint 2 workspace retarget

**Severity**: high
**Status**: open (carry-over re-filed)

**Description**: Sprint 1 tech-writer Issue 1 flagged that `internal/doctor/doctor.go:130` gates the call to `awsChecks(ctx, cctx)` behind `cctx.Workspace != nil`, so on a fresh dev box without `awsbnkctl init` run, the AWS pre-flight rows (credentials / sts / eks / ec2 quota / s3 / iam) never render. The Sprint 1 issue's proposed fix was: *"Move the `awsChecks(ctx, cctx)` call outside the `cctx.Workspace != nil` block ... preserves the existing region-from-workspace path when a workspace exists, surfaces the warning/skipped rows when it doesn't."* The same file at line 137 carries a comment promising: *"when the workspace schema is retargeted at AWS per PRD 04, the test contract is revisited and AWS checks surface unconditionally."*

Sprint 2 staff did the workspace retarget (Sprint 2 staff Issue 2, back-compat alias path) but **did not relax the visibility gate**. Verified by running `HOME=/tmp/empty-home ./bin/awsbnkctl doctor` on this host: 7 rows render (terraform, helm, kubectl, oc, dig, kubeconfig, workspace) and zero AWS rows surface even though the binary is fully AWS-aware. A first-time user installing `awsbnkctl` and running `doctor` to verify their environment will see no AWS signal at all until they run `init` first — which is exactly the failure mode Sprint 1 Issue 1 was filed to prevent (the brief's "fresh dev box = 0 warnings" stock contract has aged out of usefulness; pre-init users need a credentials warning, not silent gating).

The Sprint 2 brief explicitly asked tech-writer to verify: *"Sprint 1 carry-over verification. Tech-writer Sprint 1 Issue 1 (doctor visibility): is it closed by Sprint 2's workspace retarget?"* Answer: **no**. Staff retargeted the schema, but the doctor.go gate was not touched.

Sprint 2 staff Issue 2 mentions the back-compat alias keeps the change "additive and revisable at Sprint 3 retirement time" — fair, but the doctor visibility gate isn't blocked on schema cleanup; it's a 1-line change in `doctor.go:130`.

**Files affected**: `internal/doctor/doctor.go:130-142` (the workspace-nil gate around `awsChecks`); the inline comment promising deferral.

**Proposed fix**: Sprint 3 staff folds the gate-relaxation. The fix from Sprint 1 tech-writer Issue 1 still applies verbatim: move `awsChecks(ctx, cctx)` outside the `cctx.Workspace != nil` block; let `awsRegionFromContext` + `awsProfileFromContext` nil-check `cctx.Workspace` themselves (both already handle nil at the caller seam in `internal/doctor/aws.go`). Then update the `TestRunWithWhy_StockDevBox_NoWorkspace` test (or its Sprint 2 equivalent) to expect the AWS rows. Severity is **high** rather than blocker because the binary is functional — but the user-facing pre-flight story is half-broken and Sprint 1's promised closure didn't land.

## Issue 5: workspace retarget grep audit — 77 `IBMCloud` hits in `internal/`, far above the brief's "<=1 for alias / 0 for clean break" expectation

**Severity**: low
**Status**: open (informational; matches staff's documented choice but flags grep-noise)

**Description**: The Sprint 2 brief's workspace-retarget audit gate: `grep -r 'IBMCloud' internal/` should return ≤1 hit (deprecated alias if back-compat chosen) or zero (clean break). Actual count from this host: **77 hits across 21 files**. Sample distribution (`grep -r 'IBMCloud' internal/ | awk -F: '{print $1}' | sort -u`):

```
internal/cli/inspect.go         (3)
internal/cli/workspaces.go      (1)
internal/config/context_test.go (many)
internal/config/secrets.go      (4)
internal/config/workspace.go    (6 — including the deprecated alias field itself)
internal/cred/resolver.go       (many)
internal/cred/resolver_test.go  (many)
internal/doctor/aws.go          (2)
internal/doctor/doctor.go       (2)
internal/exec/creds.go          (many)
internal/exec/docker.go         (many)
... and more
```

Staff Sprint 2 Issue 2 documents the back-compat alias choice and explains the cred + exec packages still consume `IBMCloud.APIKeyB64` for the docker/k8s execution backends (retarget deferred to Sprint 3 per staff Issue 1 + PRD 04). The 77-hit count is consistent with this — the alias field itself accounts for ~10 hits in `workspace.go` + `inspect.go` + `workspaces.go` + `doctor/aws.go`; the rest come from deeper IBM API-key plumbing in `internal/cred/` + `internal/exec/` that staff explicitly defers.

The brief's audit gate (≤1 for alias) was authored assuming Sprint 2 would retire the cred + exec plumbing alongside the schema rename. That assumption didn't survive contact with the PRD 04 sequencing — staff Issue 1 documents the dependency. So the 77-hit count isn't a staff failure; it's an off-by-one between the brief's expectation and the multi-sprint retirement plan PLAN.md actually encodes.

**Files affected**: `prompts/sprint2/tech-writer.md` (audit-gate expectation); `internal/cli/legacy_helpers.go`, `internal/cred/resolver.go`, `internal/exec/{creds,docker}.go`, etc. (the survivors).

**Proposed fix**: Sprint 3 prompt drafting tightens the audit gate — expect IBMCloud hits to drop to single-digits once Sprint 3 retargets the cred/exec packages per PRD 04. No Sprint 2 action; staff did the right thing. Filing as informational for the Sprint 3 integrator's audit checklist.

## Issue 6: `awsbnkctl init` Short + Long blob still says "Sprint 0 stub" + "Sprint 1" — stale after Sprint 2's AWS-path landing

**Severity**: medium
**Status**: open

**Description**: `internal/cli/lifecycle.go:33-38`:

```go
Short: "Interactive setup; writes the workspace config.yaml (Sprint 1)",
Long: `awsbnkctl init walks through the AWS-shaped prompts (region, VPC,
subnets, instance types, cluster name) and writes the workspace config.

Sprint 0 stub: the AWS wizard lands in Sprint 1. See
docs/prd/07-EKS-CLUSTER-SRIOV.md for the input contract.`,
```

Verified by `./bin/awsbnkctl init --help` — both strings render to the user. The AWS wizard now ships (Sprint 2), reads FAR archive + JWT paths per PRD 08, writes `Workspace.AWS.SupplyChain`, and supports `--dry-run`. The blob's "Sprint 0 stub" framing is two sprints out of date; a first-time reader hitting `--help` will think the binary is a placeholder.

The `--dry-run` flag's help string ("Sprint 2 spike-deferral path") is fine; only the parent `Short` + `Long` need a refresh.

**Files affected**: `internal/cli/lifecycle.go:31-38`.

**Proposed fix**: Sprint 3 staff folds during their cluster.go / init.go pass. Replacement copy:

```go
Short: "Interactive setup; collects AWS region + VPC + subnets + FAR archive + JWT, writes the workspace config (PRD 08).",
Long: `awsbnkctl init walks through the AWS-shaped prompts (region, VPC, subnets,
cluster name, FAR archive path, subscription JWT path, FLO namespace) and writes
the workspace config.yaml under ~/.awsbnkctl/<workspace>/. The supply-chain
artefacts are uploaded to S3 by 'awsbnkctl up' via aws_s3_object resources, not
by init directly — see PRD 08 § "Open questions" for the rationale.

Use --dry-run to walk the wizard offline (no AWS API calls; useful for
populating a workspace for terraform plan inspection ahead of a real apply).`,
```

## Issue 7: CHANGELOG.md has no Sprint 2 entry; last edit predates the sprint

**Severity**: medium
**Status**: open

**Description**: `CHANGELOG.md` was last edited 2026-05-14 (file timestamp) and its `## Unreleased` section still describes Sprint 0/1 work (fork point + strip IBM-specific surface + author PRDs). No Sprint 2 line items appear — neither the S3 supply-chain + IRSA modules, nor the workspace schema retarget, nor the AWS init wizard, nor the doctor's five new AWS rows, nor the multi-arch Dockerfile fix. `grep 'S3 supply chain\|IRSA\|workspace schema' CHANGELOG.md` returns zero.

The architect's Sprint 2 task brief didn't include CHANGELOG.md (it's typically a staff or integrator surface), but the brief did flag "cross-document drift sweep" as my responsibility. README.md is fresher (line 13: *"S3 supply chain (or ECR mirror) to replace IBM Cloud Object Storage"*, line 14: *"IRSA / IAM OIDC to replace IBM Trusted Profiles"*); MIGRATING.md is also current (`§ From roksbnkctl on IBM Cloud` exists, mentions IRSA). Only CHANGELOG is stale.

**Files affected**: `CHANGELOG.md` § "Unreleased" (needs Sprint 2 entry).

**Proposed fix**: Sprint 2 integrator (or Sprint 3 staff) appends to `## Unreleased`:

```
### Added (Sprint 2 — PRD 08, S3 supply chain + IRSA)

- `terraform/modules/s3_supply_chain/` — KMS-encrypted S3 bucket for the FAR pull-key archive + subscription JWT, with deny-by-default bucket policy (no-TLS + unencrypted-upload deny statements).
- `terraform/modules/iam_irsa/` — IAM role + trust policy bound to the FLO service account via the EKS OIDC provider; permission policy grants `s3:GetObject` + `kms:Decrypt` on the supply-chain bucket only.
- `terraform/modules/ecr_mirror/` — v1.0-stretch scaffold for the optional air-gapped image mirror; one ECR repository per FAR image; `skopeo copy` pipeline deferred to Sprint 3.
- `internal/aws/{s3,iam}.go` — `PutObject` / `HeadObject` / `HeadBucket` / `GetOIDCProvider` / `HasIRSARole` helpers with mocked unit tests.
- `awsbnkctl init` AWS path — interactive wizard collects region + VPC + subnets + FAR + JWT + FLO namespace; writes `Workspace.AWS` block; `--dry-run` skips the upload step.
- `awsbnkctl doctor` — three new AWS pre-flight rows (`aws ec2 vCPU quota`, `aws s3:PutObject feasibility`, `aws iam:GetRole (FLO IRSA)`) on top of Sprint 1's `aws credentials` / `aws sts caller-identity` / `aws eks:DescribeCluster permission`.
- `tools/docker/aws/Dockerfile` — multi-arch (`linux/amd64` + `linux/arm64`) via `TARGETARCH` + per-arch sha256 pins for awscli + kubectl + helm; closes Sprint 1 tech-writer Issue 6.
- `cspell.json` — 9 new entries for IRSA / OIDC / KMS / CMK / OpenID / presigned / webhook / aarch64 / skopeo.
- `book/src/25-cos-supply-chain.md` — full ~2,500-word rewrite at S3 + IRSA (chapter filename retained pending Sprint 5 cascade).
- `book/src/14-credentials-resolver.md` — one-paragraph IRSA cross-link pointer.

### Deprecated

- `Workspace.IBMCloud` yaml block — kept as a back-compat alias so legacy on-disk workspaces load; new code reads from `Workspace.AWS` first. Full retirement gated on the PRD 04 cred + exec retarget (Sprint 3).
```

## Issue 8: chapter 25 cross-references chapter 26 § "Troubleshooting" failure shapes; chapter 26 is still a stub

**Severity**: low
**Status**: open

**Description**: Chapter 25 line 270 lists chapter 26 as a cross-reference: *"`AccessDenied` on IRSA assume, bucket-policy propagation lag, and the 'FLO can't reach the bucket but doctor says it can' failure shapes."* `book/src/26-troubleshooting.md` exists but is the Sprint 0 stub — none of the IRSA / bucket-policy / FLO-vs-doctor failure shapes are documented yet (chapter 26's full content lands per the Sprint 5/6 plan).

A reader following chapter 25's cross-link will hit an empty page. Pattern matches Sprint 1 tech-writer Issue 9 (the "lands in Sprint N" annotation discipline) and the existing Sprint 0/1 carry-overs for forward-referenced chapters.

**Files affected**: `book/src/25-cos-supply-chain.md:270` (cross-reference); `book/src/26-troubleshooting.md` (the stub target).

**Proposed fix**: Sprint 3 or Sprint 5 architect adds a "(lands in Sprint 5)" annotation to the chapter 25 cross-link to match the convention used elsewhere (Sprint 1 tech-writer Issue 9 + Sprint 0 patterns). Alternative: Sprint 3/5 architect drafts a short IRSA-troubleshooting subsection in chapter 26 so the cross-link resolves to actual content.

## Issue 9: chapter 25's filename retained as `25-cos-supply-chain.md` despite S3 retarget — Sprint 5 cascade (carries from architect Issue 2)

**Severity**: low
**Status**: open (architect-acknowledged)

**Description**: Carries forward Sprint 2 architect Issue 2: chapter 25's H1 + body are fully retargeted at S3 + IRSA but the filename retains the `cos-supply-chain` slug from the inherited roksbnkctl tree. `book/src/SUMMARY.md` lists the chapter as "S3 (and optional ECR) supply chain" so the navigation reads correctly; only the URL slug carries the stale word. Filed at the same severity as architect Issue 2 (low — cosmetic; user-impact is "this chapter's URL says cos but the page is about S3", not a broken link). Filing for sibling-cross-reference completeness; no separate action required beyond what architect Issue 2 + Sprint 5 rename pass already track.

**Files affected**: `book/src/25-cos-supply-chain.md` (filename); `book/src/SUMMARY.md` (link target).

**Proposed fix**: Sprint 5 architect renames to `25-s3-supply-chain.md` and updates SUMMARY.md link in the same commit. Per architect Issue 2 and Sprint 5's book-retarget cascade.

## Issue 10: chapter 25 bucket-policy JSON example does not match landed `aws_s3_bucket_policy` resource (architect Issue 4 — verified)

**Severity**: low
**Status**: open

**Description**: Sprint 2 architect Issue 4 pre-flagged a verification task for tech-writer: read staff's landed `aws_s3_bucket_policy` body, diff against chapter 25's JSON example (lines 38-66), harmonise or flag drift. Did the diff.

Chapter 25's JSON shows a two-statement policy: `AllowFLOReadOnly` (`Effect: Allow`, `Principal: <flo_role_arn>`, `s3:GetObject`) + `DenyEveryoneElse` (`Effect: Deny`, `NotPrincipal: <flo_role_arn>`, `s3:*`, with a `StringNotLike` carve-out for `aws-service-role/*`).

Staff's landed `terraform/modules/s3_supply_chain/main.tf:130-176` ships a different two-statement policy: `DenyInsecureTransport` (`Effect: Deny`, `Principal: *`, `s3:*`, condition on `aws:SecureTransport = false`) + `DenyUnencryptedUploads` (`Effect: Deny`, `Principal: *`, `s3:PutObject`, condition on `s3:x-amz-server-side-encryption != aws:kms`). The `AllowFLOReadOnly` statement is **not in the bucket policy** — it lives instead in the `iam_irsa` module's `aws_iam_role_policy.permissions` (`internal/aws/iam.go` / `terraform/modules/iam_irsa/main.tf:92-115`), with `s3:GetObject` on `${var.s3_bucket_arn}/*` + `kms:Decrypt` on the CMK.

This is a *defensible architectural choice* — staff's commentary in `main.tf:120-128` explains: *"the actual GetObject grant lives in the iam_irsa module's role-permission policy — keeping the grant alongside the IRSA role (rather than in the bucket policy) makes the trust chain inspectable from the IAM side without re-reading bucket policy JSON."* The grant-on-the-role pattern is fine for IRSA (S3 evaluates the union of bucket + role policies); the operator can read the IRSA role to know what FLO can do. **But chapter 25's prose says the bucket policy scopes `s3:GetObject` to the FLO IRSA role ARN, which is no longer true.**

Chapter 25 line 36: *"The policy scopes `s3:GetObject` to the FLO IRSA role ARN."* Chapter 25 lines 38-66: shows the policy as `AllowFLOReadOnly` + `DenyEveryoneElse`. Both claims are wrong against the as-shipped module.

**Files affected**: `book/src/25-cos-supply-chain.md:36-66` (the entire bucket-policy section); `terraform/modules/s3_supply_chain/main.tf:130-176` (the actual policy); `terraform/modules/iam_irsa/main.tf:92-115` (where the grant actually lives).

**Proposed fix**: Sprint 3 architect rewrites the chapter 25 bucket-policy section to reflect the as-shipped two-tier model: (a) bucket policy enforces transport + encryption hygiene (DenyInsecureTransport + DenyUnencryptedUploads), (b) the IRSA role's permission policy carries the `s3:GetObject` + `kms:Decrypt` grants. Explain the rationale (IAM-side inspectability) inline. The current chapter section is misleading enough that an operator debugging an AccessDenied will look at the wrong policy.

## Issue 11: PRD 08 § "Resolved-in-spike" — cross-references PRD 07 by design (carries SPIKE DEFERRAL)

**Severity**: low
**Status**: open (informational; spike-deferral)

**Description**: PRD 08 lines 211-214 explicitly note: *"This PRD's design doesn't directly depend on PRD 07's spike findings — S3, IRSA, and bucket policies are well-trodden AWS surface that `terraform validate` and mocked aws-sdk-go-v2 tests cover offline. But the live-apply path depends on the EKS OIDC provider existing, which means PRD 07's `eks_cluster` module must successfully `terraform apply` before this PRD's modules can. The Sprint 1 operator-run spike (PRD 07 § 'Spike protocol') validates that prerequisite."* — by design.

This is the SPIKE DEFERRAL the Sprint 2 prompt explicitly flagged: PRD 08's spike-resolution defers to PRD 07's spike. No agent has run `terraform apply` against live AWS; the integrator's release-gate audit for v0.2 still depends on the operator-run spike. Sprint 2 closes more offline work on top without unblocking the spike.

Filing for audit-trail completeness and to make the deferral explicit in the tech-writer issue file. Staff Sprint 2 Issue 6 covers the same ground from the staff side.

**Files affected**: `docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md` § "Resolved-in-spike" (intentional cross-ref to PRD 07); `docs/PLAN.md` § "Sprint 2 — S3 supply chain + IRSA" (spike-deferral note).

**Proposed fix**: none required this sprint. v0.2 tag still gated on operator-run spike per PRD 07 §"Spike status"; Sprint 2 lands more offline work on top.

## Issue 12: PLAN.md § "Sprint 2 close (actual)" not yet appended (architect Issue 3 — pending)

**Severity**: low
**Status**: open (architect-flagged; tech-writer can now sign off)

**Description**: Sprint 2 architect Issue 3 documented the deferral: at architect-filing time, no Sprint 2 sibling agent had filed their report, so the "Sprint 2 close (actual)" subsection in `docs/PLAN.md` was left unappended. As of this tech-writer filing, all three sibling agents have reported (architect 9 issues, staff 7, validator 10), and this tech-writer file completes the four-agent dispatch.

PLAN.md is a planning doc, not user-facing; the gap is process drift, not user impact. Tech-writer scope is read-only; I won't edit PLAN.md. The architect Issue 3 proposed-fix content (suggested ~10-line summary of what shipped + deferrals) is now actionable.

**Files affected**: `docs/PLAN.md` § "Sprint 2 — S3 supply chain + IRSA (PRD 08 — to author)" (suggested closing subsection).

**Proposed fix**: Sprint 2 integrator appends the closing subsection at commit-cut time, sourcing content from architect Issue 3's proposed-fix paragraph. Alternative: Sprint 3 architect pre-pass folds it during their PRD 09 / PLAN.md edits.

## Issue 13: chapter 25 cross-links to PRD 07 + PRD 08 use absolute github.com URLs instead of relative paths

**Severity**: low
**Status**: open

**Description**: Chapter 25 lines 7, 264, 265 cross-reference PRD 07 and PRD 08 via full GitHub URLs (e.g. `https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md`). The other cross-references in the same chapter (chapters 12, 13, 14, 24, 26) use relative paths (`./12-workspace-config.md`, `./14-credentials-resolver.md`, etc.). Mixed style — and the absolute URLs will silently 404 if the repo is ever renamed or forked.

Convention in the surviving inherited chapters (chapter 14, chapter 24 read this sprint) is relative links: `[chapter 14](./14-credentials-resolver.md)` and `[PRD 07](../prd/07-EKS-CLUSTER-SRIOV.md)`. The architect Sprint 2 commit migrated chapter 25's PRD references to absolute URLs presumably to render in mdbook without re-mapping `../docs/prd/`; the relative path `../../docs/prd/07-EKS-CLUSTER-SRIOV.md` from `book/src/25-cos-supply-chain.md` should resolve cleanly (mdbook serves from `book/book/`, source from `book/src/`; the `../../docs/prd/` traversal works in the rendered tree).

**Files affected**: `book/src/25-cos-supply-chain.md:7,264-265`.

**Proposed fix**: Sprint 3 architect normalises to relative paths during the next book pass. Spot-check the mdbook render to confirm `../../docs/prd/...` resolves; if mdbook's link checker chokes on the traversal, fall back to a stable redirect via SUMMARY.md or a documented helper page. Low priority — current absolute links work, just aren't repo-rename-safe.

---

*Total filed: 13 issues — 0 blocker, 1 high (Sprint 1 tech-writer Issue 1 NOT closed), 3 medium (versioning default contradiction, init Long-blob stale, CHANGELOG missing Sprint 2 entry), 8 low (PRD↔module input deltas, doctor row name drift, IBMCloud grep noise, chapter 25 → 26 stub link, chapter filename cascade, bucket-policy JSON drift, spike-deferral carry, PLAN close subsection deferred, absolute-URL cross-links), 1 informational. Three carries from sibling agents: architect Issue 4 (bucket-policy JSON — verified, see Issue 10), architect Issue 3 (PLAN.md close — see Issue 12), staff Issue 5 (vCPU quota partial — matches PRD 08 doctor row name reframe, see Issue 3).*

## Verification summary

- `make build` exit 0; binary at `/Users/j.lucia/Code/github/awsbnkctl/bin/awsbnkctl`.
- `./bin/awsbnkctl init --help` exit 0 — text contains stale "Sprint 0 stub" / "Sprint 1" framing (Issue 6).
- `./bin/awsbnkctl init --dry-run` exit 0 — wrote workspace "default" config; supply-chain step skipped per --dry-run.
- `./bin/awsbnkctl doctor` (with workspace) exit 0 — 14 rows; five new AWS rows visible (credentials / sts / eks / ec2 vCPU / s3:PutObject / iam:GetRole).
- `HOME=/tmp/empty-home ./bin/awsbnkctl doctor` exit 0 — 7 rows; **zero AWS rows surface** (Issue 4 — Sprint 1 tech-writer Issue 1 not closed).
- `terraform init -backend=false && terraform validate` exit 0 on each of `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}`.
- `grep -r 'IBMCloud' internal/` returns 77 hits across 21 files (Issue 5; matches staff's back-compat alias choice per their Sprint 2 Issue 2).
- `docker buildx` available on this host; multi-arch build not re-run (validator Sprint 2 Issue 3 already executed end-to-end, exit 0). Dockerfile structural read confirms TARGETARCH + per-arch sha256 pins land as documented.
- Cross-link audit: chapters 12, 13, 14, 24, 26 all exist as files (chapter 26 is a stub — Issue 8); MIGRATING.md § "From roksbnkctl on IBM Cloud" exists and references IRSA (PRD 08's cross-reference resolves); README.md mentions S3 supply chain + IRSA. CHANGELOG.md missing Sprint 2 entry (Issue 7).
- cspell.json contains IRSA / OIDC / KMS / CMK / OpenID / aarch64 / skopeo (validator Sprint 2 Issue 5 — verified).
- No project files edited this dispatch; only `issues/issue_sprint2_tech-writer.md` written.
