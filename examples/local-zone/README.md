# local-zone — telco/edge reference manifests

> [!NOTE]
> **Not a deployable example.** There is no `cluster.yaml` here. These are BNK
> custom resources you apply to a cluster you already have, and **none of them
> reached a working data path** in the AWS Local Zone trial they come from.
> They are kept as the starting point and the reproduction case, not as known
> working configuration. Read
> [`docs/local-zones-validation.md`](../../docs/local-zones-validation.md) first.

The manifests are in the BNK 2.4 shape: every Gateway carries a
`GatewaySettings` and an `infrastructure.parametersRef`, the GatewayClass name
is rendered from the cluster state, and the VIPs sit inside the Infra listener
pool (`.100`–`.199`). SNAT is Automap in the `GatewaySettings`.

| File | Declares | Result in the trial |
| --- | --- | --- |
| `manifests/http2.yaml` | HTTP/2: namespace, `GatewaySettings`, backend, `Gateway` at `10.0.10.123`, `HTTPRoute` | control plane OK; data plane timed out — the VPC CNI claimed the VIP on the node's primary ENI and return traffic bypassed TMM |
| `manifests/diameter.yaml` | Diameter over TCP 3868 at `10.0.10.122`, backend, `L4Route` | control plane OK; same asymmetric-routing failure |
| `manifests/sctp.yaml` | SCTP on 9000 at `10.0.10.121`, echo backend, `L4Route` | control plane failed: the Gateway listener rejects `protocol: SCTP` |
| `manifests/egress.yaml` | `GatewaySettings` + `EgressGateway` capturing the three namespaces, tunnel on `int-vlan-infra` | the attempted fix; accepted, data plane still timed out |

Addresses assume the standard layout (`10.0.10.0/24` external, `10.0.20.0/24`
internal); change them to match your cluster, keeping the VIPs inside the
listener pool.

For protocol paths that **are** proven on AWS, run the `http2` and `diameter`
demos (`awsbnkctl demo list`). For transparent egress that works, use
[`egress-demo`](../egress-demo/) or the `egress-snat` scenario. `egress.yaml`
here is the dual-interface shape (tunnel on `int-vlan-infra`); on an
`external-only` cluster set `networkRef` to `ext-vlan-infra`.

## Applying them

```bash
awsbnkctl k apply --config <your cluster.yaml> -f examples/local-zone/manifests/http2.yaml
awsbnkctl k delete namespace http2-scenario        # or kubectl delete -f the file
```

`--config` renders `{{.GATEWAYCLASS_NAME}}` from the cluster's state and picks
its kubeconfig. The cluster you apply to is what bills; this directory costs
nothing on its own.
