# BNK Transparent Egress & Firewall Demo

> [!NOTE]
> A live, operator-driven demo showing F5 BIG-IP Next for Kubernetes (BNK) **transparent egress**. Watch an application pod's outbound traffic get SNAT-translated to a single BNK-controlled identity with an egress firewall ACL applied, all toggled by applying a single `F5SPKEgress` CR without modifying the pod.

> [!WARNING]
> This requires an **external-only** or **sriov-external** pattern. It does NOT work on `dual-interface` host-device setups due to VXLAN limitations.

## What's Included
- `cluster.yaml`: External-only BNK cluster configuration.
- `egress-toggle.yaml`: The `F5SPKEgress` CR toggle (BNK ON / OFF).
- `firewall-policy.yaml`: The `F5BigCneAddresslist` and `F5BigFwPolicy` ACL.
- `probe.sh` / `watch.sh`: Scripts injecting an agent's-eye view.
- `workload.yaml`: The captured app pod and an uncaptured control pod.

## Quick Start

### 1. Provision Cluster
```bash
awsbnkctl up --config examples/egress-demo/cluster.yaml
export KUBECONFIG=.awsbnkctl/bnk-egress/kubeconfig
```

### 2. Setup Base Resources
```bash
kubectl apply -f examples/egress-demo/firewall-policy.yaml
kubectl apply -f examples/egress-demo/workload.yaml
kubectl -n bnk-egress-demo rollout status deploy/agent --timeout=120s
```

### 3. Run the Demo
Open two terminals.
**Terminal 1 (Agent View):**
```bash
kubectl exec -it -n bnk-egress-demo deploy/agent -c nginx -- sh /demo/watch.sh
```
**Terminal 2 (Operator Toggle):**
```bash
# Toggle BNK ON
kubectl apply -f examples/egress-demo/egress-toggle.yaml

# Toggle BNK OFF
kubectl delete -f examples/egress-demo/egress-toggle.yaml
```
Watch the IP and access control flip dynamically in Terminal 1.

### 4. Teardown
```bash
kubectl delete -f examples/egress-demo/egress-toggle.yaml
kubectl delete -f examples/egress-demo/workload.yaml
kubectl delete -f examples/egress-demo/firewall-policy.yaml
awsbnkctl down --config examples/egress-demo/cluster.yaml --yes
```
