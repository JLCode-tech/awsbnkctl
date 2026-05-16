# Sprint 4 — architect issues

Sprint 4 architect scope: PRD 04 wording fix (Sprint 3 tech-writer
Issue 1, HIGH carry-over) + chapters 20-23 (connectivity, DNS,
throughput, E2E test plan) + PLAN.md Sprint 4 close.

Off-limits surfaces (`.go`, `terraform/**`, `Makefile`, `go.mod`,
`.github/workflows/`, `cspell.json`, `tools/`, `scripts/`) respected.

SPIKE DEFERRAL carries — no live AWS validation; chapters describe
the live-tier shape, but the only paths exercised this sprint are
the offline dry-run tier and the read-only code-surface walk for
PRD 04 reconciliation.

---

## Issue 1: chapter 22 § "Tuning knobs" image tag is forward-statement; staff or release captain confirms it on tag-cut

**Severity**: low
**Status**: open

**Description**: Chapter 22's workspace-config example pins
`test.throughput.image: ghcr.io/JLCode-tech/awsbnkctl-tools-iperf3:v0.7.0`.
The `v0.7.0` tag is the M4 / Sprint 4 milestone tag per PLAN.md;
at filing time no image has been published to GHCR under that
tag. The chapter prose reads as "set this in your workspace
config", which a reader could copy-paste before the image
exists.

Two reasonable shapes:
  (a) leave the version pin in the chapter as the intended-at-tag-
      cut shape; the integrator publishes the image during the v0.7
      release process (the goreleaser / publish-tools workflow per
      Sprint 6 PLAN.md);
  (b) drop the `:v0.7.0` suffix from the example so it reads as a
      floating reference (`:latest`-equivalent) until the image is
      published.

**Files affected**: `book/src/22-throughput-testing.md` §
"Tuning knobs in workspace config".

**Proposed fix**: keep (a) — the chapter is the design surface,
the integrator's release-cut workflow is responsible for landing
the image at the documented tag. Flag this for the Sprint 6
hardening + v1.0 cut so the image-publish step is on the release
checklist.

## Issue 2: chapter 22 references "cross-node" and "`intel.com/sriov: 1` request" as v1.x refinements; verify chapter 33 carries the same scheduling story

**Severity**: low
**Status**: open

**Description**: Chapter 22 § "What 'normal' looks like on
c5n.4xlarge" enumerates expected throughput ranges for the
"east-west via SR-IOV VF (pods scheduled with `intel.com/sriov:
1`)" row. The `intel.com/sriov` resource name and the SR-IOV
device plugin's pool-naming convention are functions of the
eks_cluster module's `sriov_resource_name` input. Sprint 1
landed `intel.com/sriov` as the default in
`terraform/modules/eks_cluster/`, so chapter 22 is accurate
against the shipped default — but chapter 33 (Sprint 5 stub)
will be the single source of truth for SR-IOV-on-EKS
scheduling decisions when it lands, and chapter 22's row 5 should
cross-reference whatever the canonical resource name turns out
to be.

**Files affected**: `book/src/22-throughput-testing.md` (row 5 in
the "What 'normal' looks like" table); `book/src/33-data-plane-decision.md`
(Sprint 5 stub).

**Proposed fix**: Sprint 5 architect, when landing chapter 33,
includes a section on the device-plugin pool naming and links
chapter 22 back to it. The Sprint 4 chapter 22 wording is
deliberately vague ("pods scheduled with `intel.com/sriov: 1`")
to absorb a chapter-33 resource-name decision without rewording.

## Issue 3: chapter 23 § "Cost and time" numbers are estimates and have not been validated against a live run

**Severity**: low
**Status**: open

**Description**: The cost-and-time table in chapter 23
($0.30 for EKS control plane × 3h, $4-6 for 3× c5n.4xlarge × 3h,
$0.50-1 for NAT + transfer, $0.20-0.50 for NLBs, $0.05 for S3 +
KMS, $5-8 total per full run) is reconstructed from AWS public
pricing as of 2026-05; no live e2e run has produced an actual
bill against the suite. The numbers are right-order-of-magnitude
but a real run could come in 2× higher (cross-AZ transfer
charges scale with throughput-suite traffic; EBS gp3 root
volumes on the worker nodes aren't itemised; CloudWatch Logs
ingestion isn't itemised).

**Files affected**: `book/src/23-e2e-test-plan.md` § "Cost and
time (live tier)".

**Proposed fix**: Sprint 6 (the v1.0 hardening sprint) — on the
first authorised live run against the CI sub-account, capture
the actual Cost Explorer breakdown and update the table. Mark
the section as "estimated, pending Sprint 6 validation" in
v0.x docs; remove the caveat at v1.0 tag-cut once a real
number lands.

## Issue 4: PRD 04's "Open questions" carry IBM-era items; Sprint 5 candidates for retirement

**Severity**: low
**Status**: open (PRD hygiene; cosmetic)

**Description**: PRD 04's "Open questions" section still
carries three items, two of which are Sprint-9-resolved IBM-era
questions (the trusted-profile auto-provisioning + the cred-TTL
alignment) with strikethrough + "Resolved in Sprint 9" pointers,
and one which is still genuinely open (the kubeconfig refresh
during long-running pods). The "Centralized cred resolver"
question's recommendation ("single resolver so the keychain →
env → config-b64 → prompt chain is implemented once") has been
overtaken by events — the AWS path doesn't have a prompt or
config-b64 step, and the resolver lives in `internal/aws/` not
`internal/cred/`. The question can be marked resolved with a
pointer at § "Where the AWS chain lives in the tree".

**Files affected**: `docs/prd/04-CREDENTIALS.md` § "Open
questions".

**Proposed fix**: Sprint 5 architect (during the book retarget
sprint, which touches chapter 14 anyway) folds these items into
the resolved-section pointers. Cosmetic only; no code or contract
implication.

---

## Per-prose-surface verdict

| Surface | Verdict |
|---|---|
| `docs/prd/04-CREDENTIALS.md` § "Where the AWS chain lives in the tree" | Ships. Documents the as-shipped two-package split (`internal/aws/` AWS chain + `internal/cred/` deprecated IBM resolver) verbatim against the code; resolves Sprint 3 tech-writer Issue 1 (HIGH) end-to-end. |
| `book/src/20-connectivity-testing.md` (~1,500 words) | Ships. Documents the inherited probe surface, AWS LoadBalancer shape recognition, and the failure-mode reading guide. Word count in band. |
| `book/src/21-dns-testing-gslb.md` (~1,800 words) | Ships. Documents `--gslb-compare`, the three-vantage pattern, Route 53 resolver shapes, JSON schemas, the worked us-west-2/us-east-1/eu-west-1 example. Word count slightly above the 1,200 band; the diagram + JSON schema sections justify it. |
| `book/src/22-throughput-testing.md` (~1,500 words) | Ships. Documents the two modes, PSA compliance contract, c5n.4xlarge baselines, tuning knobs. The `:v0.7.0` image tag pin (Issue 1) is forward-statement. Word count in band. |
| `book/src/23-e2e-test-plan.md` (~2,200 words) | Ships. Phase-letter system + dry-run/live-tier semantics + cost-and-time (Issue 3, estimates pending). Word count above the 1,500 ceiling; the phase-by-phase walks justify it. |
| `docs/PLAN.md` § Sprint 4 close (actual) | Ships. Mirrors the Sprint 3 close shape; integrator extends with sibling deliverables at tag-cut per the maintenance contract at the bottom of PLAN.md. |

## Dogfooding-loop stuck-points

None this sprint. Sprint 3 tech-writer Issue 3 (first-run UX
gap on `up --dry-run` without prior `init`) is a Sprint 4 staff
fold, not architect scope.

## Cross-document drift verdict

PRD 04 ↔ `internal/aws/` is clean post-reconciliation (§ "Where
the AWS chain lives in the tree" describes
`NewClients`/`CredentialsConfigured`/`HasEnvCredentials`/`CallerIdentity`
verbatim). PRD 04 ↔ `internal/cred/resolver.go` is clean — the
PRD documents the package as deprecated-for-back-compat-naming
only, and the code matches (no AWS-chain calls, no production
caller threads a non-empty value). PRD 05 ↔ chapter 23 is clean
(chapter 23 reframes the inherited Phases I-N for the AWS
retarget, citing PRD 05 for the design rationale). Chapter
21/22/23 cross-links resolve cleanly modulo the chapter 33
stub (Sprint 0 carry; Sprint 5 lands the real content per PLAN.md).

## Issues filed: 4

- 0 blocker
- 0 high
- 0 medium
- 4 low (Issues 1-4)
- 0 roadmap

Of the 4: all open for Sprint 5 / Sprint 6 / integrator fold.
None are Sprint 4 blockers.
