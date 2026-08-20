# Reference Manifests — Telco / Edge Custom Resources

> [!NOTE]
> **This is not a deployable topology.** Unlike every other directory under
> `examples/`, there is no `cluster.yaml` here and nothing for `awsbnkctl up` to
> read. These are standalone BNK custom resources you apply to a cluster you
> already have.

Reference manifests for specialised BNK edge and telco configurations, captured
from the AWS Local Zone validation described in
[`docs/local-zones-validation.md`](../../docs/local-zones-validation.md).

> [!IMPORTANT]
> **None of the three protocol manifests reached a working data path in that
> validation.** They are preserved as a starting point and as the reproduction
> case for the findings, not as configurations known to carry traffic. Read the
> report before assuming any of them works.

| File | What it declares | Validation result |
| --- | --- | --- |
| `manifests/http2.yaml` | HTTP/2: namespace, `F5BnkGateway` VIP pool (`10.0.10.202/32`), backend, `HTTPRoute` | Control plane **passed** (`Programmed=True`); data plane **timed out** — VPC CNI claimed the VIP on the node's primary ENI, then return traffic bypassed TMM for want of SNAT |
| `manifests/diameter.yaml` | Diameter over TCP 3868: namespace, VIP pool (`10.0.10.201/32`), backend, `L4Route` | Control plane **passed**; data plane **timed out** — same asymmetric-routing cause |
| `manifests/sctp.yaml` | SCTP on 9000: namespace, VIP pool (`10.0.10.200/32`), echo backend, `L4Route` | Control plane **failed** — the Gateway listener rejects `protocol: SCTP` outright (`Listener protocol not supported: SCTP`). This manifest cannot be applied successfully as written |
| `manifests/egress.yaml` | `F5SPKEgress` (`perth-test-egress`) capturing the three namespaces above: `SRC_TRANS_AUTOMAP` + a pseudo-CNI VxLAN tunnel on the node's `ens5` | **This was the fix we tried, and it did not work.** Applied to force pod return traffic back through TMM. Control plane accepted it (`Programmed=True`); the data plane still timed out. Suspected VxLAN encapsulation trouble or Security Group drops on the worker→TMM return path |
| `manifests/snatpool.yaml` | `F5SPKSnatpool` with a shared SNAT address (`10.0.20.240`) | Part of the same SNAT remediation attempt |

The VIP and SNAT addresses assume the standard data-path layout used across
`examples/` (`10.0.10.0/24` external, `10.0.20.0/24` internal). Adjust them to
match your own subnets before applying.

For a protocol path that *is* proven on AWS, see the `http2` and `diameter`
entries in `awsbnkctl demo list` — both are rated green and drive real traffic
through a BNK VIP.

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

## About `egress.yaml` — the attempted fix

`egress.yaml` is not a working configuration and was never meant to be read as
one. The HTTP/2 and Diameter data paths failed because backend pods sent return
traffic out via the AWS default gateway instead of back through TMM, so this
`F5SPKEgress` was applied to force that traffic through TMM. It did not help:
the control plane accepted it, and the data plane still timed out. The report's
§3.1 records the suspected causes — VxLAN encapsulation, or Security Group drops
on the worker-node→TMM return path.

> [!WARNING]
> Independently of that result, this manifest uses a layout that is known not to
> work on AWS: `pseudoCNIConfig.vxlan.create: true` with
> `tmmInterfaceName: int-vlan` is the **dual-interface** egress shape, and on EKS
> with the AWS VPC CNI the node-side `vxlan100` VTEP does not come up while TMM
> still installs a broad route pointing at it — which breaks **ingress** as well
> as egress.
>
> For transparent egress that does work on AWS, use the `external-only` pattern:
> [`examples/egress-demo/`](../egress-demo/) is validated end to end.

It also hardcodes `nodeInterfaceName: ens5`, correct for AL2023 on Nitro
instances but not for every node type.
