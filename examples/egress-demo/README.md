# egress-demo — transparent egress and an egress firewall

| | |
| --- | --- |
| Cluster name | `bnk-egress` (state in `.awsbnkctl/bnk-egress/`) |
| Region / pattern | `ap-southeast-2` / `external-only` |
| Footprint | 3 × m6i.4xlarge, EKS, 1 NAT gateway, t3.small jumphost |
| Cost | about **US$3/hour**, plus NAT data charges while the probe loops |

Apply one `F5SPKEgress` CR and a pod's outbound traffic starts leaving through
TMM: its source address becomes TMM's external SelfIP and a firewall ACL applies.
Delete the CR and it goes back to the node's identity. The pod is never changed.

## How it works

The CR (`snatType: SRC_TRANS_AUTOMAP` plus a `pseudoCNIConfig.vxlan` block)
makes the BNK controller create a VXLAN tunnel end on TMM's `ext-vlan` and
tells the `f5-spk-csrc` DaemonSet to program the worker node. Outbound traffic
from pods in the captured namespace is intercepted on `eth0`, carried to TMM over
the tunnel, SNATed to the external SelfIP, filtered by the
`firewallEnforcedPolicy` ACL, and leaves the VPC through the NAT gateway.

On AWS the CR needs `nodeInterfaceName` set to the worker's primary NIC, and
TMM needs a default route plus a route back to the VPC (see
`static-routes.yaml`). Works on both `external-only` and `dual-interface`.

Only traffic leaving the VPC goes through TMM; pod-to-VPC traffic stays on the
node.

## Files

| File | Purpose |
| --- | --- |
| `cluster.yaml` | the external-only cluster |
| `firewall-policy.yaml` | `F5BigCneAddresslist` + `F5BigFwPolicy` that blocks `1.1.1.1/32` |
| `workload.yaml` | the captured `agent` pod (namespace `bnk-egress-demo`) and an uncaptured control pod |
| `egress-toggle.yaml` | the `F5SPKEgress` CR: apply = BNK on, delete = BNK off |
| `static-routes.yaml` | the two TMM routes egress needs; apply once |
| `probe.sh` / `watch.sh` | what the pod sees: its public source IP and whether `1.1.1.1` is reachable |

## Run it

```bash
awsbnkctl up -f examples/egress-demo/cluster.yaml
export KUBECONFIG=.awsbnkctl/bnk-egress/kubeconfig

kubectl apply -f examples/egress-demo/firewall-policy.yaml
kubectl apply -f examples/egress-demo/workload.yaml
kubectl -n bnk-egress-demo rollout status deploy/agent --timeout=120s
awsbnkctl k apply --config examples/egress-demo/cluster.yaml -f examples/egress-demo/static-routes.yaml
```

Terminal 1, the pod's view:

```bash
kubectl exec -it -n bnk-egress-demo deploy/agent -c nginx -- sh /demo/watch.sh
```

Terminal 2, the toggle:

```bash
awsbnkctl k apply --config examples/egress-demo/cluster.yaml -f examples/egress-demo/egress-toggle.yaml   # BNK on:  source IP changes, 1.1.1.1 blocked
kubectl delete -n f5-cne-system f5-spk-egresses.k8s.f5net.com bnk-egress-demo                              # BNK off: back to the node identity
```

The control pod in `bnk-egress-control` is never captured, so it shows the
unaffected case side by side. With the cluster registered in Forge you can flip
the same CR from **Networking → Egress** and watch the egress lane appear under
**Insights → Traffic Flow**.

## Known issue

The SNAT flip always works. The firewall block depends on the ACL blob reaching
TMM, and on BNK 2.3 a `blobd` TLS bug means it sometimes only arrives after a
TMM restart. If `1.1.1.1` stays reachable with BNK on:

```bash
kubectl delete pod -n f5-cne-system -l app=f5-tmm     # wait 2–3 minutes, re-check
```

## Scenarios

All 15 run here. `egress-snat` is the one this cluster exists for and uses the
same `ext-vlan` tunnel; it and the demo each create their own `F5SPKEgress`, so
run one at a time. `ai-inference-e2e` needs `--synthetic`. Run
`core-file-collection` last. Details: [`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).

## BGP

`bnk.bgp: true` opens the BGP/BFD ports; to peer, follow
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md) and apply
[`bgp-route-server.yaml`](bgp-route-server.yaml). Remove the Route Server
endpoint before `down`.

## Teardown

```bash
kubectl delete -f examples/egress-demo/egress-toggle.yaml
kubectl delete -f examples/egress-demo/workload.yaml
kubectl delete -f examples/egress-demo/firewall-policy.yaml
awsbnkctl down -f examples/egress-demo/cluster.yaml --yes
```
