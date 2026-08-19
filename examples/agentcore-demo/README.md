# Governing the Agentic Action Path: AWS AgentCore + F5 BNK

## Overview

As enterprises accelerate moving AI agents into production, the fundamental challenge shifts from *building* the agent to *securing its reach*. 

**Amazon Bedrock AgentCore** provides the scalable, secure framework to run your agents natively in AWS (handling reasoning, tool discovery, memory, and orchestration). However, when the agent decides to invoke an external API, a corporate data source, or an on-premise MCP server, it crosses a critical trust boundary. 

This is where **F5 BIG-IP Next for Kubernetes (BNK)** operates. BNK does not replicate the AI agent's orchestration; instead, it provides the distributed networking, security, tenancy, and token-governance services exactly where the private traffic meets the cluster. 

**This demonstration stands up a dedicated AWS EKS environment where an AgentCore agent invokes an MCP tool, with the entire data path governed by F5 BNK.**

---

## The Joint Architecture

The topology proves the **"Four Moves"** of the agentic operating model across **three distinct scenarios**:

**Test A — Egress: AWS AgentCore to Internal EKS Tool**
1.  **Build:** The AI Agent runs securely inside Bedrock AgentCore.
2.  **Connect:** We expose a Kubernetes-hosted MCP Tool Server on a private Route 53 name (`bnk-ingress.bnk-demo.internal`) resolvable inside the VPC.
3.  **Deploy:** F5 BNK sits at the internal EKS network edge, listening on the BNK VIP.
4.  **Govern:** BNK enforces L7 policies (routing, ACLs, per-client request rate limits) on traffic entering the cluster.

**Test B1 — Egress Control: AWS AgentCore Agent → Internal EKS Tool via AgentCore Gateway**
1.  **Build:** An agent running inside Bedrock AgentCore needs to call a corporate tool hosted in the EKS cluster.
2.  **Connect:** The agent routes its outbound tool call through the AWS AgentCore Gateway, which forwards to the F5 BNK VIP.
3.  **Govern (Semantic):** AgentCore Gateway validates identity (JWT/SigV4), applies Cedar per-tool authorization, and runs guardrails.
4.  **Govern (Network):** F5 BNK intercepts the forwarded traffic, enforcing TLS, ACLs, quotas, and L7 routing *before* it reaches the tool pod.

**Test B2 — Ingress: External Agent to EKS-Hosted Tool**
1.  **Build:** An unmanaged external agent (e.g., running locally, in Azure, or GCP) attempts to reach a corporate tool hosted in the EKS cluster.
2.  **Connect:** The tool is exposed via public/private Ingress through F5 BNK.
3.  **Deploy:** F5 BNK sits at the VPC edge, acting as the secure front door to the EKS cluster.
4.  **Govern:** Before the external agent can reach the EKS tool, BNK intercepts the traffic to rate limit the agent per source, apply the L4 firewall policy, and prevent unauthorized actions from entering the corporate network. Note: AgentCore Gateway is *not* in this path.

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

Because both platforms use the term "Gateway," it is important to understand their distinct roles in a Defense-in-Depth AI architecture. Below are the architectural data paths for the three test scenarios.

#### Test A — Egress: Agent → Tool in your cluster

```text
    ┌─────────┐
    │  Agent  │  MCP tool call
    └────┬────┘
         │
         ▼
    ╔══════════════════════════════════════╗   ◄── CHECKPOINT 1 (semantic)
    ║      AgentCore Gateway  (AWS)        ║
    ║  ┌────────────────────────────────┐  ║   • JWT: aud / client_id  →  admit?
    ║  │ Inbound auth (JWT / SigV4)     │  ║   • Cedar: may THIS principal
    ║  ├────────────────────────────────┤  ║     call THIS tool?
    ║  │ Cedar policy engine            │  ║   • Guardrails: violence/hate/
    ║  │  · contentFilter               │  ║     jailbreak/PII detected?
    ║  │  · promptAttack                │  ║       → forbid = 403, no mask
    ║  │  · sensitiveInformation        │  ║
    ║  ├────────────────────────────────┤  ║
    ║  │ MCP ──translate──► HTTP        │  ║
    ║  └────────────────────────────────┘  ║
    ╚══════════════════┬═══════════════════╝
                       │  plain HTTPS  (MCP framing now invisible)
                       ▼
    ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
          Kubernetes cluster boundary
    │                  ▼                                     │
       ╔══════════════════════════════════════╗  ◄── CHECKPOINT 2 (network)
    │  ║          F5 BNK  (N/S gateway)       ║              │
       ║  ┌────────────────────────────────┐  ║  • F5BigFwPolicy: ACL / trusted src
    │  ║  │ F5BigFwPolicy   (ACL, L3/L4)   │  ║  • F5BigDdosGlobal: L4 flood vectors
       ║  │ F5BigDdosGlobal (L4 DDoS)      │  ║  • TLS termination
    │  ║  │ TLS termination                │  ║  • HTTPRoute: path match,          │
       ║  │ HTTPRoute + URLRewrite         │  ║    URLRewrite, header mod
    │  ║  │ RequestHeaderModifier          │  ║  • LB to pod                       │
       ║  │ Load balance                   │  ║
    │  ║  └────────────────────────────────┘  ║  ✗ no WAF  ✗ no SNI                │
       ╚══════════════════┬═══════════════════╝  ✗ no client mTLS  ✗ no MCP parsing
    │                     ▼                                   │
                ┌───────────────────┐
    │           │  mcp-server pod   │                         │
                └───────────────────┘
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

#### Test B1 — Egress control: your agent → AWS
  
```text
    ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
         Kubernetes cluster
    │   ┌─────────────┐                              │
        │  Agent pod  │
    │   └──────┬──────┘                              │
               │
    │          ▼                                     │
        ╔══════════════════════════════╗   ◄── CHECKPOINT 1 (network, outbound)
    │   ║   F5 BNK  (egress)           ║             │
        ║  · ACL: which externals may  ║   "may this workload reach
    │   ║    this workload reach?      ║    api.bedrock-agentcore...?"
        ║  · token governance quota    ║   "has this user burned
    │   ║    (429 on exceed)           ║    their token budget?"        │
        ║  · connection limits         ║
    │   ╚══════════════┬═══════════════╝             │
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
                       ▼
        ╔══════════════════════════════╗   ◄── CHECKPOINT 2 (semantic)
        ║  AgentCore Gateway  (AWS)    ║
        ║  · JWT admission             ║   BNK gates the wire FIRST,
        ║  · Cedar per-tool authz      ║   AgentCore gates the intent SECOND
        ║  · guardrail categories      ║
        ╚══════════════┬═══════════════╝
                       ▼
              ┌─────────────────┐
              │  target / tool  │
              └─────────────────┘
```

#### Test B2 — Ingress: external agent → your tools

```text
    ┌──────────────────┐
    │  External agent  │   (unmanaged, untrusted)
    └────────┬─────────┘
             │
    ┌ ─ ─ ─ ─┼─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
             ▼   Kubernetes cluster
    │  ╔══════════════════════════════╗              │   ◄── ONLY CHECKPOINT
       ║   F5 BNK  (ingress)          ║
    │  ║  · F5BigFwPolicy ACL         ║              │   AgentCore Gateway is
       ║  · F5BigDdosGlobal (L4)      ║                  NOT in this path.
    │  ║  · TLS termination           ║              │
       ║  · rate limiting             ║                  ⚠ no WAF, no client-cert
    │  ║  · HTTPRoute → pod           ║              │     auth, no MCP inspection
       ╚══════════════┬═══════════════╝                  → tool-level authz is
    │                 ▼                              │     YOUR app's job here
          ┌───────────────────┐
    │     │  tool / mcp pod   │                      │
          └───────────────────┘
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

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

### Step 1 — Test A / B1: AgentCore agent → BNK → MCP tool

```bash
cd examples/agentcore-demo/agent
AWS_PROFILE=<profile> ./node_modules/.bin/agentcore invoke \
  --runtime FinanceAgentV2Agent --target demo-v2 --prompt "forecast AMZN"
```

Expect a forecast table with an "Expected Growth" percentage — that number is
generated by the MCP tool pod, so seeing it proves the whole path. First run
after a cold start can take ~90 s.

> The runtime's MCP client calls `http://bnk-ingress.bnk-demo.internal/v1/mcp/forecast`
> directly (port 80). The `FinanceAgentV2` **harness** additionally defines an
> `agentcore_gateway` tool, `BnkGatewayTool`, pointing at
> `https://bnk-ingress.bnk-demo.internal/v1/mcp/forecast` — which is why the
> Gateway carries a plain-HTTP listener on 443: AgentCore Gateway targets
> require an `https://` URL, but the hop into the cluster is plaintext here.

Run it three times in a row. All three must succeed — the rate limit is
per-client-IP and one invoke uses only a few requests of the budget.

### Step 2 — Test B2: external agent → BNK → MCP tool

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

## Troubleshooting

> Test B2 requires the external caller to have network reachability to the private BNK VIP. The demo does not expose the BNK VIP to the public internet.

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

## Manifest map

| File | Contains |
| --- | --- |
| `cluster.yaml` | awsbnkctl intent: VPC, EKS, BNK, host-device ENIs |
| `mcp-tool/` | The MCP finance tool: `mcp-server.py` + a Kustomize base that generates its ConfigMap |
| `gateway-deployment.yaml` | BNK `Gateway` (listeners 80/443, VIP) + the two `HTTPRoute`s |
| `mcp-security-policy.yaml` | Governance iRule, per-listener `BNKNetPolicy`, `F5BigFwPolicy`, `BNKSecPolicy` |
| `mcp-observability.yaml` | `llm-egress` namespace, Loki, `bnkgov-collector` Fluent Bit DaemonSet |
| `mcp-bedrock-token-shipper.yaml` | IRSA ServiceAccount + shipper that pulls Bedrock token counts into Loki |
| `external-agent.py` | Test B2 client (run from inside the VPC) |
| `scripts/setup-agentcore-network.sh` | SGs, SG-to-SG ingress, private Route 53 zone |
