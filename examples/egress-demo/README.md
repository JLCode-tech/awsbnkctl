# egress-demo — transparent egress and an egress firewall

| | |
| --- | --- |
| Cluster name | `bnk-egress` (state in `.awsbnkctl/bnk-egress/`) |
| Region / pattern | `ap-southeast-2` / `external-only` |
| Footprint | 3 × m6i.4xlarge, EKS, 1 NAT gateway, t3.small jumphost |
| Cost | about **US$3/hour**, plus NAT data charges while the probe loops |

Apply one `EgressGateway` (with its `GatewaySettings`) and a pod's outbound traffic
starts leaving through TMM: its source address becomes TMM's external self IP.
Delete the CR and it goes back to the node's identity. The pod is never changed.

## How it works

BNK 2.4 egress model: the `GatewaySettings` holds one `egressConfigs` entry
(Automap SNAT on the tunnel VLAN `ext-vlan-infra`) and the `EgressGateway`
references it and selects the captured namespace. The controller creates a
wildcard virtual server on TMM and the product builds the VXLAN tunnel from the
worker nodes itself; the tunnel VLAN (`Infra` `egressDefaults`) and the two TMM
routes egress needs (a default and a route back to the VPC) are part of the
`Infra` CR phase 23b applies, so nothing has to be applied by hand. Outbound
traffic from pods in the captured namespace is carried to TMM over the tunnel,
SNATed to the external self IP and leaves the VPC through the NAT gateway.
Works on both `external-only` and `dual-interface`.

The egress firewall from the 2.3 demo (`F5SPKEgress.firewallEnforcedPolicy`)
has no field on the 2.4 `EgressGateway`; `firewall-policy.yaml` is kept, but
attaching it to egress on 2.4 (via `SecPolicy`) is not validated yet.

Only traffic leaving the VPC goes through TMM; pod-to-VPC traffic stays on the
node.

## Files

| File | Purpose |
| --- | --- |
| `cluster.yaml` | the external-only cluster |
| `firewall-policy.yaml` | `F5BigCneAddresslist` + `F5BigFwPolicy` that blocks `1.1.1.1/32` |
| `workload.yaml` | the captured `agent` pod (namespace `bnk-egress-demo`) and an uncaptured control pod |
| `egress-toggle.yaml` | the `GatewaySettings` + `EgressGateway`: apply = BNK on, delete = BNK off |
| `probe.sh` / `watch.sh` | what the pod sees: its public source IP and whether `1.1.1.1` is reachable |

## Run it

```bash
awsbnkctl up -f examples/egress-demo/cluster.yaml
export KUBECONFIG=.awsbnkctl/bnk-egress/kubeconfig

kubectl apply -f examples/egress-demo/firewall-policy.yaml
kubectl apply -f examples/egress-demo/workload.yaml
kubectl -n bnk-egress-demo rollout status deploy/agent --timeout=120s
```

Terminal 1, the pod's view:

```bash
kubectl exec -it -n bnk-egress-demo deploy/agent -c nginx -- sh /demo/watch.sh
```

Terminal 2, the toggle:

```bash
awsbnkctl k apply --config examples/egress-demo/cluster.yaml -f examples/egress-demo/egress-toggle.yaml   # BNK on:  source IP changes
kubectl delete -n bnk-egress-demo egressgateway bnk-egress-demo                                            # BNK off: back to the node identity
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
same `ext-vlan-infra` tunnel; it and the demo each create their own
`EgressGateway` for different namespaces, so they can coexist. `ai-inference-e2e` needs `--synthetic`. Run
`core-file-collection` last. Details: [`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).

## BGP

`bnk.bgp: true` opens the BGP/BFD ports; to peer, follow
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md) and apply
[`bgp-route-server.yaml`](bgp-route-server.yaml). Remove the Route Server
endpoint before `down`.

## Teardown

```bash
kubectl delete -n bnk-egress-demo egressgateway bnk-egress-demo gatewaysettings bnk-egress-demo
kubectl delete -f examples/egress-demo/workload.yaml
kubectl delete -f examples/egress-demo/firewall-policy.yaml
awsbnkctl down -f examples/egress-demo/cluster.yaml --yes
```
