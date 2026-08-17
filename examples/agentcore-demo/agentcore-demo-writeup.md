# AgentCore + F5 BNK: Securing the AI Action Path

## The Challenge: External AI Agents Interacting with Internal AWS Tooling

In modern GenAI architectures, AI agents often run outside of your direct AWS perimeter (e.g., in a SaaS platform, a 3rd-party orchestrator, or a user's local rig). These external agents frequently need to invoke **Model Context Protocol (MCP)** tools hosted securely within your AWS infrastructure to perform "actions" or retrieve sensitive context.

The challenge is providing these external agents secure, governed, and deeply inspected access to your internal MCP tools *without* exposing the tools directly to the public internet or relying solely on rudimentary API keys.

## The Solution: F5 BNK as the AI Security Gateway

This demo showcases how **F5 BIG-IP Next for Kubernetes (BNK)**, provisioned automatically via `awsbnkctl`, solves this challenge. BNK sits at the edge of the EKS cluster and acts as an intelligent, L7-aware AI Security Gateway for inbound MCP traffic.

### Architecture

1.  **AWS Bedrock AgentCore:** The fully managed Agent runtime (deployed via the AgentCore CLI) living outside our EKS cluster boundary. It acts as the "Client".
2.  **F5 BNK (Ingress Data Plane):** Deployed natively in EKS, attached to multiple AWS ENIs (External/Internal) for high-performance data path routing. It terminates the inbound traffic, applies security policies, and load balances to the internal services.
3.  **Internal MCP Tool:** A real Python FastMCP server (`mcp-financial-tool`) deployed in the EKS cluster, exposing financial forecasting tools.

### Flow of Execution

1.  **Agent Action:** The AWS Bedrock AgentCore agent reasons that it needs financial data and attempts to invoke its configured MCP tool via the public endpoint (`bnk-ingress.aws.corp`).
2.  **BNK Interception:** The traffic hits the BNK data plane's external IP address (`10.0.10.100` in our data subnet).
3.  **L7 Routing (Gateway API):** BNK inspects the `Host` header and path. Using the modern Kubernetes **Gateway API** (`Gateway` and `HTTPRoute` resources), BNK dynamically routes traffic destined for `bnk-ingress.aws.corp/v1/mcp/forecast` to the correct internal pod.
4.  **Secure Delivery:** BNK forwards the sanitized request to the internal Python FastMCP tool.
5.  **Action Execution:** The MCP tool executes the Python tool logic (e.g. `get_forecast`) and returns the result (e.g., `Q3 Revenue expected to increase by 15%`).
6.  **Response:** BNK sends the response back to the AgentCore runtime, which uses the data to answer the user's prompt.

## Provisioning the Demo

The entire infrastructure and security gateway are provisioned using `awsbnkctl`.

### 1. AWS Infrastructure

`awsbnkctl up` automatically creates the foundational AWS resources:
*   VPC, Subnets (Control Plane, External Data, Internal Data).
*   EKS Cluster (`bnk-agentcore-demo`).
*   Node Groups (using high-performance `m6i.4xlarge` instances suitable for data-plane workloads).
*   Secondary ENIs attached to the EKS nodes for direct, high-throughput network access.

### 2. F5 BNK Deployment

The tool then deploys the F5 BNK software stack into the cluster:
*   Installs the F5 Lifecycle Operator (FLO).
*   Configures the CNE (Cloud Native Environment) instance.
*   Binds the F5 SPK VLANs to the secondary AWS ENIs.
*   Spins up the high-performance TMM (Traffic Management Microkernel) pods to handle the data plane.

### 3. Application & Gateway API Configuration

We deploy the Python FastMCP tool and the Gateway API resources to expose it through BNK.

```yaml
# The Gateway binds to the BNK GatewayClass and listens on port 80
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: bnk-agentcore-demo-gateway
  namespace: default
spec:
  gatewayClassName: bnk-agentcore-demo-gatewayclass
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: All
  addresses:
  - type: IPAddress
    value: 10.0.10.100 # The External VIP
```

### 4. Deploying the AgentCore Agent

Using the `agentcore` CLI, you deploy the agent defined in `examples/agentcore-demo/agent/`. 
The `agentcore.yaml` connects the agent directly to the BNK gateway IP:

```yaml
tools:
  - name: internal-finance-mcp
    type: mcp
    config:
      endpoint: "http://bnk-ingress.aws.corp/v1/mcp/forecast"
      transport: sse
```

Once deployed (`agentcore deploy`), the AWS Bedrock AgentCore environment is actively secured and governed by F5 BNK on every tool invocation!

### AgentCore CLI Project

The agent configuration in this demo was generated entirely using the official **AgentCore CLI**.

Instead of writing manual SDK code, the agent is defined declaratively using an `AgentCoreProjectSpec` and a **Harness**:

1.  **Harness:** `examples/agentcore-demo/agent/app/FinanceAgent/harness.json` defines the `FinanceAgent` using Anthropic Claude 3.5 Sonnet on Bedrock.
2.  **Tools:** The agent natively references the `remote_mcp` tool (`http://bnk-ingress.aws.corp/v1/mcp/forecast`) configured via the CLI, ensuring traffic flows correctly through the F5 BNK Gateway.
3.  **Deployment:** A user simply runs `agentcore deploy` in the `agent/` folder to synthesize the AWS CDK construct and provision the Agent into the Bedrock ecosystem.
