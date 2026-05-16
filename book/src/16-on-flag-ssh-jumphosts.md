# The --on flag and SSH jumphosts

The `--on <target>` flag (most commonly `--on jumphost`) re-runs an `awsbnkctl` passthrough command (`exec`, `shell`, `kubectl`) on a remote SSH host instead of locally. After a successful `awsbnkctl up`, a `jumphost` target is auto-populated from the upstream HCL's terraform outputs (pointing at the bastion EC2 instance), so `--on jumphost` works with no manual configuration in the common case.

This chapter covers when to reach for `--on`, the `targets:` workspace config block, the auto-population behaviour, the `awsbnkctl targets` command tree for managing your own targets, and how host-key trust is established.

The full design rationale for this feature lives in [PRD 01](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/01-SSH-AND-ON-FLAG.md). This chapter is the user-facing distillation.

## Why this exists

There are a handful of scenarios where running a command from your laptop is the wrong answer:

- **Customer-firewall scenarios.** Your customer's network policy lets the corporate jumphost reach `*.amazonaws.com` but blocks your laptop's egress to anything except web traffic. `aws sts get-caller-identity` works from the jumphost; from your laptop it times out.
- **Air-gapped environments.** The cluster lives in a VPC with no public ingress, accessible only via a bastion VM. The cluster API server isn't reachable from your laptop at all; you need to be inside the network to talk to it.
- **Private EKS endpoints.** The cluster's Kubernetes API is private-only (no public ingress); you need to be inside the VPC to talk to it. The bastion is inside the VPC and the EKS API is reachable from there.

`--on` makes those scenarios one flag rather than "ssh to the jumphost, install your tools there, copy your kubeconfig over manually". The SSH client is built into `awsbnkctl` (using `golang.org/x/crypto/ssh`); no host `ssh` binary is required.

## The `targets:` workspace config block

Targets are stored in your workspace config at `~/.awsbnkctl/<workspace>/config.yaml` under a `targets:` key:

```yaml
targets:
  jumphost:                                # auto-populated after `awsbnkctl up`
    host: 52.45.91.177
    user: ec2-user
    key_source: tf-output:bastion_shared_key
    port: 22                               # default; can be omitted

  bastion:                                 # user-defined
    host: ops.example.com
    user: ec2-user
    key_path: ~/.ssh/id_ed25519

  prod-jump:
    host: 10.0.0.5
    user: ec2-user
    key_source: agent                      # use ssh-agent
```

Each entry has at minimum `host` and `user`. Port defaults to `22`. Key resolution is determined by exactly one of `key_path` or `key_source` — see "Key sources" below.

You don't typically edit this file by hand. The auto-discovery flow populates `jumphost` for you, and `awsbnkctl targets add ...` populates other entries.

## Auto-discovery from `awsbnkctl up`

The upstream HCL provisions a small testing jumphost as part of every cluster apply. Two terraform outputs surface it:

- `testing_bastion_public_ip` — the public IP of the bastion EC2 instance.
- `bastion_shared_key` — the private key (PEM) the jumphost was provisioned with, marked `sensitive` in the HCL.

After a successful `awsbnkctl up`, `awsbnkctl` reads both outputs and writes a `jumphost` target into your workspace config:

```
✓ Auto-registered target jumphost (52.45.91.177); use `awsbnkctl --on jumphost ...`
```

The auto-registered target uses `user: ec2-user` (the upstream HCL provisions an Ubuntu cloud image whose default user is `ubuntu`).

The `key_source: tf-output:bastion_shared_key` form means the private key is **read from terraform state at SSH-connect time** rather than being copied into the workspace config or written to disk separately. The key only ever exists in terraform state and in memory during a connect; destroying and re-creating the cluster generates a new key, and `awsbnkctl` picks up the new one without any manual intervention.

If your cluster apply produced a `testing_tgw_jumphost_ip` output of `"TGW jumphost not created"` (the upstream HCL emits this string when the testing module is disabled) the auto-population is skipped. You can still add a `jumphost` target manually with `awsbnkctl targets add` if you have a different bastion in mind.

## Key sources

Three ways to tell `awsbnkctl` how to find the SSH private key:

1. **`key_path: <path>`** — a file on disk. Standard OpenSSH key formats are accepted (`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, etc.). Tilde expansion is honoured.

2. **`key_source: agent`** — talk to the user's `ssh-agent` over the socket pointed at by `$SSH_AUTH_SOCK`. The agent presents whichever keys it currently holds; `awsbnkctl` tries each in turn against the target's `authorized_keys`. This is the right setting if your team already manages keys via 1Password / hardware tokens / `gpg-agent` and you don't want a key file on disk. **Note**: ssh-agent integration is Linux/macOS-only at v1.0; Windows users should use `key_path` instead. Windows ssh-agent named-pipe support is on the v1.x roadmap.

3. **`key_source: tf-output:<output-name>`** — read the key from the workspace's terraform state output of that name. Used by the auto-discovered `jumphost` target. The terraform output must be a string-typed PEM-encoded private key; sensitive outputs work fine because `terraform output -raw <name>` returns the value regardless of the sensitive flag.

Exactly one of `key_path` or `key_source` must be set per target. `awsbnkctl targets show <name>` will tell you which is in use without printing the key material.

## Host-key TOFU on first connect

The first time you connect to a target, `awsbnkctl` shows the host key fingerprint and asks whether to trust it. The prompt is a single line:

```bash
$ awsbnkctl exec --on jumphost -- whoami
Add 52.45.91.177:22's key (SHA256:abc123def456ghi789jkl0mnopqrstuvwxyz/+=) to ~/.awsbnkctl/known_hosts? [y/N]: y
ubuntu
```

Answer `y` and the key is appended to `~/.awsbnkctl/known_hosts` (the same format as OpenSSH's `~/.ssh/known_hosts`). Subsequent connects trust silently.

Answer `n` and the connect fails with a clear "host key not trusted" error.

If the host key changes between runs — which would happen on a re-provisioned VM, or could happen as a man-in-the-middle attack — `awsbnkctl` refuses to connect:

```
error: host key mismatch: 52.45.91.177:22 known with SHA256:abc123... but server presented SHA256:zyx987...; if the host was rebuilt, edit ~/.awsbnkctl/known_hosts
```

This is "trust on first use" (TOFU) — the same model OpenSSH uses for new hosts. Exit code is 126 on host-key rejections.

### `--insecure-host-key` for CI

In automation contexts where a TOFU prompt would block forever, pass `--insecure-host-key` to skip host-key verification entirely:

```bash
awsbnkctl exec --on jumphost --insecure-host-key -- whoami
```

This is **insecure** — anyone in the network path can MITM the connection — and is intended only for short-lived CI runs against ephemeral test infrastructure. Don't use it in any context where the SSH session matters for security.

## The `awsbnkctl targets` command tree

Four subcommands for managing target entries:

```bash
awsbnkctl targets list
awsbnkctl targets show <name>
awsbnkctl targets add <name> --host ... --user ... --key-path ...
awsbnkctl targets remove <name>
```

### `targets list`

```
awsbnkctl targets list
NAME       HOST                USER     KEY
jumphost   52.45.91.177:22    ubuntu   tf-output:bastion_shared_key
bastion    ops.example.com:22  ec2-user  file:~/.ssh/id_ed25519
```

Prints every target in the current workspace's config. The `KEY` column shows the key source — never the key material itself. File-backed keys are prefixed with `file:` so they're visually distinct from `tf-output:` and `agent` sources.

### `targets show <name>`

```
awsbnkctl targets show jumphost
name:        jumphost
host:        52.45.91.177
port:        22
user:        ubuntu
key_source:  tf-output:bastion_shared_key
```

Prints the full record. Note that the key material itself is never printed — only the source descriptor (file path, ssh-agent, or terraform-output name).

### `targets add <name> ...`

```bash
awsbnkctl targets add bastion \
  --host ops.example.com \
  --user ec2-user \
  --key-path ~/.ssh/id_ed25519

# or with ssh-agent:
awsbnkctl targets add prod-jump \
  --host 10.0.0.5 \
  --user ec2-user \
  --key-source agent

# or with a non-default port:
awsbnkctl targets add custom \
  --host 10.0.0.5 \
  --user root \
  --key-path ~/.ssh/custom \
  --port 2222
```

Writes the new target into `~/.awsbnkctl/<workspace>/config.yaml`. Refuses if a target of that name already exists (use `targets remove` first).

### `targets remove <name>`

```bash
awsbnkctl targets remove bastion
```

Removes the entry from `config.yaml`. Does not remove the corresponding line from `~/.awsbnkctl/known_hosts` — the host key stays recorded so re-adding the same target later doesn't re-trigger a TOFU prompt.

## Working examples

The everyday verbs:

```bash
# Run an arbitrary command on the jumphost
awsbnkctl exec --on jumphost -- whoami
# → ubuntu

awsbnkctl exec --on jumphost -- uname -a
# → Linux jumphost-vm 5.15.0-... #... SMP ... x86_64 GNU/Linux

# Interactive PTY shell
awsbnkctl shell --on jumphost
# → drops you into the jumphost's default shell as the configured user
# → exit returns you to your local prompt

# aws CLI through the jumphost — runs `aws eks describe-cluster` from inside the VPC
# (handy when your laptop's network can't reach the EKS API)
awsbnkctl exec --on jumphost -- aws eks describe-cluster --name bnk-quickstart

# kubectl passthrough — same pattern
awsbnkctl kubectl --on jumphost get pods -A

# oc passthrough
awsbnkctl oc --on jumphost projects
```

Behaviour details worth knowing:

- **Streaming I/O.** stdout, stderr, stdin all stream in real time — the same as running the command locally. Long-running commands (`kubectl top nodes`, `aws eks describe-cluster` on a slow API call) work normally.
- **Exit code propagation.** The remote command's exit code is the local exit code. A failing remote command produces a non-zero `awsbnkctl` exit; a succeeding remote command produces `0`. CI scripts can rely on this.
- **TTY auto-detection.** `awsbnkctl shell --on` auto-allocates a PTY. Other verbs (`exec`, `kubectl`) run without a PTY at v1.0; if you need a PTY for `top` or another `isatty()`-sensitive command, fall back to `awsbnkctl shell --on jumphost` and run the command from the interactive shell.
- **Environment passthrough.** `AWS_PROFILE`, `AWS_REGION`, and `KUBECONFIG` are propagated to the remote session via SSH `SetEnv`, so commands on the jumphost see the same workspace context as your laptop. The bastion's preferred path is its own EC2 instance role (no env needed); the env passthrough is for cases where you want the laptop's credentials to win. The remote sshd must be configured to accept `AcceptEnv AWS_*` for this to work; the upstream HCL's bastion is already configured for it.

## What `--on` doesn't do (yet)

A few things deliberately deferred to later phases:

- **Lifecycle commands** (`up`, `down`, `plan`, `apply`) reject `--on` with a clear error at v1.0. Running terraform on a remote host has different state-handling considerations and is the job of the SSH execution backend ([Chapter 17](./17-execution-backends.md)) — `terraform` over `--backend ssh:<target>` is itself deferred to v1.x (state-file portability).
- **ProxyJump / multi-hop SSH.** If your jumphost itself is reached through another bastion, that's not directly supported at v1.0. The upstream HCL's bastion design lets the EC2 jumphost reach VPC-internal resources natively (including the private EKS endpoint and the worker nodes), so you usually don't need multi-hop in practice. ProxyJump support is on the v1.x roadmap.
- **`~/.ssh/config` parsing.** Targets must be defined explicitly in workspace config; `awsbnkctl` does not read your existing `~/.ssh/config`.
- **Password auth.** Keys + agent only. Passwords are not supported and won't be.
- **SCP / SFTP.** File transfer is the SSH execution backend's job (handled via `RunOpts.Files` materialisation; see [Chapter 17 §"SSH backend"](./17-execution-backends.md#ssh-backend)). `--on` does one-shot remote exec only.
- **Windows ssh-agent.** The `key_source: agent` path is Linux/macOS only at v1.0; Windows users must use `key_path` to a file. Already noted in [Key sources](#key-sources) above; called out here so a Windows reader who skipped to this section doesn't miss it.

## Cross-reference

[Chapter 17 — Execution backends](./17-execution-backends.md) extends the SSH client used here into a full execution backend with file materialisation, env-file fallback for sshd configurations that can't `AcceptEnv`, and apt-bootstrap of missing tools on Ubuntu jumphosts. The `--on` flag stays as the lightweight one-shot path; `--backend ssh` is the deeper integration. The two share the same `internal/remote.Client` so what you learn here translates directly.

For the design rationale, edge cases, and open questions, read [PRD 01](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/01-SSH-AND-ON-FLAG.md) — this chapter is the user-facing surface; PRD 01 is the developer-facing surface.
