# Sprint 4 — staff engineer issues

Sprint 4 verifies the inherited `internal/test/*.go` surface against
AWS-shaped deployments, lands PSA compliance on the iperf3 fixture,
plumbs workspace-derived defaults into the test commands, adds the
`--dry-run` planning surface to `test {connectivity,dns,throughput,all}`,
folds the Sprint 3 tech-writer Issue 3 first-run UX gap into
`runFullLifecyclePlan`, and wires an optional Service Quotas probe in
the doctor (feature-flagged off by default). Four issues filed: one
resolved-during-sprint, three carry-overs deferred to Sprint 5.

## Issue 1: missing `strings` import in `internal/cli/cluster.go` (resolved-during-sprint)

**Severity**: high (build-breaking)
**Status**: resolved-during-sprint

**Description**: The Sprint 3 tech-writer Issue 3 fold added
`translatePlanError(err error, wsName string) error` to
`internal/cli/cluster.go` (lines 269-279), which references
`strings.Contains(...)` twice. The import block didn't get the
matching `"strings"` line — `go vet ./...` failed with
`undefined: strings` at lines 274-275. The error surfaced on the
Sprint 4 first run of the build-green gate; staff added the import
and the gate passed cleanly afterwards. Likely an incomplete edit
during the Sprint 3 carry-over fold; the test suite still passed at
Sprint 3 close because `cluster.go` compiles into the cli package
which only fails at vet/build time, not at go-test time (the test
files don't reach the changed code path).

**Files affected**: `internal/cli/cluster.go` (one-line import fix).

**Proposed fix**: landed this sprint — added `"strings"` to the
import block. The build/vet/test gates pass cleanly.

## Issue 2: live-AWS Service Quotas probe not yet validated (carry-over to Sprint 5)

**Severity**: low (informational; expected under SPIKE DEFERRAL)
**Status**: open by design

**Description**: The optional Service Quotas check landed this
sprint (`internal/aws/servicequotas.go` +
`internal/doctor/aws.go::Check 4b`) as a feature-flagged probe.
Off by default — operator opts in via
`AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1`. Unit tests cover the toggle
(`internal/doctor/aws_test.go`) and the wire-shape via a fake
`ServiceQuotasAPI` (`internal/aws/servicequotas_test.go`). What
hasn't run: the live API call against a real AWS account. The
brief's SPIKE DEFERRAL constraint forbids live calls; the
operator-run spike (PRD 07 § "Spike protocol") validates the live
signal before v0.x flips the default and retires the flag.

The most likely live-AWS failure modes the spike must exercise:

1. `AccessDeniedException` — common-case: the operator's IAM
   doesn't attach `servicequotas:GetServiceQuota`. Doctor row
   renders as Warning pointing at the IAM gap; doctor remains
   green-overall because the row is optional (Warning, not Error).
2. `NoSuchResourceException` — quota code `L-1216C47A` mistyped or
   service code `ec2` rejected in a partition (e.g., GovCloud).
   Doctor row renders as Warning naming the error.
3. Throttling — Service Quotas has a low default request rate; the
   doctor invocation is once-per-run so this shouldn't trip in
   practice, but the spike should confirm.

**Files affected**: `internal/aws/servicequotas.go` (the API
wrapper), `internal/aws/client.go` (the new `serviceQuotas` field
+ `SetServiceQuotasForTest`), `internal/doctor/aws.go` (the
feature-flagged Check 4b row), `internal/aws/servicequotas_test.go`,
`internal/doctor/aws_test.go`.

**Proposed fix**: PRD 07 operator-run spike validates: (a) the
flag-ON path returns the live `L-1216C47A` value on an account
with permissive IAM; (b) the AccessDenied path renders a Warning
row; (c) the row's `Detail` text matches the documented shape.
Findings fold into Sprint 5 — flip the default ON if the live
signal is universally reliable, or keep the flag through v0.x
while the IAM guidance matures.

## Issue 3: `internal/cli/test.go` k8s + ssh client paths don't pin PSA-restricted Job spec (carry-over to Sprint 5)

**Severity**: medium (architectural drift; functional under EKS today)
**Status**: open by design

**Description**: The Sprint 4 brief named throughput as
"iperf3-via-k8s-Job under EKS Pod Security Admission `restricted`
profile". The server-side PSA contract is pinned by
`internal/k8s/iperf3_test.go::TestBuildIperf3Pod_PSARestricted` —
that test asserts `runAsNonRoot`, `seccompProfile: RuntimeDefault`,
`capabilities.drop: [ALL]` on the iperf3 server Pod. The
client-side dispatch path (`runIperf3ClientK8s` in
`internal/cli/test.go:677-729`) shells iperf3 out via the k8s
execution backend, and the Job spec that backend builds
(`internal/exec/k8s.go`) doesn't carry an equivalent unit-test
pin.

This is functionally OK under EKS today because the k8s backend's
tools-pod image (Sprint 1 deliverable, uid 1000) admits cleanly
into a `restricted` namespace via the existing namespace defaults.
But the contract isn't pinned — a future refactor could change
the k8s backend's Job spec and silently regress client-side PSA
admissibility without any test catching it.

**Files affected**: `internal/exec/k8s.go` (the k8s backend's
Job-spec builder), `internal/exec/k8s_test.go` (the test coverage
gap), `internal/cli/test.go::runIperf3ClientK8s` +
`runIperf3ClientSSH` (the consumers).

**Proposed fix**: Sprint 5 staff adds a PSA-pin test for the k8s
backend's Job spec mirroring the
`TestBuildIperf3Pod_PSARestricted` shape: assert `runAsNonRoot`,
`seccompProfile: RuntimeDefault`, `capabilities.drop: [ALL]` on
the client-side Job. If the spec currently omits any of those, fix
them at the same time. Server-side PSA contract is unchanged and
remains pinned.

## Issue 4: `internal/cli/test.go` k8s + ssh iperf3 client paths emit different JSON shape than local (carry-over to Sprint 5)

**Severity**: medium (UX drift from local-backend path)
**Status**: open by design

**Description**: The local-backend throughput path
(`internal/test/throughput.go::iperf3Probe`) parses the
`iperf3 -J` JSON output via `parseIperf3JSON` and surfaces
`%.2f Gbit/s (%d retransmits)` plus structured
`probe.Extra.{throughput_gbps,retransmits,endpoint,mode,duration_s,streams}`
fields. The k8s client-backend path
(`internal/cli/test.go::runIperf3ClientK8s` lines 705-718) leaves
`probe.Detail` as the raw JSON blob and doesn't populate `Extra`.
JSON consumers downstream
(`outputSuite` → `test.WriteJSON`) emit a different shape per
backend — a CI script that does
`jq '.results[].extra.throughput_gbps'` silently breaks when the
user passes `--backend k8s`.

Same applies to `runIperf3ClientSSH` (lines 736-796) — the SSH
backend path mirrors the k8s shape (opaque Detail; no Extra).

**Files affected**: `internal/cli/test.go::runIperf3ClientK8s`,
`internal/cli/test.go::runIperf3ClientSSH`,
`internal/test/throughput.go::parseIperf3JSON` (lift into a
package-public helper Sprint 5 reuses).

**Proposed fix**: Sprint 5 staff lifts `parseIperf3JSON` to
`test.ParseIperf3JSON`, calls it from both the k8s + ssh client
dispatch paths, and populates `probe.Detail` + `probe.Extra`
consistently with the local-backend path. Pinned by a new
`internal/cli/test_test.go::TestIperf3Backends_ConsistentShape`
contract.

## Verification summary

- `go build ./...` exit 0.
- `go vet ./...` exit 0.
- `gofmt -l .` clean.
- `go test ./...` exit 0 — all packages pass (aws, cli, config,
  cred, doctor, exec, k8s, remote, test, tf, refgen). New tests
  added this sprint: `internal/aws/servicequotas_test.go`
  (3 cases), `internal/doctor/aws_test.go` (3 cases).
- `terraform validate` exit 0 against `terraform/` root with the
  full Sprint 3 module graph (the committed
  `.terraform.lock.hcl` is preserved; the `.terraform/` cache is
  cleaned post-validate per the brief's disk note).
- `./bin/awsbnkctl test --help` shows the four subcommands
  (`connectivity`, `dns`, `throughput`, `list`) + the inherited
  `--dry-run` persistent flag.
- `./bin/awsbnkctl test {connectivity,dns,throughput,all}
  --dry-run --workspace test` plans the probe and exits 0 without
  executing — renders region / namespace / backend / target list.
- `HOME=/tmp/empty-home AWS_ACCESS_KEY_ID=test
  AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1
  ./bin/awsbnkctl up --dry-run` now returns the friendly
  "workspace not initialised — run `awsbnkctl init -w default`
  first..." message before terraform boots (closes Sprint 3
  tech-writer Issue 3 medium).
- `HOME=/tmp/empty-home AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1
  ./bin/awsbnkctl doctor` is wired to surface the Service Quotas
  row when credentials resolve through to STS (Check 2 must pass
  for Check 4b to run); the row is absent when the env var is
  unset (the default).
