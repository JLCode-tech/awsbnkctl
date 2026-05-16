# Workspaces

A **workspace** is a per-environment bundle of config + state. The shape is modelled on `kubectl` contexts: you can have many of them, exactly one is "current" at a time, and a `-w` flag lets you address a specific one for a single command without flipping the pointer.

This chapter covers the on-disk layout, the everyday `init` / `use` / `list` flow, the full `awsbnkctl workspaces` command tree, the `-w` / `--workspace` override, and the "parking-lot" pattern the end-to-end test uses to delete the workspace it's currently inside.

## The on-disk layout

Every workspace lives under `~/.awsbnkctl/<name>/`:

```
~/.awsbnkctl/
  config.yaml                          # global; current_workspace pointer
  known_hosts                          # SSH host keys (shared across workspaces)
  default/                             # workspace "default"
    config.yaml                        # this workspace's inputs
    cluster-outputs.json               # post-apply cluster identity (when present)
    state/                             # BNK trial state
      terraform.tfstate
      terraform.tfvars
      kubeconfig                       # admin kubeconfig (mode 0600)
      tf-source/                       # bundled HCL extracted to disk
      scratch/                         # docker bind-mounts, helm caches
    state-cluster/                     # cluster-phase state (separate tree)
      terraform.tfstate
      cluster-phase-override.tfvars
  prod/                                # workspace "prod"
    config.yaml
    state/
    ...
```

Three things are worth calling out:

- **`~/.awsbnkctl/config.yaml`** is *global* — non-secret user-wide preferences plus the `current_workspace` pointer. It is **not** a workspace config; the per-workspace files live one level deeper.
- **`state/` and `state-cluster/`** are intentionally separate so [`awsbnkctl cluster up`](./08-cluster-phase.md) and `awsbnkctl up` don't tangle their Terraform state. Most users won't touch either directly.
- **`cluster-outputs.json`** is the persisted identity of the workspace's EKS cluster — written by `cluster up` or [`cluster register`](./09-registering-existing-cluster.md), read by `awsbnkctl up` so BNK trials don't have to re-state cluster identity in every tfvars.

Override the base directory with the `AWSBNKCTL_HOME` env var. Test fixtures use this; everyday users shouldn't need it.

## The everyday workspace routine

The minimum daily routine:

```bash
# Initialise (creates ~/.awsbnkctl/<name>/config.yaml; defaults to "default")
awsbnkctl init

# Switch which workspace is "current"
awsbnkctl ws use prod

# See all workspaces and which one is current
awsbnkctl ws list
```

`awsbnkctl init -w <name>` is the one-shot path that creates the directory **and** populates `config.yaml` interactively. Everything else (`ws new`, `ws use`, `ws delete`) is the deconstructed form for users who want finer-grained control.

## The full command tree

```bash
awsbnkctl workspaces ...     # canonical name
awsbnkctl ws ...              # alias
```

### `ws new <name>` — empty skeleton

Creates `~/.awsbnkctl/<name>/` with no `config.yaml`. Useful when you want the directory to exist (so `ws use` works) before you run `init`.

```bash
awsbnkctl ws new staging
# ✓ Created workspace "staging" (run `awsbnkctl init -w staging` to configure)
```

Most users skip this and use `awsbnkctl init -w staging` directly, which does both steps in one go.

### `ws use <name>` — switch current

Sets the `current_workspace` pointer in `~/.awsbnkctl/config.yaml`:

```bash
awsbnkctl ws use prod
# ✓ Current workspace: prod

awsbnkctl ws current
# prod
```

Refuses to point at a non-existent workspace. The pointer is the only thing that changes — workspace state stays put.

### `ws current` — print the pointer

```bash
awsbnkctl ws current
# default
```

Prints the current workspace name on stdout. If no pointer is set, prints a hint like "no current workspace; run `awsbnkctl ws use <name>` or `awsbnkctl init`" to **stderr** and exits 0 with empty stdout — so `WS=$(awsbnkctl ws current)` produces an empty string in scripts rather than spurious output.

### `ws list` — table view

```bash
awsbnkctl ws list
NAME      CURRENT  REGION     CLUSTER          TF SOURCE
default   *        us-west-2  bnk-quickstart   embedded@v0.9.0
prod               eu-west-1  bnk-prod         embedded@v0.9.0
staging            us-west-2  bnk-staging      local:./terraform
```

The `*` marker on `CURRENT` highlights the active workspace. Other columns reflect each workspace's `config.yaml`. Rows where `config.yaml` is missing or unparseable still show the name, with the other columns blank — the list never errors out because of one corrupt workspace.

### `ws delete <name> [--force]`

Removes the workspace directory and the OS-keychain entry for its API key. Two safety rails:

1. **Refuses to delete the current workspace.** You'd be left with a dangling `current_workspace` pointer, so `delete` errors out with: `cannot delete current workspace "foo"; switch first: awsbnkctl ws use <other>`.
2. **Refuses if Terraform state lists provisioned resources** (unless `--force`). Catches the foot-gun where you forget to run `awsbnkctl down` first.

```bash
awsbnkctl ws delete staging
# Delete workspace "staging"? [y/N]: y
# ✓ Deleted workspace "staging"

# Refused — state still has resources
awsbnkctl ws delete prod
# Error: terraform state lists 77 resources; run `awsbnkctl down` first or pass --force

# I really mean it
awsbnkctl ws delete prod --force
# ✓ Deleted workspace "prod"
```

`--force` skips both the prompt and the state-non-empty check. Use it sparingly — there's no "undo" for `rm -rf ~/.awsbnkctl/<name>/`.

## The current-workspace pointer

The pointer lives at `~/.awsbnkctl/config.yaml`:

```yaml
current_workspace: prod
```

Every command that doesn't pass `-w` reads this pointer. `awsbnkctl init` writes it on first run (so the very first `init` makes `default` current automatically). `ws use` rewrites it. Nothing else touches it.

If the pointer references a workspace that doesn't exist (e.g. someone `rm -rf`'d the directory by hand), `awsbnkctl` errors out with a clear message: `workspace "prod" referenced by current_workspace does not exist; run awsbnkctl ws use <other>`.

## `-w` / `--workspace` for one-off overrides

Every command accepts `-w <name>` to override the current pointer for a single invocation:

```bash
# Doctor against "prod" without flipping the global pointer
awsbnkctl -w prod doctor

# Run init for a new workspace called "staging"
awsbnkctl init -w staging

# Get pods from the "default" cluster while currently on "prod"
awsbnkctl -w default k get pods -A
```

Use this when:

- You're scripting against multiple workspaces in a single run (CI runner that exercises `default` + `e2e-cleanup` back-to-back).
- You want to run a one-off command against a different environment without losing your current context.
- You're testing a fresh workspace before promoting it to current.

The flag only affects the running command — the pointer in `~/.awsbnkctl/config.yaml` is unchanged. After the command exits, the next bare `awsbnkctl` reads the original pointer.

## The parking-lot pattern

A subtle gotcha: `ws delete` refuses to remove the current workspace, but the end-to-end test suite needs to clean itself up after running against the `default` workspace.

The fix is the **parking-lot pattern**: have a throwaway workspace that exists only to be the "current" pointer while you delete other workspaces.

```bash
# End-to-end test cleanup (e2e-test.sh: Phase D destroys; Phase H runs the parking-lot dance below)

# Run the destroy against "default" (still current at this point)
awsbnkctl down --auto

# Park the pointer somewhere harmless
awsbnkctl ws new e2e-cleanup
awsbnkctl ws use e2e-cleanup

# Now we can drop the original workspace — it's no longer current
awsbnkctl ws delete default --force

# Optional: remove the parking lot too, by parking somewhere else first
awsbnkctl ws new tmp-park
awsbnkctl ws use tmp-park
awsbnkctl ws delete e2e-cleanup --force
awsbnkctl ws delete tmp-park --force   # leaves no current pointer
```

The pattern works because `current_workspace` only matters for commands that read workspace config. Once the pointer points elsewhere, the original workspace is just a directory and `delete` is happy to remove it.

If you want to delete *every* workspace including the parking lot, the last `delete` will leave you with an empty `current_workspace`. The next `awsbnkctl init` will populate it again with `default`.

## Using a workspace's environment in your shell

`awsbnkctl shell` drops you into a subshell with `KUBECONFIG`, `AWS_PROFILE`, and `AWS_REGION` pre-loaded from the current workspace:

```bash
awsbnkctl shell
# (now in a subshell)
echo $KUBECONFIG
# /home/you/.awsbnkctl/default/state/kubeconfig
echo $AWS_REGION
# us-west-2
exit
# (back to the parent shell)
```

Same for `-w`:

```bash
awsbnkctl -w prod shell
```

Useful when you want to run host `kubectl` / `aws` CLI / arbitrary tools with the workspace context loaded. The internalised verbs (`awsbnkctl k get`, etc.) read the same context automatically — you don't need to be in a subshell to use them.

## Common workspace patterns

A handful of patterns that come up in practice:

| Use case | Pattern |
|---|---|
| Different AWS accounts | `default` for personal, `acct-foo` pinned to a specific `AWS_PROFILE` |
| Different regions | `us-west-2`, `eu-west-1` workspaces with distinct `cluster.name` values |
| Throwaway short-lived clusters | `bnk-trial-N` workspaces; delete with `--force` after `down` |
| CI vs local dev | `dev` and `ci` workspaces; `ci` reads creds from env or web-identity token (GitHub Actions OIDC), `dev` reads from `~/.aws/credentials` |
| Parking-lot cleanup | `e2e-cleanup` workspace per "the parking-lot pattern" above |

Workspaces are cheap. If a flow benefits from isolation, make a new one rather than fighting with `--var-file` overrides on the existing one.

## Forward-link to Chapter 12

This chapter covers the *workspace-as-a-unit*: how to create, switch, list, delete. The schema of the per-workspace `config.yaml` itself — every field, default, valid range — is [Chapter 12 — Workspace config](./12-workspace-config.md).
