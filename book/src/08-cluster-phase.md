# The cluster phase (cluster up/down)

> **Available in v1.x.** The `awsbnkctl cluster up` / `cluster down` / `cluster show` / `cluster register` / `bnk up` / `bnk down` subverb subtrees described in this chapter are **roadmap surface for v1.x**. The v0.9 binary ships the single-phase unscoped lifecycle (`awsbnkctl up` / `down` / `apply` / `plan` / `init` / `status`) only — running `awsbnkctl cluster --help` against the v0.9 binary returns `unknown command "cluster"`. This chapter documents the two-phase design that v1.x will lift across; for the v0.9 single-phase workflow see [Chapter 7 — Quick start](./07-quick-start.md) and [Chapter 10 — Deploying BNK trials](./10-deploying-bnk-trials.md). The two-phase split is gated on the operator-run PRD 07 spike (see [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deferred to post-v1.0") and the staff lift from `roksbnkctl/internal/cli/cluster.go` + `bnk.go` retargeted at the EKS shape.

A `awsbnkctl` workspace is **two phases on top of each other**: a durable **cluster phase** (the EKS cluster + cluster-shared services that take 30+ minutes to provision) and a short-lived **trial phase** (the BNK trial that iterates on top in 5-10 minutes). The cluster phase is exposed as its own command pair, `awsbnkctl cluster up` / `awsbnkctl cluster down`, so the cluster survives across many BNK trial cycles.

> **As of v1.1.0 (planned), this two-phase shape is the default for every new workspace.** A fresh `awsbnkctl up` provisions the cluster phase first, then the trial phase, against separate state directories. Tearing down only the trial — the common iteration case — uses [`awsbnkctl bnk down`](./10-deploying-bnk-trials.md#the-bnk-up--bnk-down-command-group) and leaves the cluster intact. The unscoped `up` / `down` verbs are now shape-aware composites that delegate to the right phase commands underneath.
>
> Workspaces created against v0.9 / v1.0.x that have cluster modules and trial modules in the same `terraform.tfstate` (the legacy single-state shape) keep working — `awsbnkctl up` and `down` continue to operate against them in-place, byte-for-byte the way they did in v0.9 / v1.0. See [§ Legacy single-state workspaces](#legacy-single-state-workspaces) at the bottom of the chapter to identify which shape a workspace is.

This chapter covers what each phase deploys, why the two state directories are separate, the `deploy_bnk=false` override that makes "cluster only" work, the `cluster-outputs.json` artefact written on success, a worked example, and the legacy single-state shape. The companion BNK-trial chapter, [Chapter 10](./10-deploying-bnk-trials.md), covers `awsbnkctl bnk up` / `bnk down` for the trial layer.

## What's deployed where

The bundled HCL has roughly two halves. The cluster phase owns the durable, cluster-scoped resources:

- The EKS cluster itself (VPC + subnets across at least 2 AZs + self-managed SR-IOV node group)
- The Multus + SR-IOV CNI + SR-IOV device plugin DaemonSets
- The supply-chain S3 bucket (server-side encrypted with `aws:kms`) — used by the BNK trial as its FAR image / licence / schematic store
- `cert-manager` (Helm release into the cluster)
- IAM OIDC provider + IRSA roles for FLO supply-chain access
- The bastion EC2 instance (an Amazon Linux instance in a public subnet, used by `--on jumphost`)

The trial phase owns the BNK-specific resources:

- F5 Lifecycle Operator (`flo`) Helm release
- `cne_instance` Kubernetes manifest
- BNK license + admin certs
- Various cluster-side bits: ServiceAccounts, RoleBindings, Secrets

Two-phase split: cluster up provisions the first list; `awsbnkctl up` (the trial) provisions the second.

```
┌─────────────────────────────────────────────────────────┐
│  cluster phase (durable, reused across many trials)     │
│    EKS cluster + VPC + SR-IOV self-managed node group   │
│    supply-chain S3 bucket + IAM OIDC + IRSA roles       │
│    cert-manager (Helm)                                  │
│    bastion EC2 instance                                 │
├─────────────────────────────────────────────────────────┤
│  trial phase (one trial — destroyed by `awsbnkctl down`)│
│    flo (F5 Lifecycle Operator)                          │
│    cne_instance                                         │
│    license / admin cert / SCC bindings                  │
└─────────────────────────────────────────────────────────┘
```

The split exists because EKS clusters take 20-30 minutes to provision (control plane + node group ASG fulfilment) and roughly $0.30/hour to run (EKS control plane) plus $0.86/hour per `c5n.4xlarge`. Re-creating the cluster every time you want to re-test a BNK trial is wasteful; reusing one cluster for many trials cuts iteration time from "an hour" to "a few minutes".

## The two state directories

To keep cluster state and trial state from tangling, `awsbnkctl` uses **separate Terraform state directories**:

```
~/.awsbnkctl/<workspace>/
  state/                   # BNK trial state — written by `awsbnkctl up/down`
    terraform.tfstate
    terraform.tfvars
  state-cluster/           # cluster phase state — written by `awsbnkctl cluster up/down`
    terraform.tfstate
    cluster-phase-override.tfvars
```

Each phase's commands read and write only their own state directory. Both phases use the same Terraform source (the bundled HCL) but with different effective tfvars — the trick is the `deploy_bnk` flag.

## The `deploy_bnk=false` override

The bundled HCL has a top-level `deploy_bnk` boolean. When `true`, the BNK trial modules (`flo`, `cne_instance`, `license`) run; when `false`, they're skipped and Terraform only provisions the cluster-phase resources.

`awsbnkctl cluster up` and `awsbnkctl cluster down` **force `deploy_bnk = false`** by writing a small auto-generated tfvars override into the cluster state directory:

```hcl
# ~/.awsbnkctl/<workspace>/state-cluster/cluster-phase-override.tfvars
# Generated by awsbnkctl. Do not edit by hand.
# Cluster-phase override: BNK trial modules (flo / cne_instance /
# license) are skipped. cert-manager and the testing jumphost still run
# — they're cluster-shared singletons that belong with the cluster.
deploy_bnk = false
```

This file is layered onto the var-file chain *after* user-supplied `--var-file` flags so the override always wins. The user's `terraform.tfvars` and `--var-file <path>` arguments still apply for everything else (region, RG, cluster name, worker count, …) — only `deploy_bnk` is forced.

`awsbnkctl up` doesn't write this override file; its tfvars chain leaves `deploy_bnk` at the upstream default (`true`), so the trial modules run.

## `cluster-outputs.json` — the cluster identity record

When `awsbnkctl cluster up` apply succeeds, it reads the relevant Terraform outputs (cluster name, region, VPC, subnets, OIDC provider, supply-chain bucket) and writes them to a workspace-scoped JSON file:

```
~/.awsbnkctl/<workspace>/cluster-outputs.json
```

Sample contents:

```json
{
  "cluster_name": "bnk-quickstart",
  "cluster_arn": "arn:aws:eks:us-west-2:123456789012:cluster/bnk-quickstart",
  "region": "us-west-2",
  "account_id": "123456789012",
  "vpc_id": "vpc-0abc1234567890def",
  "subnet_ids": ["subnet-0aaa", "subnet-0bbb", "subnet-0ccc"],
  "oidc_provider_arn": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABC123...",
  "supply_chain_bucket": "awsbnkctl-quickstart-supply",
  "cluster_endpoint": "https://ABCD1234.gr7.us-west-2.eks.amazonaws.com",
  "kubernetes_version": "1.30",
  "source": "cluster-up",
  "recorded_at": "2026-05-14T14:22:08Z"
}
```

The `source` field discriminates between `cluster-up` (we created it) and `cluster-register` (we discovered an existing cluster — see [Chapter 9](./09-registering-existing-cluster.md)). Subsequent commands read this file to learn the workspace's cluster identity without re-hitting AWS APIs.

`awsbnkctl cluster down` deletes the file as part of its post-destroy cleanup. `awsbnkctl cluster show` pretty-prints it for human readers:

```bash
awsbnkctl cluster show
workspace:           default
source:              cluster-up
recorded_at:         2026-05-14T14:22:08Z

cluster_name:        bnk-quickstart
cluster_arn:         arn:aws:eks:us-west-2:123456789012:cluster/bnk-quickstart
region:              us-west-2
account_id:          123456789012
kubernetes_version:  1.30
endpoint:            https://ABCD1234.gr7.us-west-2.eks.amazonaws.com

vpc_id:              vpc-0abc1234567890def
subnets:             subnet-0aaa, subnet-0bbb, subnet-0ccc
oidc_provider_arn:   arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABC123
supply_chain_bucket: awsbnkctl-quickstart-supply
```

## Worked example: cluster up → kubectl get nodes → cluster down

The cluster-only flow, end to end:

### Step 1 — `awsbnkctl init`

If you don't have a workspace yet, initialise one. This is the same `init` flow as the trial path; the cluster commands reuse the workspace's config.

```bash
awsbnkctl init
```

### Step 2 — `awsbnkctl cluster up --auto`

Provisions the cluster phase only:

```bash
awsbnkctl cluster up --auto
```

Sample output (heavily abridged):

```
→ terraform plan (cluster phase: deploy_bnk=false forced)
→ Layering user tfvars from ~/.awsbnkctl/default/state-cluster/cluster-phase-override.tfvars (overrides config.yaml-derived values)
→ terraform init
→ terraform apply
  module.eks_cluster.aws_eks_cluster.main: Creating...
  module.eks_cluster.aws_eks_cluster.main: Still creating... [10m elapsed]
  ...
  module.eks_cluster.aws_eks_cluster.main: Creation complete after 38m12s
  module.cert_manager.helm_release.cert_manager: Creation complete after 2m11s
  module.testing.tls_private_key.jumphost_shared_key: Creation complete after 0s
  module.testing.aws_instance.bastion: Creation complete after 1m48s

  Apply complete! Resources: 36 added, 0 changed, 0 destroyed.

✓ Wrote ~/.awsbnkctl/default/cluster-outputs.json
✓ Wrote /home/you/.kube/config (chmod 0600)
✓ Auto-registered target jumphost (169.45.91.177); use `awsbnkctl --on jumphost ...`
```

Roughly 42 resources land — the cluster phase is about half the size of a full BNK trial. Time-to-ready is dominated by the EKS control plane + node group ASG fulfilment; everything else after the cluster comes up is fast.

### Step 3 — verify the cluster works

The post-apply admin kubeconfig is fetched automatically (unless `--no-kubeconfig`). `kubectl get nodes` confirms reachability:

```bash
kubectl get nodes
# NAME                                       STATUS   ROLES    AGE   VERSION
# ip-10-0-1-23.us-west-2.compute.internal    Ready    <none>   3m    v1.30.2-eks-abcdef0
# ip-10-0-2-45.us-west-2.compute.internal    Ready    <none>   3m    v1.30.2-eks-abcdef0
```

Or, post-Sprint 2, the same thing through the internalised verb:

```bash
awsbnkctl k get nodes
```

`awsbnkctl status` reports cluster identity + reachability:

```bash
awsbnkctl status
Workspace:    default
Region:       us-west-2
Cluster:      bnk-quickstart  (attach existing)
TF source:    embedded@v0.9.0
Last apply:   2026-05-14 14:22:08 UTC  (3m ago)
Kubeconfig:   /home/you/.kube/config
Cluster:      2/2 nodes ready
```

### Step 4 — (optional) deploy a BNK trial on top

Now that the cluster is up, `awsbnkctl up` deploys a BNK trial onto it. It reads `cluster-outputs.json` and reuses the cluster:

```bash
awsbnkctl up --auto
```

See [Chapter 10 — Deploying BNK trials](./10-deploying-bnk-trials.md) for the trial-phase walkthrough. You can run `up` / `down` many times against the same cluster — each cycle is ~5 minutes rather than the ~50 minutes of a fresh-cluster run.

### Step 5 — `awsbnkctl cluster down --auto`

Tear down the cluster phase. In v1.1.0 `cluster down` is **strictly** scoped: it refuses with a hard error (rather than the v1.0.x warning-but-prompt) on any workspace whose trial state is non-empty, so an out-of-order destroy can't accidentally orphan BNK resources. Destroy the trial first with `awsbnkctl bnk down` (or `awsbnkctl down` for both at once); see [Chapter 11](./11-tearing-down.md) for the full refusal catalogue.

```bash
awsbnkctl cluster down --auto
```

Sample output:

```
→ terraform destroy (cluster phase)
  module.testing.aws_instance.bastion: Destroying...
  module.cert_manager.helm_release.cert_manager: Destroying...
  module.eks_cluster.aws_eks_cluster.main: Destroying...
  module.eks_cluster.aws_eks_cluster.main: Still destroying... [5m elapsed]
  module.eks_cluster.aws_eks_cluster.main: Destruction complete after 8m16s

  Destroy complete! Resources: 36 destroyed.
```

Post-destroy, `cluster-outputs.json` is deleted. The workspace directory and its `config.yaml` survive — re-running `cluster up` against the same workspace re-creates the cluster with the same name and region.

## Why split cluster from trial?

Two-phase is the default because the cost of conflating them is concrete. EKS clusters take 30-50 minutes to provision and bill at roughly $0.30/hour; a BNK trial on top takes 5-10 minutes. Iterating on the trial — different `flo` versions, different `cne_instance` shapes, license bundle revisions — happens far more often than iterating on the cluster underneath. Splitting state means a `bnk down` / `bnk up` cycle is a five-minute round-trip instead of an hour.

Three scenarios this shape unlocks:

1. **Many BNK trial iterations on one cluster.** Run `cluster up` once, then loop `bnk up` / `bnk down` against the same cluster until you've covered all the trial permutations. Then `cluster down` once when you're finished. This is the headline win of the v1.1.0 surface — see [Chapter 10 §"Worked example — iterating on a BNK trial"](./10-deploying-bnk-trials.md#worked-example--iterating-on-a-bnk-trial).

2. **Pre-provisioning for a workshop or demo.** You want the cluster ready and warm before the demo starts; you'll deploy the BNK trial live in front of the audience. `cluster up` the night before; `bnk up` during the demo.

3. **Decoupling cluster lifecycle from trial lifecycle.** A long-lived cluster used by multiple team members, where one person owns the cluster phase and others own the BNK trials. Cluster-phase outputs live in `cluster-outputs.json`; trials read it. Each trial can `bnk up` / `bnk down` without affecting the cluster.

For workspaces that just want "create a cluster, deploy BNK on it, test, tear it all down", the unscoped `awsbnkctl up` / `awsbnkctl down` are still the right verbs — in v1.1.0 they're shape-aware composites that drive the cluster + trial steps in the right order without you having to think about it.

## Legacy single-state workspaces

Workspaces created against v1.0.x predate the split. Their `terraform.tfstate` under `~/.awsbnkctl/<workspace>/state/` contains **both** the cluster modules (`module.eks_cluster`, `module.cert_manager`, `module.testing`) and the trial modules (`module.flo`, `module.cne_instance`, `module.license`) in one file; `state-cluster/` either doesn't exist or is empty.

`awsbnkctl` calls this shape `LegacySingle` and identifies it by walking the trial state's resource list for cluster-module addresses. To check a workspace's shape from the outside, look at the state directories:

```
$ ls ~/.awsbnkctl/<workspace>/
config.yaml  state/  state-cluster/    # split (v1.1.0+) or cluster-only

$ ls ~/.awsbnkctl/<workspace>/
config.yaml  state/                    # legacy single-state, or empty
```

A `state/terraform.tfstate` that contains `module.eks_cluster` and friends is legacy single-state; a `state-cluster/terraform.tfstate` with content is the split shape.

The v1.1.0 binary handles both shapes:

- **Legacy single-state workspaces**: `awsbnkctl up` and `awsbnkctl down` operate monolithically the way they did in v1.0 — same plan output, same resource count, same byte-for-byte behaviour. The phase-scoped commands (`cluster up`/`down`, `bnk up`/`down`) **refuse** with a message pointing you back at the unscoped lifecycle verbs.
- **Split workspaces (the new default)**: `up` / `down` are shape-aware composites that delegate to the phase commands underneath; `cluster up`/`down` and `bnk up`/`down` work directly.

The refusal messages on a legacy workspace look like:

```
$ awsbnkctl -w legacy-eks cluster down
this workspace is legacy single-state; cluster and BNK trial share one state. Use `awsbnkctl down` to tear down both, or migrate the state first

$ awsbnkctl -w legacy-eks bnk down
this workspace is legacy single-state; `bnk down` can't isolate the trial phase. Use `awsbnkctl down` to tear down both, or migrate the state first
```

The refusals print as a single line each — wrapping is a function of your terminal width. Grep against any of the inline punctuation (e.g. `\`bnk down\` can't isolate`) lands a clean match.

There is no automatic state-migration command yet. The refusal text references migration ("or migrate the state first") because a future `awsbnkctl migrate` is planned, but until it ships, legacy workspaces stay on the unscoped `up` / `down` flow. See [Chapter 11 §"The phase-aware decision tree"](./11-tearing-down.md#the-phase-aware-decision-tree) for the full destruction-time decision matrix.

## Cross-references

- [Chapter 9 — Registering an existing cluster](./09-registering-existing-cluster.md) — the alternative to `cluster up` when you already have an EKS cluster you want `awsbnkctl` to manage.
- [Chapter 10 — Deploying BNK trials](./10-deploying-bnk-trials.md) — `awsbnkctl up` and the `bnk up` / `bnk down` command group; the dispatch matrix; iteration walkthrough.
- [Chapter 11 — Tearing down](./11-tearing-down.md) — phase-aware decision matrix and the refusal-message catalogue.
- [Chapter 24 — Day-2 ops](./24-day-2-ops.md) — `awsbnkctl k get` / `apply` / `logs` for working against the cluster after either phase.
