# Demo AI — BNK Protocol Demo + SageMaker AI Rig

> [!IMPORTANT]
> **Cost Warning:** This is a chargeable footprint (~$12–13/hr). The SageMaker `ml.g6.12xlarge` endpoint accounts for ~$7–8/hr. Remember to tear down after use!

The `demo-ai` topology extends the standard full cluster demo by adding an **AI inference rig**. It provides:
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

## Proxy Shootout (Advanced)
A manual shootout comparing BNK vs HAProxy vs Envoy AI Gateway. All proxies forward to a shared SigV4 hop that rewrites the path and signs requests before sending to SageMaker. 
> [!TIP]
> Teardown LoadBalancer Services **before** running `awsbnkctl down` to prevent AWS NLB leaks.
