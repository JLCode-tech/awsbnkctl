# Registering an existing cluster

> **Available in v1.x.** The `awsbnkctl cluster register <name>` verb described in this chapter is **roadmap surface for v1.x** — it does not ship in the v0.9 binary. Running `awsbnkctl cluster register --help` against the v0.9 binary returns `unknown command "cluster"`. The v0.9 path for working against an EKS cluster you didn't provision via `awsbnkctl` is to point `--var-file` at a tfvars file that names the cluster + region + supply-chain bucket and let the bundled HCL's `eks_cluster` module skip cluster creation (the module is idempotent on existing clusters at the AWS API level). The chapter below documents the cleaner v1.x register surface that lifts the metadata-only flow from `roksbnkctl/internal/cli/cluster_register.go` and retargets it at the AWS SDK shape. Tracked under [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deferred to post-v1.0".

`awsbnkctl cluster register <name>` wires `awsbnkctl` up to an EKS cluster that already exists in your AWS account — one you didn't provision via [`cluster up`](./08-cluster-phase.md). After a successful register, the workspace behaves exactly as if you'd done `cluster up`: `awsbnkctl up` deploys BNK trials onto the registered cluster, `awsbnkctl down` tears those trials down, `awsbnkctl status` reports the cluster's identity, and so on.

This chapter covers when registration is the right answer, what input is required vs auto-discovered, the supply-chain bucket naming convention, the `cluster-outputs.json` write, and a worked example.

## When to use this

`cluster register` is the answer when **all** of these are true:

- An EKS cluster already exists in the AWS account.
- You have IAM access to the cluster's VPC + EKS control plane.
- You want `awsbnkctl` to deploy BNK trials onto that cluster.
- You don't want `awsbnkctl` to own the cluster's lifecycle (it shouldn't be `terraform destroy`-able from your workstation).

Common scenarios:

1. **Your team operates the EKS cluster centrally.** A platform team provisioned the cluster via their own Terraform / Pulumi / `eksctl`; you just want to deploy BNK trials onto it. Register it; deploy trials; tear them back down. The cluster itself stays under the platform team's ownership.

2. **You're attaching to an existing demo cluster.** A workshop hosts a shared cluster that participants attach to. Each participant registers it in their own workspace and deploys their own trial — trials are isolated by namespace under the same cluster.

3. **You provisioned the cluster manually for testing.** You created a one-off cluster via `eksctl create cluster ...` and want to move forward with `awsbnkctl` rather than re-creating it.

If none of those apply — i.e. you want `awsbnkctl` to own cluster lifecycle end-to-end — use `cluster up` instead. Register and `cluster up` are mutually exclusive per workspace; the second one wins.

## Required input vs auto-discovery

`cluster register` takes one positional argument (the cluster name) and a few optional flags.

```bash
awsbnkctl cluster register <cluster-name> [--region <region>] [--supply-chain-bucket <bucket>]
```

Everything else is **auto-discovered** via the AWS SDK:

| Field | Source |
|---|---|
| `cluster_arn` | `eks:DescribeCluster` |
| `region` | resolved from `--region`, `AWS_REGION`, or workspace config |
| `account_id` | `sts:GetCallerIdentity` |
| `vpc_id` | from the cluster's `ResourcesVpcConfig.VpcId` |
| `subnet_ids` | from `ResourcesVpcConfig.SubnetIds` |
| `oidc_provider_arn` | from `Cluster.Identity.Oidc.Issuer` matched against `iam:ListOpenIDConnectProviders` |
| `cluster_endpoint` | from `Cluster.Endpoint` |
| `kubernetes_version` | from `Cluster.Version` |
| `supply_chain_bucket` | discovered via the supply-chain bucket lookup (see below) |

The cluster lookup goes through the same EKS endpoint `aws eks describe-cluster` uses — no host `aws` install required. If the named cluster doesn't exist in the account / region, the call returns a clear `no cluster named <foo> in region <region>` error rather than a 404 stack trace.

EKS requires the cluster to span **at least two AZs**. The `cluster register` lookup verifies this and refuses to write a record otherwise:

```
Error: cluster "single-az" has subnets in only one AZ — awsbnkctl requires at least two AZs for BNK deployments
```

## The supply-chain bucket naming convention

`awsbnkctl up` needs an S3 bucket to act as the supply chain for FAR images, JWT licences, and any other BNK artefacts. `cluster register` verifies that this bucket exists at registration time so a later `up` doesn't fail mid-apply with a missing-bucket error.

### Default convention

The bundled HCL falls back to **`awsbnkctl-<cluster-name>-supply`** if the user's tfvars don't override `supply_chain_bucket_name`. So `cluster register` defaults to looking up `awsbnkctl-<cluster-name>-supply`:

```bash
# Cluster name: "shared-eks" → expects bucket "awsbnkctl-shared-eks-supply"
awsbnkctl cluster register shared-eks
```

### Override with `--supply-chain-bucket`

If your team set `supply_chain_bucket_name` to something else in their tfvars (or named the bucket via the AWS console with a different convention), pass `--supply-chain-bucket <name>`:

```bash
awsbnkctl cluster register shared-eks \
  --supply-chain-bucket f5-bnk-shared-supply
```

The bucket name must match exactly — S3 bucket names are globally unique and case-sensitive (well, lowercase-only per AWS rules), so `F5-BNK-Shared-Supply` won't resolve.

### What if the bucket doesn't exist yet?

`cluster register` errors out:

```
Error: supply-chain bucket "awsbnkctl-shared-eks-supply" not found in account 123456789012:
  Either run `awsbnkctl cluster up` to create it, or pass --supply-chain-bucket <name>
  if your tfvars uses a different supply_chain_bucket_name
```

You have two options:

1. **Create the bucket** in the AWS console with the conventional name, with server-side encryption (`aws:kms`) and access blocked from the public, then re-run register. The bucket can be empty — `awsbnkctl up` will populate it with the FAR archive + JWT licence on its first apply.

2. **Use a different name** that already exists in the account, via `--supply-chain-bucket <name>`.

Either way, `cluster register` won't write `cluster-outputs.json` until both the cluster and its supply-chain bucket exist.

## The `cluster-outputs.json` write

On success, `cluster register` writes `~/.awsbnkctl/<workspace>/cluster-outputs.json` — the same file `cluster up` writes. The contents look identical except for one field:

```json
{
  "cluster_name": "shared-eks",
  "cluster_arn": "arn:aws:eks:us-west-2:123456789012:cluster/shared-eks",
  "region": "us-west-2",
  "account_id": "123456789012",
  "vpc_id": "vpc-0abc1234567890def",
  "subnet_ids": ["subnet-0aaa", "subnet-0bbb", "subnet-0ccc"],
  "oidc_provider_arn": "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABC123",
  "supply_chain_bucket": "awsbnkctl-shared-eks-supply",
  "cluster_endpoint": "https://ABCD1234.gr7.us-west-2.eks.amazonaws.com",
  "kubernetes_version": "1.30",
  "source": "cluster-register",
  "recorded_at": "2026-05-14T14:22:08Z"
}
```

The `source` field is `cluster-register` (vs `cluster-up` for self-provisioned clusters). Downstream commands that care about provenance — for example, `awsbnkctl cluster down` refuses to destroy a `cluster-register`-sourced cluster — read this field.

## Worked example: register shared-eks

The full flow for attaching to a hypothetical `shared-eks` cluster.

### Step 1 — create or pick a workspace

```bash
awsbnkctl ws new shared
awsbnkctl init -w shared
# (interactive — fill in region as us-west-2; cluster.name = shared-eks; cluster.create = false)
```

You can also run `cluster register` against the current workspace; the `-w` is just for clarity.

### Step 2 — `cluster register`

```bash
awsbnkctl -w shared cluster register shared-eks
```

Sample output:

```
→ Looking up cluster "shared-eks" in us-west-2
✓ Cluster shared-eks (arn:aws:eks:us-west-2:123456789012:cluster/shared-eks) — version 1.30, status ACTIVE
✓ VPC vpc-0abc1234567890def (3 subnets across us-west-2a, us-west-2b, us-west-2c)
✓ OIDC provider arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABC123
→ Verifying supply-chain bucket "awsbnkctl-shared-eks-supply"
✓ Bucket awsbnkctl-shared-eks-supply exists (region us-west-2, KMS-encrypted)
✓ Wrote ~/.awsbnkctl/shared/cluster-outputs.json
```

If the bucket naming was non-conventional:

```bash
awsbnkctl -w shared cluster register shared-eks \
  --supply-chain-bucket f5-bnk-shared-supply
```

### Step 3 — verify with `cluster show`

```bash
awsbnkctl -w shared cluster show
workspace:           shared
source:              cluster-register
recorded_at:         2026-05-14T14:22:08Z

cluster_name:        shared-eks
cluster_arn:         arn:aws:eks:us-west-2:123456789012:cluster/shared-eks
region:              us-west-2
account_id:          123456789012
kubernetes_version:  1.30
endpoint:            https://ABCD1234.gr7.us-west-2.eks.amazonaws.com

vpc_id:              vpc-0abc1234567890def
subnets:             subnet-0aaa, subnet-0bbb, subnet-0ccc
oidc_provider_arn:   arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/ABC123
supply_chain_bucket: awsbnkctl-shared-eks-supply
```

### Step 4 — generate the kubeconfig

`cluster register` does **not** automatically generate the kubeconfig — it's a metadata-only operation. Grab it explicitly:

```bash
awsbnkctl -w shared kubeconfig --download
# → Generating kubeconfig for "shared-eks"
# ✓ Wrote /home/you/.kube/config (12345 bytes)
```

Equivalent host-side: `aws eks update-kubeconfig --name shared-eks --region us-west-2`.

### Step 5 — use the cluster as if you'd done `cluster up`

From here, the workflow is identical to a self-provisioned cluster:

```bash
# Verify reachability
awsbnkctl -w shared k get nodes

# Deploy a BNK trial onto it
awsbnkctl -w shared up --auto

# Tear the trial back down (cluster survives)
awsbnkctl -w shared down --auto
```

`awsbnkctl up` reads `cluster-outputs.json` and uses the cluster identity directly — no need to re-state cluster name / region / VPC in the trial's tfvars.

## When register isn't enough

Some scenarios where `cluster register` won't get you over the line:

- **The cluster is in a different AWS account.** AWS credentials are account-scoped; you'd need credentials for the cluster's account, or cross-account `sts:AssumeRole` configured. `cluster register` doesn't cross account boundaries by default.
- **The cluster is private (no public API endpoint).** EKS supports private-only clusters where the API endpoint is reachable only from inside the VPC. `awsbnkctl up` needs to apply Helm charts and Kubernetes manifests against the API; if the API is private, route the apply through `--on jumphost` ([Chapter 16](./16-on-flag-ssh-jumphosts.md)).
- **The cluster lacks SR-IOV-capable nodes.** BNK trials need at least one node with SR-IOV VFs advertised (`intel.com/sriov` or equivalent). An existing cluster might not have this — registering still works but `cne_instance` reconciliation will hang in `Pending` until you add an SR-IOV-capable node group. [Chapter 33](./33-data-plane-decision.md) walks the data-plane decision.
- **IRSA isn't wired.** The supply-chain bucket policy must include the FLO IRSA role ARN, and the OIDC provider must be registered in IAM. If the existing cluster's OIDC provider isn't an IAM OIDC provider yet, `cluster register` reports the gap and links to [Chapter 25 §"IRSA trust chain"](./25-cos-supply-chain.md#irsa-trust-chain) for the fix.

For the first three, you may still register and operate manually; for the IRSA case, run `awsbnkctl cluster up --modules iam_irsa` to provision just the IAM pieces against the registered cluster.

## Re-registering and unregistering

To **re-register** with new data (e.g. you renamed the supply-chain bucket, or the API endpoint moved from public to public+private), just run `cluster register` again — it overwrites `cluster-outputs.json` in place.

To **unregister** without destroying anything, delete the file directly:

```bash
rm ~/.awsbnkctl/shared/cluster-outputs.json
```

The workspace's `config.yaml` and `state/` survive; only the cluster identity record is removed. The next `awsbnkctl up` will fail with `workspace has no cluster-outputs.json` until you either re-register or run `cluster up`.

There's deliberately **no** `awsbnkctl cluster unregister` command. Deleting the JSON is a single-file operation that doesn't deserve its own subcommand, and the absence of one nudges users toward "destroy the trial first, then deal with the cluster identity" rather than "unregister without thinking about the consequences".

## Cross-references

- [Chapter 8 — The cluster phase](./08-cluster-phase.md) — the alternative when you want `awsbnkctl` to provision the cluster.
- [Chapter 10 — Deploying BNK trials](./10-deploying-bnk-trials.md) — what `awsbnkctl up` does on top of a registered (or `cluster up`'d) cluster.
- [Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) — the supply-chain bucket and IRSA trust chain that `--supply-chain-bucket` points at.
