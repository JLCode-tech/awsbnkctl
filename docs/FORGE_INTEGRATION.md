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
  environment: dev                  # forge project environment (default "dev")
  projectName: awsbnkctl-my-cluster # default "awsbnkctl-<metadata.name>"
```

If `forge.enabled` is missing or `false`, the integration is skipped.

### Project naming (`forge.projectName`)

Registration creates (or reuses) a forge **project** and puts the cluster record
inside it. The project name resolves as `AWSBNKCTL_FORGE_PROJECT` env >
`forge.projectName` > `awsbnkctl-<metadata.name>` (`ForgeSpec.ResolveProjectName`).

- **Reuse on conflict.** If the project already exists, forge answers HTTP 409
  (or 400 with "already exists"); `awsbnkctl` treats that as "reuse" — it looks
  the project up by name and registers the cluster into it. The same applies to
  a cluster record that already exists in the project (its kubeconfig is
  refreshed). Several clusters can therefore share one project by setting the
  same `projectName`.
- **Purge on `down`.** `Phase09ForgeRegisterDown` unregisters the cluster and
  purges the project **only when `forge.projectName` is unset or equals the default name**
  `awsbnkctl-<metadata.name>`. A non-default `forge.projectName` is assumed to
  be shared, so the project is left in place and only the cluster record is
  removed. `awsbnkctl down --keep-forge-link` skips the unregister entirely and
  preserves `forge_link.json`.

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
| **REST URL** | `AWSBNKCTL_FORGE_URL` | `forge.url` | `http://localhost:8000` |
| **MCP URL** | `AWSBNKCTL_FORGE_MCP_URL` (see note) | `forge.mcpUrl` | `http://localhost:8081/mcp/` |
| **Username** | `AWSBNKCTL_FORGE_USERNAME` (`benchmark` subcommands only) | `forge.username` | `admin` |
| **Password** | `AWSBNKCTL_FORGE_PASSWORD` | `forge.password` | *built-in dev default* (`changeme`, with a warning) |
| **Project** | `AWSBNKCTL_FORGE_PROJECT` | `forge.projectName` | `awsbnkctl-<metadata.name>` |
| **Environment** | `AWSBNKCTL_FORGE_ENVIRONMENT` | `forge.environment` | `dev` |

Notes on the two exceptions to "env beats YAML":

- **MCP URL**: Phase 09 and `awsbnkctl forge *` pass `forge.mcpUrl` straight to
  the MCP client; `AWSBNKCTL_FORGE_MCP_URL` is consulted only when neither the
  YAML key nor the `--forge-mcp-url` flag is set (`forge.NewClient`). So for the MCP
  endpoint the order is flag > YAML > env > default.
- **Username**: Phase 09 resolves `forge.username` > `admin` and does not read
  the environment. `AWSBNKCTL_FORGE_USERNAME` is read only by the `awsbnkctl
  benchmark` family as the fallback for `--forge-user`.

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

### GenAI inference metrics

Every run carries a GenAI metric set, flattened into both Forge payloads
(`POST /api/benchmarks/results` and `POST /api/benchmarks/results/aiperf`)
and mirrored under `aiperf_metrics.genai` and `throughput` in `result_json`:

| Field | Source |
|---|---|
| `ttft_p50_ms`, `ttft_p90_ms`, `ttft_p95_ms`, `ttft_p99_ms` | aiperf `time_to_first_token` |
| `itl_p50_ms`, `itl_p95_ms`, `itl_p99_ms` | aiperf `inter_token_latency` |
| `input_tokens_per_sec`, `output_tokens_per_sec`, `total_tokens_per_sec` | aiperf `input_sequence_length.sum` / duration, `output_token_throughput` |
| `prefix_cache_hit_rate` (0.0–1.0), `prefix_cache_source` | delta of `vllm:prefix_cache_hits_total` / `vllm:prefix_cache_queries_total` (vLLM, LMI) or `inference_extension_prefix_indexer_hit_ratio` (EPP) between the scrapes before and after the run |
| `prefill_worker_utilization`, `decode_worker_utilization` (0.0–1.0) | `vllm:kv_cache_usage_perc` / `inference_pool_average_kv_cache_utilization` of the pods scraped with role `prefill` / `decode` at the end of the run |

The pointer fields are omitted when no metrics source was scraped. Sources:

```bash
# vLLM pods through the Kubernetes API pod proxy (role from the llm-d.ai/role label)
awsbnkctl benchmark run -f clusters/cluster.yaml --scenario prefix-cache \
  --metrics-pod-selector app=vllm --metrics-namespace awsbnkctl-scn-aiinference --metrics-port 8000

# disaggregated pools, scraped from the jumphost
awsbnkctl benchmark run -f clusters/cluster.yaml --scenario prefix-cache \
  --metrics-url prefill=http://10.0.20.11:8000/metrics --metrics-url decode=http://10.0.20.12:8000/metrics

# offline: artifacts and saved scrapes
awsbnkctl benchmark ingest baseline.json prefix-shared.json \
  --metrics-before before.prom --metrics-after after.prom --expect-ttft-drop 20
```

`--prefix-prompt-length`, `--num-prefix-prompts` and `--random-seed` build a
shared-prefix workload on a single run; `--genai-out` writes the metric set to
a file `benchmark ingest` reads back.


---

## 8. BNK 2.4 Scan, MCP Targets and Governance Telemetry

Forge's `scan_cluster` and `bnk_health` tools take a cluster ID and index the
cluster server-side. `awsbnkctl` reads the CRD groups Forge saw to tell a 2.3
cluster (`gateway.k8s.f5net.com`) from a 2.4 one (`gateway.k8s.f5.com`), and
reads the cluster directly for what Forge does not index yet.

### `awsbnkctl forge scan`

```bash
awsbnkctl forge scan -f clusters/<name>/cluster.yaml             # readiness + MCP endpoints
awsbnkctl forge scan -f ... --remote                              # plus Forge's own scan_cluster / bnk_health
awsbnkctl forge scan -f ... --probe --bearer-env MCP_TOKEN        # tools/list on every endpoint
awsbnkctl forge scan -f ... --register-targets                    # write endpoints to the Target Catalog
awsbnkctl forge scan -f ... -o json
```

| Check | Passes when |
|---|---|
| API generation | `gateway.k8s.f5.com` served (`2.4`); `mixed` while 2.3 CRDs remain; `2.3` fails with a pointer to `bnk upgrade` |
| `Infra` | `Programmed=True` |
| `Gateway` (BNK class) | `Accepted=True` and `Programmed=True` |
| `f5-cne-controller` | every desired replica available |
| `F5BigPersistenceProfile` | `Programmed=True` |

`GatewaySettings`, `EgressGateway`, `SecPolicy`, `NetPolicy` and `HTTPRoute` are
listed with their conditions. Legacy `F5BnkGateway`, `BNKSecPolicy`,
`BNKNetPolicy`, `F5SPKVlan` and `F5SPKEgress` objects are counted, never
required. `--remote` warns when Forge indexed a 2.4 cluster without the
`gateway.k8s.f5.com` group: that Forge predates BNK 2.4 and its Fleet view
misses the 2.4 kinds.

An `HTTPRoute` is an MCP endpoint when it carries the annotation
`bnk.f5.com/protocol: mcp` or a path match containing `/mcp`. Optional
annotations: `bnk.f5.com/auth` (recorded as the endpoint's auth method) and
`bnk.f5.com/mcp-tools` (comma-separated tool names when the endpoint cannot be
probed). Each endpoint reports its Gateway VIP URLs, hostnames, backends,
iRules and `SecPolicy` objects attached through `NetPolicy`/`SecPolicy`, and
the persistence profile pinning its sessions.

`--register-targets` creates one `BenchmarkTarget` per endpoint (name
`mcp-<namespace>-<route>`, `llm_base_url` the HTTPS VIP URL, `llm_model`
`mcp:<route>`) with the MCP metadata in `tags`: `protocol=mcp`, `route`,
`gateway`, `hostnames`, `paths`, `urls`, `backends`, `auth`, `irules`,
`sec_policies`, `persistence_profile`, `persistence_type`, `tools` and
`tool_schemas` (JSON). Registration is idempotent.

### One readiness verdict everywhere

`forge scan`, `status`, `doctor --backend k8s`, `up` (phase 25) and `bnk upgrade`
all read the same `bnkscan` index. `status` prints the API generation, `Infra`
and `Gateway` tallies, the controller rollout and the verdict; `doctor` turns
them into rows and warns with `awsbnkctl bnk migrate-2.4` when 2.3 CRDs or CRs
remain on a 2.4 cluster; phase 25 waits for the verdict after the license
activates and records `BNK_API_GENERATION` in `state.env`; `bnk upgrade`
reports it after the rollout (the Infra CR is missing until `migrate-2.4`).

### `awsbnkctl targets scan`

```bash
awsbnkctl targets scan -f clusters/<name>/cluster.yaml          # list endpoints, register them in Forge
awsbnkctl targets scan -f ... --register=false                  # list only
awsbnkctl targets scan -f ... --probe --bearer-env MCP_TOKEN    # with tools/list
awsbnkctl benchmark setup -f ... --auto-discover                # same registration during setup
```

One row per endpoint (`mcp-<namespace>-<route>`, Gateway, URL, auth,
persistence type, tools). Registration uses the cluster's `forge_link.json`
and is idempotent; without a link the endpoints are listed and a hint points
at `forge register`.

### `awsbnkctl logs tmm --governance`

```bash
awsbnkctl logs tmm --governance -f          # every BNKGOV decision, live
awsbnkctl logs tmm --mcp --since 10m        # only MCP JSON-RPC calls
```

Reads the TMM log stream: on BNK 2.4 the TMM pod's `f5-fluentbit` sidecar
only forwards to `f5-toda-fluentd`, so `awsbnkctl up` (phase 24d) and
`awsbnkctl bnk upgrade` turn on the `@type stdout` store in F5's
`f5-toda-fluentd-custom` ConfigMap and `logs tmm` tails that pod. Plain
`logs tmm` prints the TMM pod's lines; `--governance` prints one line per
record:

```text
[GOV] 200 tools/call tool=forecast session=s1 latency=12ms action=allow
[GOV] 403 tools/call tool=delete_all session=s1 latency=1ms action=tool_forbidden
```

### `awsbnkctl bnk mcp-session`

Renders (or `--apply`s) the objects that pin an MCP session to one backend on
BNK 2.4: the passphrase `Secret`, an `F5BigPersistenceProfile` with
`persistenceType: MODEL_CONTEXT_PROTOCOL` and `mcpEncryptionPassphrase.secretRef`,
and one `NetPolicy` per `--listener` attaching the profile to the Gateway.
BNK 2.4.0 programs an `F5BigPersistenceProfile` only in the controller's namespace
(`f5-cne-system`) and a `NetPolicy` resolves the profile in its own namespace, so
the Gateway, the NetPolicy and the profile have to live there for the session to
pin; `doctor --backend k8s` and `forge scan` say so when a profile elsewhere
stays unprogrammed.
Pass `--irule` for every iRule the listener already carries so the single
NetPolicy keeps both. `--type AGENT2AGENT` renders the A2A equivalent.

### Governance telemetry schema

The governance iRule writes one `BNKGOV {json}` record per decision; the Fluent
Bit collector ships it to Loki as stream `job="llm-gateway"` with labels
`model`, `status`, `action` and `rpc_method`. Forge's LLM Observability panel
reads that stream.

| Field | Value |
|---|---|
| `job` | `llm-gateway` |
| `model` | `mcp:<tool server>` |
| `status` | HTTP status as a string (`200`, `403`, `429`) |
| `latency_ms` | request latency; `0` for decisions made before the backend |
| `prompt_tk`, `comp_tk`, `total_tk`, `cached`, `cost` | `0` on the MCP hop |
| `userq` | `<rpc_method> <tool>` when the body was read, else the URI |
| `client`, `caller` | client address and resolved identity (`agent`, `external`, `anonymous`) |
| `action` | `allow`, `rate_limited`, `tool_forbidden`, `backend_refused` |
| `req_body`, `resp_body` | payloads clipped to 280 characters |
| `rpc_method` | `initialize`, `tools/list`, `tools/call` |
| `tool` | `params.name` of a `tools/call` |
| `session` | `Mcp-Session-Id` presented by the client (36 characters max) |
| `transport` | `http`, `sse`, `ws` |

`awsbnkctl forge telemetry` reads the stream (`--loki-url`, default
`http://localhost:3100` behind `awsbnkctl k port-forward -n llm-egress svc/loki 3100:3100`,
or `--file` for exported lines), validates every record against this table,
and prints counts by status, action, method, tool and caller, the 429 and 403
totals, distinct sessions and latency percentiles. It exits non-zero when a
record violates the schema.
