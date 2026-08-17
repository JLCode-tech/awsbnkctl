# AgentCore + F5 BNK: Securing the AI Action Path

## The Challenge: External AI Agents Interacting with Internal AWS Tooling

In modern GenAI architectures, AI agents often run outside of your direct AWS perimeter (e.g., in a SaaS platform, a 3rd-party orchestrator, or a user's local rig). These external agents frequently need to invoke **Model Context Protocol (MCP)** tools hosted securely within your AWS infrastructure to perform "actions" or retrieve sensitive context.

The challenge is providing these external agents secure, governed, and deeply inspected access to your internal MCP tools *without* exposing the tools directly to the public internet or relying solely on rudimentary API keys.

## The Solution: F5 BNK as the AI Security Gateway

This demo showcases how **F5 BIG-IP Next for Kubernetes (BNK)**, provisioned automatically via `awsbnkctl`, solves this challenge. BNK sits at the edge of the EKS cluster and acts as an intelligent, L7-aware AI Security Gateway for inbound MCP traffic.

### Architecture

1.  **External AI Agent:** An agent running outside the AWS VPC (simulated by our EC2 Jump Host).
2.  **F5 BNK (Ingress Data Plane):** Deployed natively in EKS, attached to multiple AWS ENIs (External/Internal) for high-performance data path routing. It terminates the inbound traffic, applies security policies, and load balances to the internal services.
3.  **Internal MCP Tool:** A simple HTTP-based MCP server (`mcp-financial-tool`) deployed in the EKS cluster, exposing a `/v1/mcp/forecast` endpoint.

### Flow of Execution

1.  **Inbound Request:** The external agent makes an HTTP request to the MCP tool's public endpoint (`bnk-ingress.aws.corp`).
2.  **BNK Interception:** The traffic hits the BNK data plane's external IP address (`10.0.10.100` in our data subnet).
3.  **L7 Routing (Gateway API):** BNK inspects the `Host` header and path. Using the modern Kubernetes **Gateway API** (`Gateway` and `HTTPRoute` resources), BNK dynamically routes traffic destined for `bnk-ingress.aws.corp/v1/mcp/forecast` to the correct internal pod.
4.  **Secure Delivery:** BNK forwards the sanitized request to the internal MCP tool.
5.  **Action Execution:** The MCP tool processes the request and returns the result (e.g., `{"forecast": "Q3 Revenue expected to increase by 15%", "status": "bullish"}`).
6.  **Response:** BNK sends the response back to the external agent.

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

Finally, we deploy the MCP tool and the Gateway API resources to expose it through BNK.

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

---
# The HTTPRoute maps the specific host/path to the MCP Tool service
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: mcp-financial-route
  namespace: default
spec:
  parentRefs:
  - name: bnk-agentcore-demo-gateway
  hostnames:
  - "bnk-ingress.aws.corp"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v1/mcp/forecast
    backendRefs:
    - name: mcp-financial-tool
      port: 80
```

## Validating the Setup

We can simulate the external AI agent querying the MCP tool by sending a request from our EC2 jump host to the BNK external IP:

```bash
curl -v -H 'Host: bnk-ingress.aws.corp' http://10.0.10.100/v1/mcp/forecast
```

**Result:**
```json
{"forecast": "Q3 Revenue expected to increase by 15%", "status": "bullish"}
```

The request successfully traverses the external network, is intercepted and routed by F5 BNK using the Gateway API, reaches the internal MCP tool, and the response is securely returned to the caller.
