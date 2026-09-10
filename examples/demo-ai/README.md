# demo-ai — protocol demos plus AI inference

| | |
| --- | --- |
| Cluster name | `bnk-demo-ai` (state in `.awsbnkctl/bnk-demo-ai/`) |
| Region / pattern | `ap-southeast-2` / `dual-interface`, demo mode on |
| Footprint | 3 × m6i.4xlarge + 1 × g5.xlarge GPU node, EKS, NAT, c6i.4xlarge jumphost, SageMaker `ml.g6.12xlarge` endpoint |
| Cost | about **US$12/hour**; the SageMaker endpoint is US$7.50 of it |

`full-cluster` with three additions: a GPU node group that runs vLLM in the
cluster, the AWS Load Balancer Controller, and a disposable SageMaker endpoint
(Qwen2.5-32B by default) that `up` creates and `down` deletes. Demo mode is on,
so the protocol demos run here too.

**The lean rig.** For inference benchmarking without the protocol demos, set
`demo.enabled: false` and switch the SageMaker block to the commented
Llama-3-8B / `ml.g5.xlarge` alternative in `cluster.yaml`. That is about
US$6/hour and is what the former `ai-rig` example was.

## Prerequisites

- F5 credentials next to `cluster.yaml` (`cne_pull_64.json`, `license.jwt`).
- A Hugging Face token in `.hf_token` for the gated models.
- A reachable BNK Forge instance if you want benchmarks; its password via
  `AWSBNKCTL_FORGE_PASSWORD`.
- Quota for one `g5.xlarge` and one SageMaker `ml.g6.12xlarge` (or `ml.g5.xlarge`) endpoint.

## Run it

```bash
awsbnkctl validate examples/demo-ai/cluster.yaml          # includes the GPU-fit check for SageMaker
HF_TOKEN=$(cat .hf_token) AWSBNKCTL_FORGE_PASSWORD=<pw> \
  awsbnkctl up -f examples/demo-ai/cluster.yaml

aws sagemaker describe-endpoint --endpoint-name bnk-demo-ai-lmi --query EndpointStatus   # wait for InService
awsbnkctl demo run --all -f examples/demo-ai/cluster.yaml
awsbnkctl scenarios run ai-inference-e2e -f examples/demo-ai/cluster.yaml
```

## Scenarios and demos

All 15 scenarios run here, and `ai-inference-e2e` runs on the real GPU node
(export `HF_TOKEN`). `ai-token-counting` and `ai-semantic-cache` can point at
the vLLM backend for their data-path step. Demo mode gives you `diameter`,
`http2` and `ingress-migration`; `bigip-cis` needs the `bigipVE:` block, which
only `full-cluster` carries. Run `core-file-collection` last.
Details: [`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).

## Proxy shootout

`shootout/bringup.sh` builds a three-way comparison of BNK, HAProxy and Envoy AI
Gateway in front of the SageMaker endpoint (via a shared SigV4 hop) and prints
the Forge benchmark commands to run. It reads the cluster name, region and its
`.130` VIP from `cluster.yaml`. Run `shootout/teardown.sh` before `awsbnkctl
down`, otherwise the LoadBalancer Services leak NLBs.

## BGP

`bnk.bgp: true` opens the BGP/BFD ports; to peer, follow
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md) and apply
[`bgp-route-server.yaml`](bgp-route-server.yaml). Remove the Route Server
endpoint before `down`.

## Teardown

```bash
awsbnkctl down -f examples/demo-ai/cluster.yaml --yes
aws sagemaker list-endpoints --region ap-southeast-2       # must be empty
```

`down` deletes the SageMaker endpoint, its config and model, then the GPU node
group, so nothing bills between sessions. The endpoint is over half the hourly
cost; confirm it is gone.
