# local-zone — telco/edge reference manifests

> [!NOTE]
> **Not a deployable example.** There is no `cluster.yaml` here. These are BNK
> custom resources you apply to a cluster you already have, and **none of them
> reached a working data path** in the AWS Local Zone trial they come from.
> They are kept as the starting point and the reproduction case, not as known
> working configuration. Read
> [`docs/local-zones-validation.md`](../../docs/local-zones-validation.md) first.

| File | Declares | Result in the trial |
| --- | --- | --- |
| `manifests/http2.yaml` | HTTP/2: namespace, `F5BnkGateway` pool at `10.0.10.202`, backend, `HTTPRoute` | control plane OK; data plane timed out — the VPC CNI claimed the VIP on the node's primary ENI and return traffic bypassed TMM |
| `manifests/diameter.yaml` | Diameter over TCP 3868 at `10.0.10.201`, backend, `L4Route` | control plane OK; same asymmetric-routing failure |
| `manifests/sctp.yaml` | SCTP on 9000 at `10.0.10.200`, echo backend, `L4Route` | control plane failed: the Gateway listener rejects `protocol: SCTP` |
| `manifests/egress.yaml` | `GatewaySettings` + `EgressGateway` capturing the three namespaces, tunnel on `int-vlan-infra` (the BNK 2.4 form of the `F5SPKEgress` used at the time) | the attempted fix; accepted, data plane still timed out |
| `manifests/snatpool.yaml` | `F5SPKSnatpool` with `10.0.20.240` | part of the same attempt |

Addresses assume the standard layout (`10.0.10.0/24` external, `10.0.20.0/24`
internal); change them to match your cluster.

For protocol paths that **are** proven on AWS, run the `http2` and `diameter`
demos (`awsbnkctl demo list`). For transparent egress that works, use
[`egress-demo`](../egress-demo/) or the `egress-snat` scenario. `egress.yaml`
here is the dual-interface shape (tunnel on `int-vlan-infra`), rewritten for the
BNK 2.4 `EgressGateway` model; set `gatewayClassName` to the class phase 23b
registered (`GATEWAYCLASS_NAME` in `state.env`).

## Applying them

```bash
export KUBECONFIG=.awsbnkctl/<your-cluster>/kubeconfig
awsbnkctl k apply -f examples/local-zone/manifests/http2.yaml     # server-side apply; kubectl apply works too
awsbnkctl k delete namespace http2-scenario                        # or kubectl delete -f the file
```

`awsbnkctl k` reads `$KUBECONFIG` and does not take `-f <cluster.yaml>`. The
cluster you apply to is what bills; this directory costs nothing on its own.
