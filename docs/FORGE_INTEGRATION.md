# Forge Integration

`awsbnkctl` can optionally register the clusters it provisions with **forge**, a GUI for operating BNK deployments. 

This integration is a **write-only handoff**: `awsbnkctl` operates AWS infrastructure, reports the cluster connection details to forge, and then forge connects directly to the cluster.

---

## 1. The Peer-Read Model

Both `awsbnkctl` and forge treat AWS as the single source of truth. They operate as peers:

```
        [AWS — Single Source of Truth]
              ▲                  ▲
              │ reads            │ reads (forge's own credentials)
              │                  │
         [awsbnkctl] ── registers pointers ──► [forge] ──► [user GUI]
```

- **Independent Operations:** `awsbnkctl` never asks forge for cluster health. It queries AWS directly.
- **Bootstrap Credentials:** `awsbnkctl` gives forge a short-lived presigned bootstrap kubeconfig. Forge handles refreshing credentials on its own identity after that.
- **No IaC Sync:** `awsbnkctl` simply points forge to the new cluster. It does not sync infrastructure state or "tfstate" equivalents.

---

## 2. Enabling Forge Integration

Forge integration is **opt-in** via the `forge:` block in `cluster.yaml`:

```yaml
forge:
  enabled: true
  url: http://localhost:8000
  mcpUrl: http://localhost:8081/mcp/
  username: admin
  credentialTemplateId: 1
```

If `forge.enabled` is missing or `false`, the integration is skipped.

---

## 3. MCP-Preferred, REST-Fallback

Forge provides both an MCP (Model Context Protocol) endpoint and a REST API.

- `awsbnkctl` will always attempt registration via **MCP first**.
- If a specific action isn't available in MCP (a catalog gap), it falls back gracefully to the **REST API**.
- On failures, it uses a soft-fail strategy so the actual cluster provisioning isn't interrupted.

---

## 4. Soft-Fail & Retry Strategy

Because AWS provisioning is time-consuming, a forge outage shouldn't break the whole pipeline.

- If forge registration fails on `up`, `awsbnkctl` writes `forge_link.json` with `status: pending` and returns success.
- The operator can retry later with `awsbnkctl forge register`.
- A successful link is saved to `.awsbnkctl/<cluster-name>/forge_link.json`.

---

## 5. Configuration Overrides

It is unsafe to store passwords in `cluster.yaml`. You can override forge settings using environment variables:

| Setting | Priority 1 (Env Var) | Priority 2 (YAML) | Default |
|---|---|---|---|
| **URL** | `AWSBNKCTL_FORGE_URL` | `forge.url` | `http://localhost:8000` |
| **Password** | `AWSBNKCTL_FORGE_PASSWORD` | `forge.password` | *built-in dev default* |

> [!WARNING]
> Always use `AWSBNKCTL_FORGE_PASSWORD` in real environments!

---

## 6. What Forge Does NOT Do

- **Manage Infrastructure:** Forge does not spin up or tear down EKS clusters. That is `awsbnkctl`'s job.
- **Act as Source of Truth:** `awsbnkctl doctor` and `awsbnkctl status` rely on AWS, not forge.

---

## 7. Benchmarks & Bidirectional Agent Daemon

`awsbnkctl` integrates with Forge's performance benchmarking subsystem to execute LLM inference benchmark suites (`aiperf`) against proxy endpoints (F5 BNK, Envoy, HAProxy, NGINX) inside AWS VPCs.

### Architecture Overview

When Forge is running locally (e.g. `http://localhost:8000`), the cloud jumphost in AWS cannot directly reach `localhost`. To bridge this securely without exposing public ingress, **`awsbnkctl benchmark daemon` runs locally on the operator's workstation / laptop**.

```
┌──────────────────────────────────────────────────────────┐
│                   OPERATOR WORKSTATION                   │
│                                                          │
│  [Forge Web UI :3000] ──► [Forge Backend :8000]          │
│                                  │                       │
│                              WebSocket                   │
│                                  │                       │
│                       [awsbnkctl daemon]                 │
└──────────────────────────────────┼───────────────────────┘
                                   │
                       AWS EICE Ephemeral SSH Tunnel
                           (via AWS IAM & API)
                                   │
┌──────────────────────────────────▼───────────────────────┐
│                    AWS CLOUD VPC DATA PLANE              │
│                                                          │
│  [EC2 Jumphost]                                          │
│   • Source IP: 10.10.10.140 (BNK External ENI)           │
│   • Runs aiperf profile against VIP                      │
│   • Injects Host: awsbnkctl-aiinference.local            │
│                 │                                        │
│                 ▼                                        │
│  [F5 BNK Gateway VIP: 10.10.10.108:80]                   │
│   • HTTPRoute scn-aiinference-route                      │
│                 │                                        │
│                 ▼                                        │
│  [vLLM Inference Pods (EKS / SageMaker)]                 │
└──────────────────────────────────────────────────────────┘
```

1. **Local WebSocket Channel**: `awsbnkctl benchmark daemon` connects to `ws://localhost:8000/ws/benchmarks/agents/{id}` and sends 15-second heartbeats.
2. **AWS EICE Tunneling**: When a benchmark run is dispatched, `awsbnkctl` opens an ephemeral SSH session over the AWS EC2 Instance Connect Endpoint (EICE).
3. **In-VPC Execution**: The jumphost executes `aiperf`, generating real traffic from its secondary ENI directly to the BNK Gateway VIP.
4. **Automatic HTTPRoute Resolution**: The daemon automatically injects required Gateway API Host headers (e.g., `awsbnkctl-aiinference.local`) to ensure traffic matches Kubernetes HTTPRoutes.
5. **Real-Time Telemetry**: Results are captured and returned to Forge over the WebSocket, updating the Forge Web UI with live latency and throughput charts.

---

### Step-by-Step Operator Guide

#### Step 1: Pre-Stage Jumphost & Register with Forge
```bash
AWS_PROFILE=<your-profile> awsbnkctl benchmark setup \
  -f clusters/cluster.yaml \
  --forge-user mcp \
  --forge-pass <mcp-token> \
  --vip 10.10.10.108
```
*This verifies `aiperf >= 0.10.0` on the jumphost and registers the jumphost agent and LLM target in Forge.*

#### Step 2: Start the Agent Daemon on your Workstation
```bash
AWS_PROFILE=<your-profile> awsbnkctl benchmark daemon \
  -f clusters/cluster.yaml \
  --forge-user mcp \
  --forge-pass <mcp-token>
```
*Leave this running in a terminal tab. You will see heartbeat acknowledgments every 15 seconds.*

#### Step 3: Trigger Benchmarks from Forge Web UI
1. Open the Forge Web UI (`http://localhost:3000`).
2. Navigate to **Benchmarks** → **Run Benchmark** (or **Scenarios**).
3. Select:
   - **Scenario**: e.g., `Mixed Workload`, `Baseline`, `High Concurrency`, `Sustained Load`.
   - **Agent**: Select the registered jumphost agent (e.g. `awsbnkctl-jumphost-i-...`).
   - **Target**: Select the discovered vLLM target.
   - **Proxy**: Select `f5-bnk` (or proxy shootout).
4. Click **Run Benchmark**.

Forge will stream each scenario phase to your local daemon, which drives the cloud jumphost and returns metrics to Forge in real time.

#### Step 4: (Alternative) Drive Runs Directly via CLI
You can also trigger sweeps and presets directly without the UI:
```bash
# Single benchmark run
AWS_PROFILE=<your-profile> awsbnkctl benchmark run \
  -f clusters/cluster.yaml \
  --vip 10.10.10.108 \
  --num-requests 100 \
  --concurrency 10 \
  --model meta-llama/Llama-3-8B-Instruct \
  --host-header awsbnkctl-aiinference.local

# Run preset scenario sweep (latency, throughput, streaming)
AWS_PROFILE=<your-profile> awsbnkctl benchmark run \
  -f clusters/cluster.yaml \
  --scenarios latency,throughput
```

