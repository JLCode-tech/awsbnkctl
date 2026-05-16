# Terraform variables (terraform.tfvars)

`awsbnkctl` is a thin orchestration layer over a Terraform HCL bundle. The HCL has its own variables — well over 60 of them — declared in `terraform/variables.tf`. The workspace's `config.yaml` covers the common knobs; for the rest, you reach into `terraform.tfvars` directly.

This chapter is the surface for that lower layer: where the example file lives, how `awsbnkctl tfvars` bootstraps a starter, what `--var-file` does, the layering rule between `config.yaml`-derived tfvars and your overrides, and the one variable that **never** goes on disk (`AWS_ACCESS_KEY_ID`).

## Where the bundled HCL lives

The Terraform HCL is bundled into the `awsbnkctl` binary via `go:embed`. On first use of a workspace, it gets extracted to:

```
~/.awsbnkctl/<workspace>/state/tf-source/embedded-terraform/
├── main.tf
├── variables.tf
├── outputs.tf
├── providers.tf
├── versions.tf
├── terraform.tfvars.example
└── modules/
```

That `terraform.tfvars.example` file is the canonical reference for what's tunable — every variable with a sensible starter value, grouped by module (EKS cluster, cert-manager, FLO, CNEInstance, License, testing). `terraform/variables.tf` (linked at the [GitHub canonical URL](https://github.com/JLCode-tech/awsbnkctl/blob/main/terraform/variables.tf)) is the formal declaration with types, descriptions, and defaults.

You don't edit the example file in place. Copy or generate from it instead.

## `awsbnkctl tfvars` — bootstrap a starter

The `awsbnkctl tfvars` subcommand prints a starter `terraform.tfvars` to stdout, populated from the **current workspace state**:

```bash
$ awsbnkctl tfvars > ~/.awsbnkctl/dev/terraform.tfvars.user
```

What gets pre-filled:

- Every field from `config.yaml` that maps to a tfvar (cluster name, region, workers, BNK fields, S3 fields)
- The cluster's identity from `cluster-outputs.json` if `cluster up` has already run
- A commented-out section for the variables you might want to tune next (jumphost profile, GSLB datacenter, license mode)

What's deliberately **excluded**:

- `AWS_ACCESS_KEY_ID` — never on disk (see "The AWS_ACCESS_KEY_ID exception" below)
- Sensitive outputs (BIG-IP passwords, S3 HMAC secrets) — left as upstream defaults

The starter is meant to be copied into `~/.awsbnkctl/<ws>/terraform.tfvars.user` (the workspace-local override file) or into a `--var-file` path you keep alongside the workspace.

## What you typically edit

The variables that matter for day-to-day BNK trial work, ordered by likely-to-touch:

| Variable | Default | What it controls |
|---|---|---|
| `openshift_cluster_name` | `tf-openshift-cluster` | Cluster name. Mirrors `config.yaml`'s `cluster.name`. |
| `roks_workers_per_zone` | `1` | Worker nodes per AZ. `2` ⇒ 6 workers in a 3-AZ MZR region. |
| `create_roks_cluster` | `true` | Set `false` to adopt an existing cluster. Pair with `roks_cluster_id_or_name`. |
| `openshift_cluster_version` | `"4.18"` | OpenShift minor. Quote it — YAML/HCL parses `4.18` as float otherwise. |
| `cneinstance_deployment_size` | `Small` | `Small`/`Medium`/`Large`. CNEInstance sizing. |
| `f5_bigip_k8s_manifest_version` | upstream pin | Pin a specific BNK manifest chart version. |
| `far_repo_url` | `repo.f5.com` | FAR Docker/Helm registry. Override only for staging. |
| `flo_namespace` | `f5-bnk` | Where the F5 Lifecycle Operator runs. |
| `testing_create_tgw_jumphost` | `true` | Create the testing jumphost in a client VPC over Transit Gateway. |
| `testing_ssh_key_name` | `""` (must set) | Existing AWS SSH key name for jumphost provisioning. |
| `cneinstance_gslb_datacenter_name` | `""` | Set when wiring BNK into an F5 BIG-IP GSLB datacenter. |
| `license_mode` | `connected` | `connected` \| `disconnected`. |

For the full list with types and per-field descriptions, see `terraform/variables.tf` directly — link [here](https://github.com/JLCode-tech/awsbnkctl/blob/main/terraform/variables.tf) — or the auto-generated [Chapter 29 — Terraform variable reference](./29-terraform-variable-reference.md).

## The layering rule

When `awsbnkctl up` (or `plan`/`apply`/`destroy`) invokes Terraform, it composes three layers of tfvars in this order:

```
1. terraform.tfvars              (rendered by awsbnkctl from config.yaml)
2. terraform.tfvars.user         (workspace-local override, optional)
3. --var-file <path> ...         (CLI flag, repeatable, later file wins)
```

Later layers override earlier ones — same rule Terraform itself uses for `-var-file` chaining.

Concretely:

```bash
# config.yaml says cluster.workers_per_zone: 2
# ~/.awsbnkctl/dev/terraform.tfvars.user contains:
#   roks_workers_per_zone = 4
# Run with no flag:
awsbnkctl up
# → terraform sees 4 (.user wins over generated .tfvars)

# Pass a CLI override:
awsbnkctl up --var-file ./perf-test.tfvars
# perf-test.tfvars contains: roks_workers_per_zone = 8
# → terraform sees 8 (.var-file wins over .user)

# Multiple --var-files; later wins:
awsbnkctl up \
  --var-file ./base.tfvars \
  --var-file ./override.tfvars
# → values in override.tfvars win over base.tfvars,
#   which both win over .user, which wins over .tfvars
```

The `--var-file` flag matches Terraform's own `--var-file` exactly — repeatable, paths interpreted relative to the working directory at invocation time.

## The `AWS_ACCESS_KEY_ID` exception

The upstream HCL declares `AWS_ACCESS_KEY_ID` as a `sensitive` variable. Every other tfvar can land in a file on disk; this one never does.

Instead, the API key flows through the resolver chain (env → keychain → config-b64 → prompt — see [Chapter 14](./14-credentials-resolver.md)), and `awsbnkctl` exports it as `AWS_ACCESS_KEY_ID` in the environment of the terraform-exec child process. Terraform reads the env var and injects it as if it had been declared in tfvars, but no plaintext key ever touches the filesystem.

If you put `AWS_ACCESS_KEY_ID = "..."` in a hand-edited tfvars and run `terraform` directly (not via `awsbnkctl`), it works — Terraform itself is happy. But this is **not** how `awsbnkctl` runs Terraform, and putting the key in a `.tfvars.user` or `--var-file` is **strongly discouraged**: the file persists on disk, gets backed up, gets committed to git by accident, and gets read by other processes. The env-var path eliminates the on-disk window entirely.

Other secrets in scope:

- `bigip_password` — upstream HCL declares it as a regular string (not `sensitive`). If you set it in tfvars, the value lands on disk. Treat that file like a credential.
- S3 HMAC keys — auto-generated by the `roks_cluster` module via the S3 service-credentials resource; they live in `terraform.tfstate` (which is itself sensitive — `chmod 0600`, never commit, treat the workspace as a secret store).

## Worked example: bigger cluster for a perf test

Default workspace, default cluster. You want to bump worker count for one perf-test run, then go back.

```bash
# 1. Confirm the current value comes from config.yaml
$ grep workers ~/.awsbnkctl/dev/config.yaml
  workers_per_zone: 2

# 2. Drop a one-off override into a file
$ cat > ~/perf-cluster.tfvars <<'EOF'
roks_workers_per_zone = 6
roks_min_worker_vcpu_count = 32
roks_min_worker_memory_gb = 128
EOF

# 3. Plan against it (note: --var-file passes through to terraform plan)
$ awsbnkctl plan --var-file ~/perf-cluster.tfvars

# 4. Apply
$ awsbnkctl apply --var-file ~/perf-cluster.tfvars

# 5. Run the throughput test
$ awsbnkctl test throughput

# 6. Roll back: re-apply WITHOUT the var-file
$ awsbnkctl apply
# → terraform sees workers_per_zone=2 again from config.yaml-derived tfvars
```

Notice step 6 — dropping the `--var-file` flag is the rollback. Terraform compares its current state to the new desired state (from `config.yaml`) and scales the worker pool back down. No special "undo" command needed.

For a more permanent override (you want this workspace to *always* run with bigger nodes), put the contents of `perf-cluster.tfvars` into `~/.awsbnkctl/dev/terraform.tfvars.user` instead. Then every `awsbnkctl up`/`apply` picks it up automatically without a CLI flag.

## When to edit `config.yaml` vs `.tfvars.user` vs `--var-file`

A rough decision matrix:

| You want to change... | Edit... |
|---|---|
| Cluster identity, region, OpenShift version, worker count | `config.yaml` (via `awsbnkctl init` or by hand) |
| BNK chart version, CNEInstance size, FAR repo | `config.yaml` (the `bnk:` block) |
| A variable not modelled in `config.yaml` (e.g. `cneinstance_gslb_datacenter_name`, `bigip_password`) | `terraform.tfvars.user` (workspace-local, persistent) |
| A one-off override for a single run (perf test, capacity bump) | `--var-file ./oneoff.tfvars` (CLI) |
| A CI-pipeline variable bundle that's checked into git | `--var-file ./ci-overrides.tfvars` (CLI; the file lives in your CI repo, not the workspace) |

The schema in `config.yaml` covers about a third of the upstream HCL variables — the ones that nearly every workspace needs to set. The other two-thirds (jumphost details, every BNK module's full surface, the testing module's full surface) are reachable through the lower layers.

## Cross-references

- [Chapter 12 — Workspace config](./12-workspace-config.md) — what `config.yaml` covers vs what falls through to tfvars.
- [Chapter 14 — Credentials and the resolver chain](./14-credentials-resolver.md) — why `AWS_ACCESS_KEY_ID` doesn't go in tfvars.
- [Chapter 29 — Terraform variable reference](./29-terraform-variable-reference.md) — auto-generated complete reference for `terraform/variables.tf`.
- The upstream `terraform/variables.tf` source: <https://github.com/JLCode-tech/awsbnkctl/blob/main/terraform/variables.tf>
- The upstream starter file: <https://github.com/JLCode-tech/awsbnkctl/blob/main/terraform/terraform.tfvars.example>
