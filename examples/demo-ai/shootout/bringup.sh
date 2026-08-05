#!/usr/bin/env bash
# bringup.sh — one-shot bring-up of the bnk-demo-ai cluster + the 3-way
# proxy-shootout-to-SageMaker wiring (BNK vs HAProxy vs Envoy AI Gateway).
#
# What it does, in order:
#   1. awsbnkctl up        — VPC/EKS/BNK + 7 protocol demos + SageMaker endpoint
#   2. IAM                 — let the sigv4 hop invoke the SageMaker endpoint
#   3. sigv4 hop           — nginx path-rewrite + aws-sigv4-proxy (shared by all legs)
#   4. BNK leg             — F5BnkGateway + Gateway + HTTPRoute -> sigv4 hop (VIP .130)
#   5. HAProxy leg         — haproxy -> sigv4 hop, internal NLB
#   6. Envoy AI Gateway    — Envoy GW v1.4.2 + AI GW v0.2.0 -> sigv4 hop, internal NLB
#   7. vLLM legs (opt)     — haproxy + Envoy AI GW fronting the in-cluster vLLM
#   8. prints the forge benchmark commands to run the shootout
#
# Prereqs (exported): AWS_PROFILE, HF_TOKEN ($(cat .hf_token)),
#   AWSBNKCTL_FORGE_PASSWORD; awsbnkctl built (go build -o awsbnkctl ./cmd/awsbnkctl);
#   helm + kubectl on PATH; F5 creds (cne_pull_64.json, license.jwt) in repo root.
#
# Run from the repo root:  bash examples/demo-ai/shootout/bringup.sh
set -euo pipefail

CFG=examples/demo-ai/cluster.yaml
CLUSTER=bnk-demo-ai
REGION=ap-southeast-2
EP=bnk-demo-ai-lmi                 # SageMaker endpoint name (cluster name + -lmi)
SM_VIP=10.0.10.130                 # free VIP in BNK_EXT for the SageMaker-via-BNK leg
NODE_ROLE="${CLUSTER}-eks-node-role"
: "${AWS_PROFILE:?set AWS_PROFILE}"; : "${HF_TOKEN:?set HF_TOKEN}"; : "${AWSBNKCTL_FORGE_PASSWORD:?set AWSBNKCTL_FORGE_PASSWORD}"
ACCT=$(aws sts get-caller-identity --query Account --output text)
export KUBECONFIG="$PWD/.awsbnkctl/${CLUSTER}/kubeconfig"
k(){ kubectl "$@"; }

echo "==> 1/8 awsbnkctl up (cluster + 7 demos + SageMaker)"
./awsbnkctl up --config "$CFG"

echo "==> 2/8 IAM: allow sigv4 hop to invoke SageMaker endpoint"
aws iam put-role-policy --role-name "$NODE_ROLE" --policy-name SageMakerInvokeDemo --policy-document "{
  \"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",
  \"Action\":[\"sagemaker:InvokeEndpoint\",\"sagemaker:InvokeEndpointWithResponseStream\"],
  \"Resource\":\"arn:aws:sagemaker:${REGION}:${ACCT}:endpoint/${EP}\"}]}"

echo "==> 3/8 shared sigv4 hop (nginx rewrite + aws-sigv4-proxy)"
k apply -f - <<YAML
apiVersion: v1
kind: Namespace
metadata: { name: sagemaker-proxy }
---
apiVersion: v1
kind: ConfigMap
metadata: { name: sagemaker-rewrite, namespace: sagemaker-proxy }
data:
  default.conf: |
    server {
      listen 8081;
      location = /v1/chat/completions { proxy_pass http://127.0.0.1:8080/endpoints/${EP}/invocations; proxy_set_header Content-Type application/json; }
      location / { proxy_pass http://127.0.0.1:8080; }
    }
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: sagemaker-proxy, namespace: sagemaker-proxy, labels: { app: sagemaker-proxy } }
spec:
  replicas: 1
  selector: { matchLabels: { app: sagemaker-proxy } }
  template:
    metadata: { labels: { app: sagemaker-proxy } }
    spec:
      containers:
        - name: rewrite
          image: nginxinc/nginx-unprivileged:1.27-alpine
          ports: [{ containerPort: 8081 }]
          volumeMounts: [{ name: conf, mountPath: /etc/nginx/conf.d }]
        - name: sigv4
          image: public.ecr.aws/aws-observability/aws-sigv4-proxy:latest
          args: ["--name=sagemaker","--region=${REGION}","--host=runtime.sagemaker.${REGION}.amazonaws.com","--port=:8080"]
          ports: [{ containerPort: 8080 }]
      volumes: [{ name: conf, configMap: { name: sagemaker-rewrite } }]
---
apiVersion: v1
kind: Service
metadata: { name: sagemaker-proxy, namespace: sagemaker-proxy, labels: { app: sagemaker-proxy } }
spec:
  selector: { app: sagemaker-proxy }
  ports: [{ name: http, port: 80, targetPort: 8081 }]   # NOTE: no appProtocol (BNK rejects it)
YAML
k -n sagemaker-proxy rollout status deploy/sagemaker-proxy --timeout=120s

echo "==> 4/8 BNK leg (VIP ${SM_VIP} -> sigv4 hop)"
k apply -f - <<YAML
apiVersion: k8s.f5net.com/v1
kind: F5BnkGateway
metadata: { name: sagemaker-bnk, namespace: sagemaker-proxy }
spec:
  ingressConfig:
    defaultListenerNetworks:
      - { name: external, ipv4BaseCidr: 10.0.10.0/24, startAddress: ${SM_VIP}, endAddress: ${SM_VIP}, provider: f5-ip-provider }
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: { name: sagemaker-gateway, namespace: sagemaker-proxy }
spec:
  gatewayClassName: ${CLUSTER}-gatewayclass
  addresses: [{ type: IPAddress, value: ${SM_VIP} }]
  listeners: [{ name: http, protocol: HTTP, port: 80, allowedRoutes: { namespaces: { from: Same } } }]
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: { name: sagemaker-route, namespace: sagemaker-proxy }
spec:
  parentRefs: [{ name: sagemaker-gateway }]
  hostnames: [sagemaker.bnk.local]
  rules: [{ matches: [{ path: { type: PathPrefix, value: / } }], backendRefs: [{ name: sagemaker-proxy, port: 80 }] }]
YAML
k -n sagemaker-proxy wait --for=condition=Programmed gateway/sagemaker-gateway --timeout=120s || true

echo "==> 5/8 HAProxy leg (-> sigv4 hop) on an internal NLB"
k apply -f - <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata: { name: haproxy-cfg, namespace: sagemaker-proxy }
data:
  haproxy.cfg: |
    global
      log stdout format raw local0
    defaults
      mode http
      timeout connect 5s
      timeout client 300s
      timeout server 300s
    frontend fe
      bind :8080
      default_backend be
    backend be
      server s1 sagemaker-proxy.sagemaker-proxy.svc.cluster.local:80
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: haproxy, namespace: sagemaker-proxy, labels: { app: haproxy } }
spec:
  replicas: 1
  selector: { matchLabels: { app: haproxy } }
  template:
    metadata: { labels: { app: haproxy } }
    spec:
      containers:
        - name: haproxy
          image: haproxy:2.9-alpine
          ports: [{ containerPort: 8080 }]
          volumeMounts: [{ name: cfg, mountPath: /usr/local/etc/haproxy }]
      volumes: [{ name: cfg, configMap: { name: haproxy-cfg } }]
---
apiVersion: v1
kind: Service
metadata:
  name: haproxy-lb
  namespace: sagemaker-proxy
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: external
    service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: ip
    service.beta.kubernetes.io/aws-load-balancer-scheme: internal
spec:
  type: LoadBalancer
  loadBalancerClass: service.k8s.aws/nlb
  selector: { app: haproxy }
  ports: [{ name: http, port: 80, targetPort: 8080 }]
YAML

echo "==> 6/8 Envoy Gateway v1.4.2 + Envoy AI Gateway v0.2.0 (needs k8s >=1.31)"
helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm --version v1.4.2 -n envoy-gateway-system --create-namespace
k wait --for=condition=Available -n envoy-gateway-system deploy/envoy-gateway --timeout=180s
helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm --version v0.2.0 -n envoy-ai-gateway-system --create-namespace
helm upgrade -i aieg     oci://docker.io/envoyproxy/ai-gateway-helm      --version v0.2.0 -n envoy-ai-gateway-system
k wait --for=condition=Available -n envoy-ai-gateway-system deploy --all --timeout=180s
# wire EG's extension manager to the AI Gateway, then restart EG
k apply -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.2.0/manifests/envoy-gateway-config/config.yaml
k apply -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.2.0/manifests/envoy-gateway-config/rbac.yaml
k rollout restart -n envoy-gateway-system deploy/envoy-gateway
k rollout status  -n envoy-gateway-system deploy/envoy-gateway --timeout=120s
# AI Gateway leg -> sigv4 hop (model-routed on x-ai-eg-model=llama3), 600s timeout (default 15s aborts LLMs)
k apply -f - <<'YAML'
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata: { name: envoy-ai }
spec: { controllerName: gateway.envoyproxy.io/gatewayclass-controller }
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: { name: ai-gateway, namespace: sagemaker-proxy }
spec:
  gatewayClassName: envoy-ai
  infrastructure:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: external
      service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: ip
      service.beta.kubernetes.io/aws-load-balancer-scheme: internal
  listeners: [{ name: http, protocol: HTTP, port: 80, allowedRoutes: { namespaces: { from: Same } } }]
---
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: Backend
metadata: { name: sagemaker-be, namespace: sagemaker-proxy }
spec: { endpoints: [{ fqdn: { hostname: sagemaker-proxy.sagemaker-proxy.svc.cluster.local, port: 80 } }] }
---
apiVersion: aigateway.envoyproxy.io/v1alpha1
kind: AIServiceBackend
metadata: { name: sagemaker-aisb, namespace: sagemaker-proxy }
spec:
  schema: { name: OpenAI }
  backendRef: { name: sagemaker-be, kind: Backend, group: gateway.envoyproxy.io }
---
apiVersion: aigateway.envoyproxy.io/v1alpha1
kind: AIGatewayRoute
metadata: { name: sagemaker-aigw, namespace: sagemaker-proxy }
spec:
  schema: { name: OpenAI }
  targetRefs: [{ name: ai-gateway, kind: Gateway, group: gateway.networking.k8s.io }]
  rules:
    - matches: [{ headers: [{ type: Exact, name: x-ai-eg-model, value: llama3 }] }]
      backendRefs: [{ name: sagemaker-aisb }]
      timeouts: { request: 600s, backendRequest: 600s }
YAML

echo "==> 7/8 (optional) vLLM legs — uncomment in this script if you want the vLLM shootout too"
# (haproxy-vllm + a second AI Gateway 'ai-gateway-vllm' fronting vllm.awsbnkctl-scn-aiinference.svc:80;
#  see docs/demo/sagemaker-leg/README.md. The vLLM Service exists only after `awsbnkctl demo run ai-inference-e2e`.)

echo "==> 8/8 resolve NLB DNS + print benchmark commands"
HAP=$(k -n sagemaker-proxy get svc haproxy-lb -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)
EVS=$(k -n envoy-gateway-system get svc -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.loadBalancer.ingress[0].hostname}{"\n"}{end}' 2>/dev/null | awk '/ai-gateway/{print $2}' | head -1)
JH=$(grep -oE 'i-[0-9a-f]+' .awsbnkctl/${CLUSTER}/state.env | head -1)
SRC=$(grep JUMPHOST_BNK_EXT_ENI_IP .awsbnkctl/${CLUSTER}/state.env | cut -d= -f2)
cat <<EOF

============================================================
 Shootout is wired. SageMaker endpoint may still be 'Creating' — wait for InService:
   aws sagemaker describe-endpoint --endpoint-name ${EP} --region ${REGION} --query EndpointStatus --output text
 NLBs (give them ~2-3 min to go active):  haproxy=${HAP:-<pending>}  envoy=${EVS:-<pending>}
 Jumphost=${JH}  src-ip=${SRC}

 Run the 3-way shootout (c=25, non-stream) once endpoint InService + NLBs active:
   B(){ ./awsbnkctl forge benchmark --instance-id ${JH} --region ${REGION} --source-ip ${SRC} \\
        --insecure-host-key --forge-pass "\$AWSBNKCTL_FORGE_PASSWORD" --model llama3 \\
        --tokenizer Qwen/Qwen2.5-32B-Instruct --num-requests 150 --concurrency 25 --stream=false \\
        --timeout 15m -f ${CFG} "\$@"; }
   B --vip ${SM_VIP} --host-header sagemaker.bnk.local --proxy f5-bnk  --run-label bnk
   B --vip ${HAP:-<haproxy-nlb>}                       --proxy haproxy --run-label haproxy
   B --vip ${EVS:-<envoy-nlb>}                         --proxy envoy   --run-label envoy
============================================================
EOF
