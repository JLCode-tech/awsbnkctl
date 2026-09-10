# Scenarios Catalogue for awsbnkctl

This document provides a detailed breakdown of all 15 automated validation scenarios available in `awsbnkctl`.

Scenarios validate live data-plane traffic, protocol handshakes, and security policies against the provisioned EKS cluster and F5 BIG-IP Next for Kubernetes (BNK) TMM interfaces.

---

## Running Scenarios

```bash
# List all registered scenarios and their rating
awsbnkctl scenarios list

# Run a specific scenario against your cluster
awsbnkctl scenarios run <scenario-name> -f my-cluster.yaml

# Clean up scenario-specific test fixtures and namespaces
awsbnkctl scenarios clean <scenario-name> -f my-cluster.yaml
```

---

## 1. Ingress & L7 Protocol Scenarios

### `http-routing-e2e`
- **Objective**: Validates standard Kubernetes Gateway API `Gateway` + `HTTPRoute` routing.
- **Traffic Path**: Client (EC2 jumphost via EICE tunnel) $\to$ TMM VIP $\to$ `http-echo` backend pods.
- **Assertions**: HTTP 200 responses, expected response body headers, and correct backend pod matching.

### `http-traffic-split`
- **Objective**: Validates weighted canary / split traffic distribution.
- **Traffic Path**: Jumphost $\to$ TMM VIP $\to$ Backend Service A (80%) vs Backend Service B (20%).
- **Assertions**: Statistical distribution of responses matches defined weights within tolerance.

### `grpc-loadbalance`
- **Objective**: Validates gRPC stream handling and balancing across microservices.
- **Traffic Path**: (intended) client $\to$ TMM VIP (`L4Route` TCP data plane; `GRPCRoute` on the control plane) $\to$ `kong/grpcbin` backend pods.
- **Assertions**: Control plane only — `grpcbin` Deployment Available, both Gateways Programmed, `GRPCRoute` and `L4Route` Accepted. No traffic is sent; no jumphost needed.

---

## 2. L4 Protocol & Transport Scenarios

### `tcp-l4-loadbalance`
- **Objective**: Validates raw L4 TCP stream proxying via `L4Route`.
- **Traffic Path**: Jumphost TCP socket client $\to$ TMM L4 VIP:8080 $\to$ TCP echo servers.
- **Assertions**: TCP connection establishment, payload mirroring, 70/30 weight distribution.

### `udp-l4-loadbalance`
- **Objective**: Validates stateless UDP datagram routing and load distribution.
- **Traffic Path**: (intended) UDP client $\to$ TMM L4 VIP:5353 $\to$ UDP echo servers.
- **Assertions**: Control plane only — `udp-echo` Deployment Available, Gateway Programmed, `L4Route` Accepted. No datagrams are sent; no jumphost needed.

### `proxy-protocol-l4`
- **Objective**: Validates Proxy Protocol (v1 and v2) header processing.
- **Traffic Path**: Client sending Proxy Protocol header $\to$ TMM VIP $\to$ Backend pod.
- **Assertions**: Preservation of originating client IP address through TMM to backend logs.

---

## 3. Hybrid & Multi-Tenancy Scenarios

### `external-resource-pool`
- **Objective**: Validates BNK routing traffic to external endpoints located outside the Kubernetes cluster (e.g., bare-metal VMs or AWS RDS/ALB).
- **Traffic Path**: Jumphost $\to$ TMM VIP $\to$ External IP targets outside EKS pod CIDR.
- **Assertions**: Successful response proxying from non-cluster IP pools.

### `cluster-wide-watch` (CWC)
- **Objective**: Validates `ClusterWideWatch` CR for multi-tenant cross-namespace routing without full cluster admin permissions.
- **Traffic Path**: (intended) tenant client $\to$ TMM VIP $\to$ backend in a namespace created after BNK was installed.
- **Assertions**: Control plane only — the new namespace's Deployment becomes Available, its Gateway is Programmed and its HTTPRoute Accepted, proving the single controller reconciles namespaces it did not exist for. No traffic probe.

### `cwc-admin-access`
- **Objective**: Validates RBAC isolation and mTLS certificate verification within CWC.
- **Traffic Path**: Tenant vs Admin RBAC boundary checks.
- **Assertions**: Tenant service accounts cannot access or mutate unauthorized Gateways.

### `multi-vip`
- **Objective**: Validates binding and processing traffic across multiple distinct VIPs on the same secondary ENI.
- **Traffic Path**: Simultaneous curls to VIP 1 (App A) and VIP 2 (App B).
- **Assertions**: Complete isolation and correct service responses for each VIP.

---

## 4. AI Gateway Scenarios

### `ai-token-counting`
- **Objective**: Validates AI Gateway LLM token measurement, quota enforcement, and rate-limiting.
- **Traffic Path**: Client HTTP POST with chat completions payload $\to$ TMM AI Gateway $\to$ Model endpoint.
- **Assertions**: Control plane always — backend Deployment Available, Gateway Programmed, HTTPRoute Accepted, and the `k8s.f5.com/ai-token-counting` annotation persists. With a live vLLM-compatible backend the data-path step additionally asserts token metering and HTTP 503 on overload.
- **Rating**: Amber. The static rating cannot assume an LLM backend is present; a live run records the data-path result in the scenario output.

### `ai-semantic-cache`
- **Objective**: Validates semantic similarity prompt caching to reduce LLM latency and compute costs.
- **Traffic Path**: Initial prompt POST $\to$ Cache miss (backend computed); Second similar prompt POST $\to$ Cache hit (TMM cached response).
- **Assertions**: Control plane always — Gateway Programmed, HTTPRoute Accepted, and the `k8s.f5.com/ai` / `k8s.f5.com/sse-enabled` annotations persist. When a ModelCache backend address is supplied, both responses return HTTP 200 with SSE framing and the second is faster by at least the configured speed-up (default 100 ms).
- **Rating**: Amber. The EKS cluster ships no ModelCache backend, so the data-path step only runs when one is pointed at explicitly.

### `ai-inference-e2e`
- **Objective**: Validates end-to-end LLM inference through BNK: a vLLM Deployment serving `Llama-3-8B-Instruct` on the GPU node group (requires a `gpu: true` node group and an `hf-token` Secret for the gated model).
- **Traffic Path**: Jumphost `POST /v1/chat/completions` with `stream=true` $\to$ TMM VIP (`Gateway` + `HTTPRoute`) $\to$ vLLM pods on GPU nodes.
- **Assertions**: vLLM Deployment Available, Gateway Programmed, HTTPRoute Accepted, then HTTP 200 with SSE framing (`data:` chunks and a `[DONE]` terminator) via the VIP.

---

## 5. Security & Diagnostics Scenarios

### `egress-snat`
- **Objective**: Validates outbound Source NAT (SNAT) and egress firewall inspection.
- **Traffic Path**: In-cluster workload pod $\to$ VXLAN pseudo-CNI overlay $\to$ TMM (`ext-vlan` on single-interface patterns, `int-vlan` on dual-interface — chosen from the cluster.yaml pattern, override with `tmm-int-vlan`) $\to$ AUTOMAP SNAT $\to$ External destination. The data path is validated only on `external-only` (see `examples/egress-demo`); on dual-interface the VXLAN shape is known to disturb ingress on AWS, so run it last or skip it there.
- **Assertions**: Control plane only — the egress client pod becomes Ready and the `F5SPKEgress` CR (VXLAN pseudo-CNI overlay, AUTOMAP SNAT) is accepted. The source-IP proof at an external destination is recorded as informational, not gating.
- **Rating**: Amber. Promoting to Green requires a live cycle that asserts the external destination sees the TMM self-IP as source.

### `core-file-collection`
- **Objective**: Validates BNK's core-dump collection infrastructure: enabling `spec.coreCollection.enabled` on the `CNEInstance` makes FLO reconcile a `CoreMond` CR and DaemonSet and mount host crash directories into the TMM pods.
- **Verification Method**: Kubernetes API inspection — the `CoreMond` CR exists, its DaemonSet is rolled out and reports Ready, and the TMM pods carry the core-dump volume mounts.
- **Assertions**: CoreMond CR present, DaemonSet ready, TMM pod volumes mounted. No crash is induced; the scenario proves the collection path is wired, not that a core file exists.

---

## 6. Where each scenario runs

Every deployable example under `examples/` enables the jumphost, so all 15
scenarios are runnable on all six. What differs is whether the data-path step
is real, and what else has to be true.

| Scenario | Needs jumphost | Needs GPU node group | Other prerequisites | Runs on |
| --- | :---: | :---: | --- | --- |
| `http-routing-e2e` | Yes | – | Owns VIP `.100`. On `agentcore-demo` the demo Gateway already holds `.100`, so pass `--vip 10.0.10.150` (or run before applying `gateway-deployment.yaml`) | all six |
| `http-traffic-split` | Yes | – | – | all six |
| `external-resource-pool` | Yes | – | Uses the jumphost as the "external" backend | all six |
| `proxy-protocol-l4` | Yes | – | – | all six |
| `tcp-l4-loadbalance` | Yes | – | – | all six |
| `udp-l4-loadbalance` | – | – | Control plane only (Gateway Programmed, L4Route Accepted); no traffic probe | all six |
| `grpc-loadbalance` | – | – | Control plane only (Gateways Programmed, GRPCRoute + L4Route Accepted); no traffic probe | all six |
| `multi-vip` | Yes | – | Pool `.115`–`.117` | all six |
| `cluster-wide-watch` | – | – | Control plane only; no traffic probe | all six |
| `cwc-admin-access` | – | – | Control plane only | all six |
| `ai-token-counting` | – | – | Amber: control plane only unless a vLLM-compatible backend is pointed at | all six; data path on `ai-rig`, `demo-ai` |
| `ai-semantic-cache` | (probe) | – | Amber: control plane only unless a ModelCache backend is supplied | all six |
| `ai-inference-e2e` | Yes | **Yes** | `HF_TOKEN` for the gated model; `--synthetic` runs a GPU-free simulator anywhere | `ai-rig`, `demo-ai` (real); others with `--synthetic` |
| `egress-snat` | – | – | Amber: control plane only. Tunnel VLAN follows the pattern (`ext-vlan` / `int-vlan`). Data path proven on `external-only` only | all six; natural home `egress-demo` |
| `core-file-collection` | – | – | Patches the CNEInstance `f5-cne-system/<cluster>-bnk`; FLO rolls TMM to add the crash mounts, so run it **last** | all six |

Demo use-cases (`awsbnkctl demo run …`) are separate from scenarios: they need
`demo.enabled: true` (or `up --demo`) and the jumphost. `demo-ai` ships with
demo mode on; `full-cluster` carries it as a commented block. `bigip-cis`
additionally needs the `bigipVE:` block, which only `full-cluster` carries, and
a dual-interface pattern.

## 7. VIP plan

Every scenario and demo owns one fixed last octet in the external data-path
subnet (`network.dataPath.external.cidr`, `10.0.10.0/24` in every example), so
`scenarios run --all` never has two F5BnkGateway pools claiming the same
address. `http-routing-e2e` alone uses the cluster default VIP (`<subnet>.100`,
`intent.DefaultVIP`); the others replace the last octet. `--vip` moves the base
address but keeps each scenario's octet.

| Last octet | Owner | Kind |
| --- | --- | --- |
| `.100` | `http-routing-e2e` (cluster default) — also the `agentcore-demo` Gateway | scenario / example |
| `.101` | `http-traffic-split` | scenario |
| `.102` | `external-resource-pool` | scenario |
| `.103` | `proxy-protocol-l4` | scenario |
| `.104` | `ai-token-counting` | scenario |
| `.105` | `cluster-wide-watch` | scenario |
| `.106` | `tcp-l4-loadbalance` | scenario |
| `.107` | `udp-l4-loadbalance` | scenario |
| `.108` | `grpc-loadbalance` | scenario |
| `.109` | `ai-semantic-cache` | scenario |
| `.110` | `diameter` | demo |
| `.111` | `http2` | demo |
| `.112` | `ai-inference-e2e` | scenario |
| `.113` | `ingress-migration` | demo |
| `.115`–`.117` | `multi-vip` (VIP A `.115`, VIP B `.116`, pool end `.117`) | scenario |
| `.120` | `bigip-cis` (BIG-IP VE virtual server, `bigipVE.vip`) | demo |
| `.130` | `demo-ai` proxy shootout | example |
| `.200`–`.202` | `local-zone` reference manifests | example |
| `.240` | TMM external SelfIP (`<subnet>.240`, Phase 17) | infrastructure |

`cwc-admin-access`, `core-file-collection` and `egress-snat` allocate no VIP.
