# full-cluster — the reference BNK-on-EKS cluster

| | |
| --- | --- |
| Cluster name | `full-cluster` (state in `.awsbnkctl/full-cluster/`) |
| Region / pattern | `ap-southeast-2` / `dual-interface` |
| Footprint | 3 × m6i.4xlarge, EKS, 1 NAT gateway, t3.small jumphost |
| Cost | about **US$3/hour** |

Start here. This `cluster.yaml` builds the complete stack: VPC and subnets, the
two TMM data-path subnets, EKS with a three-node group sized for BNK, the BNK
2.3 control plane and TMM on dedicated ENIs, and a jumphost that can send test
traffic into the external data path. Every other example is this file plus a few
toggles.

## Run it

```bash
cp -r examples/full-cluster my-cluster && cd my-cluster
# edit cluster.yaml: metadata.name, region/azs if needed, bnk.farArchive, bnk.jwt

awsbnkctl validate cluster.yaml
AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up -f cluster.yaml --dry-run
awsbnkctl up -f cluster.yaml                                  # ~25 min
awsbnkctl scenarios run http-routing-e2e -f cluster.yaml     # traffic through the VIP
awsbnkctl down -f cluster.yaml --yes
```

## Toggles in this file

| Toggle | Default | Turn it on by |
| --- | --- | --- |
| Single-interface pattern | off (dual-interface) | `pattern: external-only` **and** delete the `internal:` block under `network.dataPath`. Same cost, one ENI fewer. Needed for transparent egress ([`egress-demo`](../egress-demo/)) |
| SR-IOV / DPDK data plane | off | the two edits above, then `pattern: sriov-external`. **Experimental**; use a fresh cluster |
| Demo mode | off | uncomment `demo:` (or `up --demo`). Enables `awsbnkctl demo run` for the Diameter, HTTP/2 and ingress-migration demos |
| BIG-IP VE appliance | off | uncomment `bigipVE:` with demo mode on. Adds a chargeable c5n.2xlarge for the `bigip-cis` demo; password via `AWSBNKCTL_BIGIP_PASSWORD` |
| BNK release | 2.4.0 | `bnk.manifestVersion` (a 2.3.3 pin is shown commented) |
| BGP ports | on | `bnk.bgp`; see below |

### The single-interface patterns

`external-only` gives TMM one ENI and reaches pods over the CNI. `sriov-external`
is the same topology with TMM driving the NIC over DPDK instead of the kernel:

| | `external-only` | `sriov-external` |
| --- | --- | --- |
| Data-plane driver | kernel socket | DPDK over `vfio-pci` |
| Node preparation | none | a DaemonSet rebinds the external ENA (phase 20b) |
| Network attachment | `external` | `external-sriov` (type `passthru`) |

Both are validated end to end; both stay under CI as fixtures in
`internal/intent/testdata/`.

## Demo mode

With `demo:` on, `up` tags every resource `awsbnkctl:demo=true`, pre-stages the
test clients on the jumphost, and `down` cleans the demos before the
infrastructure. `demo.ttl` (default 24h) is only a countdown shown by `status`;
nothing deletes the cluster when it expires.

```bash
awsbnkctl demo list
awsbnkctl demo run http2 -f cluster.yaml
awsbnkctl demo run --all -f cluster.yaml
```

`ingress-migration` runs ingress-nginx, HAProxy and a BNK Gateway side by side
in front of one backend. `bigip-cis` shows the external BIG-IP VE model BNK
replaces and needs the `bigipVE:` block.

## Scenarios

All 15 run here. `ai-inference-e2e` needs `--synthetic` (no GPU node group).
Run `core-file-collection` last: it patches the CNEInstance and TMM restarts.
`egress-snat` is control-plane only on dual-interface; switch to
`external-only` for the real data path. Prerequisites and VIPs:
[`docs/SCENARIOS.md`](../../docs/SCENARIOS.md).

## BGP

`bnk.bgp: true` opens TCP 179 and UDP 3784 from the external subnet on the
data-plane security group and the external VLAN. To actually peer, build a
Route Server endpoint in that subnet and apply
[`bgp-route-server.yaml`](bgp-route-server.yaml) with the endpoint's address in
place of `10.0.10.31`. Procedure, verification and the teardown order (the
endpoint bills ~US$0.75/hour and blocks subnet deletion) are in
[`docs/BGP-ROUTE-SERVER.md`](../../docs/BGP-ROUTE-SERVER.md).

## Teardown

```bash
awsbnkctl down -f cluster.yaml --yes
```

`down` runs the phases in reverse, is safe to re-run, and finds resources by the
`awsbnkctl:cluster=<name>` tag even if `.awsbnkctl/` is gone. Remove any Route
Server endpoint first.
