# Tearing down

> **Available in v1.x.** The `awsbnkctl bnk down` and `awsbnkctl cluster down` phase-scoped destroy verbs described in this chapter are **roadmap surface for v1.x** — only `awsbnkctl down` ships in the v0.9 binary. The v0.9 destroy story is the single-phase `awsbnkctl down` (drives a monolithic `terraform destroy` against the workspace's only state directory), plus `awsbnkctl ws delete` for local cleanup; the three-verb / phase-aware refusal catalogue documented here lands when the v1.x two-phase split (see [Chapter 8 §"Available in v1.x"](./08-cluster-phase.md)) ships. Until then, on a v0.9 workspace `down` is byte-for-byte equivalent to the legacy single-state branch of the decision tree below. Tracked under [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deferred to post-v1.0".

`awsbnkctl down`, `awsbnkctl bnk down`, and `awsbnkctl cluster down` are the three destroy verbs — the inverses of [`up`](./10-deploying-bnk-trials.md), [`bnk up`](./10-deploying-bnk-trials.md#the-bnk-up--bnk-down-command-group), and [`cluster up`](./08-cluster-phase.md) respectively. This chapter covers what each one removes, the ordering constraint between them, the refusal messages you'll hit if you ask for the wrong one, what survives a destroy, the `--auto` flag for non-interactive runs, and the workspace-cleanup story.

## The phase-aware decision tree

Which verb do you want? The shape of your workspace and your intent both matter. Start here:

```
I want to keep the cluster and just tear down the BNK trial:
    → awsbnkctl bnk down

I want to tear down everything (cluster + trial):
    → awsbnkctl down

I want to tear down only the cluster (no trial currently deployed):
    → awsbnkctl cluster down

I'm on a v1.0.x workspace (cluster + trial in one state):
    → awsbnkctl down       (tears down everything in one shot)
    → see Chapter 8 §"Legacy single-state workspaces" to confirm your shape
```

Quick shape check: `ls ~/.awsbnkctl/<workspace>/` — if you see `state-cluster/`, you're on the v1.1.0 split shape; if you see only `state/`, you're on legacy single-state.

The big rule, stated up front: **destroy in reverse of create**. Trial first (`bnk down`), cluster second (`cluster down`). The unscoped `awsbnkctl down` does this ordering for you — on a split workspace it runs the trial destroy first and then the cluster destroy. On a legacy single-state workspace it runs a monolithic destroy (the v1.0.x behaviour, byte-for-byte). Either way you don't have to think about ordering; `down` is the safe default.

The phase-scoped commands (`bnk down`, `cluster down`) are the precision tools — they let you keep one phase across many cycles of the other. They also **refuse loudly** if you ask them to do something that would orphan resources or that the shape doesn't allow. The full refusal catalogue is in [§"Refusal messages catalogue"](#refusal-messages-catalogue) below; the rule of thumb is that the error message always names the verb that would actually work.

## The three destroys

There are three teardown verbs matching the three slices of state:

### `awsbnkctl down` — shape-aware composite

The unscoped `down` is a **shape-aware composite** in v1.1.0: it detects the on-disk shape of the workspace and dispatches to the right phase destroys in the right order.

```bash
awsbnkctl down
```

| Workspace shape | `down` does |
|---|---|
| Split (cluster + trial) | trial destroy → cluster destroy |
| ClusterOnly (only cluster applied) | cluster destroy |
| LegacySingle (v1.0.x — both in one state) | monolithic destroy (v1.0.x behaviour, byte-for-byte) |
| Empty | error: `nothing to destroy in this workspace` |

This is the safe default — `down` always does the right thing regardless of shape, and it's the only verb you can run on a legacy single-state workspace.

### `awsbnkctl bnk down` — destroy the BNK trial only

New in v1.1.0. Tears down everything the trial phase created — the `flo` Helm release, `cne_instance`, the license module, cluster-side ServiceAccounts / RoleBindings / IRSA annotations, and the null_resources that bootstrap admin tokens — and leaves the cluster running.

```bash
awsbnkctl bnk down
```

What survives:

- The EKS cluster itself
- cert-manager
- The supply-chain S3 bucket and its contents (FAR archive, licence JWT)
- The IAM OIDC provider + IRSA roles (cluster-scoped)
- The bastion EC2 instance
- All cluster-phase Terraform state under `state-cluster/`
- `cluster-outputs.json` (the cluster is still registered)
- The workspace's `config.yaml`

Roughly **41 resources destroyed** on a clean trial-only `bnk down`. Time is dominated by Helm's pre-delete hooks and the cne_instance finaliser unwind — usually 2-5 minutes total.

`bnk down` **refuses** on Empty, ClusterOnly, and LegacySingle workspaces — there's nothing to destroy on the first two, and the trial-only isolation isn't possible on the third. See [§"Refusal messages catalogue"](#refusal-messages-catalogue) for the exact text.

### `awsbnkctl cluster down` — destroy the cluster phase

Tears down the cluster + cluster-shared services: the EKS cluster, VPC + subnets (if `cluster up` created them), supply-chain S3 bucket, IAM OIDC provider + IRSA roles, cert-manager Helm release, and the bastion EC2 instance.

```bash
awsbnkctl cluster down
```

What survives:

- The workspace's `config.yaml`
- `~/.awsbnkctl/<workspace>/state/` (now empty of resources but the directory persists)
- `~/.awsbnkctl/<workspace>/state-cluster/` Terraform state files (the cluster-side state itself is empty; the directory and `terraform.tfstate` persist)

Roughly **36 resources destroyed**. The EKS cluster destroy alone is 5-10 minutes; everything else is fast.

The post-destroy cleanup deletes `cluster-outputs.json` automatically — the workspace no longer has a registered cluster.

## Order matters: trial first, then cluster

The upstream HCL's resource graph requires this ordering. The trial-phase resources have implicit dependencies on cluster-phase resources (they live *in* the cluster, after all), and Terraform's destroy graph traverses dependencies in reverse. If the cluster phase tries to destroy first, the trial phase's resources are still there — finalisers block the destroy of the cluster's namespaces, the IRSA-annotated ServiceAccounts reference IAM roles that are in the way, and so on.

In v1.1.0 `awsbnkctl cluster down` enforces this ordering with a **hard refusal**: if the trial state has any resources in it, `cluster down` errors out and points you at `bnk down` (or `down`) instead. The v1.0.x "warning-but-prompt" behaviour is gone — even `--auto` won't bypass the guard, because correctness, not confirmation, is the issue. The full refusal text:

```
$ awsbnkctl cluster down
BNK trial state exists in this workspace; run `awsbnkctl bnk down` first
(or `awsbnkctl down` to tear down both phases)
```

So in practice, **always destroy the trial before the cluster**. The unscoped `down` does this ordering for you on a split workspace; the phase-scoped pair is `bnk down` then `cluster down`.

The clean teardown sequence — split workspace, explicit phase commands:

```bash
# 1. Destroy the BNK trial
awsbnkctl bnk down --auto

# 2. Now safe to destroy the cluster phase
awsbnkctl cluster down --auto

# 3. (Optional) Delete the workspace itself
awsbnkctl ws delete <name> --force
```

Or the one-shot equivalent:

```bash
# 1. Tear down both phases in order
awsbnkctl down --auto

# 2. (Optional) Delete the workspace itself
awsbnkctl ws delete <name> --force
```

If you `awsbnkctl up` against a registered cluster (one you didn't `cluster up` yourself), step 2 doesn't apply — the cluster wasn't yours to destroy. Just `bnk down` the trial and stop there, then optionally unregister by deleting `cluster-outputs.json`.

## Refusal messages catalogue

The phase-scoped destroy verbs refuse loudly when the shape doesn't allow what you've asked for. Every refusal names the verb that would actually work. If you hit one in the wild, grep your terminal output for the message text and you should land here:

| Command + shape | Refusal text | Resolution |
|---|---|---|
| `bnk down` on **LegacySingle** | `this workspace is legacy single-state; `bnk down` can't isolate the trial phase. Use `awsbnkctl down` to tear down both, or migrate the state first` | Use `awsbnkctl down`; the legacy state has the trial and cluster in one file, so a trial-only destroy isn't possible. See [Chapter 8 §"Legacy single-state workspaces"](./08-cluster-phase.md#legacy-single-state-workspaces). |
| `bnk down` on **Empty** or **ClusterOnly** | `no BNK trial state to destroy in this workspace` | Nothing to do — no trial is deployed. If you want to destroy the cluster, use `awsbnkctl cluster down`. |
| `cluster down` on **LegacySingle** | `this workspace is legacy single-state; cluster and BNK trial share one state. Use `awsbnkctl down` to tear down both, or migrate the state first` | Use `awsbnkctl down`. |
| `cluster down` on **Split** | ``BNK trial state exists in this workspace; run `awsbnkctl bnk down` first (or `awsbnkctl down` to tear down both phases)`` | Run `bnk down` first to remove the trial, then `cluster down` for the cluster — or `awsbnkctl down` to do both in one shot. |
| `cluster down` on **Empty** | `nothing to destroy in this workspace` | Nothing to do — the cluster hasn't been provisioned. |
| `down` on **Empty** | `nothing to destroy in this workspace` | Nothing to do — the workspace has no state. |
| `cluster up` on **LegacySingle** | ``this workspace was provisioned with v1.0.x single-state — its cluster lives in the trial state file. Use `awsbnkctl up` to operate on it, or migrate the state to two-phase shape first`` | Use `awsbnkctl up`. The cluster already exists in the trial state; applying the cluster phase separately would create a second one. |
| `bnk up` on **LegacySingle** | ``this workspace is legacy single-state; `bnk up` can't isolate the trial phase. Use `awsbnkctl up` for in-place behavior, or migrate the state first`` | Use `awsbnkctl up`. |

The "migrate the state first" references in two of the messages describe a future `awsbnkctl migrate` command that does not exist in v1.1.0. The refusals point at it so the wording stays valid once migrate ships; until then, the unscoped `up` / `down` is the working alternative for legacy workspaces.

## What survives a destroy

The contract: **`awsbnkctl` never destroys local state without explicit consent**, and never destroys cloud resources outside its Terraform state.

After a successful `down`:

| Survives | Where |
|---|---|
| Workspace config | `~/.awsbnkctl/<name>/config.yaml` |
| Workspace directory + state files | `~/.awsbnkctl/<name>/` (empty `state/`; `state-cluster/` untouched if `cluster down` not run) |
| OS keychain entries | awsbnkctl doesn't manage AWS credentials in the keychain (the AWS standard chain reads `~/.aws/credentials` / SSO cache directly), so there's nothing to clean up |
| `~/.kube/config` | left in place |
| The cluster (if only trial was destroyed) | runs and bills as before |
| The supply-chain S3 bucket's contents | FAR archive, JWT licence — survive cluster destroy too if the bucket was created outside the bundled HCL |
| `~/.awsbnkctl/known_hosts` | SSH host keys persist; deleting a workspace does not clear them |

Re-running `up` against a `down`'d workspace re-creates everything from scratch. The workspace's `config.yaml` is preserved precisely so this re-create can use the same inputs without re-prompting.

The S3 bucket point is worth highlighting: the bundled HCL provisions the bucket and writes the FAR archive + JWT licence into it on `up`. When `cluster down` destroys the bucket, the objects go with it (the bucket is configured to allow `force_destroy` for this reason) — but if the bucket was created out-of-band (e.g. by a registered cluster's owner) and `awsbnkctl` is just attaching, then `cluster down` doesn't apply and the bucket survives. There's also the ENI / LoadBalancer cleanup wrinkle: see [§"Orphan ENIs and LoadBalancers after destroy"](#orphan-enis-and-loadbalancers-after-destroy) below.

## `--auto` for non-interactive runs

All three destroy commands prompt for confirmation by default:

```
$ awsbnkctl down
This will destroy workspace "default"'s resources.
Continue? [y/N]: 
```

```
$ awsbnkctl bnk down
This will destroy the BNK trial for workspace "default". The cluster phase
will remain in place — run `awsbnkctl cluster down` to remove it too.
Continue? [y/N]: 
```

```
$ awsbnkctl cluster down
This will destroy the cluster phase for workspace "default" (EKS + VPC + supply-chain S3 + IRSA + cert-manager + bastion).
Continue? [y/N]: 
```

`--auto` skips the prompt — required for CI / scripted pipelines:

```bash
awsbnkctl down --auto
awsbnkctl bnk down --auto
awsbnkctl cluster down --auto
```

`--auto` does **not** override the shape-based refusals (see [§"Refusal messages catalogue"](#refusal-messages-catalogue) above) — those are correctness guards, not confirmation prompts. If trial state is present, `cluster down --auto` still refuses; on a legacy single-state workspace, `bnk down --auto` and `cluster down --auto` still refuse.

## Like `up`, transient errors retry

`down` doesn't share `up`'s explicit retry-on-transient-error logic, but Terraform's destroy is naturally idempotent: re-running `down` after a partial destroy picks up where the previous run left off. If you see a transient network error during destroy, just re-run:

```bash
awsbnkctl down --auto
# (some resources destroyed, then transient error)

awsbnkctl down --auto
# (picks up where it left off, completes)
```

The same applies to `cluster down`. EKS cluster destroy specifically can take longer than expected when the master is propagating its delete state — wait a few minutes and re-try if you see master-not-found errors.

## Cleaning up workspaces

A successful `down` leaves the workspace directory in place. You usually want to clean that up too:

```bash
awsbnkctl ws delete <name> --force
```

Two safety rails on `ws delete`:

- **Refuses to delete the current workspace.** Use the [parking-lot pattern](./06-workspaces.md#the-parking-lot-pattern) if you need to drop your current workspace.
- **Refuses if Terraform state still lists resources** (unless `--force`). Catches the case where you forgot to run `down` first.

The `--force` flag overrides both checks — but if you `ws delete --force` a workspace that still has provisioned cloud resources, you'll have leaked them. There's no auto-recovery; you'd need to find them via the AWS console and delete them by hand.

The full clean-as-you-go pattern from `scripts/e2e-test.sh` (Phase D destroys; Phase H parks and deletes):

```bash
# 1. Destroy the trial
awsbnkctl down --auto

# 2. Destroy the cluster phase
awsbnkctl cluster down --auto

# 3. Park the current-workspace pointer somewhere harmless
awsbnkctl ws new e2e-cleanup
awsbnkctl ws use e2e-cleanup

# 4. Now the original workspace is no longer current — safe to delete
awsbnkctl ws delete default --force

# 5. (Optional) clean up the parking lot too
awsbnkctl ws delete e2e-cleanup --force
```

Step 3-5 is the parking-lot pattern from [Chapter 6](./06-workspaces.md). It's specifically necessary when the workspace you want to delete is currently the active one — `ws delete` refuses to remove the current workspace because that would leave a dangling `current_workspace` pointer.

## Cost note: an undestroyed cluster keeps billing

EKS clusters bill at roughly **$0.30/hour** for the control plane plus $0.86/hour for each `c5n.4xlarge` worker. Two workers + control plane = ~$2.02/hour, ~$48/day. NLBs add ~$0.025/hour per load balancer; cross-AZ data transfer adds usage-based cost; S3 storage is pennies. A forgotten cluster can rack up real cost over a weekend.

To verify what's still running in your account:

1. **AWS Console → EKS → Clusters** — every cluster, regardless of state.
2. **AWS Console → VPC** — VPCs, NAT gateways, ENIs left over after a partial destroy.
3. **AWS Console → S3 → Buckets** — supply-chain buckets that survived if `force_destroy` was off.
4. **AWS Cost Explorer** — exhaustive view of what's billing, filterable by region / service / tag.

If you find a leaked cluster from a past `awsbnkctl` run, the right move is to re-attach to it via `awsbnkctl cluster register <name>` and then `cluster down --auto` — `awsbnkctl` cleans up cleanly when it has the cluster in its state. Manually deleting via the console works too but leaves dangling VPCs and security groups that the bundled HCL would have cleaned up.

`awsbnkctl status` and `awsbnkctl cluster show` both report the cluster identity recorded in `cluster-outputs.json`, but they don't probe for "are there other clusters in this account?" — that's deliberately not their job. The AWS Console (or `aws eks list-clusters` across regions) is the canonical source of truth for what's billing.

## Workspace deletion ≠ destroy

A subtle but important distinction. `awsbnkctl ws delete` removes the **local** workspace directory and the OS-keychain API key entry. It does **not** destroy any cloud resources. If you `ws delete --force` without first running `down` / `cluster down`, the cloud resources keep running and you've lost the local Terraform state that `awsbnkctl` would use to destroy them.

In that scenario, recovery is:

1. Find the leaked cluster in the AWS Console.
2. Recreate the workspace: `awsbnkctl init -w recovery`.
3. Register the existing cluster: `awsbnkctl cluster register <leaked-cluster-name>`.
4. Then run `awsbnkctl cluster down --auto` to destroy it cleanly.

The Terraform state is regenerated implicitly during register + plan; the resources `awsbnkctl` would otherwise have tracked get re-discovered through the AWS SDK lookups. It's not seamless, but it's recoverable.

The `ws delete` `--force` flag's "still has resources" check exists exactly to prevent this scenario — don't bypass it without thinking about the consequences.

## Worked example: register an existing cluster, deploy BNK, tear down

End-to-end Part III scenario: somebody on your team already provisioned an EKS cluster manually via the AWS Console or `eksctl` (or via a different terraform tree); you need to deploy BNK on top of it using `awsbnkctl`, validate, and tear the whole thing down cleanly. The flow exercises [Chapter 9](./09-registering-existing-cluster.md), [Chapter 10](./10-deploying-bnk-trials.md), and this chapter end-to-end.

```bash
# 1. Workspace bootstrap — same as a fresh deploy
awsbnkctl init -w preexisting
# (answer prompts for region + resource group; pick the values matching
#  the existing cluster's location)

# 2. Register the already-running cluster into the workspace
awsbnkctl cluster register existing-bnk-cluster -w preexisting
# Expected:
#   → Discovering cluster "existing-bnk-cluster" via AWS EKS DescribeCluster ...
#   ✓ Cluster ID: <crn>
#   ✓ Wrote ~/.awsbnkctl/preexisting/cluster-outputs.json
#   ✓ Fetched admin kubeconfig to ~/.kube/config (chmod 0600)

# 3. Verify awsbnkctl sees the cluster
awsbnkctl status -w preexisting
# Expected: cluster Ready, workers count, no BNK pods yet

# 4. Deploy BNK on top — `up` is idempotent over the existing cluster
awsbnkctl up --auto -w preexisting
# Expected: terraform applies the cert-manager + flo + cne_instance +
# license modules only; the eks_cluster module sees the cluster already
# exists and skips. ~10-15 min vs ~50 min for a from-scratch up.

# 5. Validate
awsbnkctl test -w preexisting
# Expected: green across connectivity + dns

# 6. Tear down — destroys the BNK overlay; the registered cluster survives
awsbnkctl down --auto -w preexisting
# Expected:
#   → terraform destroy (auto-approved)
#   Destroy complete! Resources: N destroyed.
#   ✓ Workspace "preexisting" state retained at ~/.awsbnkctl/preexisting/
```

The destroy count `N` is the BNK overlay + bastion only — typically 30-40 resources, **not** the from-scratch ~80 count. `cluster register` is a discovery-only path: terraform state holds the overlay modules (`cert_manager`, `flo`, `cne_instance`, `license`) and the `testing` bastion, but **not** the `eks_cluster` module, because the cluster pre-existed awsbnkctl. `down` destroys only what terraform knows about, so the registered cluster survives untouched.

If you also want to release the underlying cluster, you have to tear it down through whatever provisioned it originally (the AWS Console, `eksctl delete cluster`, or the separate terraform tree your teammate used). `awsbnkctl cluster down` only works against clusters `awsbnkctl cluster up` created in the first place — see [Chapter 8](./08-cluster-phase.md) for the cluster-phase boundary.

The full register → up → test → down loop above is what Phase E + Phase H of the e2e plan exercise; see [Chapter 23](./23-e2e-test-plan.md) for the CI version.

## Cross-references

- [Chapter 6 — Workspaces](./06-workspaces.md) — `ws delete` mechanics and the parking-lot pattern.
- [Chapter 8 — The cluster phase](./08-cluster-phase.md) — what `cluster up` provisions and `cluster down` removes.
- [Chapter 9 — Registering an existing cluster](./09-registering-existing-cluster.md) — the `cluster register` mechanics the walkthrough builds on.
- [Chapter 10 — Deploying BNK trials](./10-deploying-bnk-trials.md) — what `up` provisions and `down` removes.
- [Chapter 26 — Troubleshooting](./26-troubleshooting.md) — recovery from partial-destroy and orphan-state scenarios.
