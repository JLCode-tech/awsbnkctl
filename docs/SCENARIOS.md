# Scenarios & Demos Catalogue for awsbnkctl

This document provides a comprehensive, diagram-rich breakdown of all **15 automated validation scenarios** and **5 curated demonstration walkthroughs** (including the **AgentCore AI Gateway demo**) built into `awsbnkctl`.

Scenarios validate live data-plane traffic, protocol handshakes, and security policies against the provisioned EKS cluster and F5 BIG-IP Next for Kubernetes (BNK) TMM interfaces. They run from the in-VPC EC2 jumphost via an ephemeral AWS EC2 Instance Connect (EICE) tunnel, providing end-to-end assurance directly inside the isolated VPC data plane.

---

## Running Scenarios & Demos

```bash
# List all registered scenarios and their rating (Green / Amber)
awsbnkctl scenarios list

# Run a specific scenario against your cluster
awsbnkctl scenarios run <scenario-name> -f my-cluster.yaml

# Run all 15 scenarios sequentially
awsbnkctl scenarios run --all -f my-cluster.yaml

# Clean up scenario-specific test fixtures and namespaces
awsbnkctl scenarios clean <scenario-name> -f my-cluster.yaml

# List and run curated live demos
awsbnkctl demo list
awsbnkctl demo run <demo-name> -f my-cluster.yaml
```

---

## 1. Ingress & L7 Protocol Scenarios

### `http-routing-e2e` (VIP `.100`)
- **Objective**: Validates standard Kubernetes Gateway API `Gateway` + `HTTPRoute` routing.
- **Traffic Path**: Client (EC2 jumphost via EICE tunnel) $\to$ TMM VIP (`<subnet>.100`) $\to$ `http-echo` backend pods.
- **Provisions**: `Gateway` referencing `GatewaySettings`, `HTTPRoute` matching path `/`, Deployment `http-echo`.
- **Assertions**: 5/5 jumphost curls return HTTP 200 through the VIP.
- **Rating**: **Green**.

```mermaid
flowchart LR
    Client([Jumphost Test Runner<br/>10.0.10.200])

    subgraph IngressNet["F5 BNK Data Plane (BNK_EXT: 10.0.10.0/24)"]
        direction TB
        VIP["Gateway VIP: 10.0.10.100:80<br/>(default-gateway)"]
        TMM["TMM Microkernel (DPDK)<br/>(HTTPRoute: path /)"]
        VIP --> TMM
    end

    subgraph PodNet["EKS Pod Network (10.0.64.0/19)"]
        Echo[("http-echo Pods<br/>awsbnkctl-scn-http")]
    end

    Client -->|HTTP GET /| VIP
    TMM -->|Direct Pod Forwarding| Echo
    Echo -->|HTTP 200 OK| Client
```

---

### `http-traffic-split` (VIP `.101`)
- **Objective**: Validates weighted canary traffic distribution across multiple backend deployments.
- **Traffic Path**: Jumphost $\to$ TMM VIP (`<subnet>.101`) $\to$ `backend-a` (70%) vs `backend-b` (30%).
- **Provisions**: `Gateway`, `HTTPRoute` with weighted `backendRefs` (70/30), Deployments `backend-a` and `backend-b`.
- **Assertions**: Both `backend-a` and `backend-b` appear across 10 jumphost curls.
- **Rating**: **Green**.

```mermaid
flowchart LR
    Client([Jumphost Test Client])

    subgraph BNKGateway["F5 BNK Gateway (VIP: 10.0.10.101:80)"]
        direction TB
        Route["HTTPRoute: split-route"]
        WeightEngine["Weighted Traffic Distribution<br/>(Round Robin Ratio)"]
        Route --> WeightEngine
    end

    subgraph Workloads["EKS Canary Deployments"]
        BackendA[("backend-a (v1)<br/>Weight: 70 (70%)")]
        BackendB[("backend-b (v2)<br/>Weight: 30 (30%)")]
    end

    Client -->|10 Consecutive Requests| Route
    WeightEngine -->|70% Requests| BackendA
    WeightEngine -->|30% Requests| BackendB
```

---

### `grpc-loadbalance` (VIP `.108`)
- **Objective**: Validates gRPC stream handling and balancing across microservices.
- **Traffic Path**: Client $\to$ TMM VIP (`<subnet>.108`, `L4Route` TCP data plane; `GRPCRoute` on the control plane) $\to$ `kong/grpcbin` backend pods.
- **Assertions**: Control plane validation — `grpcbin` Deployment Available, both Gateways Programmed, `GRPCRoute` and `L4Route` Accepted.
- **Rating**: **Amber** (control plane only; live data-path probe requires `grpcurl` on the jumphost driving VIP:50052 and asserting status `OK`).

```mermaid
flowchart LR
    Client([Jumphost gRPC Client<br/>grpcurl / probe])

    subgraph BNKGateway["F5 BNK Gateway (VIP: 10.0.10.108:50052)"]
        direction TB
        L4["L4Route TCP Listener (:50052)"]
        H2["HTTP/2 Protocol Engine<br/>gRPC Stream Dispatcher"]
        L4 --> H2
    end

    subgraph Workload["gRPC Service"]
        GRPCBin[("kong/grpcbin Pods<br/>grpc.testing.TestService")]
    end

    Client -->|gRPC Stream over HTTP/2| L4
    H2 -->|Multiplexed RPC Calls| GRPCBin
    GRPCBin -->|gRPC Status: 0 OK| Client
```

---

## 2. L4 Protocol & Transport Scenarios

### `tcp-l4-loadbalance` (VIP `.106`)
- **Objective**: Validates raw L4 TCP stream proxying via `L4Route`.
- **Traffic Path**: Jumphost curl $\to$ `L4Route` on VIP:8080 (`<subnet>.106`) $\to$ two nginx marker backends.
- **Assertions**: Deployments Available, Gateway Programmed, `L4Route` Accepted, both weighted backends reached.
- **Rating**: **Green**.

```mermaid
flowchart LR
    Client([TCP Client / Jumphost])

    subgraph L4Plane["F5 BNK L4 Gateway (VIP: 10.0.10.106:8080)"]
        direction TB
        L4R["L4Route (TCP Listener)"]
        LB["TCP Load Balancer"]
        L4R --> LB
    end

    subgraph Backends["Target Marker Services"]
        M1[("nginx-marker-1<br/>Marker Pod 1")]
        M2[("nginx-marker-2<br/>Marker Pod 2")]
    end

    Client -->|Raw TCP Stream :8080| L4R
    LB -->|Round Robin Flow 1| M1
    LB -->|Round Robin Flow 2| M2
```

---

### `udp-l4-loadbalance` (VIP `.107`)
- **Objective**: Validates stateless UDP datagram routing and load distribution.
- **Traffic Path**: UDP client $\to$ TMM L4 VIP:5353 (`<subnet>.107`) $\to$ UDP echo servers.
- **Assertions**: Control plane — `udp-echo` Deployment Available, Gateway Programmed, `L4Route` Accepted.
- **Rating**: **Amber** (control plane only; green requires `socat`/`nc` on the jumphost driving VIP:5353).

```mermaid
flowchart LR
    Client([UDP Client<br/>socat / nc])

    subgraph L4Plane["F5 BNK L4 Gateway (VIP: 10.0.10.107:5353)"]
        direction TB
        L4R["L4Route (UDP Listener)"]
        Hash["Stateless 5-Tuple Hash Engine"]
        L4R --> Hash
    end

    subgraph Backends["UDP Echo Service"]
        U1[("udp-echo Pod 1")]
        U2[("udp-echo Pod 2")]
    end

    Client -->|UDP Datagrams :5353| L4R
    Hash -->|Hash Bucket 1| U1
    Hash -->|Hash Bucket 2| U2
```

---

### `proxy-protocol-l4` (VIP `.103`)
- **Objective**: Validates PROXY protocol v1 header injection via an `F5BigCneIrule` + `NetPolicy` on the TCP listener.
- **Traffic Path**: Client sending traffic $\to$ TMM VIP (`<subnet>.103`) $\to$ Backend pod.
- **Assertions**: The backend echoes `$proxy_protocol_addr`; it must match the jumphost `BNK_EXT` ENI IP (`<subnet>.200`).
- **Rating**: **Green**.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Jumphost (10.0.10.200)
    participant TMM as F5 TMM VIP (10.0.10.103:80)
    participant Backend as Nginx Echo Pod

    Client->>TMM: TCP Handshake (SYN -> SYN/ACK -> ACK)
    Note over TMM: F5BigCneIrule + NetPolicy<br/>Injects PROXY v1 Header
    TMM->>Backend: PROXY TCP4 10.0.10.200 10.0.10.103 45123 80\r\nGET / HTTP/1.1
    Backend-->>TMM: HTTP 200 OK (body: $proxy_protocol_addr=10.0.10.200)
    TMM-->>Client: HTTP 200 OK (body verified)
```

---

## 3. Hybrid & Multi-Tenancy Scenarios

### `external-resource-pool` (VIP `.102`)
- **Objective**: Validates BNK routing traffic to external endpoints located outside the Kubernetes cluster (e.g., bare-metal VMs or AWS RDS/ALB).
- **Traffic Path**: Jumphost $\to$ TMM VIP (`<subnet>.102`) $\to$ External IP targets outside EKS pod CIDR.
- **Assertions**: Successful response proxying from non-cluster IP pools.
- **Rating**: **Green**.

```mermaid
flowchart LR
    Client([Jumphost Test Client])

    subgraph BNKGateway["F5 BNK Gateway (VIP: 10.0.10.102:80)"]
        direction TB
        VIP["Gateway VIP: 10.0.10.102:80"]
        TMM["TMM Microkernel<br/>external-pool-gw"]
        VIP --> TMM
    end

    subgraph ExtTargets["Targets Outside EKS Pod CIDR"]
        RDS[("AWS RDS Database<br/>Private VPC Subnet")]
        BareMetal[("Bare-Metal Server / VM<br/>Enterprise Network")]
    end

    Client -->|HTTP GET /| VIP
    TMM -->|Direct L3 Forwarding| RDS
    TMM -->|Direct L3 Forwarding| BareMetal
```

---

### `cluster-wide-watch` (CWC) (VIP `.105`)
- **Objective**: Validates `ClusterWideWatch` CR for multi-tenant cross-namespace routing without full cluster admin permissions.
- **Traffic Path**: Tenant client $\to$ TMM VIP (`<subnet>.105`) $\to$ backend in a namespace created after BNK was installed.
- **Assertions**: The new namespace's Deployment becomes Available, its Gateway is Programmed and HTTPRoute Accepted. Jumphost curls VIP with `Host: cwatch.awsbnkctl.local`; every probe must return HTTP 200.
- **Rating**: **Green**.

```mermaid
flowchart LR
    subgraph ControlPlane["F5 CNE Control Plane"]
        direction TB
        CNE["Single f5-cne-controller<br/>Namespace: f5-cne-system"]
        CWC["ClusterWideWatch CR<br/>Watches All Namespaces"]
        CNE --- CWC
    end

    subgraph Tenant["Dynamic Tenant Namespace"]
        direction TB
        GW["Tenant Gateway (.105)"]
        Route["Tenant HTTPRoute<br/>Host: cwatch.awsbnkctl.local"]
        Pod[("Tenant Backend Pod")]
        GW --> Route --> Pod
    end

    Client([Tenant Client / Jumphost]) -->|Host: cwatch.awsbnkctl.local| GW
    CWC -.->|Reconciles Tenant Gateway| GW
```

---

### `cwc-admin-access` (In-Cluster)
- **Objective**: Validates that CWC admin and licensing endpoints require both a client certificate and a Bearer token.
- **Traffic Path**: In-cluster probe Deployment $\to$ CWC HTTPS endpoints.
- **Assertions**: Authenticated request accepted; missing token rejected; bogus token rejected.
- **Rating**: **Green**.

```mermaid
sequenceDiagram
    autonumber
    participant Probe as In-Cluster Probe Pod
    participant CWC as CWC Admin Endpoint (:8443)

    Probe->>CWC: HTTPS GET (No Token)
    CWC-->>Probe: 401 Unauthorized (Token missing)
    Probe->>CWC: HTTPS GET (Bogus Bearer Token)
    CWC-->>Probe: 403 Forbidden (Token rejected)
    Probe->>CWC: HTTPS GET (Valid Client Cert + Bearer Token)
    CWC-->>Probe: 200 OK (Admin Access Granted)
```

---

### `multi-vip` (VIPs `.115`, `.116`, `.117`)
- **Objective**: Validates binding and processing traffic across multiple distinct VIPs on the same secondary ENI.
- **Traffic Path**: Simultaneous curls to VIP 1 (`<subnet>.115`, App A) and VIP 2 (`<subnet>.116`, App B).
- **Assertions**: Complete isolation and correct service responses for each VIP.
- **Rating**: **Green**.

```mermaid
flowchart LR
    ClientA([Client A])
    ClientB([Client B])

    subgraph ENI["Single Secondary ENI (BNK_EXT: 10.0.10.0/24)"]
        VIP_A["VIP 1: 10.0.10.115:80<br/>Gateway A"]
        VIP_B["VIP 2: 10.0.10.116:80<br/>Gateway B"]
    end

    subgraph TMMCore["F5 TMM Microkernel"]
        VS_A["Virtual Server A"]
        VS_B["Virtual Server B"]
        VIP_A --> VS_A
        VIP_B --> VS_B
    end

    subgraph Workloads["Isolated Workloads"]
        PodA[("App A Pods<br/>Namespace A")]
        PodB[("App B Pods<br/>Namespace B")]
        VS_A --> PodA
        VS_B --> PodB
    end

    ClientA -->|Traffic to App A| VIP_A
    ClientB -->|Traffic to App B| VIP_B
```

---

## 4. AI Gateway Scenarios

### `ai-token-counting` (VIP `.104`)
- **Objective**: Validates AI Gateway LLM token measurement, quota enforcement, and rate-limiting.
- **Traffic Path**: Client HTTP POST with chat completions payload $\to$ TMM AI Gateway (`<subnet>.104`) $\to$ Model endpoint.
- **Assertions**: Control plane always — backend Deployment Available, Gateway Programmed, HTTPRoute Accepted, annotation `k8s.f5.com/ai-token-counting` persists. Live vLLM data path asserts token metering and HTTP 503 on overload.
- **Rating**: **Amber** (control plane only unless live vLLM backend is pointed at; full data path on `demo-ai`).

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client / Jumphost
    participant TMM as TMM Gateway VIP: .104<br/>(ai-token-counting)
    participant Limiter as Token Bucket Rate Limiter
    participant LLM as vLLM / SageMaker Backend

    Client->>TMM: POST /v1/chat/completions (Tokens within quota)
    TMM->>Limiter: Decrement Token Quota
    Limiter-->>TMM: Quota Allowed
    TMM->>LLM: Forward Request
    LLM-->>Client: 200 OK + Chat Completion
    
    Client->>TMM: POST /v1/chat/completions (Token Quota Exceeded)
    TMM->>Limiter: Decrement Token Quota
    Limiter-->>TMM: Over Quota Limit
    TMM-->>Client: 503 Service Unavailable (Quota Exceeded)
```

---

### `ai-semantic-cache` (VIP `.109`)
- **Objective**: Validates semantic similarity prompt caching to reduce LLM latency and compute costs.
- **Traffic Path**: Initial prompt POST $\to$ Cache miss (backend computed); Second similar prompt POST $\to$ Cache hit (TMM cached response).
- **Assertions**: Control plane always — Gateway Programmed, HTTPRoute Accepted, annotations `k8s.f5.com/ai` and `k8s.f5.com/sse-enabled` persist. When ModelCache backend is present, both return HTTP 200 with SSE framing and the second is faster by at least the configured threshold (default 100 ms).
- **Rating**: **Amber** (control plane only unless a ModelCache backend is pointed at).

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client / Jumphost
    participant TMM as TMM AI Gateway (.109)
    participant Cache as Semantic Vector Cache
    participant LLM as vLLM Inference Engine

    Note over Client,LLM: Step 1: Initial Prompt (Cold Cache)
    Client->>TMM: POST "Explain quantum computing in simple terms"
    TMM->>Cache: Query Semantic Hash
    Cache-->>TMM: Cache Miss
    TMM->>LLM: Forward to GPU
    LLM-->>TMM: Generate Tokens (250ms)
    TMM->>Cache: Store Vector Embedding & Output
    TMM-->>Client: 200 OK (SSE Stream)

    Note over Client,LLM: Step 2: Semantically Equivalent Prompt
    Client->>TMM: POST "What is quantum computing simply explained?"
    TMM->>Cache: Query Semantic Hash
    Cache-->>TMM: Cache Hit (Cosine Similarity > 0.95)
    TMM-->>Client: 200 OK (Cached SSE Stream in < 15ms)
```

---

### `ai-inference-e2e` (VIP `.112`)
- **Objective**: Validates end-to-end LLM inference through BNK: a vLLM Deployment serving `Llama-3-8B-Instruct` on the GPU node group (requires a `gpu: true` node group and an `hf-token` Secret for the gated model).
- **Traffic Path**: Jumphost `POST /v1/chat/completions` with `stream=true` $\to$ TMM VIP (`<subnet>.112`, `Gateway` + `HTTPRoute`) $\to$ vLLM pods on GPU nodes.
- **Assertions**: vLLM Deployment Available, Gateway Programmed, HTTPRoute Accepted, then HTTP 200 with SSE framing (`data:` chunks and `[DONE]` terminator) via the VIP.
- **Synthetic mode** (`ai.synthetic` or `--synthetic`): `llm-d-inference-sim` pods stand in for vLLM. Each pod tokenizes with the real model tokenizer (a `vllm-render` sidecar), serves the profile's Hugging Face id (`llama3` is an alias), and publishes KV-cache events like vLLM: per pod on port 20080, replay on 20081. Endpoint pickers (the F5 EPP, llm-d) connect to each pod as they would to vLLM. For agentic benchmarks with long shared contexts, set `maxNumSeqs` to what the KV cache holds per request (for the 70B profile and 17k-token requests, `maxNumSeqs: 40`) and `timeFactorUnderLoad: 3`, so a busy replica slows down the way a real one does.
- **Rating**: **Green** on `demo-ai` (real GPU) or with `--synthetic`.

```mermaid
flowchart LR
    Client([Jumphost Test Client<br/>aiperf Engine])

    subgraph Gateway["F5 BNK AI Gateway (VIP: 10.0.10.112:80)"]
        direction TB
        VIP["Gateway VIP: 10.0.10.112:80<br/>Host: awsbnkctl-aiinference.local"]
        TMM["TMM Microkernel<br/>HTTPRoute: scn-aiinference-route"]
        VIP --> TMM
    end

    subgraph InferenceNodes["Inference Worker Group"]
        Model[("vLLM / Simulator<br/>meta-llama/Llama-3-8B-Instruct")]
    end

    Client -->|POST /v1/chat/completions (stream=true)| VIP
    TMM -->|Balanced Inference Stream| Model
    Model -->|SSE Stream: data: {...}| Client
```

*(For comprehensive benchmarking, multi-proxy shootouts, and TTFT latency analysis, see [`docs/BENCHMARKS.md`](BENCHMARKS.md).)*

---

## 5. Security & Diagnostics Scenarios

### `egress-snat`
- **Objective**: Validates outbound Source NAT (SNAT) and egress firewall inspection.
- **Traffic Path**: Pod $\to$ Node $\to$ VXLAN over the pod network to TMM (`ext-vlan-infra` on single-interface patterns, `int-vlan-infra` on dual-interface) $\to$ AUTOMAP SNAT to the external self-IP $\to$ `ext-vlan-infra` $\to$ NAT gateway $\to$ external destination. Intra-VPC traffic stays on the node.
- **Provisions**: Client pod, `GatewaySettings` with `egressConfigs` (`default-egress`, Automap), and `EgressGateway` selecting the namespace.
- **Assertions**: Client pod Ready; `GatewaySettings` Accepted and ResolvedRefs; `EgressGateway` Programmed; 3 curls from pod to an out-of-VPC URL must raise TMM's egress virtual-server connections and SNAT connections by 3 (read via `tmctl`).
- **Rating**: **Green**.

```mermaid
flowchart LR
    subgraph Pods["EKS Worker Node Pod Network"]
        ClientPod[("Workload Pod<br/>(Selected Namespace)")]
        Router["Node Routing Policy"]
        ClientPod --> Router
    end

    subgraph BNK["F5 BNK Data Plane (TMM)"]
        Tunnel["VXLAN Tunnel Interface<br/>(ext-vlan-infra / int-vlan-infra)"]
        SNAT["EgressGateway & AUTOMAP<br/>SNAT Engine"]
        SelfIP["External Self-IP<br/>(10.0.10.224 - .254)"]
        Tunnel --> SNAT --> SelfIP
    end

    subgraph Egress["AWS Network Egress"]
        NAT["AWS NAT Gateway"]
        Internet([External SaaS API / Registry])
        NAT --> Internet
    end

    Router -->|Out-of-VPC Egress Traffic| Tunnel
    Router -.->|Intra-VPC Traffic (Bypasses TMM)| LocalVPC[("Local VPC Resources")]
    SelfIP --> NAT
```

---

### `core-file-collection`
- **Objective**: Validates BNK's core-dump collection infrastructure: enabling `spec.coreCollection.enabled` on the `CNEInstance` causes FLO to reconcile `CoreMond` and mount host crash directories into TMM pods.
- **Verification Method**: Kubernetes API inspection polled for up to 5 minutes: `CoreMond` CR exists, `f5-coremond` DaemonSet is Ready, CNEInstance reports `CoreMon*` condition True, and `f5-tmm` DaemonSet carries crash volume mounts.
- **Assertions**: Proves collection path is wired without inducing an artificial crash.
- **Note**: Run this scenario **last** because FLO rolling TMM to add crash mounts briefly restarts the data plane.
- **Rating**: **Green**.

```mermaid
flowchart LR
    Trigger([Operator Patch<br/>coreCollection.enabled=true]) --> CNE["CNEInstance Patch<br/>(f5-cne-system)"]

    subgraph Reconcile["F5 Lifecycle Operator (FLO)"]
        FLO["FLO Controller"]
        CNE --> FLO
    end

    subgraph DaemonSets["Cluster DaemonSets"]
        CoreDS["CoreMond DaemonSet<br/>(app=f5-coremond)"]
        TMM_DS["TMM DaemonSet<br/>(app=f5-tmm)"]
        FLO -->|Applies CoreMond CR| CoreDS
        FLO -->|Mounts Crash Volume| TMM_DS
    end

    subgraph Storage["Host Filesystem"]
        CrashDir[("/var/crash<br/>Crash Dump Storage")]
        TMM_DS --> CrashDir
        CoreDS --> CrashDir
    end
```

---

## 6. Curated Demonstrations & Walkthroughs

In addition to automated tests, `awsbnkctl` includes 5 turnkey demonstrations with live narrations and telemetry:

### `agentcore-demo` — AI Agent Tool Call Governance (VIP `.150`)
- **Objective**: Demonstrates governing generative AI agent tool calls using F5 BNK. An **Amazon Bedrock AgentCore** agent making Model Context Protocol (MCP) tool calls is authenticated, rate-limited, firewall-checked, and logged in real-time.
- **Location**: `examples/agentcore-demo/`
- **Traffic Path**: Bedrock AgentCore Agent $\to$ Private Route 53 (`bnk-ingress.bnk-demo.internal`) $\to$ BNK Gateway VIP (`10.0.10.150:80/443`) $\to$ MCP Finance Tool Pod (`forecast`, `get_account_balance`).

```mermaid
flowchart LR
    subgraph Callers["AI Agents & Callers"]
        Agent([Amazon Bedrock AgentCore<br/>Private VPC Agent])
        ExtCaller([External Caller / Script<br/>Jumphost])
        Stranger([Out-of-VPC Source<br/>Outside 10.0.0.0/16])
    end

    subgraph BNK["F5 BNK Governance Gateway (VIP: 10.0.10.150)"]
        direction TB
        FW["1. L4 Firewall Policy<br/>Drop out-of-VPC traffic"]
        Auth["2. Bearer Authentication<br/>Require valid agent token"]
        ToolGov["3. Tool Access Rule<br/>Block privileged tools for external"]
        RateLimit["4. Rate Limiting iRule<br/>10 req/min rate limit"]
        SessionPin["5. Session Persistence<br/>Pin MCP session to pod"]

        FW --> Auth --> ToolGov --> RateLimit --> SessionPin
    end

    subgraph Tools["Kubernetes Tool Cluster"]
        MCPPod[("MCP Server Pod<br/>forecast, get_account_balance")]
    end

    subgraph Observability["Telemetry Stream"]
        Loki["Loki Server"]
        Forge["BNK Forge UI"]
        Loki --> Forge
    end

    Stranger -->|Any Traffic| FW
    Agent -->|Authorized Call| FW
    ExtCaller -->|External Call| FW
    SessionPin -->|Approved Execution| MCPPod
    BNK -.->|Stream Audit Events| Loki
```

---

### `diameter` Demo (VIP `.110`)
- **Objective**: Demonstrates carrier-grade telecom Diameter protocol (RFC 6733) L4 proxying over TCP.
- **Traffic Path**: Jumphost Python client (`diameter_client.py`) $\to$ TMM L4 Gateway VIP:3868 (`<subnet>.110`) $\to$ `diameter-responder` pod.
- **Validation**: Completes full Capabilities Exchange Request (CER) $\to$ Capabilities Exchange Answer (CEA) handshake and asserts `Result-Code=2001 (DIAMETER_SUCCESS)`.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Jumphost Client (diameter_client.py)
    participant TMM as TMM L4 VIP: 10.0.10.110:3868<br/>(L4Route: diameter-l4route)
    participant Responder as diameter-responder Pod

    Client->>TMM: TCP Handshake (:3868)
    TMM->>Responder: Proxy TCP Handshake
    Client->>TMM: Capabilities-Exchange-Request (CER)
    TMM->>Responder: Forward CER
    Responder-->>TMM: Capabilities-Exchange-Answer (CEA, Result-Code=2001)
    TMM-->>Client: Forward CEA (DIAMETER_SUCCESS)
```

---

### `http2` Demo (VIP `.111`)
- **Objective**: Demonstrates end-to-end HTTP/2 multiplexing (h2c) with prior knowledge.
- **Traffic Path**: Jumphost `curl --http2-prior-knowledge` $\to$ TMM VIP (`<subnet>.111`) $\to$ `h2c-backend` pod (`appProtocol: kubernetes.io/h2c`).
- **Validation**: 5/5 requests return HTTP 200 with wire-level HTTP/2 framing and backend marker confirmation.

```mermaid
flowchart LR
    Client["Jumphost Client<br/>curl --http2-prior-knowledge"]
    VIP["Gateway VIP: 10.0.10.111:80<br/>Gateway: http2-gateway"]
    TMM["TMM Microkernel<br/>HTTP/2 Multiplexer Engine"]
    Backend["h2c-backend Pod<br/>appProtocol: kubernetes.io/h2c"]

    Client -->|h2c Prior Knowledge Stream| VIP
    VIP --> TMM
    TMM -->|Multiplexed h2c Frames| Backend
    Backend -->|HTTP/2.0 200 OK| Client
```

---

### `bigip-cis` Demo (VIP `.120`)
- **Objective**: Demonstrates the traditional ingress pattern (in-cluster CIS controller + external BIG-IP Virtual Edition appliance) in contrast to BNK's modern in-cluster TMM Gateway API.
- **Traffic Path**: Client $\to$ External BIG-IP VE VIP (`10.0.10.120`) $\to$ Shared `whoami` backend pods.
- **Validation**: CIS reconciles `VirtualServer` CRD and routes traffic through the external BIG-IP appliance.

```mermaid
flowchart LR
    Client([Client / Jumphost])

    subgraph ExternalAppliance["External BIG-IP Virtual Edition (VE)"]
        VE["BIG-IP VE Appliance<br/>VIP: 10.0.10.120"]
    end

    subgraph InCluster["EKS Cluster (kube-system & demo-bigip-cis)"]
        direction TB
        CIS["k8s-bigip-ctlr (CIS)<br/>Watching VirtualServer CRD"]
        Backend[("traefik/whoami Pods<br/>app=cis-backend")]
    end

    Client -->|curl 10.0.10.120| VE
    CIS -.->|AS3 API Declarations| VE
    VE -->|Direct Cluster Pod Routing| Backend
```

---

### `ingress-migration` Demo (VIP `.113`)
- **Objective**: Demonstrates zero-downtime migration to BNK Gateway API by running three ingress controllers simultaneously in front of ONE shared `whoami` backend.
- **Traffic Path**: Jumphost curls each ingress path using distinct host headers:
  - BNK Gateway (`web.bnk.migration.local` on VIP `10.0.10.113`)
  - NGINX Ingress (`web.nginx.migration.local` on ClusterIP)
  - HAProxy Ingress (`web.haproxy.migration.local` on ClusterIP)
- **Validation**: All 3 paths return HTTP 200 with the backend `Hostname` marker, proving seamless side-by-side migration.

```mermaid
flowchart LR
    Client([Test Client / Jumphost])

    subgraph IngressControllers["Simultaneous Ingress Front-Ends"]
        direction TB
        BNK_GW["F5 BNK Gateway VIP: 10.0.10.113<br/>Host: web.bnk.migration.local"]
        Nginx_Ing["ingress-nginx Controller<br/>Host: web.nginx.migration.local"]
        HAProxy_Ing["haproxy-ingress Controller<br/>Host: web.haproxy.migration.local"]
    end

    SharedBackend[("Shared Backend Service<br/>traefik/whoami Deployment")]

    Client -->|Path 1: Modern Gateway API| BNK_GW --> SharedBackend
    Client -->|Path 2: Ingress Nginx| Nginx_Ing --> SharedBackend
    Client -->|Path 3: HAProxy Ingress| HAProxy_Ing --> SharedBackend
```

---

## 7. Where Each Scenario & Demo Runs

Every deployable example under `examples/` enables the test jumphost, so all 15 scenarios and demos can be run across configurations:

| Scenario / Demo | Needs Jumphost | Needs GPU Node | Prerequisites / Notes | Runs On |
|---|:---:|:---:|---|---|
| `http-routing-e2e` | Yes | – | Owns default cluster VIP (`.100`) | All examples |
| `http-traffic-split` | Yes | – | Owns VIP `.101` | All examples |
| `external-resource-pool` | Yes | – | Uses jumphost as external backend (VIP `.102`) | All examples |
| `proxy-protocol-l4` | Yes | – | Owns VIP `.103` | All examples |
| `tcp-l4-loadbalance` | Yes | – | Owns VIP `.106` | All examples |
| `udp-l4-loadbalance` | – | – | Amber: control plane only (VIP `.107`) | All examples |
| `grpc-loadbalance` | – | – | Amber: control plane only (VIP `.108`) | All examples |
| `multi-vip` | Yes | – | Pool `.115`–`.117` | All examples |
| `cluster-wide-watch` | Yes | – | Dynamic tenant namespace (VIP `.105`) | All examples |
| `cwc-admin-access` | – | – | In-cluster probe (no VIP) | All examples |
| `ai-token-counting` | – | – | Amber: control plane only without vLLM (VIP `.104`) | All examples; data path on `demo-ai` |
| `ai-semantic-cache` | (probe) | – | Amber: control plane only without ModelCache (VIP `.109`) | All examples |
| `ai-inference-e2e` | Yes | **Yes** | `HF_TOKEN` for gated model; `--synthetic` runs on CPU | `demo-ai` (real); others with `--synthetic` |
| `egress-snat` | (proof) | – | Tunnel VLAN matches pattern; data path proven via `tmctl` | All examples; real on dual-interface |
| `core-file-collection` | – | – | Rolls TMM pods; run **last** (no VIP) | All examples |
| `agentcore-demo` | Yes | – | Bedrock AgentCore runtime + MCP finance tool (VIP `.150`) | `examples/agentcore-demo` |
| `diameter` | Yes | – | Demo mode; Diameter CER/CEA handshake (VIP `.110`) | `demo.enabled: true` |
| `http2` | Yes | – | Demo mode; h2c wire multiplexing (VIP `.111`) | `demo.enabled: true` |
| `bigip-cis` | Yes | – | Needs `bigipVE:` block provisioned (VIP `.120`) | `full-cluster` with BIG-IP VE |
| `ingress-migration` | Yes | – | Demo mode; installs Nginx & HAProxy controllers (VIP `.113`) | `demo.enabled: true` |

---

## 8. VIP Address Allocation Plan

Every scenario and demo owns one deterministic last octet in the external data-path subnet (`network.dataPath.external.cidr`, `10.0.10.0/24` in all examples):

| Last Octet | Allocated Address | Owner | Kind |
|:---:|---|---|---|
| `.100` | `10.0.10.100` | `http-routing-e2e` (cluster default) | Scenario |
| `.101` | `10.0.10.101` | `http-traffic-split` | Scenario |
| `.102` | `10.0.10.102` | `external-resource-pool` | Scenario |
| `.103` | `10.0.10.103` | `proxy-protocol-l4` | Scenario |
| `.104` | `10.0.10.104` | `ai-token-counting` | Scenario |
| `.105` | `10.0.10.105` | `cluster-wide-watch` (CWC) | Scenario |
| `.106` | `10.0.10.106` | `tcp-l4-loadbalance` | Scenario |
| `.107` | `10.0.10.107` | `udp-l4-loadbalance` | Scenario |
| `.108` | `10.0.10.108` | `grpc-loadbalance` | Scenario |
| `.109` | `10.0.10.109` | `ai-semantic-cache` | Scenario |
| `.110` | `10.0.10.110` | `diameter` | Demo |
| `.111` | `10.0.10.111` | `http2` | Demo |
| `.112` | `10.0.10.112` | `ai-inference-e2e` | Scenario |
| `.113` | `10.0.10.113` | `ingress-migration` | Demo |
| `.115`–`.117` | `10.0.10.115`–`.117` | `multi-vip` (VIP A `.115`, VIP B `.116`, pool end `.117`) | Scenario |
| `.120` | `10.0.10.120` | `bigip-cis` (BIG-IP VE virtual server) | Demo |
| `.121`–`.123` | `10.0.10.121`–`.123` | `local-zone` reference manifests (SCTP, Diameter, HTTP/2) | Example |
| `.130` | `10.0.10.130` | `demo-ai` proxy shootout | Example |
| `.150` | `10.0.10.150` | `agentcore-demo` Gateway (`gateway-deployment.yaml`) | Example / Demo |
| `.200` | `10.0.10.200` | Jumphost External ENI (Phase 17b) | Infrastructure |
| `.224`–`.254` | `10.0.10.224`–`.254` | TMM External Self-IP Pool (/27 allocated via F5 IPAM controller) | Infrastructure |

> [!NOTE]
> `cwc-admin-access`, `core-file-collection`, and `egress-snat` do not allocate external VIPs.
