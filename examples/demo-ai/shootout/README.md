# Demo AI Shootout — One-Command Bring-Up / Teardown

> [!NOTE]
> Brings up the entire `bnk-demo-ai` infrastructure plus the 3-way proxy-shootout wiring (BNK vs HAProxy vs Envoy AI Gateway) targeting SageMaker.

This automates the manual wiring described in the `demo-ai` advanced section.

## Quick Start

### 1. Setup Environment
```bash
export AWS_PROFILE=<your-sso-profile>
export HF_TOKEN=$(cat .hf_token)
export AWSBNKCTL_FORGE_PASSWORD=admin123
```

### 2. Bring-Up
```bash
# Provisions cluster, demos, SageMaker, and shootout legs
bash examples/demo-ai/shootout/bringup.sh
```
Follow the printed `forge-benchmark` commands once the endpoint is `InService`.

### 3. Prefix cache
```bash
# baseline vs 80 % shared prefix through the BNK VIP; TTFT p50 must drop >= 20 %
bash examples/demo-ai/shootout/prefix-cache.sh
# in-cluster vLLM leg, scraped for prefix_cache_hit_rate (after `awsbnkctl scenarios run ai-inference-e2e`)
LEG=vllm bash examples/demo-ai/shootout/prefix-cache.sh
```
Both runs are pushed to Forge with `ttft_*`, `itl_*`, token throughput and `prefix_cache_hit_rate`.

### 4. Teardown
```bash
# Safely tears down NLBs, EIP, cluster, and IAM policies
bash examples/demo-ai/shootout/teardown.sh
```

## Key Architectural Decisions
- **k8s 1.34+ (cluster runs 1.35):** the Envoy AI Gateway CRDs use the `isIP` CEL function, which needs Kubernetes 1.31 or newer; the `awsbnkctl` floor of 1.34 already clears it.
- **Shared SigV4 Hop:** Ensures apples-to-apples proxy comparison.
- **Envoy Timeout:** Increased to 600s to support long LLM responses.
