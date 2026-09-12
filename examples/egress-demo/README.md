# egress-demo — transparent egress and an egress firewall

| | |
| --- | --- |
| Cluster name | `bnk-egress` (state in `.awsbnkctl/bnk-egress/`) |
| Region / pattern | `ap-southeast-2` / `external-only` |
| Footprint | 3 × m6i.4xlarge, EKS, 1 NAT gateway, t3.small jumphost |
| Cost | about **US$3/hour**, plus NAT data charges while the probe loops |

Apply one `EgressGateway` (with its `GatewaySettings`) and a pod's outbound
traffic starts leaving through TMM. Delete it and the pod goes back to the
node's own path. The pod is never changed.

## How it works

The `GatewaySettings` holds one `egressConfigs` entry (Automap SNAT on the
external VLAN `ext-vlan-infra`); the `EgressGateway` references it and selects
the captured namespace. The tunnel from the worker nodes and the two TMM routes
egress needs come from the `Infra` CR `up` applies, so nothing else is set by
hand. Captured traffic is SNATed to TMM's external self IP and leaves the VPC
through the NAT gateway; pod-to-VPC traffic stays on the node.

The firewall is a `F5BigFwPolicy` attached to the `EgressGateway` with a
`SecPolicy`. It drops `1.1.1.1` for captured pods and logs everything else.

## Files

| File | Purpose |
| --- | --- |
| `cluster.yaml` | the external-only cluster |
| `workload.yaml` | the captured `agent` pod (namespace `bnk-egress-demo`) and an uncaptured control pod |
| `firewall-policy.yaml` | `F5BigCneAddresslist` + `F5BigFwPolicy` + the `SecPolicy` that attaches them to the `EgressGateway` |
| `egress-toggle.yaml` | the `GatewaySettings` + `EgressGateway`: apply = BNK on, delete = BNK off |
| `probe.sh` / `watch.sh` | what the pod sees: its public source IP and whether `1.1.1.1` is reachable |

## Run it

```bash
awsbnkctl up -f examples/egress-demo/cluster.yaml
export KUBECONFIG=.awsbnkctl/bnk-egress/kubeconfig

kubectl apply -f examples/egress-demo/workload.yaml
kubectl apply -f examples/egress-demo/firewall-policy.yaml
kubectl -n bnk-egress-demo rollout status deploy/agent --timeout=120s
```

Terminal 1, the pod's view:

```bash
kubectl exec -it -n bnk-egress-demo deploy/agent -c nginx -- sh /demo/watch.sh
```

Terminal 2, the toggle:

```bash
awsbnkctl k apply --config examples/egress-demo/cluster.yaml -f examples/egress-demo/egress-toggle.yaml   # BNK on
kubectl delete -n bnk-egress-demo egressgateway bnk-egress-demo                                            # BNK off
```

With BNK on, the source IP the internet sees becomes the NAT gateway's address
and `1.1.1.1` stops answering. With BNK off, it is the node's public IP and
`1.1.1.1` answers. The control pod in `bnk-egress-control` is never captured,
so it shows the unaffected case side by side. With the cluster registered in
Forge you can flip the same CR from **Networking → Egress** and watch the egress
lane appear under **Insights → Traffic Flow**.

## Scenarios

All 15 run here. `egress-snat` is the one this cluster exists for and uses the
same `ext-vlan-infra` tunnel; it and the demo each create their own
`EgressGateway` for different namespaces, so they can coexist.
`ai-inference-e2e` needs `--synthetic`. Run `core-file-collection` last.
Details: [`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).

## BGP

`bnk.bgp: true` opens the BGP/BFD ports; to peer, follow
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md) and apply
[`bgp-route-server.yaml`](bgp-route-server.yaml). Remove the Route Server
endpoint before `down`.

## Teardown

```bash
kubectl delete -n bnk-egress-demo egressgateway/bnk-egress-demo gatewaysettings/bnk-egress-demo
kubectl delete -f examples/egress-demo/firewall-policy.yaml
kubectl delete -f examples/egress-demo/workload.yaml
awsbnkctl down -f examples/egress-demo/cluster.yaml --yes
```
