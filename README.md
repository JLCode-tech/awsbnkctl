# awsbnkctl

![BNK](https://img.shields.io/badge/BNK-2.3.0-0a3a5c)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.34--1.35-326ce5?logo=kubernetes&logoColor=white)
![AWS EKS](https://img.shields.io/badge/AWS-EKS-ff9900?logo=amazon-aws&logoColor=white)
[![CI](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/JLCode-tech/awsbnkctl?label=download)](https://github.com/JLCode-tech/awsbnkctl/releases)

A single-binary CLI to provision **F5 BIG-IP Next for Kubernetes (BNK)** on AWS EKS, manage secondary ENIs for high-performance TMM data planes, and validate the deployment with built-in end-to-end traffic scenarios.

No Terraform. No host `kubectl`. **One binary, one intent file.**

---

## Contents

- [The `*bnkctl` Family](#the-bnkctl-family)
- [Highlights](#highlights)
- [Pinned Versions](#pinned-versions)
- [Installation](#installation)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Data-Plane Patterns](#data-plane-patterns)
- [Configuration](#configuration)
- [Scenarios Catalogue](#scenarios-catalogue)
- [Agentic Workflow](#agentic-workflow)
- [BNK Forge Integration](#bnk-forge-integration)
- [Command Reference](#command-reference)
- [Repository Layout](#repository-layout)
- [Contributing & License](#contributing--license)

---

## The `*bnkctl` Family

`awsbnkctl` is part of the coordinated `*bnkctl` family of single-binary CLIs for deploying and operating F5 BIG-IP Next for Kubernetes across cloud, container, and bare-metal environments:

| Tool | Target Environment | Data Plane & Networking | Upstream Repo |
|---|---|---|---|
| **`awsbnkctl`** | **AWS EKS & EC2** | AWS Secondary ENIs (host-device / SR-IOV DPDK), Multus | [JLCode-tech/awsbnkctl](https://github.com/JLCode-tech/awsbnkctl) |
| **`roksbnkctl`** | **IBM Cloud ROKS / OpenShift** | IBM Cloud VPC secondary subnets, Calico / OVN-K | [jgruberf5/roksbnkctl](https://github.com/jgruberf5/roksbnkctl) |
| **`ocibnkctl`** | **Local OCI / k3s Containers** | Docker/Podman container netns virtio demo mode, Anycast BGP | [mwiget/ocibnkctl](https://github.com/mwiget/ocibnkctl) |
| **`gkebnkctl`** | **GCP GKE** | GCP host-device secondary VPC interfaces, Multus | [JLCode-tech/gkebnkctl](https://github.com/JLCode-tech/gkebnkctl) |
| **`bnkctl-index`** | **BNK Forge Catalog** | Curated catalog index packaging all `*bnkctl` runner modules | [mwiget/bnkctl-index](https://github.com/mwiget/bnkctl-index) |

---

## Highlights

- **Imperative Phased Provisioner** — 39 deterministic, ordered phases (two of them conditional) driven directly via the AWS Go SDK. AWS resource tags (`awsbnkctl:cluster`, `awsbnkctl:component`, `awsbnkctl:managed`) serve as the single source of truth; local state caches accelerate subsequent runs and are fully reconstructible from AWS tags.
- **Embedded Kubernetes Engine (`k`)** — Native `client-go` implementation provides `get`, `apply`, `describe`, `delete`, `logs`, `exec`, and `port-forward` with zero host `kubectl` dependencies.
- **End-to-End Traffic Scenarios** — 15 built-in scenarios validate live L4/L7 traffic, Gateway API HTTP/gRPC routing, AI Gateway semantic caching, and CWC security policies.
- **Agentic Workflow Built-In** — First-class AI pair-programming support with `awsbnkctl agent <cli>` (claude, gemini, aider, openai, pi, opencode) and append-only operational journaling (`awsbnkctl journal`). The binary embeds no LLM and no MCP server — bring your own coding-agent CLI.
- **BNK Forge Integration** — One-line cluster registration with a BNK Forge instance over MCP (`awsbnkctl forge register`), plus AI benchmark telemetry (`awsbnkctl benchmark`).
- **Cross-Platform Single Binary** — Statically compiled for Linux (amd64, arm64), macOS (Apple Silicon arm64, Intel amd64), and Windows.

---

## Pinned Versions

| Component | Pinned Version / Range | Notes |
|---|---|---|
| **BNK** | `2.3.0` | Primary supported release (`2.3.x` via `manifestVersion` override) |
| **CNE Release Manifest** | `2.3.0-3.2598.3-0.0.170` (default) | Pulled from `repo.f5.com`; override per cluster with `bnk.manifestVersion` (e.g. `2.3.2-3.2598.3-0.0.392`) |
| **Kubernetes (EKS)** | `1.34` – `1.35` (Default: `1.34`) | `validate` rejects < 1.34; 1.36+ warns (BNK CRDs fail to apply) |
| **AWS SDK for Go** | `v2` (`github.com/aws/aws-sdk-go-v2 v1.42.0`) | Direct AWS API communication, no Terraform |
| **cert-manager** | `v1.16.1` | Embedded upstream YAML applied via `client-go` (no Helm); override with `bnk.certManagerVersion` |
| **FLO Helm Chart** | `v2.21.13-0.0.28` (default) | Override with `addons.flo.version` |
| **Go** | `1.26` | `go.mod` toolchain floor |

---

## Installation

### Option 1: Pre-Built Binary (Recommended)

Download the archive for your OS and architecture from [GitHub Releases](https://github.com/JLCode-tech/awsbnkctl/releases/latest), unpack, and place on your `PATH`. Assets are named `awsbnkctl_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) alongside a `checksums.txt`:

```bash
VERSION=1.3.1   # see https://github.com/JLCode-tech/awsbnkctl/releases/latest

# macOS (Apple Silicon)
curl -fsSL https://github.com/JLCode-tech/awsbnkctl/releases/download/v${VERSION}/awsbnkctl_${VERSION}_darwin_arm64.tar.gz | tar -xz
sudo mv awsbnkctl /usr/local/bin/

# Linux (x86_64)
curl -fsSL https://github.com/JLCode-tech/awsbnkctl/releases/download/v${VERSION}/awsbnkctl_${VERSION}_linux_amd64.tar.gz | tar -xz
sudo mv awsbnkctl /usr/local/bin/
```

Verify installation:
```bash
awsbnkctl version
```

Once installed, `awsbnkctl self update` fetches the latest release for the host OS/arch, and `awsbnkctl install` copies the running binary onto your `PATH`.

### Option 2: Go Install (Go 1.26+)

```bash
go install github.com/JLCode-tech/awsbnkctl/cmd/awsbnkctl@latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/JLCode-tech/awsbnkctl.git
cd awsbnkctl
make build
# Binary created at bin/awsbnkctl
```

### Option 4: In-Place Self Upgrade

```bash
awsbnkctl self update
```

---

## Prerequisites

| Requirement | Purpose |
|---|---|
| **AWS Account & IAM Credentials** | VPC, EKS, EC2, IAM roles, S3 bucket provisioning |
| **F5 FAR Pull Secret & JWT License** | Authenticating to `repo.f5.com` and activating BNK licenses |
| **`aws` CLI & `ssh` (Optional)** | Only required when running live scenario probes via EC2 Instance Connect jumphost |
| **Go 1.26+ (Optional)** | Only required when compiling from source |

> [!NOTE]
> You do **not** need `terraform`, `kubectl`, `helm`, or `docker` installed on your machine.

---

## Quick Start

### 1. Initialize Configuration

Scaffold a starter configuration from one of the reference blueprints:

```bash
# Copy reference full-cluster configuration
cp examples/full-cluster/cluster.yaml my-cluster.yaml
```

Edit `my-cluster.yaml` to specify your AWS region, CIDRs, node types, and credentials (the loader is strict — unknown keys are rejected by `validate`):
```yaml
apiVersion: awsbnkctl/v1
kind: Cluster

metadata:
  name: sydney-e2e-cluster
  region: ap-southeast-2

pattern: dual-interface

cluster:
  kubernetesVersion: "1.34"
  nodeGroups:
    - name: tmm-workers
      instanceType: m6i.4xlarge
      desiredSize: 2

bnk:
  farArchive: ./secrets/f5-far-credentials.json
  jwt: ./secrets/license.jwt
```

### 2. Preflight Diagnostics

Run `doctor` and `validate` to verify your AWS credentials and configuration syntax before making any changes:

```bash
awsbnkctl doctor
awsbnkctl validate my-cluster.yaml
```

Preview the deployment plan without AWS credentials using `--dry-run`:
```bash
AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up -f my-cluster.yaml --dry-run
```

### 3. Deploy Environment

Provision the VPC, EKS cluster, secondary ENIs, and BNK control/data plane:

```bash
awsbnkctl up -f my-cluster.yaml
```

Add `--demo` to enable the interactive, audience-facing live launch console.

### 4. Run Verification Scenarios

Validate live data-plane traffic through the TMM interfaces and Gateway API routes:

```bash
# List available scenarios
awsbnkctl scenarios list

# Run end-to-end HTTP routing scenario
awsbnkctl scenarios run http-routing-e2e -f my-cluster.yaml
```

### 5. Teardown

Safely delete all AWS infrastructure, ENIs, security groups, and EKS resources:

```bash
awsbnkctl down -f my-cluster.yaml --yes
```

---

## Data-Plane Patterns

The top-level `pattern` field selects the TMM data-plane interface topology and binding:

| Pattern | Interfaces | Datapath Binding | Min ENIs | Primary Use Case |
|---|---|---|---|---|
| **`external-only`** | External only | `host-device` (Kernel) | 2 | Single-arm Ingress Gateway / North-South ingress |
| **`dual-interface`** | External + Internal | `host-device` (Kernel) | 3 | Dual-arm firewall / Ingress + Egress inspection |
| **`sriov-external`** | External only | SR-IOV (`vfio-pci` DPDK) | 2 | High-throughput line-rate packet processing |

`host-device` is accepted as a legacy alias for `dual-interface`.

---

## Configuration

Every knob lives in `cluster.yaml` (schema: `internal/intent/cluster.go`). Two optional blocks worth knowing about:

- **BGP peering** — `bnk.bgp: true` (alias `bnk.dynamicRouting: true`) admits TCP 179 / UDP 3784 from the external data-path subnet into the data-plane security group (phase 07) and opens the same ports on the external `F5SPKVlan` (phase 23b), so TMM can peer with an AWS Route Server endpoint in that subnet. Every deployable example sets it. The peer itself and the `RoutingTemplate` / `GlobalRoutingConfig` CRs are yours to add: each example ships a `bgp-route-server.yaml`, and [`docs/BGP-ROUTE-SERVER.md`](docs/BGP-ROUTE-SERVER.md) is the end-to-end procedure.
- **Shared Forge project** — `forge.projectName` registers the cluster into an existing BNK Forge project instead of the auto-created `awsbnkctl-<cluster>` one (env override `AWSBNKCTL_FORGE_PROJECT`). When a non-default name is set, `down` unregisters the cluster but does not purge the shared project.

Environment variables recognised by the binary:

| Variable | Effect |
|---|---|
| `AWSBNKCTL_SKIP_AUTH=1` | Skip AWS credential resolution; only valid together with `--dry-run` on `up` / `down` |
| `AWSBNKCTL_HOME` | Override the workspace/state root directory (legacy alias `ROKSBNKCTL_HOME` is still honoured) |
| `AWSBNKCTL_FORGE_URL` | BNK Forge REST base URL (overrides `forge.url`; default `http://localhost:8000`) |
| `AWSBNKCTL_FORGE_MCP_URL` | BNK Forge MCP endpoint (overrides `forge.mcpUrl`; default `http://localhost:8081/mcp/`) |
| `AWSBNKCTL_FORGE_USERNAME` / `AWSBNKCTL_FORGE_PASSWORD` | Forge credentials; the password is never read from YAML in production use |
| `AWSBNKCTL_FORGE_PROJECT` | Forge project name to register into (overrides `forge.projectName`) |
| `AWSBNKCTL_FORGE_ENVIRONMENT` | Forge project environment, e.g. `dev`, `staging`, `prod` (overrides `forge.environment`) |
| `AWSBNKCTL_BIGIP_PASSWORD` | BIG-IP VE admin password for the `bigipVE` onboarding phase and the `bigip-cis` demo |
| `AWSBNKCTL_GPU_AZ_DENY` | Extra GPU instance-type AZ deny entries, format `region:az1,az2;region2:az3` |
| `AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1` | Opt `doctor` into the AWS Service Quotas checks |
| `AWSBNKCTL_SSH_TARGET` / `AWSBNKCTL_K8S_LONG_LIVED` | Internal sentinels the CLI sets when re-dispatching a command to an `ssh:<target>` or `k8s` execution backend; not meant to be set by operators |

---

## Scenarios Catalogue

`awsbnkctl` includes 15 automated validation scenarios covering L4/L7 routing, security policies, AI Gateway features, and platform diagnostics:

| Scenario Name | Category | Description | Verification Method |
|---|---|---|---|
| **`http-routing-e2e`** | Ingress L7 | Gateway API HTTPRoute path-based routing | EICE Jumphost curl / In-cluster |
| **`http-traffic-split`** | Traffic Mgmt | Weighted traffic splitting across multiple service backends | EICE Jumphost curl |
| **`external-resource-pool`** | Hybrid Routing | Routing traffic to non-Kubernetes external endpoints | EICE Jumphost curl |
| **`proxy-protocol-l4`** | L4 Protocol | Proxy Protocol v1/v2 client IP preservation | EICE raw socket / curl |
| **`tcp-l4-loadbalance`** | L4 Protocol | L4Route TCP weighted load balancing (70/30) | Multi-request TCP probe |
| **`udp-l4-loadbalance`** | L4 Protocol | L4Route UDP datagram routing and load balancing | UDP echo probe |
| **`grpc-loadbalance`** | L7 Protocol | gRPC stream routing over GRPCRoute & L4Route | `grpcurl` VIP probe |
| **`cluster-wide-watch`** | Multi-Tenancy | Cross-namespace HTTP routing via CWC | Multi-namespace curl |
| **`cwc-admin-access`** | Security | ClusterWideWatch RBAC isolation & cert validation | RBAC assertion & mTLS probe |
| **`ai-token-counting`** | AI Gateway | Token usage measurement and rate limiting | AI Gateway HTTP POST |
| **`ai-semantic-cache`** | AI Gateway | Semantic prompt cache hit/miss verification | AI Gateway HTTP POST |
| **`ai-inference-e2e`** | AI Gateway | End-to-end SageMaker / GPU inference routing | AWS SDK / EICE Jumphost |
| **`multi-vip`** | Scalability | Multiple Gateway VIPs on a single TMM instance | EICE Jumphost curl |
| **`egress-snat`** | Egress Security | Outbound SNAT and egress firewall filtering | EICE Jumphost curl |
| **`core-file-collection`** | Observability | TMM core dump probe and health diagnostics | Pod diagnostic inspect |

Run scenarios individually or execute the complete test suite:
```bash
awsbnkctl scenarios run <scenario-name> -f my-cluster.yaml
```

---

## Agentic Workflow

`awsbnkctl` is built from the ground up for human-AI pair programming and autonomous execution:

### 1. Agent Scaffolding (`agent`)
Scaffold the agentic-mode files into the workspace, then print the invocation for your coding-agent CLI. `awsbnkctl` embeds no LLM — bring your own CLI:

```bash
awsbnkctl agent init      # Scaffold AGENTS.md + personas/ + journal/ into the workspace
awsbnkctl agent           # List supported CLIs and this workspace's default
awsbnkctl agent claude    # Print the invocation to launch Claude Code against the workspace
awsbnkctl agent gemini    # Likewise for gemini; also: aider, openai, pi, opencode
```

### 2. Operational Journal (`journal`)
Maintain an audit-safe, append-only operational run log:

```bash
awsbnkctl journal add "Provisioned Sydney E2E cluster and ran full scenario suite"
awsbnkctl journal list
awsbnkctl journal report   # Assembles report.md from decisions.md + the journal timeline
```

### 3. Model Context Protocol (MCP)
`awsbnkctl` does **not** ship an MCP server. It is an MCP *client*: `forge register`, `up --register-with-forge`, and `benchmark` talk to a BNK Forge instance over its MCP endpoint (see below).

---

## BNK Forge Integration

Connect your `awsbnkctl` clusters to [BNK Forge](https://github.com/f5devcentral/bnk-forge) for centralized fleet management and benchmark telemetry. The Forge endpoints and credentials come from the `forge:` block in `cluster.yaml` or the `AWSBNKCTL_FORGE_*` environment variables (see [Configuration](#configuration)):

```bash
# Register the workspace's EKS cluster with Forge (idempotent); --scan runs a smoke check afterwards
awsbnkctl forge register -f my-cluster.yaml --scan
awsbnkctl forge register --cluster-name my-eks --kubeconfig ~/.kube/config --project-name shared-project

# Check this workspace's registration state
awsbnkctl forge status

# Unregister the cluster; `cleanup` purges every awsbnkctl benchmark artifact for the workspace
awsbnkctl forge unregister
awsbnkctl forge cleanup
```

`awsbnkctl up --register-with-forge` performs the registration automatically after a successful apply, and `down` removes it unless `--keep-forge-link` is passed.

---

## Command Reference

Run `awsbnkctl <command> --help` for the full flag list. Global flags on every command: `-w/--workspace`, `-o/--output text|json`, `-v/--verbose`, `-q/--quiet`, `--no-color`, `--backend local|docker|k8s|ssh:<target>`, `--on <target>`, `--bootstrap`, `--insecure-host-key`.

### Lifecycle

| Command | Description |
|---|---|
| **`init`** | Interactive AWS setup; collects region, VPC, subnets, FAR archive and JWT, writes the workspace config (`--dry-run` skips the upload step) |
| **`validate <path>`** | Parse and validate a `cluster.yaml` (no AWS API calls) |
| **`up -f <config>`** | Provision the EKS cluster + BNK stack via the phased path (flags: `--dry-run`, `--auto`, `--demo`, `--no-kubeconfig`, `--register-with-forge`, `--skip-activation-poll`) |
| **`down -f <config>`** | Destroy everything provisioned by `up` (flags: `--dry-run`, `--yes`, `--auto`, `--keep-irsa`, `--keep-forge-link`) |
| **`status`** | Summary of the workspace: cluster, components, deploy state (`-f` locates the phased path's `state.env`) |
| **`doctor`** | Check prerequisites and report missing pieces (`--backend k8s|ssh:<target>`, `--target <name>` add per-backend probes) |
| **`topology -f <config>`** | Render the cluster data-path topology (VPC, TMM VLANs, jumphost, gateways) as `--format ascii` or `mermaid` |
| **`version`** | Print version, commit, and build date |

### Validation & Demos

| Command | Description |
|---|---|
| **`test [suite]`** | Run deployment validation tests (default: all; `--dry-run` prints the probe plan, `--insecure` skips TLS validation) |
| **`test connectivity`** | HTTP/HTTPS reachability against configured hosts |
| **`test dns`** | DNS resolution probe (single-vantage, GSLB-compare, or workspace-driven) |
| **`test throughput`** | iperf3 throughput; deploys the server pod automatically |
| **`test traffic`** | Drive HTTP traffic through TMM from the test jumphost (alias for `scenarios run http-routing-e2e`) |
| **`test list`** | List available test suites |
| **`test hosts {add,clear,list,remove}`** | Manage `test.connectivity.extra_hosts` in the workspace config |
| **`scenarios list`** | Print registered scenarios |
| **`scenarios run <name>`** | Run a scenario (or `--all`) |
| **`scenarios clean <name>`** | Invoke a scenario's Cleanup hook |
| **`demo list`** | Print registered demo use-cases and Green scenarios |
| **`demo run <name>`** | Run a demo use-case (or `--all`); requires a demo cluster |
| **`demo clean <name>`** | Invoke a demo use-case's idempotent Cleanup hook (or `--all`) |
| **`demo preview`** | Play the up/down rocket animation locally (no AWS) to preview the demo UX |

### Kubernetes & BNK Runtime

| Command | Description |
|---|---|
| **`k apply`** | Server-side apply YAML/JSON manifests, directories, or kustomize bases |
| **`k delete`** | Delete resources by name or label selector |
| **`k describe`** | Show detailed human-readable resource info (events, conditions, related objects) |
| **`k exec`** | Exec into a pod via SPDY (kubectl-equivalent in-process) |
| **`k get`** | Get one or more resources (pods, nodes, services, CRDs, …) |
| **`k logs`** | Stream pod logs (kubectl-equivalent direct path) |
| **`k port-forward`** | Forward local port(s) to a pod via SPDY |
| **`get <resource> [name]`** | Top-level alias of `k get` (`-n`, `-A`, `-l`, `-o yaml|json|wide|name|jsonpath=…`) |
| **`logs <component>`** | Tail logs for a BNK component (`flo`, `cis`, `cert-manager`, `cneinstance`); `-f`, `--since`, `--tail`, `--previous`, `-c` |
| **`bnk resync`** | Force the F5 cne-controller to re-resolve stale TMM pool members |
| **`manifest probe`** | Pull and inspect a BNK release manifest from `repo.f5.com` |

### AI Benchmarking & BNK Forge

| Command | Description |
|---|---|
| **`benchmark`** | Runs the default `benchmark run` workflow when invoked directly |
| **`benchmark setup`** | Prepare the jumphost (aiperf) and register the benchmark agent and target in Forge |
| **`benchmark run`** | Drive an aiperf run, preset (`--scenarios`), native Forge scenario sweep (`--scenario`), or proxy shootout (`--proxies`) |
| **`benchmark list`** | List available native Forge scenarios and smoke presets |
| **`benchmark status`** | Check the benchmark environment, jumphost, and Forge linkage |
| **`benchmark daemon`** | Run the persistent Forge benchmark agent daemon |
| **`forge register`** | Register the workspace's EKS cluster with Forge (idempotent); `--cluster-name`, `--kubeconfig`, `--project-name`, `--scan` |
| **`forge status`** | Show this workspace's Forge registration state |
| **`forge unregister`** | Remove this workspace's Forge registration |
| **`forge cleanup`** | Delete all awsbnkctl benchmark artifacts from Forge for a workspace (full purge) |
| **`forge benchmark`** | Alias for `benchmark run` |

### Agentic Workflow

| Command | Description |
|---|---|
| **`agent`** | List supported coding-agent CLIs and this workspace's default |
| **`agent init`** | Scaffold `AGENTS.md`, `personas/`, and `journal/` into the workspace |
| **`agent <cli>`** | Print the invocation to launch `claude`, `gemini`, `aider`, `openai`, `pi`, or `opencode` against the workspace |
| **`journal add <note>`** | Append a note to today's journal entry |
| **`journal list`** | List journal entries (chronological) with one-line summaries |
| **`journal report`** | Assemble `report.md` from `decisions.md` + the journal timeline |

### Workspaces, Targets & Maintenance

| Command | Description |
|---|---|
| **`workspaces list`** | List workspaces and their states |
| **`workspaces current`** | Print the current workspace name |
| **`workspaces new <name>`** | Create a new (empty) workspace skeleton; run `init -w <name>` to populate |
| **`workspaces use <name>`** | Set the current workspace pointer |
| **`workspaces delete <name>`** | Delete a workspace (refuses if state is non-empty unless `--force`) |
| **`targets {add,list,remove,show}`** | Manage the SSH targets used by `--on` / `--backend ssh:<target>` |
| **`install`** | Copy the running binary into a directory on `PATH` (`--dir`, `--force`) |
| **`self update`** | Pull the latest release matching the host OS/arch |
| **`completion <shell>`** | Generate the shell completion script |
| **`help [command]`** | Help about any command |

---

## Repository Layout

```
awsbnkctl/
├── cmd/awsbnkctl/         # CLI binary entrypoint (main.go)
├── internal/
│   ├── aws/               # aws-sdk-go-v2 wrappers (VPC, EKS, EC2, IAM, S3, STS), tags, state
│   │   └── phases/        # The 39 ordered provisioning/teardown phases
│   ├── bnkconst/          # BNK-wide constants shared across packages
│   ├── cli/               # Cobra command tree (all commands; version vars stamped via ldflags)
│   ├── config/            # Workspace paths and global config (not the cluster.yaml schema)
│   ├── demo/              # Demo use-case registry + narration (diameter, http2, bigip-cis, ingress-migration)
│   ├── doctor/            # Prerequisite checks behind `awsbnkctl doctor`
│   ├── embedded/          # Agentic-mode scaffolding shipped inside the binary
│   ├── exec/              # Execution backends: local, docker, k8s, ssh:<target>
│   ├── forge/             # BNK Forge MCP/REST client (register, unregister, benchmark)
│   ├── intent/            # cluster.yaml schema (v1), loader, and validation
│   ├── jumphost/          # SSH-via-EICE probe utilities for the test jumphost
│   ├── k8s/               # Embedded client-go wrapper (k verbs), embedded manifests, renderers
│   ├── manifest/          # F5 release-manifest (BOM) fetch and probe
│   ├── remote/            # Embedded SSH client and target plumbing
│   ├── scenarios/         # 15 end-to-end validation scenarios
│   ├── test/              # connectivity / dns / throughput probe runners
│   ├── topology/          # Data-path topology model + ASCII/mermaid renderers
│   └── ui/                # Terminal output primitives (spinners, progress, colour)
├── pkg/bnk/               # Exported BNK runtime helpers (pool-member resync)
├── docs/                  # Architecture, phases, scenarios, Forge integration, release guides
├── examples/              # Ready-to-deploy cluster topologies and reference blueprints
│   ├── full-cluster/      # Complete dual-interface reference stack (demo + BIG-IP VE as commented blocks)
│   ├── external-only/     # Single-arm ingress blueprint (one-line swap to sriov-external)
│   ├── egress-demo/       # Transparent egress + egress firewall ACL blueprint
│   ├── ai-rig/            # BNK fronting GPU inference, optional SageMaker endpoint
│   ├── demo-ai/           # full-cluster + ai-rig composed: all protocol demos plus managed inference
│   ├── agentcore-demo/    # One MCP tool pod behind a BNK Gateway; AgentCore runtime → BNK → tool governance
│   └── local-zone/        # Reference telco/edge CRs (SCTP, Diameter, HTTP/2, SNAT pool); no cluster.yaml
├── scripts/               # e2e and gate scripts (pre-commit, govulncheck, integration)
├── tools/                 # BNK Forge runner packaging, docker helpers, ciwatch/sprintwatch
└── Makefile               # Build, test, lint, and release recipes
```

---

## Contributing & License

Contributions are welcome! Please refer to [`CONTRIBUTING.md`](CONTRIBUTING.md) for local development workflows, testing requirements, and release procedures.

Distributed under the [MIT License](LICENSE). © 2026 JLCode-tech.
