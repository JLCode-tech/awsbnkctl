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
- **Traffic Path**: Jumphost `grpcurl` probe $\to$ TMM VIP (HTTP/2 / gRPC) $\to$ `grpc-health-probe` endpoints.
- **Assertions**: Successful gRPC status code `OK` and round-robin connection balancing.

---

## 2. L4 Protocol & Transport Scenarios

### `tcp-l4-loadbalance`
- **Objective**: Validates raw L4 TCP stream proxying via `L4Route`.
- **Traffic Path**: Jumphost TCP socket client $\to$ TMM L4 VIP:8080 $\to$ TCP echo servers.
- **Assertions**: TCP connection establishment, payload mirroring, 70/30 weight distribution.

### `udp-l4-loadbalance`
- **Objective**: Validates stateless UDP datagram routing and load distribution.
- **Traffic Path**: Jumphost UDP client $\to$ TMM L4 VIP:5353 $\to$ UDP echo servers.
- **Assertions**: Datagram integrity, zero drop rate under standard load.

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
- **Traffic Path**: Multi-tenant client $\to$ TMM VIP $\to$ Tenant namespace backends.
- **Assertions**: Routes configured across disparate namespaces successfully attach to root Gateway.

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
- **Assertions**: Header telemetry reporting prompt tokens, completion tokens, and rate-limit counters.

### `ai-semantic-cache`
- **Objective**: Validates semantic similarity prompt caching to reduce LLM latency and compute costs.
- **Traffic Path**: Initial prompt POST $\to$ Cache miss (backend computed); Second similar prompt POST $\to$ Cache hit (TMM cached response).
- **Assertions**: Response latency reduction > 90% and cache-hit response header presence.

### `ai-inference-e2e`
- **Objective**: Validates high-throughput inference routing in front of AWS SageMaker and EC2 GPU worker nodes.
- **Traffic Path**: Inference client $\to$ TMM VIP $\to$ SageMaker Endpoint / Triton server.
- **Assertions**: Successful inference tensor response and streaming token chunks.

---

## 5. Security & Diagnostics Scenarios

### `egress-snat`
- **Objective**: Validates outbound Source NAT (SNAT) and egress firewall inspection.
- **Traffic Path**: In-cluster workload pod $\to$ TMM internal interface $\to$ SNAT translation $\to$ External destination.
- **Assertions**: External destination observes TMM's configured SNAT IP as source.

### `corefiles`
- **Objective**: Probes TMM container filesystems and diagnostic mounts for abnormal termination core dumps.
- **Verification Method**: Direct Kubernetes pod inspection across all worker nodes.
- **Assertions**: Zero core dumps present in `/var/core` and TMM uptime continuity.
