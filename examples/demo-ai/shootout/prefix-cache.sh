#!/usr/bin/env bash
# prefix-cache.sh — show that prompt prefix caching behind BNK cuts TTFT.
#
# Runs two aiperf workloads of the same total prompt length through the BNK
# Gateway VIP and compares them:
#   baseline       every prompt unique (ISL 5000, OSL 128)
#   prefix-shared  80 % shared prefix (4000 prefix tokens from a pool of 20,
#                  1000 unique tokens), same concurrency, same seed
# The model servers are scraped before and after each run, so the second run
# also reports prefix_cache_hit_rate. Both runs are pushed to Forge; the
# metrics land in --genai-out files and `awsbnkctl benchmark ingest` prints the
# comparison and fails when TTFT p50 did not drop by --expect-ttft-drop percent.
#
# Prereqs: bringup.sh done (or any cluster with a BNK-fronted inference leg),
#   AWS_PROFILE and AWSBNKCTL_FORGE_PASSWORD exported, awsbnkctl in the repo root.
#
# Run from the repo root:
#   bash examples/demo-ai/shootout/prefix-cache.sh                 # SageMaker leg (no scrape)
#   LEG=vllm bash examples/demo-ai/shootout/prefix-cache.sh        # in-cluster vLLM leg, scraped
#   CONCURRENCY=50 REQUESTS=250 EXPECT_DROP=30 bash examples/demo-ai/shootout/prefix-cache.sh
set -euo pipefail

CFG="${CFG:-examples/demo-ai/cluster.yaml}"
yaml_scalar() { awk -v key="$2" '$0 ~ "^" key ":" { sub(/#.*/, ""); sub(/^[^:]*:[ \t]*/, ""); gsub(/["'"'"']/, ""); print; exit }' "$1"; }
CLUSTER="${CLUSTER:-$(yaml_scalar "$CFG" "  name")}"
REGION="${REGION:-$(yaml_scalar "$CFG" "  region")}"
: "${CLUSTER:?could not read metadata.name from $CFG}"; : "${REGION:?could not read metadata.region from $CFG}"
: "${AWS_PROFILE:?set AWS_PROFILE}"; : "${AWSBNKCTL_FORGE_PASSWORD:?set AWSBNKCTL_FORGE_PASSWORD}"

LEG="${LEG:-sagemaker}"            # sagemaker | vllm
CONCURRENCY="${CONCURRENCY:-25}"
REQUESTS="${REQUESTS:-$((CONCURRENCY * 5))}"
ISL="${ISL:-5000}"
OSL="${OSL:-128}"
PREFIX_LEN=$((ISL * 8 / 10))
UNIQUE_LEN=$((ISL - PREFIX_LEN))
SEED="${SEED:-42}"
EXPECT_DROP="${EXPECT_DROP:-20}"
OUT_DIR="${OUT_DIR:-.awsbnkctl/${CLUSTER}/reports/prefix-cache}"
mkdir -p "$OUT_DIR"

STATE=".awsbnkctl/${CLUSTER}/state.env"
JH=$(grep '^JUMPHOST_INSTANCE_ID=' "$STATE" | cut -d= -f2-)
SRC=$(grep '^JUMPHOST_BNK_EXT_ENI_IP=' "$STATE" | cut -d= -f2-)
EXT_CIDR="$(awk '/^  dataPath:/ { d = 1 } d && /^    external:/ { e = 1 } d && e && /cidr:/ { sub(/#.*/, ""); print $2; exit }' "$CFG")"

SCRAPE=()
case "$LEG" in
  sagemaker)
    VIP="${VIP:-${EXT_CIDR%.*}.130}"; HOST="${HOST:-sagemaker.bnk.local}"
    MODEL="${MODEL:-llama3}"; TOKENIZER="${TOKENIZER:-Qwen/Qwen2.5-32B-Instruct}"
    echo "leg=sagemaker: the LMI container exposes no /metrics, prefix_cache_hit_rate stays n/a; TTFT is the evidence" ;;
  vllm)
    VIP="${VIP:-$(kubectl --kubeconfig ".awsbnkctl/${CLUSTER}/kubeconfig" -n awsbnkctl-scn-aiinference get gateway -o jsonpath='{.items[0].spec.addresses[0].value}')}"
    HOST="${HOST:-awsbnkctl-aiinference.local}"
    MODEL="${MODEL:-llama3}"; TOKENIZER="${TOKENIZER:-NousResearch/Meta-Llama-3-8B-Instruct}"
    SCRAPE=(--metrics-pod-selector app=vllm --metrics-namespace awsbnkctl-scn-aiinference --metrics-port 8000) ;;
  *) echo "LEG must be sagemaker or vllm" >&2; exit 2 ;;
esac

B() {
  ./awsbnkctl benchmark run -f "$CFG" --instance-id "$JH" --region "$REGION" --source-ip "$SRC" \
    --vip "$VIP" --host-header "$HOST" --proxy f5-bnk --model "$MODEL" --tokenizer "$TOKENIZER" \
    --forge-pass "$AWSBNKCTL_FORGE_PASSWORD" --concurrency "$CONCURRENCY" --num-requests "$REQUESTS" \
    --osl "$OSL" --random-seed "$SEED" --stream --timeout 20m "${SCRAPE[@]}" "$@"
}

echo "==> 1/3 baseline: ISL ${ISL} unique, c=${CONCURRENCY}, ${REQUESTS} requests, VIP ${VIP}"
B --isl "$ISL" --run-label prefix-baseline --genai-out "$OUT_DIR/baseline.json"

echo "==> 2/3 prefix-shared: ${PREFIX_LEN} shared + ${UNIQUE_LEN} unique tokens, 20 prefixes"
B --isl "$UNIQUE_LEN" --prefix-prompt-length "$PREFIX_LEN" --num-prefix-prompts 20 \
  --run-label prefix-shared --genai-out "$OUT_DIR/prefix-shared.json"

echo "==> 3/3 compare (TTFT p50 must drop >= ${EXPECT_DROP}%)"
./awsbnkctl benchmark ingest "$OUT_DIR/baseline.json" "$OUT_DIR/prefix-shared.json" --expect-ttft-drop "$EXPECT_DROP"
