# Governing the Agentic Action Path: F5 BNK + AWS Bedrock AgentCore

## The Challenge: External Agents Accessing AWS AgentCore
In a multi-cloud or distributed AI architecture, you often have **external AI agents** (running outside of AWS, such as in another cloud, a local datacenter, or an external SaaS orchestrator). These external agents frequently need to coordinate with or invoke your internal **Amazon Bedrock AgentCore** agents using the Agent-to-Agent (A2A) protocol.

The challenge is securely governing this inbound traffic. How do you allow an external, non-AWS agent to reach your Bedrock AgentCore endpoint while enforcing strict token counting, L7 API security, tenant attribution, and AI risk governance?

## The Solution: F5 BNK as the Ingress AI Gateway
Instead of exposing the Bedrock AgentCore API directly to the internet, we route the inbound external agent traffic through an **F5 BIG-IP Next for Kubernetes (BNK)** gateway deployed on AWS EKS. 

BNK acts as a high-performance, L7-aware AI Security Gateway that governs the traffic *before* it reaches the AWS Bedrock service.

### Architecture

1.  **External AI Agent (The Client):** An agent running outside AWS that needs to communicate with the AWS AgentCore agent.
2.  **F5 BNK (Ingress Data Plane):** Deployed natively in EKS, attached to AWS ENIs for high-performance data path routing. It terminates the inbound A2A traffic from the external agent, applies security policies (like Token Counting via dSSM and API firewalling), and load balances.
3.  **Amazon Bedrock AgentCore (The Target):** The fully managed Agent runtime hosted by AWS. BNK securely proxies the sanitized traffic to the Bedrock AgentCore runtime (`bedrock-agent-runtime.ap-southeast-2.amazonaws.com`).

### Flow of Execution

1.  **Inbound Invocation:** The external non-AWS agent attempts to invoke the AWS AgentCore agent via our public EKS endpoint (`bnk-ingress.aws.corp/v1/agentcore/invoke`).
2.  **BNK Interception:** The traffic hits the BNK data plane's external IP address (`10.0.10.100`).
3.  **Governance & Inspection:** BNK inspects the request. It can apply Identity scoping, count inference tokens, enforce tenant rate limits, and scrub the payload.
4.  **L7 Routing (Gateway API):** Using the Kubernetes Gateway API (`Gateway` and `HTTPRoute` resources), BNK dynamically routes the traffic to a Kubernetes `ExternalName` service that points directly to the Amazon Bedrock AgentCore runtime endpoint.
5.  **AgentCore Execution:** AWS Bedrock AgentCore receives the governed request, performs the reasoning/action, and returns the response.
6.  **Response & Telemetry:** BNK returns the response to the external agent while emitting usage telemetry (tokens consumed, latency) to your observability stack.

## Provisioning the Demo

### 1. AWS Infrastructure & BNK Deployment
`awsbnkctl up` automatically creates the foundational AWS resources (VPC, Subnets, EKS Cluster, ENIs) and deploys the F5 BNK software stack into the cluster.

### 2. Application & Gateway API Configuration
We deploy the Kubernetes Gateway API resources to configure BNK, along with an `ExternalName` service representing the AWS Bedrock AgentCore endpoint.

**External Service (`mcp-tool-deployment.yaml`):**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: bedrock-agentcore-runtime
  namespace: default
spec:
  type: ExternalName
  externalName: bedrock-agent-runtime.ap-southeast-2.amazonaws.com
```

**Gateway Route (`gateway-deployment.yaml`):**
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: external-agent-route
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
        value: /v1/agentcore/invoke
    backendRefs:
    - name: bedrock-agentcore-runtime
      port: 443
```

By pointing the external agent to `http://bnk-ingress.aws.corp`, all agent-to-agent communication flowing back into AWS is fully secured and governed by F5 BNK!
