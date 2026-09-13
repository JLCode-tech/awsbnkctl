# local-zone — telco/edge reference manifests

No `cluster.yaml` here. These are BNK custom resources for an AWS Local Zone
deployment; apply them to a cluster you already have. The validation notes are
in [`docs/local-zones-validation.md`](../../docs/local-zones-validation.md).

Every Gateway carries a `GatewaySettings` and an `infrastructure.parametersRef`,
the GatewayClass name is rendered from the cluster state, and the VIPs sit
inside the Infra listener pool (`.100`–`.199`). SNAT is Automap in the
`GatewaySettings`.

| File | Declares |
| --- | --- |
| `manifests/http2.yaml` | HTTP/2: namespace, `GatewaySettings`, backend, `Gateway` at `10.0.10.123`, `HTTPRoute` |
| `manifests/diameter.yaml` | Diameter over TCP 3868 at `10.0.10.122`, backend, `L4Route` |
| `manifests/sctp.yaml` | SCTP on 9000 at `10.0.10.121`, echo backend, `L4Route` |
| `manifests/egress.yaml` | `GatewaySettings` + `EgressGateway` capturing the three namespaces, tunnel on `int-vlan-infra` |

Addresses assume the standard layout (`10.0.10.0/24` external, `10.0.20.0/24`
internal); change them to match your cluster, keeping the VIPs inside the
listener pool. `egress.yaml` is the dual-interface shape; on an `external-only`
cluster set `networkRef` to `ext-vlan-infra`.

The `http2` and `diameter` demos (`awsbnkctl demo list`) and the `egress-snat`
scenario cover the same protocols on the standard examples.

## Applying them

```bash
awsbnkctl k apply --config <your cluster.yaml> -f examples/local-zone/manifests/http2.yaml
awsbnkctl k delete namespace http2-scenario        # or kubectl delete -f the file
```

`--config` renders `{{.GATEWAYCLASS_NAME}}` from the cluster's state and picks
its kubeconfig. The cluster you apply to is what bills; this directory costs
nothing on its own.
