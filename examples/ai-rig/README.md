# AI Inference Rig Topology

This directory contains the `cluster.yaml` intent file for standing up a **BNK Load-Balancer + GPU Inference** rig on EKS.

## Architecture

This topology is specifically designed to run high-performance AI inference workloads (like vLLM) behind F5 BNK. 

It deploys two distinct EKS node groups:
1. **BNK Control Node Group:** 3x `m6i.4xlarge` nodes dedicated to running the F5 TMM data plane and dSSM quorum.
2. **GPU Inference Node Group:** A dedicated `g5.xlarge` (NVIDIA A10G) node group explicitly tainted (`nvidia.com/gpu=present:NoSchedule`) to isolate AI workloads like vLLM. 

### External-Only Pattern
The BNK instance uses the `external-only` pattern. TMM gets a single ingress interface, and the GPU inference pods reach the backend over the CNI. The BNK VIP acts as the secure entry point for AI traffic.

### Managed SageMaker Endpoint (Optional)
The configuration includes an optional `ai.sagemaker` block to provision a disposable SageMaker LMI (vLLM) endpoint (e.g., Llama 3 8B or Qwen 32B) for managed inference benchmarks. This is created on `up` and fully destroyed on `down` to optimize costs.

## Usage

1. Copy the example configuration:
   ```bash
   cp cluster.yaml my-cluster.yaml
   # Ensure farArchive and jwt paths point to your real F5 credentials.
   ```

2. Provision the cluster (Requires `HF_TOKEN` if deploying gated SageMaker models):
   ```bash
   HF_TOKEN=$(cat .hf_token) awsbnkctl up --config my-cluster.yaml
   ```

3. Clean up (Ensures the GPU nodes and SageMaker endpoints are destroyed):
   ```bash
   awsbnkctl down --config my-cluster.yaml --yes
   ```

## Cost & teardown

> [!IMPORTANT]
> **Cost warning:** roughly **$6/hour** with the SageMaker endpoint enabled —
> about twice the plain BNK topologies, because of the GPU node and the managed
> endpoint.

Billable while up, at approximate `ap-southeast-2` on-demand rates (excluding
data transfer and EBS):

| Component | Qty | Approx. $/hr |
| --- | --- | --- |
| `m6i.4xlarge` BNK worker | 3 | 2.80 |
| EKS control plane | 1 | 0.10 |
| NAT gateway | 1 | 0.06 |
| `c6i.2xlarge` load-generator jumphost | 1 | 0.45 |
| `g5.xlarge` GPU inference node | 1 | 1.30 |
| SageMaker `ml.g5.xlarge` endpoint | 1 | 1.50 |

The jumphost is deliberately non-burstable: a `t3.small` throttles on CPU
credits and becomes the bottleneck, which makes benchmark numbers
irreproducible.

Destroy everything when you are done:

```bash
awsbnkctl down --config my-cluster.yaml --yes
```

`down` deletes the SageMaker Endpoint, EndpointConfig and Model in reverse order
so no managed inference bills between sessions. The endpoint is the single
largest line item — confirm it is gone before you walk away:

```bash
aws sagemaker list-endpoints --region ap-southeast-2
```
