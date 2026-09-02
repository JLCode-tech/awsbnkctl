# *bnkctl Family Architecture & Feature Alignment Reference

This document records the comparative analysis, architectural mapping, and feature alignment across the coordinated family of F5 BIG-IP Next for Kubernetes (BNK) CLI tools:

1. **`awsbnkctl`** (`JLCode-tech/awsbnkctl`) — AWS EKS & EC2 cloud target (Phased AWS Go SDK).
2. **`roksbnkctl`** (`jgruberf5/roksbnkctl`) — IBM Cloud ROKS / OpenShift target.
3. **`ocibnkctl`** (`mwiget/ocibnkctl`) — Local OCI / k3s container runtime target.
4. **`gkebnkctl`** (`JLCode-tech/gkebnkctl`) — GCP GKE target (host-device secondary VPC interfaces).
5. **`bnkctl-index`** (`mwiget/bnkctl-index`) — Curated index of `*bnkctl` tools packaged for BNK Forge.

---

## 1. Architectural Philosophy

All tools across the `*bnkctl` ecosystem share common core tenets:
- **Single-binary Go CLI**: Zero external deployment orchestrator dependencies (Terraform/kubectl decoupled from runtime where possible).
- **Declarative Intent**: One intent configuration (`cluster.yaml` or `config.yaml` or `poc.yaml`) driving lifecycle operations.
- **Embedded Kubernetes Management (`k`)**: Direct `client-go` integration providing `get`, `apply`, `describe`, `delete`, `logs`, `exec`, `port-forward`.
- **Preflight Diagnostics (`doctor`)**: Comprehensive reachability, credential, and subsystem health checking before mutations.
- **End-to-End Validation (`scenarios` / `test`)**: Verification suites running live L4/L7 traffic against TMM data-plane VIPs.
- **Platform Integrations**: First-class support for `bnk-forge` registration, topology telemetry, and MCP tool orchestration.

---

## 2. Feature & Command Alignment Matrix

| Feature Area | `roksbnkctl` | `ocibnkctl` | `gkebnkctl` | `awsbnkctl` | Alignment Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Lifecycle** | `init`, `up`, `down`, `plan`, `apply` | `init`, `cluster up`, `deploy`, `destroy`, `scale` | `init`, `up`, `down`, `validate` | `init`, `up`, `down`, `validate` | Full parity (Cloud-specific) |
| **Agentic Workflow** | `agent [init, claude, ...]`, `journal`, personas | `agent [claude, ...]`, `CLAUDE.md` | `agent [init, claude, ...]`, `journal`, personas | `agent [init, claude, gemini, ...]`, `journal`, personas | Full parity |
| **Kubernetes Passthrough** | `k {get, apply, describe, delete, logs, exec, port-forward}` | `kubectl` wrapper | `k {get, apply, describe, delete, logs, exec, port-forward}` | `k {get, apply, describe, delete, logs, exec, port-forward}` | Full parity |
| **Release Manifest Probe** | `registry bom` / `cos` | `manifest probe [version]` | `manifest probe [version]` | `manifest probe [version] [--far]` | Full parity |
| **Configuration Test Hosts**| `test hosts {list, add, remove, clear}` | Via `poc.yaml` | `test hosts {list, add, remove, clear}` | `test hosts {list, add, remove, clear}` | Full parity |
| **Workspaces** | `workspaces {list, new, use, current, delete}` | `cluster {use, list}` | `workspaces {list, new, use, current, delete}` | `workspaces {list, new, use, current, delete}` | Full parity |
| **Diagnostics & Health** | `doctor` | `doctor` | `doctor` | `doctor` | Full parity |
| **Self Management** | `self upgrade` | Pinned version | `self update` | `self update` | Full parity |
| **BNK Forge Integration** | `bnkforge` | `bnk-forge` | `forge` | `forge` (with MCP server) | Full parity |
| **Data Visualization** | mdBook docs | Asciinema / video | ASCII topology | `topology` ASCII render + `--demo` engine | Super-set in AWS |

---

## 3. Scenarios Catalogue Matrix

| Scenario Name | Focus Area | `ocibnkctl` | `roksbnkctl` | `gkebnkctl` | `awsbnkctl` | Verification Method |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| `http-routing-e2e` | Gateway API HTTPRoute basic routing | Yes | Yes (`test connectivity`) | Yes | Yes | EICE Jumphost curl / In-cluster |
| `http-traffic-split` | Weighted traffic splitting across services | Yes (in L4) | - | Yes | Yes | EICE Jumphost curl / In-cluster |
| `external-resource-pool`| Routing to external non-K8s endpoints | Yes | - | Yes | Yes | EICE Jumphost curl / In-cluster |
| `proxy-protocol-l4` | Proxy Protocol v1/v2 header preservation | Yes | - | Yes | Yes | EICE Jumphost raw socket / curl |
| `ai-token-counting` | AI Gateway token counting & rate limits | Yes | - | Yes | Yes | AI Gateway HTTP POST |
| `ai-semantic-cache` | AI Gateway semantic caching | Yes | - | Yes | Yes | AI Gateway HTTP POST |
| `ai-inference-e2e` | SageMaker / GPU model inference pipeline | - | - | - | Yes | AWS SDK / EICE Jumphost |
| `multi-vip` | Multiple Gateway VIPs on same TMM | Yes | - | Yes | Yes | EICE Jumphost curl |
| `egress-snat` | Egress gateway SNAT & firewalling | - | - | Yes | Yes | EICE Jumphost curl |
| `grpc-loadbalance` | gRPC over L4Route & GRPCRoute | Yes | - | Yes | Yes | `grpcurl` probe to Gateway VIP |
| `tcp-l4-loadbalance` | L4Route TCP weighted load balancing (70/30) | Yes | - | Yes | Yes | Multi-request TCP probe |
| `udp-l4-loadbalance` | L4Route UDP packet routing | Yes | - | Yes | Yes | UDP echo probe |
| `cluster-wide-watch` | Cross-namespace HTTP routing with CWC | Yes | - | Yes | Yes | Multi-namespace curl |
| `cwc-admin-access` | ClusterWideWatch RBAC & admin isolation | Yes | - | Yes | Yes | RBAC assertion & curl |
| `corefiles` | TMM crash dump detection and pod health | Yes | - | Yes | Yes | Pod diagnostic inspect |
