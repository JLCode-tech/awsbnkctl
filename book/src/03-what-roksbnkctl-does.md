# What awsbnkctl does (and doesn't do)

`awsbnkctl` is a single-binary CLI for deploying and validating F5 BIG-IP Next for Kubernetes (BNK) onto AWS EKS. It exists to compress a multi-step deployment — clone the right Terraform, configure it, run terraform, fetch a kubeconfig, install BNK, run smoke tests — into a four-command lifecycle.

This chapter is about scope. What `awsbnkctl` owns, what it deliberately does not own, and what's coming in future releases. Read it before you reach for the tool to do something it isn't trying to do.

## The 4-command lifecycle

The everyday user-facing flow is four commands:

```bash
awsbnkctl init        # answer a few prompts about region, RG, cluster name
awsbnkctl up          # terraform plan + apply (~50 min for fresh EKS + BNK)
awsbnkctl test        # connectivity + DNS + throughput against the deployment
awsbnkctl down        # tear it all back down when you're done evaluating
```

That's it. From "I have an AWS APIs key" to "deployed BNK with a passing throughput test" with no manual `terraform apply`, no hand-editing kubeconfig paths, no chasing down BNK Helm charts — then a clean tear-down when you're done so you stop paying for the cluster.

[Chapter 7](./07-quick-start.md) walks through this end-to-end with sample output.

## What awsbnkctl owns

`awsbnkctl`'s scope is everything between "you have an AWS APIs key" and "you have a working BNK install you can run tests against". Concretely:

- **Workspace state** — kubectl-style per-environment isolation under `~/.awsbnkctl/<workspace>/`. Each workspace has its own config, terraform state, kubeconfig, scratch artefacts. Switch with `awsbnkctl ws use <name>` or override per-command with `-w <name>`.
- **Terraform-exec orchestration** — wraps HashiCorp's `terraform-exec` library to drive `terraform init/plan/apply/destroy` with the right state file, the right `TF_DATA_DIR`, the right tfvars layering. You don't run `terraform` directly; `awsbnkctl up` does.
- **Kubeconfig fetch** — after a successful `up`, fetches the admin kubeconfig from AWS's container service API and writes it to `~/.kube/config` at mode 0600. Retries on the 404s that happen during cluster propagation lag.
- **S3 supply chain** — the BNK install needs FAR archives and JWT licences staged in S3. `awsbnkctl init` uploads the local artefacts to the workspace's supply-chain bucket; the bucket policy is scoped to the FLO IRSA role so FLO can read it from inside the cluster without any static keys. See [Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md).
- **Post-deploy validation** — `awsbnkctl test` runs three suites: HTTP/HTTPS connectivity (built-in `net/http`, no external `curl`), DNS resolution (built-in `net.Resolver`, no external `dig`), and iperf3 throughput (deploys an `iperf3 -s` pod into the cluster, runs the client, parses JSON output, tears down).
- **Credentials handling** — AWS APIs key resolution chain: env vars (`AWS_ACCESS_KEY_ID` etc.), OS keychain (macOS Keychain / libsecret / Windows Credential Manager via `zalando/go-keyring`), opt-in base64 in workspace config, interactive prompt as last resort. Plaintext keys in `config.yaml` are rejected.

If any of those words don't make sense yet, don't worry — later chapters cover each in depth.

## What awsbnkctl does *not* try to do

Equally important: the explicit non-goals. `awsbnkctl` deliberately stays out of these spaces because well-established tools already cover them:

- **Not a generic AWS CLI.** That's `aws`. If you want to manage VPCs, IAM policies, classic infrastructure, Watson, or any of the hundred-plus other AWS services, use `aws`. `awsbnkctl exec -- <args...>` exists as a convenience passthrough that loads workspace credentials, but it doesn't try to replace `aws`'s surface.
- **Not a generic Kubernetes CLI.** That's `kubectl`. `awsbnkctl kubectl <args...>` is again a passthrough that loads the workspace's kubeconfig; it does not try to be a kubectl re-implementation. (Phase 2 internalises a small subset — `awsbnkctl k get/apply/logs/exec/port-forward` — so the happy path doesn't require a host `kubectl` binary, but that's targeted convenience, not replacement.)
- **Not an OpenShift admin tool.** That's `oc`. Same story: `awsbnkctl oc <args...>` passthrough, no attempt to re-implement.
- **Not a BNK runtime UI.** Once BNK is deployed, you configure it through its CRDs (`F5BigIpCtx`, `F5IngressTls`, etc.). `awsbnkctl` doesn't ship a TUI / web UI for editing those — it gets you to a deployed BNK and steps out of the way.
- **Not a Terraform authoring tool.** The HCL lives in this repo's `terraform/` directory and is embedded into the binary at build time. `awsbnkctl` runs that HCL; it doesn't help you write more of it. If you fork the HCL, point `awsbnkctl` at your fork via `tf_source: github` or `tf_source: local`.
- **Not an arbitrary workload deployer.** BNK is the workload. The iperf3 / nginx fixtures used by `awsbnkctl test` exist only to validate BNK; they're not a general-purpose deployment surface.

The principle is "do one thing well". `awsbnkctl` does BNK-on-EKS lifecycle and validation. Every other concern is delegated to the right purpose-built tool.

## The relationship to bundled HCL

A core design decision worth surfacing: the Terraform that drives the deployment lives **in this repo** under `terraform/`, and is embedded into the `awsbnkctl` binary at build time via Go's `embed` package.

This means:

- **One install** gets you the CLI + a matched HCL pair. No "clone the right tag of the terraform repo separately" step.
- **Versioning is unified.** A `awsbnkctl v1.0` release ships with a specific snapshot of the HCL. Upgrading the binary upgrades the HCL atomically. There's no skew between "binary version" and "Terraform version".
- **Power users can override.** The workspace config has a `tf_source:` block:

  ```yaml
  tf_source:
    type: embedded     # default; uses HCL bundled into the binary
    # type: local
    # path: /path/to/your/terraform
    # type: github
    # repo: yourfork/awsbnkctl-terraform
    # ref: my-branch
  ```

  `tf_source: local` is the right setting if you're iterating on the HCL itself. `tf_source: github` lets you point at a fork of the terraform repo if you've published one separately. The default — `embedded` — covers the everyday case.

[Chapter 13](./13-terraform-variables.md) covers the tfvars layering rules; this is just the elevator pitch for "the HCL ships with the binary".

## What v1.0 ships and what's queued for v1.x

This book ships with **v1.0**. The surface it documents:

- **kubectl internalisation** — `awsbnkctl k get/apply/logs/exec/port-forward` is a first-class verb talking to the cluster directly via `client-go`. Host `kubectl` is informational only; the only required prereq on PATH is `terraform`.
- **Four execution backends** — every external tool (`aws`, `iperf3`, `terraform`) selectable across `local | docker | k8s | ssh` via `--backend`. iperf3 runs entirely in-cluster by default; aws runs in a pinned-version Docker container if you don't want to install it; any tool proxies through a jumphost via `--backend ssh:<target>`. [Chapter 17](./17-execution-backends.md) is the user-facing surface; [PRD 03](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/03-EXECUTION-BACKENDS.md) is the design rationale.
- **GSLB-aware DNS testing** — the DNS probe is [miekg/dns](https://github.com/miekg/dns)-based with multi-vantage support, so you can verify that BNK's GSLB is returning different answers from different network locations. [Chapter 21](./21-dns-testing-gslb.md) covers it.
- **Polished book** — all 32 chapters, every code example verified, four Mermaid diagrams (architecture / lifecycle / GSLB cross-vantage / backend matrix), per-Part worked examples.

A handful of items are explicitly **deferred to v1.x**:

- `terraform` over `--backend k8s` and `--backend ssh` (state-file portability design needed).
- Multi-hop SSH `ProxyJump` for the `--on` and `ssh:<target>` paths.
- Windows full TTY (interactive `shell` on Windows ships as line-buffered; full PTY is a v1.x item).
- Typed OpenShift CRDs (today's unstructured printer works; richer per-type output is queued).
- Cross-driver cluster-sharing for `e2e-test-full.sh` (each driver brings up its own cluster today).

See [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deliberately deferred to post-v1.0" for the full roadmap.

## Pointers to the next chapters

- [Chapter 4 — Installation](./04-installation.md) gets the binary on your machine.
- [Chapter 7 — Quick start](./07-quick-start.md) walks the 4-command lifecycle with sample output.
- [Chapter 16 — The --on flag and SSH jumphosts](./16-on-flag-ssh-jumphosts.md) covers running passthrough commands over SSH against an auto-discovered jumphost — useful in customer-firewalled and air-gapped scenarios.
