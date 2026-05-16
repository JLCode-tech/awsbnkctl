# The E2E test plan

`awsbnkctl` ships a layered end-to-end test suite that exercises the full surface — install, lifecycle, four execution backends, internalised kubectl, the DNS probe, the cred-leak audit, and a mixed-mode lifecycle — against a real AWS account. This chapter is the user-facing guide: what the suite is, how to run it locally, what each phase validates, what it costs, and how it is re-run when (not if) part of it flakes.

The design rationale lives in [PRD 05](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/05-E2E-TEST-PLAN.md); read that for the *why*. This chapter is the *how* and *what*.

## What the E2E suite is

The suite is organised into two tiers plus a manual integrator step:

| Tier | Phases | What it covers | Driver script |
|---|---|---|---|
| **Baseline (AWS lifecycle)** | A, B, C, D, E, F, G, H | install + `init`, `up cluster --dry-run` plan, full-graph plan, `up`, post-apply checks, `test` verbs, `down` | `scripts/e2e-test.sh` |
| **Backends + extras** | I, K, L, L-DNS, M, N | SSH backend, docker backend, k8s backend + ops pod, DNS probe with GSLB compare, cred-leak audit, mixed-mode lifecycle | `scripts/e2e-test-backends.sh` |
| **Manual** | J | kubectl internalisation (PATH-stripped — integrator-driven) | per-release checklist |

A combined driver, `scripts/e2e-test-full.sh`, runs both automated tiers in sequence: A-H to bring up, exercise, and tear down a baseline cluster, then I-N which provisions a fresh cluster via Phase N's mixed-mode-lifecycle step.

### The SPIKE DEFERRAL gate

Sprints 0-3 land the offline tier — `awsbnkctl up --dry-run` plans the full module graph (eks_cluster → cert_manager → s3_supply_chain + iam_irsa → flo → cne_instance → license → testing), and the e2e driver's apply-tier phases skip cleanly against fake creds. **Live `apply` against AWS gates on the operator-run PRD 07 spike** — the Sprint 1 PRD documents the day-1/day-2/day-3 protocol (provision EKS 1.30 + self-managed `c5n.4xlarge` node group → install Multus + SR-IOV CNI + device plugin DaemonSets → schedule a pod requesting `intel.com/sriov: 1` and confirm the VF surfaces). Until the spike clears, every phase that consumes a real AWS resource emits a skip banner pointing at PRD 07 § 4. Run with `--spike-mode` to see the protocol text inline.

### Phase coverage

| Phase | Tier | Validates | PRD |
|---|---|---|---|
| A | baseline | sanity — `version`, `doctor`, `init`, tfvars write | — |
| B | baseline | `up cluster --dry-run` — eks_cluster module plan | [07](../../docs/prd/07-EKS-CLUSTER-SRIOV.md) |
| C | baseline | register an existing cluster + `down cluster` | — |
| D | baseline | full lifecycle — `up` against the eight-module graph, CNEInstance Ready | [07](../../docs/prd/07-EKS-CLUSTER-SRIOV.md), [08](../../docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md) |
| E | baseline | post-apply checks — `status`, `k get nodes`, `logs flo`; workspace ops in D's idle window | [02](../../docs/prd/02-KUBECTL-INTERNAL.md) |
| F | baseline | S3 object CRUD against the supply-chain bucket (the AWS-shaped replacement for the inherited COS phase) | [08](../../docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md) |
| G | baseline | `test connectivity` + `test throughput` (iperf3) | — |
| H | baseline | `down` — destroy + cleanup; no orphan ENIs / VPCs / IAM roles | — |
| I | backends | SSH backend — `--on bastion`, host-key TOFU | [01](../../docs/prd/01-SSH-AND-ON-FLAG.md) |
| J | **manual** | kubectl internalisation (PATH-stripped — requires `sudo mv`) | [02](../../docs/prd/02-KUBECTL-INTERNAL.md) |
| K | backends | docker backend (`aws` CLI + iperf3 client) | [03](../../docs/prd/03-EXECUTION-BACKENDS.md) |
| L | backends | k8s backend — `ops install`, in-cluster `aws`, `test throughput --backend k8s` | [03](../../docs/prd/03-EXECUTION-BACKENDS.md) |
| L-DNS | backends | DNS probe — `--backend k8s`, `--server cluster`, `--gslb-compare` divergence | [03](../../docs/prd/03-EXECUTION-BACKENDS.md) |
| M | backends | cred-leak audit — no AWS credentials in `docker inspect`, k8s events, SSH tempfiles | [04](../../docs/prd/04-CREDENTIALS.md) |
| N | backends | mixed-mode lifecycle — `up`/`down` cycle where the teardown backend differs from the init backend | all of the above |

## How to run it locally

The three driver scripts all live under [`scripts/`](https://github.com/JLCode-tech/awsbnkctl/tree/main/scripts):

```bash
# Baseline only (A-H) — ~60-90 minutes against live AWS, ~2 minutes dry-run
./scripts/e2e-test.sh                    # live; gates on spike per phase
DRY_RUN=1 ./scripts/e2e-test.sh          # dry-run tier; uses the Sprint-3 plan path
PHASE_FROM=D ./scripts/e2e-test.sh       # resume from phase D

# Backends + extras only (I-N + L-DNS) — needs a live cluster (D)
./scripts/e2e-test-backends.sh

# Combined — A-H baseline, then I-N + L-DNS against a fresh cluster
./scripts/e2e-test-full.sh
```

### Dry-run tier (CI today)

`DRY_RUN=1` short-circuits every `awsbnkctl` invocation that would touch AWS to its plan-tier equivalent. Phases A-H walk through the Sprint-3 `runFullLifecyclePlan` orchestrator with `--dry-run` and assert it returns exit 0 with a well-formed plan for the modules that don't require live AWS data sources to render (`s3_supply_chain.random_id.bucket_suffix`, `testing.tls_private_key.jumphost_shared_key`). The rest of the graph short-circuits at the STS-GetCallerIdentity 403 step — expected behaviour against fake creds; documented in Sprint 3 staff Issue 2.

The dry-run tier is what `.github/workflows/ci.yml`'s `full-up-dryrun` job exercises on every PR. It is the regression gate that catches PLAN/PRD drift before it lands on `main`.

### Live tier (post-spike)

Once the PRD 07 spike clears and the integrator wires the operator-run validation into CI (Sprint 6, per PLAN.md), the same drivers run against a real AWS account in a CI-controlled sub-account. Pre-reqs in that mode:

| Pre-req | Required for | Notes |
|---|---|---|
| AWS account + credentials | every phase | Standard chain — `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, or an EC2 instance role on the runner. The CI job uses GitHub-Actions OIDC against an AWS role (`AWS_WEB_IDENTITY_TOKEN_FILE` chain link). |
| `terraform` ≥ 1.5 on PATH | phases B-H | The only strictly-required host tool; `aws`, `kubectl`, and `iperf3` are internalised. |
| Docker daemon | phase K | `dockerd`, `colima`, or Rancher Desktop. |
| `kind` binary | phase L on CI | The in-CI k8s backend uses a kind cluster; on a real run it uses the EKS cluster from D. |
| EC2 bastion / jumphost | phases I, N | Provisioned automatically by phase D's terraform when `testing_create_jumphost = true` (the default). |
| Sufficient quota for `c5n` family | phases D, N | Doctor's `aws ec2 vCPU quota` row pre-flights the `Running On-Demand Standard (c, m, r, t, …) instances` quota; greenfield accounts often hit the default 32-vCPU ceiling on first cluster apply. |

### Resuming a partial run

`PHASE_FROM=<letter>` fast-forwards past every prior phase. The drivers do not try to remember whether earlier phases succeeded — that is the operator's job (the per-phase log files at `/tmp/awsbnkctl-e2e/run-*.log` are the evidence).

Per-phase actions are themselves idempotent where possible: `awsbnkctl up` on an already-applied workspace is a terraform no-op; `awsbnkctl ops install` on an already-installed cluster is a no-op; the cred resolver and the redactor short-circuit cleanly on repeated invocations.

## What each phase validates

### A — `init` and `doctor`

`awsbnkctl init` prompts for region, instance types, cluster name, and BNK version, then writes `~/.awsbnkctl/<workspace>/config.yaml`. The phase asserts the file exists, contains no plaintext AWS credentials, and that `awsbnkctl doctor` reports green for terraform and informational for kubectl/aws-cli absence (both are internalised, so neither needs to be installed).

### B — `up cluster --dry-run`

`awsbnkctl up cluster --dry-run` plans the eks_cluster module in isolation — VPC inputs, node group, Multus + SR-IOV manifests. The phase asserts the plan diff is non-empty and renders the expected resource count from the module. No AWS resources are provisioned.

### C — `cluster register` and `down cluster`

`awsbnkctl cluster register <existing-cluster>` ties an existing EKS cluster's outputs into the workspace; `awsbnkctl down cluster` destroys the eks_cluster module without touching the BNK trial modules.

### D — full lifecycle

The dominant cost phase. `awsbnkctl up --auto` runs `terraform apply` against the full eight-module graph. Live-tier wall time: 25-40 minutes on a clean apply, longer when AWS control planes are slow. The phase asserts terraform exits zero, the EKS kubeconfig was fetched, the CNEInstance reports `Ready`. Post-apply, the phase auto-registers the `jumphost` target so subsequent phases can `--on jumphost` without manual config.

### E — post-apply checks

`awsbnkctl status` shows the deployed BNK components. `awsbnkctl k get nodes` lists the worker nodes Ready. `awsbnkctl logs flo` prints recent log lines. Workspace ops (`ws list`, `ws show`) exercise the workspace surface during D's idle window.

### F — S3 supply-chain CRUD

`awsbnkctl init` uploaded the FAR + JWT artefacts to the workspace's S3 bucket; this phase asserts the bucket is reachable host-side (`s3:HeadBucket`), readable in-cluster via the FLO IRSA role, and that re-running `init` with the `--rotate-supply-chain` flag overwrites the objects atomically (versioned, per PRD 08's enable-bucket-versioning decision).

### G — `test connectivity` + `test throughput`

`awsbnkctl test connectivity` walks the workspace's `extra_hosts` list. `awsbnkctl test throughput` deploys the iperf3 fixture, runs a 30-second client measurement, and tears down. Pass criteria: connectivity returns 2xx on every URL; throughput > 1 Gbps (a conservative floor on `c5n.4xlarge`); no PSA admission failures on the iperf3 pod.

### H — `down`

`awsbnkctl down --auto` runs `terraform destroy`. Pass criteria: terraform reports `Destroy complete!`; no orphan ENIs / NAT gateways / IAM roles / OIDC providers / S3 buckets surface via `aws resourcegroupstaggingapi get-resources --tag-filters Key=workspace,Values=<ws>`.

### I — SSH backend

`awsbnkctl exec --on jumphost -- whoami` returns the cloud-init user. `awsbnkctl aws --on jumphost sts get-caller-identity` validates AWS-credential propagation over SSH (the `--on` path propagates `AWS_PROFILE` and the short-lived session token via `ssh -o SetEnv=`). A negative test mutates `~/.awsbnkctl/known_hosts` to a wrong fingerprint and asserts the next call exits 126 with a clear "host key mismatch" error.

### J — kubectl internalisation (manual)

Strips `kubectl` and `oc` from PATH and verifies `awsbnkctl k get/apply/describe/exec/port-forward/delete` all work against the cluster. The supplementary byte-equivalence check diffs `kubectl get nodes -o yaml` against `awsbnkctl k get nodes -o yaml` (excluding `managedFields`, `resourceVersion`, `creationTimestamp`).

### K — docker backend

`awsbnkctl aws --backend docker sts get-caller-identity` pulls the `awsbnkctl-tools-aws` image and runs the AWS CLI inside it. The phase asserts the AWS credentials are not baked into the image (via `docker history`) and not exposed in the running container's env (via `docker inspect`) — the bind-mount-of-`~/.aws/`-read-only pattern PRD 04 § "Backend × credential matrix" mandates.

### L — k8s backend and ops pod

`awsbnkctl ops install` creates the `awsbnkctl-ops` namespace, deploys the long-lived ops pod, and binds its ServiceAccount to the IRSA-shaped IAM role (no static AWS keys in any Secret). `awsbnkctl aws --backend k8s sts get-caller-identity` executes inside the ops pod via the injected pod-identity webhook env. RBAC assertions confirm the SA can create Jobs in `awsbnkctl-test` but cannot delete Pods in `default`.

### L-DNS — DNS probe and GSLB compare

Exercises the [`miekg/dns`-backed probe](./21-dns-testing-gslb.md): single-vantage A/AAAA against `8.8.8.8`, NXDOMAIN negative, 10-iteration RTT, k8s-backend probe, `--server cluster` (CoreDNS → VPC `.2` resolver), `--gslb-compare` against a geo-resolved name where local and cluster IPs hit different DCs, and the docker-rejection negative.

### M — cred-leak audit

Cross-cutting check that runs **after** I-L. Confirms no AWS credentials leaked: `docker history` clean, `docker inspect` clean, `kubectl get events -n awsbnkctl-ops` clean, `kubectl logs <ops-pod>` clean (the redactor masks any tool output that prints them), `ssh bastion ls /tmp/awsbnkctl.*` empty, sshd auth.log shows the SetEnv var name but no value. A leak in any of M1-M7 is a stop-ship for v1.0.

### N — mixed-mode lifecycle

A realistic scenario: workspace config routes terraform=`local`, aws=`ssh:bastion`, iperf3=`k8s` — three backends in one lifecycle. Asserts state is preserved across the per-tool dispatch and that `down` cleanly destroys everything.

## How CI runs it

`.github/workflows/ci.yml` runs **unit + integration + the dry-run tier** on every PR (`go test ./...`, the `runFullLifecyclePlan` dry-run job, cspell, terraform-validate). The full live e2e suite is too expensive ($5-10 of AWS spend per run; ~3-5 hours wall-time) to gate on every PR.

A separate manual-trigger workflow runs `scripts/e2e-test-full.sh` against live AWS on demand and on release branches (gates on the operator-run PRD 07 spike per Sprint 6 PLAN.md). Release-cut policy: do not tag `vX.Y.Z` until the most recent manual-trigger run on the release branch is green for three consecutive nights.

## Cost and time (live tier)

| Resource | Approximate cost (USD) |
|---|---|
| EKS control plane (~3h uptime) | $0.30 |
| 3× `c5n.4xlarge` nodes (~3h uptime) | $4-6 |
| NAT gateway + data transfer | $0.50-1 |
| NLBs (throughput north-south) | $0.20-0.50 |
| S3 + KMS for the supply chain | $0.05 |
| **Total per full run** | **$5-8** |

Phase D is the dominant cost (~25-40 min EKS apply + node group bring-up); Phase N runs a second full lifecycle (~25-40 min more). Contributors who want a shorter loop should skip N and rely on D + I-M coverage; full N is a release-gate concern.

## Cross-references

- [Chapter 26 — Troubleshooting](./26-troubleshooting.md) — symptom → root cause → fix for failures D-N can surface.
- [Chapter 17 — Execution backends](./17-execution-backends.md) — the four-backend matrix phases I, K, L exercise.
- [Chapter 19 — The in-cluster ops pod](./19-in-cluster-ops-pod.md) — what phase L installs and what RBAC it carries.
- [Chapter 21 — DNS testing for GSLB](./21-dns-testing-gslb.md) — the probe phase L-DNS exercises.
- [Chapter 22 — Throughput testing](./22-throughput-testing.md) — the iperf3 fixture phase G uses.
- [PRD 05](../../docs/prd/05-E2E-TEST-PLAN.md) — design spec for the suite.
- [PRD 07 § 4](../../docs/prd/07-EKS-CLUSTER-SRIOV.md) — the SPIKE protocol that gates the live tier.
