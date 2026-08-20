# awsbnkctl

[![CI](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/JLCode-tech/awsbnkctl?include_prereleases&sort=semver)](https://github.com/JLCode-tech/awsbnkctl/releases)

**awsbnkctl** is a single Go binary that provisions **F5 BIG-IP Next for Kubernetes (BNK)** on AWS EKS. It manages the entire lifecycle — VPC, EKS cluster, node groups, IAM, secondary ENIs for the data plane, BNK install, and end-to-end traffic validation — using the AWS SDK directly.

No Terraform. No host `kubectl`. No `aws` CLI. **One binary, one intent file.**

---

## Quick Start

### 1. Installation

Build the binary from source or download from the latest release:

```bash
go build -o awsbnkctl ./cmd/awsbnkctl
```

### 2. Configuration

Copy an example configuration and customize it. For a complete BNK deployment, start from `full-cluster`:

```bash
cp examples/full-cluster/cluster.yaml my-cluster.yaml
```

Edit `my-cluster.yaml` to set:
- `metadata.name` & `metadata.region`
- Network CIDRs
- `cluster.nodeGroups`
- `bnk.farArchive` (FAR pull credentials JSON)
- `bnk.jwt` (subscription JWT)

### 3. Authenticate to AWS

Authenticate using the standard credential chain (e.g., if using SSO):

```bash
export AWS_PROFILE=my-profile
aws sso login --profile $AWS_PROFILE
```

### 4. Deploy

Validate your intent file (no AWS API calls made), and then provision everything! Add `--dry-run` to preview first if you like.

```bash
./awsbnkctl validate my-cluster.yaml
./awsbnkctl up -f my-cluster.yaml
```

You can also preview the plan without any AWS credentials at all by setting
`AWSBNKCTL_SKIP_AUTH=1` together with `--dry-run`:

```bash
AWSBNKCTL_SKIP_AUTH=1 ./awsbnkctl up -f my-cluster.yaml --dry-run
```

`AWSBNKCTL_SKIP_AUTH=1` is only valid with `--dry-run`; a live `up` or `down`
run must have real AWS credentials.

### 5. Validate & Teardown

Run the built-in data-plane traffic validation, and once finished, tear down the environment safely.

```bash
./awsbnkctl scenarios run http-routing-e2e -f my-cluster.yaml
./awsbnkctl down -f my-cluster.yaml --yes
```

---

## Features & Capabilities

- **Imperative phased provisioner:** ~30 ordered phases run via the AWS Go SDK. AWS resource tags act as the single source of truth; a local `state.env` cache speeds up re-runs and is rebuildable from tags.
- **`cluster.yaml` intent file:** Declarative inputs (VPC, network, node group, BNK credentials) seamlessly map to imperative AWS calls. Validated up-front before any mutation.
- **Built-in `scenarios` framework:** End-to-end traffic validation against the provisioned cluster (5 green data-plane scenarios + curated demo catalogue).
- **`demo` experience:** Audience-friendly walkthrough surface with a rocket-themed launch renderer (gated on `--demo` + TTY), including migration scenarios that run BNK side-by-side with ingress-nginx/HAProxy and external BIG-IP VE + CIS.

---

## Architecture

The Go-SDK phased path runs end-to-end without Terraform: VPC, subnets, IGW, NAT, EKS control plane, node group, kubeconfig, S3 supply chain, IRSA, Multus, host-device secondary ENIs, BNK activation, jumphost, forge registration. Terraform has been removed entirely from the production path.

For more deep-dive context, check out our [Architecture Guide](docs/ARCHITECTURE.md) and [Forge Integration](docs/FORGE_INTEGRATION.md).

---

## Patterns

The `pattern:` field selects the TMM data-plane interface topology and binding. Backend pods are always reached over the CNI.

| `pattern:` | Interfaces | Binding | Min ENIs | Status |
|---|---|---|---|---|
| `external-only` | external only | host-device (kernel) | 2 | supported |
| `dual-interface` | external + internal | host-device (kernel) | 3 | supported |
| `sriov-external` | external only | SR-IOV / `vfio-pci` DPDK | 2 | experimental |

*Note: `host-device` is a legacy alias for `dual-interface` so existing configs keep working unchanged.*

---

## Examples & Demos

Ready-to-edit topologies live under [`examples/`](examples/) — see the
[examples index](examples/README.md) for patterns and running costs side by side.

- **[`examples/full-cluster/`](examples/full-cluster/)** - Complete BNK cluster reference config; also the demo cluster (`demo:` block) and the BIG-IP migration story
- **[`examples/external-only/`](examples/external-only/)** - Single-interface `external-only` pattern, and the experimental `sriov-external` DPDK variant (one-line swap)
- **[`examples/demo-ai/`](examples/demo-ai/)** - Combined BNK protocol demo + SageMaker AI rig
- **[`examples/ai-rig/`](examples/ai-rig/)** - BNK in front of GPU inference + a managed SageMaker endpoint
- **[`examples/egress-demo/`](examples/egress-demo/)** - Transparent egress + egress firewall, toggled by one CR
- **[`examples/local-zone/`](examples/local-zone/)** - Reference telco/edge custom resources (no `cluster.yaml`)

Check out the [Demo Guide](examples/full-cluster/README.md#demo-mode) for full walkthroughs.

---

## Commands

Run `awsbnkctl --help` for the complete command tree. Some highlights:

- `up -f <cfg>` : Provision everything. Add `--dry-run` to preview, `--demo` for audience-mode.
- `down -f <cfg> --yes` : Tear down in reverse.
- `status` : Workspace summary (cluster state, BNK components, phases).
- `doctor` : Health check for AWS creds, reachability, and BNK subsystem.
- `k <verb> [args]` : Kubernetes passthrough (`get`, `apply`, `delete`, etc.). No host `kubectl` needed!
- `forge {register,status,unregister}` : Optional handoff to a running [bnk-forge](docs/FORGE_INTEGRATION.md) instance.

---

## Contributing

We welcome contributions! Please see [`CONTRIBUTING.md`](CONTRIBUTING.md) for how to get started locally, run tests, and ship changes.

## License

[MIT](LICENSE) © 2026 JLCode-tech
