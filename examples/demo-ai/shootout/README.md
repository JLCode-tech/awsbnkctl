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

### 3. Teardown
```bash
# Safely tears down NLBs, EIP, cluster, and IAM policies
bash examples/demo-ai/shootout/teardown.sh
```

## Key Architectural Decisions
- **k8s 1.31:** Required for Envoy AI Gateway CRDs.
- **Shared SigV4 Hop:** Ensures apples-to-apples proxy comparison.
- **Envoy Timeout:** Increased to 600s to support long LLM responses.
