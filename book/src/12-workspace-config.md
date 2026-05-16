# Workspace config (config.yaml)

This chapter is the field-by-field reference for the per-workspace `config.yaml`. If you've read [Chapter 6 — Workspaces](./06-workspaces.md) you've seen the on-disk layout; this chapter zooms in on the YAML file that drives everything else (`init`, `up`, `down`, `cluster up`, the test suite, the SSH targets, the execution backends).

You don't usually edit this file by hand. `awsbnkctl init` generates it interactively; later runs read it. But because every other knob in the tool reads from here, it's worth knowing what every field means and what defaults apply when you leave one out.

## File location

Each workspace's config lives at:

```
~/.awsbnkctl/<workspace>/config.yaml
```

Override the base directory with the `AWSBNKCTL_HOME` env var (test fixtures use this; everyday users shouldn't need it). The file is created mode `0644` — readable by your user, the same trust posture as the surrounding workspace directory.

There's also a *global* `~/.awsbnkctl/config.yaml` at the top level — it holds the `current_workspace` pointer and other user-wide preferences. That's a different file with a different schema; this chapter is about the per-workspace one.

## When it gets written

| Action | Effect on `config.yaml` |
|---|---|
| `awsbnkctl init` | Creates the file from interactive prompts. Existing file? Asks before overwriting. |
| `awsbnkctl init --upgrade-tf` | Updates `tf_source:` only; leaves every other field alone. |
| `awsbnkctl targets add <name> ...` | Adds an entry under `targets:`. |
| `awsbnkctl targets remove <name>` | Removes the entry. |
| `awsbnkctl up` (post-apply) | Auto-populates `targets.jumphost` if the upstream HCL emitted a bastion EC2 output. |
| Anything else | Reads the file. Doesn't write back. |

Direct hand-editing is supported (the file is plain YAML) but discouraged for fields that have dedicated commands — adding an SSH target via `awsbnkctl targets add` keeps the schema validation in one place.

## Top-level structure

```yaml
aws:             # AWS account + region + credential source
  region: us-west-2
  profile: awsbnkctl-dev          # OPTIONAL — pins AWS_PROFILE; otherwise standard chain

cluster:         # EKS cluster identity
  create: true
  name: bnk-quickstart
  kubernetes_version: "1.30"
  node_instance_types: ["c5n.4xlarge"]
  node_desired_size: 2
  node_min_size: 2
  node_max_size: 4
  vpc_mode: create                # create | existing
  vpc_id: ""                      # required when vpc_mode = existing
  subnet_ids: []                  # required when vpc_mode = existing

s3:              # supply-chain S3 bucket
  bucket: awsbnkctl-quickstart-supply
  kms_key_arn: ""                 # OPTIONAL — defaults to bucket-default SSE-S3 if empty
  far_archive: ./keys/f5-far-auth-key.tgz
  licence_jwt: ./keys/trial.jwt

bnk:             # BNK trial knobs (optional; falls through to upstream HCL defaults)
  cneinstance_size: Small
  far_repo_url: repo.f5.com
  manifest_version: 2.3.0-3.2598.3-0.0.170

test:            # test-suite tuning (optional)
  throughput:
    duration: 30
    streams: 8
  connectivity:
    extra_hosts:
      - https://my.gslb.example.com

tf_source:       # where the Terraform HCL comes from
  type: embedded         # embedded | github | local

targets:         # SSH targets (see Chapter 15)
  jumphost:
    host: 52.45.91.177
    user: ec2-user
    key_source: tf-output:bastion_shared_key

exec:            # per-tool execution backend defaults (see Chapter 17)
  iperf3:    { backend: k8s }
  terraform: { backend: local }
```

Every block except `aws:`, `cluster:`, and `tf_source:` is optional. Omit a block and the tool falls through to either a documented default (covered below) or the upstream HCL's own default for terraform variables.

## `aws:`

```yaml
aws:
  region: us-west-2
  profile: awsbnkctl-dev
```

| Field | Type | Default | Notes |
|---|---|---|---|
| `region` | string | none — required | AWS region for cluster, VPC, S3 bucket, IRSA OIDC provider. Examples: `us-west-2`, `us-east-1`, `eu-west-1`. |
| `profile` | string | empty (standard chain) | Pins `AWS_PROFILE` for this workspace's commands. Useful when your laptop has multiple AWS accounts configured and you want this workspace pinned to a specific one. Leave empty to let the AWS standard chain (env / shared-config default / SSO / IMDS) resolve naturally. |

Credentials are **never** stored in `config.yaml`. The workspace records the *source* (via `profile`) but not the secret. See [Chapter 14](./14-credentials-resolver.md) for the full resolution chain.

The plaintext field name `aws_access_key_id:` or `aws_secret_access_key:` is **rejected** at load time — `awsbnkctl` refuses to read a workspace config that contains either.

## `cluster:`

```yaml
cluster:
  create: true
  name: bnk-quickstart
  kubernetes_version: "1.30"
  node_instance_types: ["c5n.4xlarge"]
  node_desired_size: 2
  node_min_size: 2
  node_max_size: 4
  vpc_mode: create
```

| Field | Type | Default | Notes |
|---|---|---|---|
| `create` | bool | `true` | When `true`, `awsbnkctl cluster up` provisions a new EKS cluster. When `false`, `cluster register <name>` adopts an existing one. |
| `name` | string | none — required | EKS cluster name when `create=true`; cluster name to adopt when `create=false`. |
| `kubernetes_version` | string | `"1.30"` | EKS supported Kubernetes minor. Quote it — YAML otherwise parses `1.30` as a float. |
| `node_instance_types` | []string | `["c5n.4xlarge"]` | Instance types for the self-managed SR-IOV node group. `c5n` / `m5n` families carry ENA + SR-IOV at 25 Gbps; everything else is unsupported for the data plane. |
| `node_desired_size` | int | `2` | Initial node count. |
| `node_min_size` | int | `2` | ASG minimum. |
| `node_max_size` | int | `4` | ASG maximum. |
| `vpc_mode` | enum | `create` | `create` (provision a new VPC across 3 AZs) or `existing` (use the supplied `vpc_id` + `subnet_ids`). |
| `vpc_id` | string | empty | Required when `vpc_mode = existing`. |
| `subnet_ids` | []string | empty | Required when `vpc_mode = existing`. Must span at least 2 AZs. |

The `cluster:` block translates to terraform variables `create_eks_cluster`, `eks_cluster_name`, `kubernetes_version`, `node_instance_types`, `node_desired_size`, `vpc_mode`, etc. — see [Chapter 13](./13-terraform-variables.md) and [Chapter 29](./29-terraform-variable-reference.md) for the full mapping.

## `s3:`

```yaml
s3:
  bucket: awsbnkctl-quickstart-supply
  kms_key_arn: ""
  far_archive: ./keys/f5-far-auth-key.tgz
  licence_jwt: ./keys/trial.jwt
```

| Field | Type | Default | Notes |
|---|---|---|---|
| `bucket` | string | `awsbnkctl-<cluster>-supply` | S3 bucket name for FAR archive + licence JWT. Must be globally unique per AWS rules. |
| `kms_key_arn` | string | empty (SSE-S3 default) | If set, the bucket uses `aws:kms` encryption with the named CMK. If empty, falls back to bucket-default SSE-S3 (AES-256). |
| `far_archive` | string | none — required | Local path to the FAR auth-key tarball. Uploaded to `s3://<bucket>/f5-far-auth-key.tgz` on `up`. |
| `licence_jwt` | string | none — required | Local path to the licence JWT file. Uploaded to `s3://<bucket>/trial.jwt` on `up`. |

[Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) is the deep reference for the supply-chain bucket layout, bucket policy, and IRSA trust chain.

## `bnk:`

```yaml
bnk:
  cneinstance_size: Small
  far_repo_url: repo.f5.com
  manifest_version: 2.3.0-3.2598.3-0.0.170
```

| Field | Type | Default | Notes |
|---|---|---|---|
| `cneinstance_size` | enum | upstream HCL default (`Small`) | `Small` \| `Medium` \| `Large`. Sets `cneinstance_deployment_size`. |
| `far_repo_url` | string | upstream HCL default (`repo.f5.com`) | The FAR Docker/Helm repo. Override only for staging/internal repos. |
| `manifest_version` | string | upstream HCL default | Pin a specific BNK manifest chart version. Leave empty to track the upstream HCL's pin. |

Every field here is optional — leave the block out entirely and you get the upstream HCL's defaults for all three.

## `test:`

```yaml
test:
  throughput:
    image: ghcr.io/JLCode-tech/awsbnkctl-tools-iperf3:v0.9.0
    duration: 30
    streams: 8
    default_mode: north-south
  connectivity:
    extra_hosts:
      - https://my.gslb.example.com
      - https://internal.example.test
```

| Field | Type | Default | Notes |
|---|---|---|---|
| `throughput.image` | string | `networkstatic/iperf3:latest` (will flip to the bundled image after Sprint 6 publishes it) | iperf3 image used by the throughput test. The bundled `awsbnkctl-tools-iperf3` image satisfies EKS 1.25+ Pod Security Admission `restricted` (runs as UID 1000); the stock `networkstatic/iperf3:latest` does **not**. See [Chapter 22 §"EKS 1.25+ Pod Security Admission compliance"](./22-throughput-testing.md#eks-125-pod-security-admission-compliance) for the constraint. |
| `throughput.duration` | int seconds | `30` | iperf3 `-t` flag. |
| `throughput.streams` | int | `8` | iperf3 `-P` flag. |
| `throughput.default_mode` | enum | `north-south` | `north-south` \| `east-west`. The connectivity vector to test by default. |
| `connectivity.extra_hosts` | []string | empty | Extra URLs the connectivity test probes alongside the canonical AWS/F5 endpoints. |

## `tf_source:`

```yaml
tf_source:
  type: embedded
```

| `type` | Other fields | Use case |
|---|---|---|
| `embedded` (default) | none | Use the HCL bundled into the `awsbnkctl` binary via `go:embed`. The recommended path for users — install one binary, get matched CLI + Terraform together. |
| `github` | `repo: "owner/name"`, `ref: "v0.9.0"` | Pull a tarball from a GitHub release. Useful for testing forks or pinning to a specific upstream tag. |
| `local` | `path: "/abs/path/to/tf-source"` | Point Terraform at an on-disk directory. For active development on the HCL itself. |

An empty `type` is treated as `embedded` (legacy / forgot-to-set).

`awsbnkctl init --upgrade-tf` is the helper for bumping the source between versions without retyping the rest of the config.

## `targets:` — SSH targets

```yaml
targets:
  jumphost:
    host: 52.45.91.177
    user: ec2-user
    key_source: tf-output:bastion_shared_key
  bastion:
    host: ops.example.com
    user: ec2-user
    key_path: ~/.ssh/id_ed25519
```

Each entry has `host`, `user`, optional `port` (default `22`), and exactly one of `key_path` or `key_source`. The `key_source` enum supports `agent` and `tf-output:<name>`.

The deep reference is [Chapter 15 — SSH targets](./15-ssh-targets.md), and the user-facing prose is [Chapter 16 — The --on flag and SSH jumphosts](./16-on-flag-ssh-jumphosts.md). This chapter just notes the schema's place in the overall config.

You don't typically edit this block by hand. `awsbnkctl up` auto-populates `jumphost` post-apply (from the bastion EC2 instance's public IP + the bundled HCL's `tls_private_key`), and `awsbnkctl targets add ...` populates the rest.

## `exec:` — execution-backend defaults

```yaml
exec:
  iperf3:    { backend: k8s }
  terraform: { backend: local }
```

Per-tool defaults for the `--backend` system. Each entry is keyed by the tool name and selects which execution backend that tool uses by default. Allowed backend values:

| Backend | Notes |
|---|---|
| `local` | `os/exec` against the host binary. The default for `terraform`. |
| `docker` | Runs inside a vendored container image. Frozen toolchain version, no host install. |
| `k8s` | Runs inside the cluster (long-lived ops pod or one-shot Job). Default for `iperf3`. |
| `ssh` | Runs on a registered SSH target. Format: `ssh:<target-name>`. |

A `--backend <value>` flag on the command line overrides the workspace config for that single invocation. The flag wins; the config sets the default.

The `iperf3` default is `k8s` because measuring throughput from a laptop's internet uplink isn't useful — you want the test to run from a network location adjacent to or inside the cluster. The `local` default is wrong for that tool, so the workspace config flips it.

[Chapter 17 — Execution backends](./17-execution-backends.md) covers the full backend matrix; [Chapter 18 — Choosing a backend per tool](./18-choosing-backend.md) is the decision tree.

## Behaviour when fields are missing

`awsbnkctl` falls through three layers in order: **workspace config → upstream HCL default → fail**.

| Missing field | Behaviour |
|---|---|
| `aws.region` | `awsbnkctl init` prompts; programmatic loads error with "region is empty". |
| `aws.profile` | Standard credential chain (env → shared-config default profile → SSO → IMDS) walked normally. |
| `cluster.name` | `init` prompts; programmatic loads error. |
| `cluster.kubernetes_version` | Falls back to the binary's pinned default (currently `"1.30"`). |
| `cluster.node_desired_size` | Falls through to `2`. |
| `s3.bucket` | Defaults to `awsbnkctl-<cluster-name>-supply`. |
| `s3.far_archive` / `s3.licence_jwt` | `init` prompts; programmatic loads error if these are needed and missing. |
| `bnk.*` | Field is omitted from the generated `terraform.tfvars` and the upstream HCL default applies. |
| `tf_source` | Treated as `type: embedded` (legacy default). |
| `targets.*` | Block absent ⇒ `awsbnkctl --on jumphost` errors with "no target named jumphost"; auto-populated by `up`. |
| `exec.*` | Per-tool defaults: `terraform`→`local`, `iperf3`→`k8s`. Override per-tool via this block, or per-invocation via `--backend`. |

The general rule: **if you don't write it in `config.yaml`, `awsbnkctl` doesn't write it into `terraform.tfvars`**, and the upstream HCL's `default = ...` clause takes over. The full upstream defaults are listed in [Chapter 29](./29-terraform-variable-reference.md).

## How `--var-file` interacts with `config.yaml`

Both `awsbnkctl up` and `awsbnkctl plan/apply/destroy` accept the same `--var-file` flag terraform itself accepts (repeatable, later files win). The layering rule is:

```
1. config.yaml-derived terraform.tfvars        (written first by awsbnkctl)
2. ~/.awsbnkctl/<ws>/terraform.tfvars.user     (optional manual override)
3. --var-file <path>                           (CLI; repeatable)
```

Later layers override earlier. Concretely: `config.yaml`'s `cluster.node_desired_size: 2` writes `node_desired_size = 2` into the generated tfvars. If you then pass `--var-file ./bigger.tfvars` containing `node_desired_size = 5`, terraform sees `5`. The `config.yaml` value didn't get re-applied; `--var-file` wins.

The `terraform.tfvars.user` middle layer is for when you want a workspace-local override that survives across runs without modifying `config.yaml` — it's typically used for fields the YAML schema doesn't model.

AWS credentials are passed via the standard chain (env / shared-config / SSO / IMDS) — they **never** go through tfvars on disk. There's no `TF_VAR_aws_access_key_id`; the AWS provider in terraform reads the same credential chain awsbnkctl uses. The resolver chain in [Chapter 14](./14-credentials-resolver.md) is the only path.

## Editing by hand vs helpers

Several commands manage subsets of `config.yaml` so you don't have to:

| Subset | Helper |
|---|---|
| Whole file (interactive) | `awsbnkctl init` |
| `tf_source:` only | `awsbnkctl init --upgrade-tf` |
| `targets:` block | `awsbnkctl targets add/remove` |

When you do edit by hand, the load-time validators run on next `awsbnkctl` invocation:

- Plaintext-secret heuristic rejects `aws_access_key_id:`, `aws_secret_access_key:`, `aws_session_token:` fields.
- Workspace name validation runs on directory access (workspace names must match `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}`).
- YAML parse errors surface a line number.

If a hand edit breaks the file, every command that reads the workspace fails fast with the parse error path, so you'll know within one invocation.

## Worked example: bootstrap a workspace from scratch

End-to-end scenario: brand-new laptop, no `awsbnkctl` workspaces yet, AWS credentials in `~/.aws/credentials`. Goal: a usable workspace with the right region, instance types, and supply-chain bucket name.

```bash
# 1. awsbnkctl init — interactive bootstrap
$ awsbnkctl init
Workspace name [default]: dev
AWS region [us-west-2]:
AWS profile [default]: awsbnkctl-dev
  ✓ verified via sts:GetCallerIdentity (account: 123456789012, user: you)
Cluster name [bnk-quickstart]: dev-cluster
Node instance types [c5n.4xlarge]:
Node desired size [2]: 2
Supply-chain bucket [awsbnkctl-dev-cluster-supply]:
Local FAR archive [./keys/f5-far-auth-key.tgz]:
Local licence JWT [./keys/trial.jwt]:
✓ Created workspace "dev"
```

The resulting `~/.awsbnkctl/dev/config.yaml`:

```yaml
aws:
  region: us-west-2
  profile: awsbnkctl-dev
cluster:
  create: true
  name: dev-cluster
  kubernetes_version: "1.30"
  node_instance_types: ["c5n.4xlarge"]
  node_desired_size: 2
  vpc_mode: create
s3:
  bucket: awsbnkctl-dev-cluster-supply
  far_archive: ./keys/f5-far-auth-key.tgz
  licence_jwt: ./keys/trial.jwt
tf_source:
  type: embedded
```

That's the minimum. Everything else (`bnk:`, `test:`, `targets:`, `exec:`) is empty and falls through to defaults.

Verify the workspace is healthy before the first real `up`:

```bash
# 2. Sanity-check
$ awsbnkctl doctor -w dev
✓ terraform     1.6.2  on PATH
✓ helm          3.20.2 on PATH
✓ aws creds     resolved via shared-config (profile=awsbnkctl-dev)
✓ aws sts       OK (account: 123456789012, arn: arn:aws:iam::123456789012:user/you)
✓ aws region    us-west-2
✓ aws eks perms eks:DescribeCluster, eks:ListClusters granted
```

From here, `awsbnkctl up --auto -w dev` is the next step (see [Chapter 7 — Quick start](./07-quick-start.md)). You can layer on `bnk:`, `test:`, `targets:`, `exec:` blocks by hand-editing `config.yaml` whenever you need them.

## Cross-references

- [Chapter 13 — Terraform variables](./13-terraform-variables.md) — the layering between `config.yaml` and `terraform.tfvars`.
- [Chapter 14 — Credentials and the AWS resolver chain](./14-credentials-resolver.md) — how AWS credentials are resolved.
- [Chapter 15 — SSH targets](./15-ssh-targets.md) — the `targets:` block.
- [Chapter 17 — Execution backends](./17-execution-backends.md) — the `exec:` block.
- [Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) — the `s3:` block and the bucket layout `awsbnkctl up` populates.
- [Chapter 28 — Configuration reference](./28-configuration-reference.md) — auto-generated complete field list.
- [Chapter 29 — Terraform variable reference](./29-terraform-variable-reference.md) — the upstream HCL variables `config.yaml` translates to.
