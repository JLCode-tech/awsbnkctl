# Governing the Agentic Action Path: AWS AgentCore + F5 BNK

## Overview

As enterprises accelerate moving AI agents into production, the fundamental challenge shifts from *building* the agent to *securing its reach*. 

**Amazon Bedrock AgentCore** provides the scalable, secure framework to run your agents natively in AWS (handling reasoning, tool discovery, memory, and orchestration). However, when the agent decides to invoke an external API, a corporate data source, or an on-premise MCP server, it crosses a critical trust boundary. 

This is where **F5 BIG-IP Next for Kubernetes (BNK)** operates. BNK does not replicate the AI agent's orchestration; instead, it provides the distributed networking, security, tenancy, and token-governance services exactly where the private traffic meets the cluster. 

**This demonstration stands up a dedicated AWS EKS environment where an AgentCore agent invokes an MCP tool, with the entire data path governed by F5 BNK.**

---

## The Joint Architecture

### First, why a tool call exists at all

The model does not know the answer. Ask the agent to forecast NFLX and Bedrock's
first response is not prose — it is a request:

```json
"text":    "Sure! Let me fetch the latest forecast for Netflix (NFLX)."
"toolUse": { "name": "forecast", "input": {"symbol": "NFLX"} }
"stopReason": "tool_use"          ← it stopped. It did not answer.
```

The runtime executes that call — **through BNK** — and gets back `growth = 12%`,
a number our Python pod generated a second earlier with `random.randint(5, 20)`.
Only then does Bedrock produce prose, and it quotes `12%`.

**Bedrock supplies the reasoning and the language. The tool supplies the facts.**
Bedrock is called twice per request — once to decide, once to narrate — and the
tool hop sits between them. That hop is the one F5 BNK is in. An agent without
its tool call is useless, so whoever controls that hop controls the agent's
reach.

### The three paths

Three ways a caller can reach that same MCP tool. They differ by **who is
calling** and **how much of the stack is allowed to police them**.

| Path | Who calls | Governed by | Status |
| --- | --- | --- | --- |
| **Trusted agent path** | Our own AgentCore runtime | F5 BNK only | ✅ runs today |
| **Double-checked path** | Our own agent, via AgentCore Gateway | AgentCore Gateway **and** F5 BNK | ⚠️ not built — see below |
| **Stranger path** | Anything else — another cloud, a script, a compromised workload | F5 BNK only | ✅ runs today |

**Trusted agent path.** The agent runs in Bedrock AgentCore with VPC-mode ENIs in
our subnets. It resolves `bnk-ingress.bnk-demo.internal` via a private Route 53
zone to the BNK VIP and calls the tool. BNK routes it, rate limits it per source
IP, and logs the decision. AWS never inspects this hop — as far as AgentCore is
concerned the agent made an ordinary outbound HTTP call.

**Double-checked path.** The same agent, but its tool call is routed through an
AgentCore Gateway first. The Gateway adds *semantic* authority BNK does not
have: it knows which principal is asking for which tool, and can apply Cedar
per-tool authorization and guardrails before anything reaches the network. BNK
then applies network policy on the forwarded traffic. Two independent checks,
different questions. **This is not currently wired** — see "Making the
double-checked path real" below.

**Stranger path.** A caller that never touched AWS. No JWT, no Cedar policy, no
guardrail — because none of those components are in the path. BNK is the *only*
thing between them and the tool. This is the case AgentCore structurally cannot
help with, and it is why the two products are complementary rather than
redundant.

### The Chain of Governed Actions

When the Agent acts, it follows this workflow:

| Step | Action | Owner | What Happens |
| :--- | :--- | :--- | :--- |
| **1** | **Reason** | AWS AgentCore | The LLM decides what data it needs to fulfill the user prompt. |
| **2** | **Discover** | AWS AgentCore | The AgentCore Gateway provides a list of available tools. |
| **3** | **Authorize** | AWS AgentCore | Identity scoping ensures the agent is allowed to make the call. |
| **4** | **Cross** | **F5 BNK** | The request hits the network boundary. BNK enforces TLS, peer allowlists, and API gateway policies. |
| **5** | **Invoke** | **F5 BNK** | The tool (MCP server) is reached. BNK enforces the per-client request rate in real time and returns `429` when it is exceeded. |
| **6** | **Observe** | Shared | BNK emits one record per decision into Loki; Forge renders it. Token consumption is reported by Bedrock, the only party in the model path. |

### Architectural Data Paths

Because both platforms use the term "Gateway," their roles need separating. The
split is **not** "AWS does semantics, F5 does network" — BNK can validate JWTs,
run access policy, and (via iRule integration) apply guardrails itself. The real
difference is **coverage**: AgentCore Gateway only sees agents that route
through it; BNK sees everything that reaches the cluster.

In the diagrams below, `[on]` marks a control active in this demo today and
`[available]` marks one BNK supports but which this demo has not configured.

#### Path 1 — Trusted agent path  ✅ runs today

Our own AgentCore agent calling its tool. Note the model loop: Bedrock is hit
twice, and the tool hop between them is the only leg BNK is in.

```text
    ┌──────────────────────┐         ┌────────────────────┐
    │  AgentCore Runtime   │────1───►│  Amazon Bedrock    │  "I need forecast()"
    │  ENI 10.0.11.15      │◄────────│  stopReason:       │   ← no answer yet
    │  (VPC mode, your     │         │    tool_use        │
    │   subnets)           │         └────────────────────┘
    └──────────┬───────────┘                   ▲
               │                               │
               │ 2. MCP tools/call             │ 4. tool result appended,
               │    POST /v1/mcp/forecast      │    model narrates "12%"
               ▼                               │
    ┌ ─ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
       EKS cluster                             │
    │          ▼                               │                  │
       ╔══════════════════════════════════════╗│  ◄── THE ONLY CHECKPOINT
    │  ║          F5 BNK  (TMM)               ║│                  │
       ║  VIP 10.0.10.100  :80  :443          ║│
    │  ║ ┌──────────────────────────────────┐ ║│                  │
       ║ │ HTTPRoute + URLRewrite      [on] │ ║│
    │  ║ │ rate limit 10/60s per IP    [on] │ ║│                  │
       ║ │ MCP payload capture         [on] │ ║│
    │  ║ │ F5BigFwPolicy ACL           [on] │ ║│                  │
       ║ │ JWT validation        [available]│ ║│
    │  ║ │ OAuth / access policy [available]│ ║│                  │
       ║ │ F5BigDdosGlobal       [available]│ ║│
    │  ║ └──────────────────────────────────┘ ║│                  │
       ╚══════════════════┬═══════════════════╝│
    │                     ▼                    │                  │
                ┌───────────────────┐   3. forecast = 12%
    │           │  mcp-server pod   │──────────┘                  │
                └───────────────────┘
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘

    AgentCore is not in step 2 by construction — the agent simply made an
    outbound call. BNK is what governs that hop.
```

#### Path 2 — Double-checked path  ⚠️ not built

The same agent, but the tool call is routed through an AgentCore Gateway first,
so two independent policy engines see it. The value here is **separation of
duties**, not extra capability — BNK can do JWT and policy itself; putting
AWS-owned authorization in front means a BNK misconfiguration alone does not
open the tool, and vice versa.

```text
    ┌──────────────────────┐
    │  AgentCore Runtime   │
    └──────────┬───────────┘
               │  MCP over SigV4
               ▼
    ╔══════════════════════════════════════╗   ◄── CHECKPOINT 1 (AWS-owned)
    ║      AgentCore Gateway  (AWS)        ║
    ║  · inbound auth (JWT / SigV4)        ║   • may THIS principal call
    ║  · Cedar per-tool authorization      ║     THIS tool?
    ║  · guardrails (content, prompt       ║   • jailbreak / PII detected?
    ║    attack, sensitive info)           ║   • MCP ──translate──► HTTP
    ╚══════════════════┬═══════════════════╝
                       │  VPC Lattice resource gateway
                       │  (ENIs in your subnets)
    ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
          EKS cluster  ▼
    │  ╔══════════════════════════════════════╗              │  ◄── CHECKPOINT 2
       ║          F5 BNK  (TMM)               ║                    (yours)
    │  ║  everything from Path 1, applied to  ║              │
       ║  the forwarded traffic               ║
    │  ╚══════════════════┬═══════════════════╝              │
                          ▼
    │           ┌───────────────────┐                        │
                │  mcp-server pod   │
    │           └───────────────────┘                        │
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘

    Blocked on: no AgentCore Gateway is deployed in this project
    (`agentcore status` lists no gateways), and reaching a private VIP from a
    Gateway needs VPC-Lattice egress. See "7. Future capabilities" below.
```

#### Path 3 — Stranger path  ✅ runs today

A caller that never touched AWS: another cloud, a script, a compromised
workload, a partner integration. **No AgentCore component is in this path** — no
JWT check, no Cedar policy, no guardrail — because none of them are reachable
from here. Whatever protects the tool pod has to be in the cluster.

```text
    ┌──────────────────┐
    │  External agent  │   unmanaged, untrusted, never authenticated to AWS
    └────────┬─────────┘
             │  POST /v1/mcp/forecast
    ┌ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
             ▼   EKS cluster
    │  ╔══════════════════════════════════════╗     │  ◄── THE ONLY CHECKPOINT
       ║          F5 BNK  (TMM)               ║           No AgentCore component
    │  ║ ┌──────────────────────────────────┐ ║     │     is in this path.
       ║ │ rate limit 10/60s per IP    [on] │ ║
    │  ║ │   → 429 at request 11            │ ║     │
       ║ │ F5BigFwPolicy ACL           [on] │ ║
    │  ║ │ MCP payload capture         [on] │ ║     │
       ║ │ JWT validation        [available]│ ║
    │  ║ │ MCP method/tool allowlist        │ ║     │
       ║ │                       [proposed] │ ║
    │  ║ └──────────────────────────────────┘ ║     │
       ╚══════════════════┬═══════════════════╝
    │                     ▼                         │
                ┌───────────────────┐
    │           │  mcp-server pod   │               │
                └───────────────────┘
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

### The AWS + F5 story: what each side brings

This is an **AND**, not a comparison. Bedrock AgentCore and F5 BNK solve
adjacent problems, and the interesting architecture is the one that uses both.

**What AgentCore brings.** A managed runtime so you are not operating agent
infrastructure. Managed model access. And — where a Gateway is in the path —
identity-aware, per-tool authorization in Cedar, guardrails for content and
prompt attacks, and MCP↔HTTP translation for tools that are not MCP-native.
That is real authorization semantics, owned and operated by AWS, that you do
not have to build.

**What BNK adds.** BNK sits in the cluster, next to the workloads, and is in the
path for *everything* that reaches them:

1.  **Reach beyond the managed path.** Path 3 is traffic that never touched
    AWS — another cloud, a partner, a script, a compromised workload. BNK
    extends the same governance to callers no managed front door is positioned
    to see.
2.  **One policy surface across agent estates.** The same BNK config governs an
    AgentCore agent, a self-hosted agent, and a third party's caller. As agent
    frameworks and clouds multiply, the enforcement point stays put.
3.  **Protection of the workload itself.** DDoS vectors, per-source rate limits
    and connection limits sit beside the pods, protecting them from
    over-consumption and abuse regardless of which door traffic came through.
4.  **Consolidation where you want it.** JWT validation, OAuth, access policy
    and iRule-based guardrail integration are available as BNK CRDs — 25
    identity-related CRDs ship in this cluster. Teams that prefer authorization
    at the network edge can put it there; teams that prefer it in AgentCore can
    leave it there. Both work, and they compose.
5.  **Unified evidence.** One telemetry stream covering every path, joined with
    Bedrock's own token records so the governance view and the cost view sit
    side by side (section 6).

**Together.** Path 2 is the shape worth aiming at: AWS makes the authorization
decision it is best placed to make, BNK enforces network and workload policy in
the cluster, and neither is a single point of failure for the other. Path 3
exists because not every caller will take Path 2 — and that gap is where the
in-cluster data plane earns its place.

---

## 1. Preparing the Infrastructure

We use `awsbnkctl` to deterministically provision the entire underlying network and EKS environment in AWS (`ap-southeast-2`), integrating directly with the local Forge instance.

> [!NOTE]
> This creates a dedicated VPC, EKS cluster (`1.31`), subnets, and host-device ENIs for the BNK data path without affecting any existing workloads.

### Intent File (`cluster.yaml`)

We have created the declarative intent file at [`examples/agentcore-demo/cluster.yaml`](cluster.yaml).

*   **Pattern:** `host-device` (Dual-interface for BNK inspection)
*   **Region:** `ap-southeast-2`
*   **Integration:** Local Forge `http://localhost:8000`

### F5 credentials (you must supply these)

The `bnk:` block references two private files that are **not in this repo and
must never be committed** — both are gitignored repo-wide (`cne_pull_64.json`,
`*.jwt`):

| File | What it is |
| --- | --- |
| `cne_pull_64.json` | F5 FAR pull credentials (a `dockerconfigjson`) from the F5 portal — grants registry pull rights |
| `license.jwt` | Your F5 subscription JWT |

Drop your own copies into this directory, or symlink them in from wherever you
keep them:

```bash
cd examples/agentcore-demo
ln -sfn /path/to/your/cne_pull_64.json cne_pull_64.json
ln -sfn /path/to/your/subscription.jwt license.jwt
```

Paths in `cluster.yaml` are relative to **the config file's directory**, not
your shell's working directory, so `./cne_pull_64.json` resolves here.

### Provisioning the Cluster

To dry-run and validate the infrastructure setup, you can execute:
```bash
cd /path/to/awsbnkctl
./awsbnkctl validate examples/agentcore-demo/cluster.yaml
AWSBNKCTL_SKIP_AUTH=1 ./awsbnkctl up --config examples/agentcore-demo/cluster.yaml --dry-run
```

Once ready to deploy (this takes ~25 minutes and incurs standard AWS EKS/EC2 costs):
```bash
AWSBNKCTL_FORGE_PASSWORD=admin123 ./awsbnkctl up --config examples/agentcore-demo/cluster.yaml
```

---

## 2. Deploying the Sample MCP Tool

We will select an example from the `awslabs/agentcore-samples` repository—specifically, a **Weather/Financial MCP Server**. 

Once the cluster is up, deploy the MCP tool and BNK gateway to EKS:
```bash
cd examples/agentcore-demo
./awsbnkctl k apply -f mcp-tool/          # kustomize base — see below
./awsbnkctl k apply -f gateway-deployment.yaml
```

The tool lives in `mcp-tool/` as a Kustomize base rather than a single manifest.
`mcp-server.py` stays a real Python file and the ConfigMap that carries it is
**generated** from it, so the two can never drift — and because the generator
appends a content hash to the ConfigMap name, editing the server rolls the
Deployment automatically. `awsbnkctl k apply -f <dir>` detects
`kustomization.yaml` and builds it via krusty, so no separate `kustomize` binary
is needed (`kubectl apply -k mcp-tool/` works too).

> [!NOTE]
> Migrating a cluster that already ran the older single-file manifest? The first
> apply fails with `conflict with "kubectl-client-side-apply"` on
> `.spec.template.spec.volumes[...].configMap.name` — server-side apply refuses
> to take a field another manager owns. Re-run once with `--force`, then delete
> the now-orphaned unhashed ConfigMap:
> ```bash
> ./awsbnkctl k apply -f mcp-tool/ --force
> kubectl delete cm mcp-server-code -n default
> ```

**Network seam:** AgentCore runtimes in VPC mode attach ENIs in your VPC. To route them to the BNK VIP (a secondary ENI on the TMM node), run:
```bash
cd examples/agentcore-demo
AWS_PROFILE=<profile> KUBECONFIG=.awsbnkctl/bnk-agentcore-demo/kubeconfig ./scripts/setup-agentcore-network.sh
```
This creates the agent security group, SG-to-SG ingress to the BNK data-plane SG, and a private Route 53 zone with `bnk-ingress.bnk-demo.internal` pointing at the BNK VIP.

---

## 3. Configuring F5 BNK Governance

Routing lives in `gateway-deployment.yaml`. Security policy lives in `mcp-security-policy.yaml`:

```bash
./awsbnkctl k apply -f examples/agentcore-demo/mcp-security-policy.yaml --config examples/agentcore-demo/cluster.yaml
```

BNK 2.3 attaches policy to a Gateway through two policy-attachment CRs:

| CR | Attaches | Used here for |
| --- | --- | --- |
| `BNKNetPolicy` | `F5BigCneIrule` | L7 request rate limiting on the MCP route |
| `BNKSecPolicy` | `F5BigFwPolicy`, `F5BigLogProfile`, `F5BigDdosGlobal` | L4 firewall attached to the Gateway |

**Rate limiting (`F5BigCneIrule` + `BNKNetPolicy`).** A per-client-IP window of
10 requests / 60 s on `/v1/mcp/forecast`, returning `429` with a JSON-RPC error
body. Because the key is the client IP, the Bedrock AgentCore runtime and an
external caller get independent budgets — a single `agentcore invoke` never
trips the limit, but a tight external loop does. `/.well-known/agent-card.json`
is exempt so Forge discovery keeps working.

**Token counting is intentionally NOT enabled on this Gateway.** BNK configures
it via `spec.infrastructure.annotations` (not `metadata.annotations`) and it
applies at Gateway scope — there is no per-route opt-in. With
`token_counting=enabled`, TMM's AI profile requires an LLM `model` field in
every request body and rejects anything else with
`400 {"error":"model_missing"}`, which breaks all MCP JSON-RPC traffic. The
demo's only LLM leg (AgentCore runtime → Bedrock) is AWS-internal and never
crosses this Gateway, so there is nothing here to meter. `gateway-deployment.yaml`
carries the correct annotation block, commented out, for use on a Gateway that
does front an LLM backend.

> [!TIP]
> Enforcement happens in TMM on the data path, so a runaway agent is stopped at
> the network edge — before it reaches the tool, the model, or the GPU behind it.

### Gotchas that cost us time

| Symptom | Cause |
| --- | --- |
| `400 {"error":"model_missing"}` on every MCP call | `token_counting=enabled` on the Gateway. It is Gateway-scoped and demands an LLM `model` field in every body. |
| iRule rejected: `braces are required around the expression` | The `f5validate.f5net.com` admission webhook rejects **any** `#` comment line inside an `F5BigCneIrule` script. Keep the TCL comment-free; explain in YAML comments. |
| Rate limit trips at half the configured count | The same iRule attached at *both* Gateway and listener scope. Gateway-level rules run first, listener-level second — the rule executes twice per request. |
| Rate-limit window never rolls over | `table incr` + `table lifetime` refreshes the idle timer on every hit. Use `table set KEY VALUE TIMEOUT LIFETIME` once, then `-notouch` on every read. |
| VIP goes dark after applying a policy file | A second manifest re-declared the `Gateway` with only `metadata`. `kubectl apply` prunes `spec` fields from the previous last-applied — including `listeners`. |
| `HSL::send` from an iRule delivers nothing | Not wired in BNK 2.3. `log local0.` works; that is what the collector tails. |

---

## 4. Observability — seeing the governance decisions

BNK's enforcement decisions are only useful if you can see them. The iRule emits
one JSON record per MCP request — allowed or rate-limited — and a Fluent Bit
DaemonSet ships those into Loki, where BNK Forge's **LLM Observability** panel
reads them.

```text
   external agent / AgentCore runtime
                 │  POST /v1/mcp/forecast
                 ▼
   ╔═══════════════════════════════════════════════════╗
   ║  F5 BNK  (TMM)                                    ║
   ║   F5BigCneIrule "mcp-rate-limit-irule"            ║
   ║   ┌─────────────────────────────────────────────┐ ║
   ║   │ HTTP_REQUEST                                │ ║
   ║   │  table incr  mcp_rl:<client-ip>             │ ║
   ║   │  > 10 in 60s ?                              │ ║
   ║   │     yes ─► log local0. BNKGOV {...429...}   │ ║──► HTTP 429
   ║   │            HTTP::respond 429                │ ║    (never reaches pod)
   ║   │     no  ─► forward                          │ ║
   ║   ├─────────────────────────────────────────────┤ ║
   ║   │ HTTP_RESPONSE                               │ ║
   ║   │  log local0. BNKGOV {...200, latency...}    │ ║
   ║   └─────────────────────────────────────────────┘ ║
   ╚════════════════════════┬══════════════════════════╝
                            │              │
              mcp-financial-tool pod       │  stdout of the f5-fluentbit
                                           │  sidecar in the f5-tmm pod
                                           ▼
                      ┌──────────────────────────────────┐
                      │ bnkgov-collector  (DaemonSet)    │  ns: llm-egress
                      │  tail  /var/log/containers/      │
                      │        f5-tmm-*_f5-fluentbit-*   │
                      │  grep  BNKGOV                    │
                      │  parse JSON  ─► label job/model/ │
                      │                       status     │
                      └────────────────┬─────────────────┘
                                       ▼
                      ┌──────────────────────────────────┐
                      │ loki.llm-egress:3100             │
                      └────────────────┬─────────────────┘
                                       ▼
                      ┌──────────────────────────────────┐
                      │ BNK Forge → LLM Observability    │
                      │  requests / success rate /       │
                      │  latency / model rankings        │
                      └──────────────────────────────────┘
```

Deploy it:

```bash
cd examples/agentcore-demo
./awsbnkctl k apply -f mcp-observability.yaml --config cluster.yaml
```

The namespace, service name and port are load-bearing. Forge queries
`http://loki.llm-egress:3100` and expects streams labelled `job="llm-gateway"`
with `model` and `status`. Change any of those and the panel stays empty.

**Reused from the Forge module catalog** (`JLCode-tech/bnk-forge-modules`):
the Loki 3.0 deployment from `app/demo-observability`, and the log-tailing
Fluent Bit DaemonSet shape from `modules/live-observability-collector`.
**Deliberately not reused:** `app/demo-ai-proxy` (a LiteLLM shim in front of
Bedrock — this demo calls Bedrock directly, so a proxy would be a fiction),
`app/demo-ai-analyzer` (F5BigAnalyzer, not needed), and the `app/demo-irules`
"token counting" rule, which despite its name only stamps `X-BNK-*` headers and
logs — it counts nothing.

### What the token columns mean

`prompt_tk` / `comp_tk` / `total_tk` / `cost` are **zero, honestly**. BNK is
in-path for the agent-to-tool hop, not for the model call, so it has no tokens
of its own to report. The agent's Bedrock call never crosses this Gateway.

Real token counts have to come from the side that *is* in that path — Bedrock.
See "Closing the token gap" below.

---

## 5. Running the End-to-End Test

A runbook you can follow top to bottom. Each step states what you should see.

### Step 0 — Preflight

```bash
cd examples/agentcore-demo
export KUBECONFIG=$PWD/.awsbnkctl/bnk-agentcore-demo/kubeconfig
export AWS_PROFILE=<profile>

kubectl get gateway bnk-agentcore-demo-gateway -n default
kubectl get f5-big-cne-irules,bnknetpolicies,bnksecpolicies -n default
kubectl get pods -n llm-egress
```

Expect the Gateway `PROGRAMMED=True` with address `10.0.10.100`, the iRule
`READY=True` ("CR config sent to all grpc endpoints"), two BNKNetPolicies
(`-http`, `-http443`), and `loki` + `bnkgov-collector` pods Running.

### Step 1 — Trusted agent path: AgentCore agent → BNK → MCP tool

```bash
cd examples/agentcore-demo/agent
AWS_PROFILE=<profile> ./node_modules/.bin/agentcore invoke \
  --runtime FinanceAgentV2Agent --target demo-v2 --prompt "forecast AMZN"
```

Expect a forecast table with an "Expected Growth" percentage — that number is
generated by the MCP tool pod, so seeing it proves the whole path. First run
after a cold start can take ~90 s.

> [!IMPORTANT]
> This is the **trusted agent path**, not the double-checked one. The runtime's
> MCP client (`agent/app/FinanceAgentV2Agent/mcp_client/client.py`) calls
> `http://bnk-ingress.bnk-demo.internal/v1/mcp/forecast` directly on port 80 —
> the URL is hardcoded. The `FinanceAgentV2` harness does declare an
> `agentcore_gateway` tool called `BnkGatewayTool`, but that reference is
> currently dangling: `agentcore status` reports no deployed gateways, and
> `agentcore invoke --gateway BnkGateway` answers
> `Gateway 'BnkGateway' is not deployed`.
>
> The evidence is in the telemetry — every request BNK has logged on this route
> came from either the runtime ENI (`10.0.11.15`, interface type `agentic_ai`)
> or the jumphost (`10.0.10.29`). Nothing has ever arrived from a Gateway.
>
> The port-443 listener exists because AgentCore Gateway targets require an
> `https://` URL scheme; it is plain HTTP on the wire and currently unused by
> any AWS component.

Run it three times in a row. All three must succeed — the rate limit is
per-client-IP and one invoke uses only a few requests of the budget.

### Step 2 — Stranger path: external agent → BNK → MCP tool

The BNK VIP is private. Run the external agent from a host inside the VPC.
The jumphost has a second ENI on the VIP's subnet, so it works unmodified:

```bash
# Get the jumphost instance id
aws ec2 describe-instances --region ap-southeast-2 \
  --filters "Name=tag:Name,Values=bnk-agentcore-demo-jumphost" \
            "Name=instance-state-name,Values=running" \
  --query 'Reservations[].Instances[].InstanceId' --output text
```

Copy `external-agent.py` to that host and run it, or drive it over SSM:

```bash
INSTANCE=<jumphost-instance-id>
aws ssm send-command --region ap-southeast-2 --instance-ids "$INSTANCE" \
  --document-name AWS-RunShellScript \
  --parameters 'commands=["for i in $(seq 1 20); do curl -s -o /dev/null -w \"%{http_code} \" -X POST http://10.0.10.100/v1/mcp/forecast -H \"Host: bnk-ingress.bnk-demo.internal\" -H \"Content-Type: application/json\" -H \"Accept: application/json\" -H \"Authorization: Bearer external-agent-token-123\" -d \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":1,\\\"method\\\":\\\"tools/call\\\",\\\"params\\\":{\\\"name\\\":\\\"forecast\\\",\\\"arguments\\\":{\\\"symbol\\\":\\\"NVDA\\\",\\\"days\\\":30}}}\"; done"]' \
  --query 'Command.CommandId' --output text
# then: aws ssm get-command-invocation --region ap-southeast-2 \
#         --command-id <id> --instance-id "$INSTANCE" --query StandardOutputContent --output text
```

Expected — the limit biting at exactly the 11th request:

```text
200 200 200 200 200 200 200 200 200 200 429 429 429 429 429 429 429 429 429 429
```

And the 429 body:

```json
{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"rate limit exceeded: 10 requests per 60s per client"}}
```

with `Retry-After: 60`. Wait 65 s and the window rolls over — the next 10
requests succeed again.

Discovery stays exempt; this returns `200` however many times you run it:

```bash
curl -s -o /dev/null -w '%{http_code}\n' \
  http://10.0.10.100/.well-known/agent-card.json \
  -H 'Host: bnk-ingress.bnk-demo.internal'
```

### Step 3 — See the decisions in Loki

```bash
kubectl run lq --rm -i --restart=Never -n llm-egress --image=curlimages/curl:8.8.0 -- \
  -s --get 'http://loki:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="llm-gateway"}' --data-urlencode 'limit=5'
```

You should see records with `"action":"allow"` and `"action":"rate_limited"`,
each carrying the real `client` IP, `status` and `latency_ms`.

### Step 4 — See it in BNK Forge

| What | Where in Forge |
| --- | --- |
| The iRule, its line count and event handlers | **Gateway Topology** → your Gateway → under each listener (`http`, `http443`) → *Network Policies* |
| The firewall policy and its rules | **Gateway Topology** → your Gateway → *Security Policies* (`mcp-sec-policy` → `mcp-firewall`) |
| Request volume, success rate, latency, model rankings | **LLM Observability** for cluster `bnk-agentcore-demo` |

> [!IMPORTANT]
> Forge only renders an iRule attachment when the `BNKNetPolicy` names a
> listener via `sectionName`. A Gateway-scoped BNKNetPolicy enforces correctly
> but is invisible in the UI — which is why this demo ships one policy per
> listener.

Same data over the API:

```bash
TOKEN=$(curl -s -X POST http://localhost:8000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

CLUSTER=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/k8s/clusters \
  | python3 -c 'import sys,json;print([c["id"] for c in json.load(sys.stdin) if c["name"]=="bnk-agentcore-demo"][0])')

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8000/api/k8s/clusters/$CLUSTER/llm-observability/stats" | python3 -m json.tool
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8000/api/k8s/clusters/$CLUSTER/f5bnk/gateway-topology" | python3 -m json.tool
```

After the runs above, `stats` looks like this — 13 requests, 10 allowed and 3
rate-limited, hence the 0.77 success rate:

```json
{
  "available": true,
  "endpoint": "http://loki.llm-egress:3100",
  "total_requests": 13,
  "success_rate": 0.769,
  "avg_latency_ms": 5.75,
  "total_tokens": 0,
  "models": 1
}
```

---

## 6. Closing the token gap — the Bedrock leg

BNK's records carry `total_tk: 0`, and that is honest: BNK never sees the model
call. The side that *does* see it is Bedrock.

**Model invocation logging** writes one CloudWatch record per call containing
`input.inputTokenCount`, `output.outputTokenCount`, and the full prompt and
completion bodies. `mcp-bedrock-token-shipper.yaml` polls that log group and
rewrites each record into the same `job="llm-gateway"` Loki schema, so Forge
shows both legs side by side:

```text
   AgentCore runtime
        │
        ├─ model call ──► Amazon Bedrock ──► CloudWatch Logs ──┐
        │                 (real token counts)                  │
        │                                                      ▼
        └─ tool call ───► F5 BNK ──► MCP pod                 shipper
                            │        (governed: 429s)          │
                            └──► TMM log ──► Fluent Bit ───────┤
                                                               ▼
                                                    loki.llm-egress:3100
                                                               │
                                                               ▼
                                                  Forge LLM Observability
                                          BNK: who was allowed/throttled
                                      Bedrock: what it cost in tokens
```

### 6.1 One-time AWS setup

Two IAM roles are needed — one Bedrock assumes to *write* the logs, one the
shipper pod assumes via IRSA to *read* them.

```bash
export ACCT=<account-id> REGION=ap-southeast-2
aws logs create-log-group --log-group-name /aws/bedrock/modelinvocations
aws logs put-retention-policy --log-group-name /aws/bedrock/modelinvocations --retention-in-days 7

# 1. Role Bedrock assumes to write invocation logs
cat > trust.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow",
 "Principal":{"Service":"bedrock.amazonaws.com"},"Action":"sts:AssumeRole",
 "Condition":{"StringEquals":{"aws:SourceAccount":"${ACCT}"}}}]}
EOF
cat > perm.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow",
 "Action":["logs:CreateLogStream","logs:PutLogEvents","logs:DescribeLogStreams"],
 "Resource":["arn:aws:logs:${REGION}:${ACCT}:log-group:/aws/bedrock/modelinvocations",
             "arn:aws:logs:${REGION}:${ACCT}:log-group:/aws/bedrock/modelinvocations:*"]}]}
EOF
aws iam create-role --role-name BedrockModelInvocationLogging \
  --assume-role-policy-document file://trust.json
aws iam put-role-policy --role-name BedrockModelInvocationLogging \
  --policy-name WriteInvocationLogs --policy-document file://perm.json

aws bedrock put-model-invocation-logging-configuration --logging-config "$(cat <<EOF
{"cloudWatchConfig":{"logGroupName":"/aws/bedrock/modelinvocations",
 "roleArn":"arn:aws:iam::${ACCT}:role/BedrockModelInvocationLogging"},
 "textDataDeliveryEnabled":true,"imageDataDeliveryEnabled":false,
 "embeddingDataDeliveryEnabled":false}
EOF
)"

# 2. IRSA role for the shipper pod (needs the cluster OIDC provider)
OIDC=$(aws eks describe-cluster --name bnk-agentcore-demo \
  --query 'cluster.identity.oidc.issuer' --output text)
HOST=${OIDC#https://}
cat > shipper-trust.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow",
 "Principal":{"Federated":"arn:aws:iam::${ACCT}:oidc-provider/${HOST}"},
 "Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{
   "${HOST}:aud":"sts.amazonaws.com",
   "${HOST}:sub":"system:serviceaccount:llm-egress:bedrock-token-shipper"}}}]}
EOF
cat > shipper-perm.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow",
 "Action":["logs:FilterLogEvents","logs:DescribeLogStreams","logs:GetLogEvents"],
 "Resource":["arn:aws:logs:${REGION}:${ACCT}:log-group:/aws/bedrock/modelinvocations",
             "arn:aws:logs:${REGION}:${ACCT}:log-group:/aws/bedrock/modelinvocations:*"]}]}
EOF
aws iam create-role --role-name BNKDemoBedrockTokenShipper \
  --assume-role-policy-document file://shipper-trust.json
aws iam put-role-policy --role-name BNKDemoBedrockTokenShipper \
  --policy-name ReadInvocationLogs --policy-document file://shipper-perm.json
```

> [!WARNING]
> Two traps that cost real time here. **In zsh, write `${ACCT}` not `$ACCT`** —
> `$ACCT:log-group` applies zsh's `:l` lowercase modifier and silently eats the
> `:l`, producing a mangled ARN and a `Failed to validate permissions` error
> that points at the wrong thing. And **IAM needs ~30 s to propagate** before
> `put-model-invocation-logging-configuration` will validate the role.

### 6.2 Deploy the shipper

Put the IRSA role ARN in the ServiceAccount annotation, then:

```bash
./awsbnkctl k apply -f examples/agentcore-demo/mcp-bedrock-token-shipper.yaml \
  --config examples/agentcore-demo/cluster.yaml
kubectl logs -n llm-egress deploy/bedrock-token-shipper --tail=5
# [shipper] group=/aws/bedrock/modelinvocations loki=http://loki:3100/... interval=20.0s
# [shipper] pushed 2 record(s)
```

Cost is computed from `PRICE_IN_PER_1K` / `PRICE_OUT_PER_1K` env vars. The
defaults are Anthropic's published list rate for Sonnet 4.6 ($3 / $15 per 1M) —
**Bedrock is partner-operated and priced separately**, so check
[Bedrock pricing](https://aws.amazon.com/bedrock/pricing/) and override them, or
set both to `0` to leave cost out.

`BODY_LIMIT` (default 4000) caps how much prompt/completion text is copied into
Loki. Lower it if prompt content must not leave the AWS account.

### 6.3 What you get

Run one `agentcore invoke`, then open **Forge → LLM Observability**. Both legs
appear as separate models in the rankings:

| model | requests | tokens | cost | what it tells you |
| --- | --- | --- | --- | --- |
| `mcp:finance-tool` | 25 | 0 | 0 | BNK's view: who called the tool, who got throttled |
| `global.anthropic.claude-sonnet-4-6` | 4 | 4867 | 0.0204 | Bedrock's view: what the reasoning actually cost |

Opening a log record shows the raw request and response for either leg:

```text
BNK leg (mcp:finance-tool)
  userq     tools/call forecast
  req_body  {"method":"tools/call","params":{"name":"forecast",
             "arguments":{"symbol":"META"}},"jsonrpc":"2.0","id":2}
  resp_body {"jsonrpc":"2.0","id":2,"result":{...
             "text":"Forecast for META over 30 days: up (expected growth: 16%)."}]

Bedrock leg (global.anthropic.claude-sonnet-4-6)
  userq     forecast META
  req_body  {"messages":[{"role":"user","content":[{"text":"forecast META"}]},...
  resp_body {"output":{"message":{"role":"assistant","content":[{"text":
             "Here is the latest **30-day forecast for META...
```

BNK can show the MCP bodies because it terminates that hop in plaintext —
`HTTP::collect` buffers up to 4 KB and the iRule logs it. Rate-limited records
carry an empty `req_body` on purpose: the limit fires *before* the payload is
buffered, so a throttled request is never read or forwarded.

> [!NOTE]
> The distinction this makes explicit, and which the demo should not blur: BNK
> is doing **request-rate enforcement**, not token enforcement. BNK's token
> quota feature (`k8s.f5.com/ai-token-counting`) is real, but it only functions
> when an LLM backend sits behind the Gateway — which is not this topology.

---

## 7. Future capabilities

The demo is deliberately light — it proves the paths, not the full control set.
These are the capabilities BNK already has that a fuller build would turn on.
None of them are wired today.

### 7.1 Completing the double-checked path

AWS has published a lab for exactly this topology:
[`01-features/.../01-gateway/03-private-connectivity/connect-gateway-to-private-resources/05-eks-deployment`](https://github.com/awslabs/agentcore-samples/tree/main/01-features/07-centralize-and-govern-your-ai-infrastructure/01-gateway/03-private-connectivity/connect-gateway-to-private-resources/05-eks-deployment).
Its reference data path is:

```text
AgentCore Gateway
  → VPC Lattice (routingDomain = NLB *.elb.amazonaws.com)
    → Resource gateway ENIs (in your subnets)
      → Internal NLB (TLS :443, ACM public cert)
        → NGINX Ingress Controller (HTTP :80, path-based routing)   ◄── BNK's slot
          → EKS pods (FastMCP :8000/:8080)
```

**That NGINX Ingress Controller position is where F5 BNK goes.** AWS's own
architecture says an in-cluster ingress data plane belongs in this path; the lab
fills it with NGINX. Substituting BNK is a drop-in that adds per-source rate
limiting, L4 DDoS vectors, MCP payload visibility, iRule extensibility and the
unified telemetry in section 4 — in a slot the reference design already requires.

Requirements the lab makes explicit, which this demo does **not** currently meet:

| Requirement | Today | Needed |
| --- | --- | --- |
| Target presents a **publicly trusted TLS cert** | BNK VIP is plain HTTP on :80/:443 | Internal NLB terminating TLS with an ACM public cert, forwarding plain HTTP to BNK. Private CA / self-signed needs the ALB proxy workaround. |
| **Inbound auth** on the Gateway | n/a | `privateEndpoint` targets cannot use `NO_AUTH` — Cognito (or another IdP) OAuth client-credentials, unless an interceptor Lambda is configured |
| **DNS** | private Route 53 zone → VIP | private hosted zone plus `routingDomain` pointing at the NLB's public DNS name |
| **Gateway + target** | none deployed | managed VPC resource mode: create the target with `--vpc-id --subnet-ids --security-group-ids` so AgentCore provisions the VPC Lattice resource gateway |

awsbnkctl already provisions the AWS Load Balancer Controller, so the internal
NLB in front of BNK is available rather than new work.

### 7.2 Identity enforcement at BNK

BNK can validate tokens itself — 25 identity CRDs are installed in this cluster.
`F5BigAccessJwtConfig` carries `audience`, `allowedSigningAlgorithms`,
`allowedKeys` (JWK references) and token blacklisting; there are also OAuth
provider/server, SAML and access-policy CRDs with explicit allow/deny endings.

This matters for Path 3, where no AWS component is present to check a token. For
Path 1 and Path 2 it is a placement choice, not a capability gap on either side:
AgentCore Identity and Policy cover the managed path well, and BNK can carry the
same check at the network edge for callers that never reach a Gateway.

### 7.3 Guardrails via iRule integration

BNK supports iRule-based integration with external inspection services — the
hook for content filtering, prompt-attack detection and PII handling. Scope this
carefully: for traffic through an AgentCore Gateway, Guardrails already feed
those signals into Cedar policy decisions. The BNK hook is for the traffic that
never passes a Gateway, or for estates spanning clouds where a single inspection
path is wanted.

### 7.4 MCP-aware inspection — and what NOT to build

The obvious next step looks like enforcement in the iRule: a JSON-RPC method
allowlist, a per-tool allowlist, argument-shape checks. **Do not build it as a
differentiator.** AgentCore already does this, natively and better:

*   **Policy in AgentCore** is a Cedar-based authorization layer that intercepts
    every tool call through a Gateway. The principal comes from the JWT, the
    action is the tool, and the **context is the tool arguments** — so
    parameter-level decisions are first-class. Default-deny and forbid-wins are
    enforced automatically.
*   **Tool filtering at list time** uses Cedar partial evaluation to omit tools
    the caller could never invoke, so the model never even sees them.
*   **Fine-grained access control** is documented at four levels — gateway,
    tool, **operation** (`tools/list`, `tools/call`) and **parameter**.
*   **Temporal policies (Dogwood)** add session-aware rules and **rate limiting**
    at the gateway, judging a request against what the agent already did.
*   **Guardrails** feed prompt-injection and sensitive-information signals into
    the same deterministic policy decision.

A TCL reimplementation of that would be strictly worse and would not survive
contact with an AWS architect. Where BNK's payload visibility genuinely earns
its place is the traffic Cedar never evaluates — **Path 3**, where no Gateway is
in the path, and where the question is not "may this principal call this tool"
but "is my pod being abused". For that, per-source rate limiting (running today)
and `F5BigDdosGlobal` L4 vectors are the right controls, and they are network
controls, not authorization ones.

Refs: [Policy in AgentCore](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/policy.html) ·
[Fine-grained access control](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/gateway-fine-grained-access-control.html) ·
[Why Cedar](https://aws.amazon.com/blogs/security/why-policy-in-amazon-bedrock-agentcore-chose-cedar-for-securing-agentic-workflows/) ·
[Temporal policies and rate limiting](https://aws.amazon.com/blogs/machine-learning/control-agent-behaviors-and-cost-beyond-a-single-action-new-capabilities-in-amazon-bedrock-agentcore/)

### 7.5 Where this demo sits relative to the AWS samples

An earlier draft of this README claimed no AWS sample runs an MCP server on
Kubernetes. **That was wrong** — the claim came from the `02-use-cases`
assessment file, and the relevant material is under `01-features`. Corrected:

*   **`01-features/.../03-private-connectivity/05-eks-deployment`** covers this
    directly: FastMCP servers on EKS, behind NGINX Ingress and an internal NLB,
    connected to an AgentCore Gateway over VPC Lattice egress. Two labs —
    `mcp-server-gateway-managed` and `api-server-gateway-managed`.
*   `01-features/.../03-registry/03-advanced/strands-mcp-ecs-registry` covers the
    ECS equivalent, and `04-ecs-deployment` sits alongside the EKS lab.
*   In `02-use-cases`, `customer-support-assistant-vpc` is about running the
    *Runtime* in a VPC with private endpoints, not about governing traffic to a
    self-hosted tool. `claude-code-gateway-mcp-server` fronts the AWS-managed
    Knowledge MCP Server — no VPC, no self-hosted compute, notebook only, and
    marked "not intended for direct use in production".

So self-hosted MCP on EKS is a pattern AWS has already documented, not a gap.
The honest position is narrower and stronger: **AWS's reference design puts an
ingress data plane in front of the pods and fills it with NGINX. That slot is
the one F5 BNK is built for.** The demo's contribution is showing what changes
when a full ADC occupies it — governance, workload protection and unified
evidence — rather than claiming a scenario nobody has covered.

---

## Troubleshooting

> The stranger path requires the external caller to have network reachability to the private BNK VIP. The demo does not expose the BNK VIP to the public internet.

### AgentCore Initialization Timeout (ECR Image Missing)
If `agentcore invoke` times out after 120s with "Runtime initialization time exceeded", the ECS task is likely failing to start because its container image is missing.
This typically happens if the `aws ecr delete-repository --force` command is used out of band. CloudFormation still believes the repository exists, so `agentcore deploy` will not recreate it or push a new image. 

To fix this:
1. Recreate the ECR repository manually:
   `aws ecr create-repository --repository-name bnkagent/financeagentv2agent`
2. Modify `main.py` slightly (e.g. adding a newline) to change the source hash and force a new build.
3. Run `npx agentcore deploy --target demo-v2 -y` to build and push the new image.

---

## Next Steps

1. Review the [`cluster.yaml`](examples/agentcore-demo/cluster.yaml) configuration.
2. Run `awsbnkctl up` to provision the cluster and BNK.
3. Deploy the MCP tool and gateway, run `setup-agentcore-network.sh`, then test A / B1 / B2.
4. Apply the governance policies (`mcp-security-policy.yaml`) once the base paths are proven.
5. Apply the observability stack (`mcp-observability.yaml`) and confirm Forge's LLM Observability panel goes `available: true`.
6. Enable Bedrock model invocation logging and deploy `mcp-bedrock-token-shipper.yaml` (section 6) so real token counts appear alongside BNK's governance records.
7. Review [section 7](#7-future-capabilities) for the capabilities a fuller build would add — completing the double-checked path, identity enforcement at BNK, guardrail integration, and MCP-aware inspection.

## Manifest map

| File | Contains |
| --- | --- |
| `cluster.yaml` | awsbnkctl intent: VPC, EKS, BNK, host-device ENIs |
| `mcp-tool/` | The MCP finance tool: `mcp-server.py` + a Kustomize base that generates its ConfigMap |
| `gateway-deployment.yaml` | BNK `Gateway` (listeners 80/443, VIP) + the two `HTTPRoute`s |
| `mcp-security-policy.yaml` | Governance iRule, per-listener `BNKNetPolicy`, `F5BigFwPolicy`, `BNKSecPolicy` |
| `mcp-observability.yaml` | `llm-egress` namespace, Loki, `bnkgov-collector` Fluent Bit DaemonSet |
| `mcp-bedrock-token-shipper.yaml` | IRSA ServiceAccount + shipper that pulls Bedrock token counts into Loki |
| `external-agent.py` | Stranger-path client (run from inside the VPC) |
| `scripts/setup-agentcore-network.sh` | SGs, SG-to-SG ingress, private Route 53 zone |
