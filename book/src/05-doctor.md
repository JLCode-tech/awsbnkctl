# Doctor: checking your environment

`awsbnkctl doctor` is the prereq + credentials report. It runs in under five seconds, exits non-zero on any hard error, and prints a tabular report that maps one-to-one to the runtime dependencies the rest of the tool reaches for.

This chapter walks every check, explains what each row's "why we care" blurb means, covers the six AWS-shaped rows added in Sprint 4, and describes the `--target` SSH probe.

## What `doctor` checks

A bare `awsbnkctl doctor` runs the **general** checks: tooling on `PATH`, kubeconfig location, the resolved workspace, and the AWS credential chain. Sample output on a healthy machine looks like this:

```
awsbnkctl doctor
✓  terraform         /usr/bin/terraform (Terraform v1.15.2)                                   (required for `awsbnkctl up`)
✓  helm              /usr/local/bin/helm (v3.20.2)                                            (required for `awsbnkctl up`; terraform `local-exec` shells out to helm)
⚠  iperf3            not on PATH                                                              (needed for `awsbnkctl test throughput --backend local`)
✓  kubectl           /usr/local/bin/kubectl (clientVersion: ...)                              (internalised in awsbnkctl k *; passthrough still works if installed)
✓  aws cli           /usr/local/bin/aws (aws-cli/2.15.0)                                      (optional; awsbnkctl uses the SDK directly)
✓  kubeconfig        /home/you/.kube/config                                                   (needed for cluster-side ops)
✓  workspace         default                                                                  (per-environment config + state)
✓  aws creds         resolved via shared-config (profile=default)                             (auth for terraform + AWS SDK calls)
✓  aws sts           OK (account: 123456789012, arn: arn:aws:iam::123456789012:user/you)      (verifies the cred chain via sts:GetCallerIdentity)
✓  aws region        us-west-2                                                                (resolved from config / env / workspace)
✓  aws eks perms     eks:DescribeCluster, eks:ListClusters granted                            (verified via dry-run)
✓  aws s3 perms      s3:PutObject, s3:GetObject on workspace bucket                           (verified via dry-run)
```

Each row has the same shape:

```
<status> <name> <detail> <why we care>
```

- **status** is one of `✓` (green / OK), `⚠` (yellow / warning), or `✗` (red / error). `Skipped` checks render as `⚠`.
- **name** is the dependency or capability being checked.
- **detail** is the resolved value — usually a path, a version line, or an error message.
- **why we care** is a parenthetical clause naming the `awsbnkctl` feature that depends on this row.

## Each check explained

### `terraform` — required

One of two **hard-required** binaries for the `awsbnkctl up` happy path. `awsbnkctl` shells out to `terraform` via `terraform-exec` for plan/apply/destroy; without it nothing in the cluster lifecycle works.

Pass condition: a binary on `PATH`, version `1.5` or newer.

Failure mode: `not on PATH`. Fix: install Terraform from [terraform.io](https://www.terraform.io/downloads), or your distro's package manager, then re-run `doctor`.

### `helm` — required

The second **hard-required** binary. The bundled terraform modules (`cert_manager`, `flo`, `cne_instance`) use `null_resource` + `local-exec` provisioners that shell out to `helm upgrade --install` from inside terraform's apply phase. Without `helm` on `PATH`, the apply fails partway through the cluster lifecycle with:

```
Error: local-exec provisioner error
Error running command 'helm upgrade --install cert-manager ...':
exit status 127. Output: /bin/sh: 1: helm: not found
```

Pass condition: a `helm` (v3.x) binary on `PATH`. Doctor parses `helm version --short` for the version detail.

Failure mode: `not on PATH`. Fix: install Helm 3 from [helm.sh/docs/intro/install/](https://helm.sh/docs/intro/install/), or via your distro's package manager:

```bash
# Linux (Ubuntu/Debian — official Helm apt repo):
curl https://baltocdn.com/helm/signing.asc | sudo gpg --dearmor -o /usr/share/keyrings/helm.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update && sudo apt-get install -y helm

# macOS:
brew install helm

# Windows:
choco install kubernetes-helm
```

### `iperf3` — informational

Used only by `awsbnkctl test throughput` in its host-iperf3 modes. The default `--backend k8s` mode runs iperf3 entirely in-cluster as a one-shot Job, so a missing host `iperf3` doesn't block the everyday workflow.

Failure mode: `not on PATH`. Fix: install iperf3 if you plan to use `--backend local` or `--backend ssh:<t>` for throughput; otherwise ignore.

### `kubectl` — informational

The everyday verbs (`get`, `apply`, `describe`, `delete`, `logs`, `exec`, `port-forward`) are native Go via `client-go` and live under [`awsbnkctl k`](./24-day-2-ops.md). Missing host `kubectl` no longer disables the happy path; it only disables the `awsbnkctl kubectl <args...>` passthrough.

If `kubectl` is on `PATH`, the row is `✓` and shows the version line. If it's missing, the row is informational, not a warning, and the detail explains where the equivalent functionality lives.

### `aws` CLI — informational

`awsbnkctl` talks to AWS via the embedded aws-sdk-go-v2. The `aws` CLI is **never** invoked from any production code path — the row is informational only, useful for noting "yes, you have the CLI available if you want to use it for ad-hoc inspection". The most common ad-hoc uses on an `awsbnkctl`-managed workspace: `aws sts get-caller-identity`, `aws eks update-kubeconfig --name <cluster>`, `aws ec2 describe-instances --filters ...`.

Failure mode: `not on PATH`. Fix: install the AWS CLI per [aws-cli/install](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) if you want ad-hoc inspection.

### `kubeconfig`

Resolves the kubeconfig path via `$KUBECONFIG` first, then `~/.kube/config`. Cluster-side commands (`status`, `logs`, every `k <verb>`) need it.

`awsbnkctl up` writes the admin kubeconfig at `~/.kube/config` (mode 0600) on a fresh apply by calling EKS's `DescribeCluster` API and generating the IAM authenticator config. If you already have a multi-cluster `~/.kube/config`, point `$KUBECONFIG` at the workspace's state directory instead:

```bash
export KUBECONFIG=~/.awsbnkctl/<workspace>/state/kubeconfig
```

Failure mode: `$KUBECONFIG and ~/.kube/config both missing`. Fix: run `awsbnkctl kubeconfig --download` to regenerate the kubeconfig from the workspace's EKS cluster identity, or run `aws eks update-kubeconfig --name <cluster> --region <region>` if you have the AWS CLI installed.

### `workspace`

Reports the resolved workspace name and whether its `config.yaml` exists.

- `✓ default` — the current workspace pointer at `~/.awsbnkctl/config.yaml` resolves and the named workspace has a populated `config.yaml`.
- `⚠ "default" not initialised` — the directory may exist (created by `awsbnkctl ws new`) but `config.yaml` is empty. Run `awsbnkctl init` to populate.
- `✗ no config context` — the global config can't be loaded at all.

The one-off `-w / --workspace` flag overrides which workspace `doctor` reports against. See [Chapter 6 — Workspaces](./06-workspaces.md).

### `aws creds` — required

Resolves AWS credentials via the AWS standard chain (env / shared-config profile / SSO cached token / IMDS / container task role / web-identity) — the same chain `terraform` and the `aws` CLI walk. The chain is documented in [Chapter 14 — Credentials](./14-credentials-resolver.md).

Pass condition: the chain produces a non-empty credential. The row reports the **source** that won ("env", "shared-config (profile=default)", "sso", "imds", "container", "web-identity") — never the secret value.

Failure mode: `no AWS credentials resolved — set AWS_ACCESS_KEY_ID, run aws configure, or run aws sso login`. Fix: pick a chain link and populate it. The supported paths on a stock dev box are `aws configure` (writes `~/.aws/credentials` for a named profile; export `AWS_PROFILE=<name>` if not `default`), `aws sso login` (writes a cached token under `~/.aws/sso/cache/`), or directly setting `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` in env.

### `aws sts` — required

Round-trips the resolved credentials against AWS STS via `sts:GetCallerIdentity`. Confirms the credentials are not just present but actually authenticate.

Pass condition: STS accepts the credentials; the row reports `account:<id>, arn:<arn>` of the resolved principal.

Failure modes:
- `InvalidClientTokenId` / `SignatureDoesNotMatch` — credentials are malformed or rotated; refresh via `aws configure` or `aws sso login`.
- `ExpiredToken` / `TokenRefreshRequired` — SSO cached token expired; re-run `aws sso login`.
- `network is unreachable` / `i/o timeout` — your workstation can't reach `sts.amazonaws.com`. Common in corp-firewall scenarios; route through a jumphost ([Chapter 16](./16-on-flag-ssh-jumphosts.md)) to confirm credentials work from inside the VPC.

### `aws region`

Resolves the active AWS region via the standard precedence chain: workspace `config.yaml` → `AWS_REGION` env → `AWS_DEFAULT_REGION` env → `~/.aws/config` profile → SSO session region. The row reports the resolved value, e.g., `us-west-2`.

Failure mode: `no AWS region resolved — set AWS_REGION or workspace.aws.region`. Fix: `export AWS_REGION=us-west-2` (or the region you want), or re-run `awsbnkctl init` to capture the region in workspace config.

### `aws eks perms`

Verifies the resolved IAM principal can call `eks:DescribeCluster` and `eks:ListClusters` against the workspace's region. The probe is a dry-run `DescribeCluster` against a sentinel name that's expected to 404 — the success criterion is `ResourceNotFoundException` (auth works, the named cluster doesn't exist) rather than `AccessDenied`.

Pass condition: the API call returns either a real cluster or `ResourceNotFoundException` (both prove the principal has `eks:DescribeCluster`).

Failure mode: `AccessDenied: ... not authorized to perform: eks:DescribeCluster`. Fix: attach an IAM policy granting `eks:DescribeCluster`, `eks:ListClusters`, and `eks:UpdateClusterConfig` to the principal — or pick a different principal via `AWS_PROFILE`.

### `aws s3 perms`

Verifies the resolved IAM principal can call `s3:PutObject` and `s3:GetObject` against the workspace's supply-chain bucket. The probe is a dry-run `HeadBucket` followed by a small no-op `PutObject` to a sentinel key (`_awsbnkctl-doctor-probe`) and then a `GetObject` on the same key, then a `DeleteObject` cleanup.

Pass condition: round-trip succeeds.

Failure mode: `AccessDenied: ... s3:PutObject`. Fix: review the bucket policy + the IAM principal's policy; the principal needs `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject` against `arn:aws:s3:::<workspace-bucket>/*`. [Chapter 25](./25-cos-supply-chain.md) walks the supply-chain bucket policy in detail.

### `aws quotas` — optional, feature-flagged

The optional Service Quotas check. Off by default; enable with `AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1`. When enabled, the row reports the running-instance vCPU quota for the workspace's instance family (`c5n.*` by default) against the `node_desired_size × instance_vcpus` math.

Pass condition: the resolved quota >= the required vCPU count.

Failure mode: `VcpuLimitExceeded would trigger on apply — quota is 5 vCPUs, required is 32`. Fix: open a quota-increase request via `aws service-quotas request-service-quota-increase --service-code ec2 --quota-code L-1216C47A --desired-value 64`. Typical turnaround is 1-24 hours.

This row is feature-flagged off in v0.9 because the Service Quotas API rate-limits aggressively on fresh accounts and the failure shape is misleading without context. Sprint 6 hardening enables it by default once the failure-mode messaging is polished.

## Common failures and how to fix them

The chapter readers most often land on. Each row maps a real-world symptom to its fix:

| Symptom | Likely cause | Fix |
|---|---|---|
| `terraform not on PATH` | not installed | install Terraform `>= 1.5`; re-run `doctor` |
| `kubeconfig: $KUBECONFIG and ~/.kube/config both missing` | never ran `up` against this workspace | `awsbnkctl kubeconfig --download` or run `awsbnkctl up` |
| `aws creds: no AWS credentials resolved` | new shell, no `AWS_PROFILE` exported and no SSO session | `aws sso login`, `export AWS_PROFILE=<name>`, or set `AWS_ACCESS_KEY_ID`+`AWS_SECRET_ACCESS_KEY` |
| `aws sts: ExpiredToken` | SSO cached token expired | `aws sso login` |
| `aws sts: InvalidClientTokenId` | bad / rotated access key | regenerate the key in the IAM console; `aws configure` to update |
| `aws sts: i/o timeout` | corp-firewalled workstation | use [`--on jumphost`](./16-on-flag-ssh-jumphosts.md) to test from inside the VPC |
| `aws region: no region resolved` | no region in env or config | `export AWS_REGION=us-west-2` |
| `aws eks perms: AccessDenied` | principal doesn't have EKS perms | attach `AmazonEKSClusterPolicy` or equivalent to the IAM principal |
| `aws s3 perms: AccessDenied` | bucket policy / IAM mismatch | see [Chapter 25 §"IRSA trust chain"](./25-cos-supply-chain.md#irsa-trust-chain) |
| `workspace "foo" not initialised` | `ws new` was run but `init` was not | run `awsbnkctl init -w foo` |
| `workspace: no config context` | `~/.awsbnkctl/config.yaml` corrupt | inspect the file; worst case delete it and re-run `init` |

If a fix isn't here, [Chapter 26 — Troubleshooting](./26-troubleshooting.md) covers the longer tail.

## The `--target <name>` SSH check

The `--on jumphost` flag introduces an optional second mode for `doctor`: probe an SSH target before you try to use it.

```bash
awsbnkctl doctor --target jumphost
```

This adds one row per resolved target:

```
✓  ssh:jumphost      ec2-user@52.45.91.177:22 (TOFU recorded)             (verifies the target is reachable)
```

The probe:

1. Resolves the target's `host`, `user`, `port`, and key source from `~/.awsbnkctl/<workspace>/config.yaml`.
2. Connects via the `internal/remote` SSH client.
3. Validates the host key against `~/.awsbnkctl/known_hosts` (TOFU prompt on first contact, unless `--insecure-host-key`).
4. Runs a no-op command (`true`) to confirm the channel works end-to-end.

Failure modes specific to the SSH probe:

- `host key mismatch` — the target was rebuilt; edit `~/.awsbnkctl/known_hosts` to clear the entry, then re-probe.
- `unable to authenticate` — the key source resolved but the remote rejected it. Check `key_path` / `key_source` in workspace config; if `key_source: agent`, verify `ssh-add -l` shows the right key.
- `dial tcp: i/o timeout` — the `host:port` is unreachable. Verify with `nc -vz <host> 22` from a known-good network.

Pass `--target all` to probe every target listed in the workspace's `targets:` block. Useful in CI when you want a single command that asserts every entry is reachable.

## Reading the exit code

`doctor` exits with:

- `0` — all checks are green or warnings only. Warnings do not fail `doctor`. The everyday workflow can proceed.
- non-zero — at least one row produced an `✗` error. The first error string is also written to stderr so wrapper scripts can grep it.

This is the contract `scripts/e2e-test.sh` and the `Makefile` rely on: a script that runs `awsbnkctl doctor && awsbnkctl up --auto` will only proceed past `doctor` if the environment is genuinely ready.

The "warnings don't fail" rule is deliberate. An `iperf3 not on PATH` warning is informational — the everyday `up` / `test connectivity` flow doesn't need it. Forcing exit-1 on every warning would be too aggressive for the common case.

If you want to gate scripts strictly (e.g. CI workflows that must have iperf3 installed because they run the throughput suite), parse the output rather than relying on the exit code:

```bash
if ! awsbnkctl doctor | grep -q '^✓  iperf3'; then
  echo "iperf3 missing — install it before running test throughput" >&2
  exit 1
fi
```

## What `doctor` is not

A few deliberate non-features worth naming:

- **Not a fix-it tool.** `doctor` reports; it never installs, never modifies workspace config, never calls AWS APIs that mutate state. The STS call is read-only. The S3 probe writes and immediately deletes a sentinel key — the only mutating call doctor makes, and it's scoped to a one-byte object in the workspace's own bucket. If `doctor` could break things, users couldn't run it freely.
- **Not a backend probe.** Per-backend availability checks (docker daemon reachable, k8s ops pod healthy, ssh target reachable) ship as separate `BackendName`-tagged rows via `doctor --backend <name>`. The `--target` probe was the early prototype of that pattern.
- **Not concurrent-safe.** The CLI invokes `doctor` once per command; the side-channel for "why we care" blurbs doesn't synchronise. Don't run two `doctor`s against the same process.

## Cross-references

- [Chapter 4 — Installation](./04-installation.md) introduces `doctor` as the post-install verification step.
- [Chapter 6 — Workspaces](./06-workspaces.md) explains the `workspace` row and the `-w` override.
- [Chapter 14 — Credentials](./14-credentials-resolver.md) is the deep dive on the AWS credential resolution chain.
- [Chapter 16 — The `--on` flag](./16-on-flag-ssh-jumphosts.md) covers the `--target` probe's underlying SSH client.
- [Chapter 24 — Day-2 ops](./24-day-2-ops.md) is the canonical reference for the internalised `k <verb>` commands that make `kubectl` informational.
