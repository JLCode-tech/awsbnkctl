# Demo AI — BNK Protocol Demo + SageMaker AI Rig

> [!IMPORTANT]
> **Cost Warning:** This is a chargeable footprint (~$12–13/hr). The SageMaker `ml.g6.12xlarge` endpoint accounts for ~$7–8/hr. Remember to tear down after use!

The `demo-ai` topology extends the standard full cluster demo by adding an **AI inference rig**. It is the one AI example: the former `ai-rig` (Llama-3-8B on `ml.g5.xlarge`, no protocol demos) lives on here as a commented alternative in `cluster.yaml` — swap the SageMaker model lines and set `demo.enabled: false` to get that leaner, ~$6/hr rig. It provides:
1. **GPU Node Group:** `g5.xlarge` for in-cluster vLLM.
2. **SageMaker LMI Endpoint:** Disposable managed endpoint (defaults to Qwen2.5-32B-Instruct) created on `up` and destroyed on `down`.

## Prerequisites

- **AWS Account:** Credentials configured.
- **F5 Supply-Chain Files:** `./cne_pull_64.json` and `./license.jwt`.
- **Hugging Face Token:** Saved in a gitignored `.hf_token` file.
- **BNK Forge:** A reachable bnk-forge instance with password provided via `AWSBNKCTL_FORGE_PASSWORD`.
- **Quotas:** Standard demo quotas + 1-node `g5.xlarge` and SageMaker endpoint capacity.

## Quick Start

### 1. Provision
```bash
# Validate (includes GPU-fit preflight for SageMaker)
awsbnkctl validate examples/demo-ai/cluster.yaml

# Provision (pass HF token and forge password inline)
HF_TOKEN=$(cat .hf_token) AWSBNKCTL_FORGE_PASSWORD=admin123 \
  awsbnkctl up --config examples/demo-ai/cluster.yaml
```
> [!NOTE]
> The SageMaker endpoint is created asynchronously. Ensure it reaches `InService` before benchmarking using `aws sagemaker describe-endpoint`.

### 2. Run Protocol Demos
```bash
awsbnkctl demo run --all --config examples/demo-ai/cluster.yaml
```

### 3. Teardown
```bash
awsbnkctl down --config examples/demo-ai/cluster.yaml --yes
```

## BGP peering with AWS Route Server

`cluster.yaml` sets `bnk.bgp: true`, so `up` opens TCP 179 (BGP) and UDP 3784
(BFD) from the external subnet (`10.0.10.0/24`) on the data-plane security group
and on the external `F5SPKVlan`. That makes TMM's external SelfIP
(`10.0.10.240`) reachable as a BGP peer. Nothing peers until you create a Route
Server endpoint in that subnet and apply [`bgp-route-server.yaml`](bgp-route-server.yaml),
replacing its `10.0.10.31` placeholder with the endpoint's address. The full
procedure, verification and teardown order are in
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md); state for this
cluster lives in `.awsbnkctl/bnk-demo-ai/`. BGP is optional here: the Gateway VIP
(`10.0.10.100`) is reachable inside the VPC without it, because the
cne-controller assigns it as a secondary IP on TMM's external ENI.

> [!NOTE]
> A Route Server endpoint bills about $0.75/hour and blocks deletion of the
> external subnet. Remove it before `awsbnkctl down`.

## Scenarios

All 15 scenarios run here with `ai-inference-e2e` on the real GPU node group (`HF_TOKEN`). Demo mode is on, so `diameter`, `http2` and `ingress-migration` run too; `bigip-cis` does not (no `bigipVE:` block). Run `core-file-collection` last, and treat `egress-snat` the same way on this dual-interface cluster.

```bash
awsbnkctl scenarios list
awsbnkctl scenarios run http-routing-e2e -f examples/demo-ai/cluster.yaml
awsbnkctl scenarios run --all -f examples/demo-ai/cluster.yaml
```

Which scenario needs what, and the VIP each one owns, is in
[`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).


## Cost & teardown

The most expensive topology in `examples/`. Approximate `ap-southeast-2`
on-demand rates while up, excluding data transfer and EBS:

| Component | Qty | Approx. $/hr |
| --- | --- | --- |
| SageMaker `ml.g6.12xlarge` endpoint | 1 | 7.50 |
| (alternative) SageMaker `ml.g5.xlarge` endpoint, Llama-3-8B | 1 | 1.50 |
| `m6i.4xlarge` BNK worker | 3 | 2.80 |
| `g5.xlarge` GPU inference node | 1 | 1.30 |
| `c6i.4xlarge` load-generator jumphost | 1 | 0.90 |
| EKS control plane | 1 | 0.10 |
| NAT gateway | 1 | 0.06 |

`down` deletes the SageMaker Endpoint, EndpointConfig and Model in reverse order,
then the GPU node group, so no AI infrastructure bills between sessions. The
endpoint alone is over half the hourly cost — confirm it is gone:

```bash
aws sagemaker list-endpoints --region ap-southeast-2
```

## Proxy Shootout (Advanced)
A manual shootout comparing BNK vs HAProxy vs Envoy AI Gateway. All proxies forward to a shared SigV4 hop that rewrites the path and signs requests before sending to SageMaker. 
> [!TIP]
> Teardown LoadBalancer Services **before** running `awsbnkctl down` to prevent AWS NLB leaks.
