# awsbnkctl

![BNK](https://img.shields.io/badge/BNK-2.3.2-0a3a5c)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.32--1.35-326ce5?logo=kubernetes&logoColor=white)
![AWS EKS](https://img.shields.io/badge/AWS-EKS-ff9900?logo=amazon-aws&logoColor=white)
[![CI](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](go.mod)
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

- **Imperative Phased Provisioner** — 39 deterministic, ordered phases driven directly via the AWS Go SDK. AWS resource tags serve as the single source of truth; local state caches accelerate subsequent runs and are fully reconstructible from AWS tags.
- **Embedded Kubernetes Engine (`k`)** — Native `client-go` implementation provides `get`, `apply`, `describe`, `delete`, `logs`, `exec`, and `port-forward` with zero host `kubectl` dependencies.
- **End-to-End Traffic Scenarios** — 15 built-in scenarios validate live L4/L7 traffic, Gateway API HTTP/gRPC routing, AI Gateway semantic caching, and CWC security policies.
- **Agentic Workflow Built-In** — First-class AI pair-programming support with `awsbnkctl agent [claude, gemini, chatgpt, ...]`, append-only operational journaling (`awsbnkctl journal`), and an embedded Model Context Protocol (MCP) server.
- **BNK Forge Integration** — Instant one-line cluster registration and real-time data plane topology visualization with `awsbnkctl forge register`.
- **Cross-Platform Single Binary** — Statically compiled for Linux (amd64, arm64), macOS (Apple Silicon arm64, Intel amd64), and Windows.

---

## Pinned Versions

| Component | Pinned Version / Range | Notes |
|---|---|---|
| **BNK** | `2.3.2` | Primary supported release |
| **CNE Release Manifest** | `2.3.2-3.2598.3-0.0.392` | Resolved dynamically from FAR registry |
| **Kubernetes (EKS)** | `1.32` – `1.35` (Default: `1.32`) | Preflight gate rejects < 1.32; 1.36+ warns |
| **AWS Go SDK** | `v2` (`v1.44.300+`) | Direct AWS API communication |
| **cert-manager** | `v1.16.2` | Managed via embedded Helm installer |
| **FLO Helm Chart** | `v2.21+` | Resolved at deploy time from release manifest |

---

## Installation

### Option 1: Pre-Built Binary (Recommended)

Download the archive for your OS and architecture from [GitHub Releases](https://github.com/JLCode-tech/awsbnkctl/releases/latest), unpack, and place on your `PATH`:

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/JLCode-tech/awsbnkctl/releases/latest/download/awsbnkctl_darwin_arm64.tar.gz | tar -xz
sudo mv awsbnkctl /usr/local/bin/

# Linux (x86_64)
curl -fsSL https://github.com/JLCode-tech/awsbnkctl/releases/latest/download/awsbnkctl_linux_amd64.tar.gz | tar -xz
sudo mv awsbnkctl /usr/local/bin/
```

Verify installation:
```bash
awsbnkctl version
```

### Option 2: Go Install (Go 1.25+)

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
| **Go 1.25+ (Optional)** | Only required when compiling from source |

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

Edit `my-cluster.yaml` to specify your AWS region, CIDRs, node types, and credentials:
```yaml
metadata:
  name: sydney-e2e-cluster
  region: ap-southeast-2

cluster:
  kubernetesVersion: "1.32"
  nodeGroups:
    - name: tmm-workers
      instanceType: m5.xlarge
      desiredCapacity: 2

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

The `pattern` field selects the TMM data-plane interface topology and binding:

| Pattern | Interfaces | Datapath Binding | Min ENIs | Primary Use Case |
|---|---|---|---|---|
| **`external-only`** | External only | `host-device` (Kernel) | 2 | Single-arm Ingress Gateway / North-South ingress |
| **`dual-interface`** | External + Internal | `host-device` (Kernel) | 3 | Dual-arm firewall / Ingress + Egress inspection |
| **`sriov-external`** | External only | SR-IOV (`vfio-pci` DPDK) | 2 | High-throughput line-rate packet processing |

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
| **`corefiles`** | Observability | TMM core dump probe and health diagnostics | Pod diagnostic inspect |

Run scenarios individually or execute the complete test suite:
```bash
awsbnkctl scenarios run <scenario-name> -f my-cluster.yaml
```

---

## Agentic Workflow

`awsbnkctl` is built from the ground up for human-AI pair programming and autonomous execution:

### 1. Agent Scaffolding (`agent`)
Generate AI coding agent instructions and workspace prompt bundles:

```bash
awsbnkctl agent claude   # Outputs CLAUDE.md tailored for Claude Code
awsbnkctl agent gemini   # Outputs instructions for Gemini / Antigravity
awsbnkctl agent chatgpt  # Outputs instructions for ChatGPT / OpenAI Codex
```

### 2. Operational Journal (`journal`)
Maintain an audit-safe, append-only operational run log:

```bash
awsbnkctl journal add "Provisioned Sydney E2E cluster and ran full scenario suite"
awsbnkctl journal list
awsbnkctl journal report --format markdown
```

### 3. Model Context Protocol (MCP) Server
Integrate `awsbnkctl` directly into AI desktop environments and IDEs via its built-in MCP server:

```json
{
  "mcpServers": {
    "awsbnkctl": {
      "command": "awsbnkctl",
      "args": ["mcp", "serve"]
    }
  }
}
```

---

## BNK Forge Integration

Connect your `awsbnkctl` clusters to [BNK Forge](https://github.com/f5devcentral/bnk-forge) for centralized fleet management, live traffic flow visualization, and policy orchestration:

```bash
# Register cluster with BNK Forge instance
awsbnkctl forge register --endpoint https://forge.example.com --token $FORGE_TOKEN

# Check registration and telemetry status
awsbnkctl forge status

# Unregister cluster during teardown
awsbnkctl forge unregister
```

---

## Command Reference

| Command | Description |
|---|---|
| **`validate <config>`** | Validate configuration schema and network CIDR consistency |
| **`up -f <config>`** | Provision AWS infrastructure and deploy BNK (flags: `--dry-run`, `--demo`, `--auto`) |
| **`down -f <config>`** | Teardown BNK and AWS infrastructure (flags: `--dry-run`, `--yes`) |
| **`status`** | Display workspace status, phase state, and pod health |
| **`doctor`** | Run preflight diagnostics on AWS credentials, IAM, and networking |
| **`scenarios {list,run,clean}`** | Manage and execute data-plane verification scenarios |
| **`demo {list,run,clean}`** | Run interactive audience-facing walkthroughs and migration stories |
| **`topology`** | Render ASCII diagram of VPC, subnets, TMM VLANs, and gateways |
| **`k <verb> [args]`** | Embedded Kubernetes passthrough (`get`, `apply`, `describe`, `logs`, `exec`) |
| **`manifest probe [version]`** | Inspect F5 FAR release manifest charts and image digests |
| **`workspaces {list,use,new}`** | Manage isolated multi-cluster deployment environments |
| **`journal {add,list,report}`** | Maintain operational execution journal |
| **`agent {claude,gemini,...}`** | Generate AI pair-programming workspace bundles |
| **`forge {register,status}`** | Manage BNK Forge fleet registration |
| **`self update`** | In-place self-upgrade to the latest release |

---

## Repository Layout

```
awsbnkctl/
├── cmd/awsbnkctl/         # CLI binary entrypoint (main.go)
├── internal/
│   ├── aws/               # AWS SDK client, resource graph, and 39 provisioning phases
│   ├── bnk/               # BNK manifests, FAR client, CNE & FLO templates
│   ├── cli/               # Cobra commands, agentic workflows, journal, workspaces
│   ├── k8s/               # Embedded client-go wrapper, CRDs, resource helpers
│   ├── scenarios/         # 15 automated traffic and AI validation scenarios
│   └── topology/          # ASCII topology visualizer
├── docs/                  # Architecture, phases, scenarios, and alignment guides
├── examples/              # Ready-to-deploy cluster topologies and reference blueprints
│   ├── full-cluster/      # Comprehensive reference configuration
│   ├── external-only/     # Single-arm ingress blueprint
│   ├── egress-demo/       # Outbound SNAT and firewall blueprint
│   ├── ai-rig/            # AI Gateway + SageMaker GPU inference blueprint
│   └── agentcore-demo/    # AI Agent infrastructure + MCP tool orchestration
├── tools/                 # Container runner packaging for BNK Forge
└── Makefile               # Build, test, lint, and release recipes
```

---

## Contributing & License

Contributions are welcome! Please refer to [`CONTRIBUTING.md`](CONTRIBUTING.md) for local development workflows, testing requirements, and release procedures.

Distributed under the [MIT License](LICENSE). © 2026 JLCode-tech.
