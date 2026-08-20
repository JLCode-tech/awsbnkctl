# Reference Manifests — Telco / Edge Custom Resources

> [!NOTE]
> **This is not a deployable topology.** Unlike every other directory under
> `examples/`, there is no `cluster.yaml` here and nothing for `awsbnkctl up` to
> read. These are standalone BNK custom resources you apply to a cluster you
> already have.

Reference manifests for specialised BNK edge and telco configurations, originally
captured from Local Zone / edge work:

| File | What it declares |
| --- | --- |
| `manifests/sctp.yaml` | SCTP routing: namespace, `F5BnkGateway` VIP pool (`10.0.10.200/32`), echo backend |
| `manifests/diameter.yaml` | Diameter routing: namespace, `F5BnkGateway` VIP pool (`10.0.10.201/32`), backend |
| `manifests/http2.yaml` | HTTP/2 routing: namespace, `F5BnkGateway` VIP pool (`10.0.10.202/32`), backend |
| `manifests/snatpool.yaml` | `F5SPKSnatpool` with a shared SNAT address (`10.0.20.240`) |
| `manifests/egress.yaml` | `F5SPKEgress` capturing the three namespaces above — **see the warning below** |

The VIP and SNAT addresses assume the standard data-path layout used across
`examples/` (`10.0.10.0/24` external, `10.0.20.0/24` internal). Adjust them to
match your own subnets before applying.

## Prerequisites

An existing cluster running BNK 2.3 with the F5 CRDs installed — for instance one
brought up from `examples/full-cluster/cluster.yaml`. There is no cost attached to
this directory itself; the cluster you apply it to is what bills.

## Usage

The `awsbnkctl k` subcommands read `$KUBECONFIG` (falling back to
`~/.kube/config`). They do **not** take a `--config` flag, so point `KUBECONFIG`
at the cluster you want before applying:

```bash
export KUBECONFIG=.awsbnkctl/<your-cluster-name>/kubeconfig

awsbnkctl k apply -f examples/local-zone/manifests/sctp.yaml
awsbnkctl k apply -f examples/local-zone/manifests/snatpool.yaml
```

`k apply` is a server-side apply, so re-applying an unchanged manifest is a no-op.
Plain `kubectl apply -f` works identically if you would rather use it.

To remove them, note that `awsbnkctl k delete` takes a resource and name rather
than a file, so delete the namespace (which takes the routes and backends with
it) or use `kubectl` with the manifest:

```bash
awsbnkctl k delete namespace sctp-scenario
# or
kubectl delete -f examples/local-zone/manifests/sctp.yaml
```

## Warning: `egress.yaml` uses the pattern that does not work on AWS

> [!WARNING]
> `manifests/egress.yaml` sets `pseudoCNIConfig.vxlan.create: true` with
> `tmmInterfaceName: int-vlan` — the **dual-interface** egress layout. On EKS with
> the AWS VPC CNI this black-holes TMM traffic: the node-side `vxlan100` VTEP
> fails to come up, while TMM still installs a broad route pointing at it, which
> breaks **ingress** as well as egress.
>
> For transparent egress on AWS use the `external-only` pattern instead — see
> [`examples/egress-demo/`](../egress-demo/), which is validated end to end.
> Treat `egress.yaml` here as a reference for non-AWS / edge CNI environments
> only.

It also hardcodes `nodeInterfaceName: ens5`, which is correct for AL2023 on Nitro
instances but will not match every node type.
