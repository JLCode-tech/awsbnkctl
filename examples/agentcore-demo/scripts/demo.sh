#!/bin/bash
# Guided walk-through of the BNK + AgentCore governance demo.
#
# Drives both live paths and narrates what each step proves, so the demo can be
# run by someone who did not build it.
#
#   AWS_PROFILE=<profile> ./scripts/demo.sh            # full run
#   AWS_PROFILE=<profile> ./scripts/demo.sh --quick    # skip agent invokes (~2 min)
#   AWS_PROFILE=<profile> ./scripts/demo.sh --check    # preflight only
#
# The BNK VIP is private, so stranger-path calls are driven on the jumphost over
# SSM. Agent invokes run locally against the AgentCore control plane.
#
# Nothing here mutates cluster config — it is read-and-exercise only.

set -uo pipefail

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-$DEMO_DIR/.awsbnkctl/bnk-agentcore-demo/kubeconfig}"

CLUSTER="${CLUSTER:-bnk-agentcore-demo}"
REGION="${REGION:-ap-southeast-2}"
VIP="${VIP:-10.0.10.100}"
INGRESS_HOST="${INGRESS_HOST:-bnk-ingress.bnk-demo.internal}"
RUNTIME="${RUNTIME:-FinanceAgentV2Agent}"
TARGET="${TARGET:-demo-v2}"
FORGE="${FORGE:-http://localhost:8000}"

# Demo tokens, matching mcp-tool/kustomization.yaml.
AGENT_TOKEN="${AGENT_TOKEN:-demo-agent-token-a7f3c1}"
EXTERNAL_TOKEN="${EXTERNAL_TOKEN:-demo-external-token-4b9e2d}"

QUICK=0; CHECK_ONLY=0
for arg in "$@"; do
  case "$arg" in
    --quick) QUICK=1 ;;
    --check) CHECK_ONLY=1 ;;
    -h|--help) sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

if [ -t 1 ]; then
  B=$'\033[1m'; DIM=$'\033[2m'; GRN=$'\033[32m'; RED=$'\033[31m'; YEL=$'\033[33m'; R=$'\033[0m'
else
  B=""; DIM=""; GRN=""; RED=""; YEL=""; R=""
fi

FAILED=0
say()  { printf '\n%s▏%s%s\n' "$B" "$1" "$R"; }
note() { printf '  %s%s%s\n' "$DIM" "$1" "$R"; }
ok()   { printf '  %s✓%s %s\n' "$GRN" "$R" "$1"; }
bad()  { printf '  %s✗%s %s\n' "$RED" "$R" "$1"; FAILED=$((FAILED+1)); }
warn() { printf '  %s!%s %s\n' "$YEL" "$R" "$1"; }

pause() { [ -t 0 ] && { printf '\n  %s[enter to continue]%s' "$DIM" "$R"; read -r _; } || true; }

need() { command -v "$1" >/dev/null 2>&1 || { echo "required tool missing: $1" >&2; exit 2; }; }

# ── preflight ────────────────────────────────────────────────────────────────
say "Preflight"
need aws; need kubectl; need python3
[ -n "${AWS_PROFILE:-}" ] || warn "AWS_PROFILE is unset; relying on ambient credentials"

if ! kubectl get gateway "$CLUSTER-gateway" -n default >/dev/null 2>&1; then
  bad "cannot reach the cluster, or the Gateway is missing. Is SSO current?"
  echo; echo "  try: aws sso login --profile \${AWS_PROFILE}"; exit 1
fi
PROGRAMMED=$(kubectl get gateway "$CLUSTER-gateway" -n default \
  -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null)
[ "$PROGRAMMED" = "True" ] && ok "BNK Gateway programmed on $VIP" || bad "Gateway not programmed"

IRULE_READY=$(kubectl get f5-big-cne-irules mcp-rate-limit-irule -n default \
  -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null)
[ "$IRULE_READY" = "True" ] && ok "governance iRule programmed on TMM" || bad "iRule not programmed"

POLICIES=$(kubectl get bnknetpolicies -n default --no-headers 2>/dev/null | wc -l | tr -d ' ')
[ "$POLICIES" -ge 2 ] && ok "$POLICIES listener-scoped BNKNetPolicies attached" \
  || bad "expected 2 BNKNetPolicies (one per listener), found $POLICIES"

TOOL_READY=$(kubectl get deploy mcp-financial-tool -n default \
  -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
[ "${TOOL_READY:-0}" -ge 1 ] && ok "MCP tool pod ready" || bad "MCP tool pod not ready"

OBS=$(kubectl get pods -n llm-egress --no-headers 2>/dev/null | grep -c Running)
[ "${OBS:-0}" -ge 3 ] && ok "observability stack up ($OBS pods: loki, collectors, shipper)" \
  || warn "observability stack incomplete ($OBS pods running) — Forge panels may be empty"

JUMPHOST=$(aws ec2 describe-instances --region "$REGION" \
  --filters "Name=tag:Name,Values=$CLUSTER-jumphost" "Name=instance-state-name,Values=running" \
  --query 'Reservations[].Instances[].InstanceId' --output text 2>/dev/null | awk '{print $1}')
[ -n "$JUMPHOST" ] && ok "jumphost $JUMPHOST (drives the stranger path)" \
  || bad "no running jumphost found — stranger-path steps cannot run"

[ "$FAILED" -gt 0 ] && { echo; echo "  ${RED}preflight failed${R} — fix the above before demoing."; exit 1; }
[ "$CHECK_ONLY" -eq 1 ] && { echo; ok "preflight clean"; exit 0; }

# ── helper: run a curl on the jumphost via SSM and echo "<status> <body>" ────
ssm_curl() {
  local body="$1"; shift
  local hdrs=""
  for h in "$@"; do hdrs="$hdrs -H '$h'"; done
  local cmd="curl -s -o /tmp/dbody -w '%{http_code}' --max-time 15 -X POST http://$VIP/v1/mcp/forecast \
-H 'Host: $INGRESS_HOST' -H 'Content-Type: application/json' -H 'Accept: application/json' $hdrs \
-d '$body'; echo; head -c 220 /tmp/dbody"
  local id
  id=$(aws ssm send-command --region "$REGION" --instance-ids "$JUMPHOST" \
        --document-name AWS-RunShellScript \
        --parameters "commands=[$(python3 -c 'import json,sys;print(json.dumps(sys.argv[1]))' "$cmd")]" \
        --query 'Command.CommandId' --output text 2>/dev/null)
  for _ in $(seq 1 40); do
    sleep 3
    local st
    st=$(aws ssm get-command-invocation --region "$REGION" --command-id "$id" \
          --instance-id "$JUMPHOST" --query 'Status' --output text 2>/dev/null || echo Pending)
    case "$st" in Success|Failed|Cancelled|TimedOut) break ;; esac
  done
  aws ssm get-command-invocation --region "$REGION" --command-id "$id" \
    --instance-id "$JUMPHOST" --query 'StandardOutputContent' --output text 2>/dev/null
}

FORECAST_BODY='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"forecast","arguments":{"symbol":"NVDA","days":30}}}'
BALANCE_BODY='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_account_balance","arguments":{"account_id":"ACC-1001"}}}'

expect() { # expect <wanted-status> <label> <output>
  local want="$1" label="$2" out="$3"
  local got; got=$(printf '%s' "$out" | head -1 | tr -d '[:space:]')
  local snippet; snippet=$(printf '%s' "$out" | tail -n +2 | head -c 150)
  if [ "$got" = "$want" ]; then ok "$label → $got"; else bad "$label → got $got, wanted $want"; fi
  [ -n "$snippet" ] && note "$snippet"
}

# ── act 1: the agent's own path ──────────────────────────────────────────────
say "Act 1 — the trusted agent path"
note "The AgentCore runtime calls its MCP tool through the BNK VIP. Bedrock is"
note "hit twice: once to decide it needs the tool, once to narrate the answer."
if [ "$QUICK" -eq 1 ]; then
  warn "--quick: skipping agent invokes"
else
  ( cd "$DEMO_DIR/agent" && ./node_modules/.bin/agentcore invoke \
      --runtime "$RUNTIME" --target "$TARGET" --prompt "forecast AMZN" 2>&1 ) \
    | tr '\r' '\n' | grep -iE "expected growth" | head -2 | sed 's/^/  /' \
    && ok "agent got a forecast — the whole path works" \
    || bad "agent invoke produced no forecast"
  note "That percentage was generated by the pod. Seeing it proves the tool ran."
fi
pause

# ── act 2: the stranger, and what stops it ───────────────────────────────────
say "Act 2 — the stranger path: same tool, no AWS involved"
note "This caller never touched AWS. No JWT check, no Cedar policy, no guardrail,"
note "because none of those components are in this path. BNK is the only checkpoint."
expect 401 "no credential at all       " "$(ssm_curl "$FORECAST_BODY")"
expect 401 "wrong bearer token         " "$(ssm_curl "$FORECAST_BODY" "Authorization: Bearer not-a-real-token")"
expect 200 "valid token, benign tool   " "$(ssm_curl "$FORECAST_BODY" "Authorization: Bearer $EXTERNAL_TOKEN")"
pause

say "Act 3 — authorization: the sensitive tool"
note "get_account_balance is privileged. BNK decides, in the data path, before the"
note "request reaches the pod."
expect 403 "external caller → BALANCE  " "$(ssm_curl "$BALANCE_BODY" "Authorization: Bearer $EXTERNAL_TOKEN")"
expect 200 "agent's token  → BALANCE  " "$(ssm_curl "$BALANCE_BODY" "Authorization: Bearer $AGENT_TOKEN")"
note "Same route, same tool server. The network made the distinction."
pause

# ── act 4: the throttle ──────────────────────────────────────────────────────
say "Act 4 — rate limiting: stopping a runaway caller"
note "10 requests per 60s per caller identity. Watch where it flips to 429."
note "Acts 2-3 already spent part of this caller's budget — the bucket is shared"
note "across everything that identity does, which is the point of keying on it."
LOOP_CMD="for i in \$(seq 1 13); do printf '%s ' \"\$(curl -s -o /dev/null -w '%{http_code}' --max-time 12 \
-X POST http://$VIP/v1/mcp/forecast -H 'Host: $INGRESS_HOST' -H 'Content-Type: application/json' \
-H 'Accept: application/json' -H 'Authorization: Bearer $EXTERNAL_TOKEN' -d '$FORECAST_BODY')\"; done; echo"
LOOP_ID=$(aws ssm send-command --region "$REGION" --instance-ids "$JUMPHOST" \
  --document-name AWS-RunShellScript \
  --parameters "commands=[$(python3 -c 'import json,sys;print(json.dumps(sys.argv[1]))' "$LOOP_CMD")]" \
  --query 'Command.CommandId' --output text 2>/dev/null)
for _ in $(seq 1 40); do
  sleep 3
  ST=$(aws ssm get-command-invocation --region "$REGION" --command-id "$LOOP_ID" \
        --instance-id "$JUMPHOST" --query 'Status' --output text 2>/dev/null || echo Pending)
  case "$ST" in Success|Failed|Cancelled|TimedOut) break ;; esac
done
SEQ=$(aws ssm get-command-invocation --region "$REGION" --command-id "$LOOP_ID" \
  --instance-id "$JUMPHOST" --query 'StandardOutputContent' --output text 2>/dev/null | tr -d '\n')
note "$SEQ"
if printf '%s' "$SEQ" | grep -q 429; then
  ok "the limit bit — throttled requests never reached the pod"
else
  bad "expected 429s in the sequence"
fi
note "A throttled request is never buffered or forwarded: it costs nothing downstream."
pause

# ── act 5: the evidence ──────────────────────────────────────────────────────
say "Act 5 — the evidence, in Forge"
CLUSTER_ID=""
TOKEN=$(curl -s -X POST "$FORGE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' --max-time 8 2>/dev/null \
  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("token",""))' 2>/dev/null)
if [ -n "$TOKEN" ]; then
  CLUSTER_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "$FORGE/api/k8s/clusters" --max-time 8 2>/dev/null \
    | python3 -c "import sys,json;print(next((c['id'] for c in json.load(sys.stdin) if c['name']=='$CLUSTER'),''))" 2>/dev/null)
fi
if [ -n "$CLUSTER_ID" ]; then
  curl -s -H "Authorization: Bearer $TOKEN" \
    "$FORGE/api/k8s/clusters/$CLUSTER_ID/llm-observability/rankings" --max-time 15 2>/dev/null \
    | python3 -c '
import sys,json
d=json.load(sys.stdin)
print(f"  available: {d.get(\"available\")}")
for r in d.get("rows",[]):
    print(f"  {r[\"model\"]:<38} req={r[\"requests\"]:<4} tokens={r[\"tokens\"]:<7} cost={round(r.get(\"cost\") or 0,4)}")
' 2>/dev/null || warn "could not read Forge rankings"
  ok "two legs, one view"
  note "mcp:finance-tool  = BNK's view: who was allowed, who was refused, 0 tokens"
  note "claude-sonnet-4-6 = Bedrock's view: what the reasoning actually cost"
  echo
  note "Open these:"
  note "  $FORGE  →  LLM Observability  →  $CLUSTER"
  note "  $FORGE  →  Gateway Topology   →  $CLUSTER-gateway  (iRule under each listener)"
else
  warn "Forge not reachable at $FORGE — skipping. Start the local stack to see the panels."
fi

# ── wrap ─────────────────────────────────────────────────────────────────────
say "Summary"
if [ "$FAILED" -eq 0 ]; then
  ok "every expected outcome matched"
  cat <<'EOS'

  Three layers, four distinct refusals, one route:
    401  no / wrong credential            (the tool)
    403  privileged tool, wrong caller    (BNK, before the pod)
    429  too many requests                (BNK, before the pod)
    ---  non-VPC source                   (BNK firewall, before TCP completes)

  And the point of it: an agent is useless without its tool call, so whoever
  controls that hop controls the agent's reach. That hop is where BNK sits.
EOS
else
  bad "$FAILED step(s) did not match — see above"
fi
exit $((FAILED > 0 ? 1 : 0))
