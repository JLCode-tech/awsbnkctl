# Examples

Each directory is a self-contained topology: a `cluster.yaml` intent file that
`awsbnkctl up` can provision, plus a README explaining what it demonstrates and
what it costs. Copy one, point the two F5 credential paths at your own files, and
run it. (`local-zone` is the exception — reference manifests, no `cluster.yaml` of its
own.)

| Example | Pattern | Approx. $/hr | What it demonstrates |
| --- | --- | --- | --- |
| [`full-cluster`](full-cluster/) | `dual-interface` (`host-device`) | ~3 | The complete reference stack: VPC, both TMM data-path subnets, EKS, BNK 2.3, jumphost. Uncomment `demo:` for the protocol walkthroughs, `bigipVE:` for the BIG-IP migration story |
| [`external-only`](external-only/) | `external-only`, or `sriov-external` | ~3 | Single-interface TMM reaching pods over the CNI — no internal VLAN. One-line `pattern:` swap gets the experimental SR-IOV / `vfio-pci` DPDK data path |
| [`egress-demo`](egress-demo/) | `external-only` | ~3 | Transparent egress and an egress firewall ACL, flipped on and off by applying one CR |
| [`ai-rig`](ai-rig/) | `external-only` | ~6 | BNK fronting GPU inference, with an optional disposable SageMaker endpoint |
| [`demo-ai`](demo-ai/) | `dual-interface` (`host-device`) | ~12 | `full-cluster` and `ai-rig` composed into one cluster: all protocol demos plus managed inference |
| [`agentcore-demo`](agentcore-demo/) | `dual-interface` (`host-device`) | ~4 | One MCP tool pod behind a BNK Gateway: an Amazon Bedrock AgentCore runtime calls the tool through BNK, demonstrating AgentCore runtime → BNK → tool governance |
| [`local-zone`](local-zone/) | n/a — no `cluster.yaml` | n/a | Reference telco/edge custom resources (SCTP, Diameter, HTTP/2, SNAT pool) to apply to an existing cluster |

Seven directories, not one per permutation. Where two topologies differed by a
single field, they are one file with the alternative documented in place —
`full-cluster` carries demo mode and the BIG-IP appliance as commented blocks,
and `external-only` carries the SR-IOV pattern as a one-line swap. That keeps
the variants from drifting apart, which is how the old `sriov-external` file
ended up describing the wrong pattern in three of its comments.

Costs are rough `ap-southeast-2` on-demand estimates for the whole footprint while
it is up, excluding data transfer and EBS. None of these topologies scales to
zero — an idle cluster bills the same as a busy one, so tear them down. Each
README has the breakdown and the `down` command.

## Network layout

Every deployable example builds the same VPC shape; the pattern decides whether
TMM gets one data-path ENI or two, and the extras decide what sits around it.

```
                Shared VPC layout — all six deployable examples, 10.0.0.0/16

  public 10.0.1.0/24 · 10.0.2.0/24 ──► IGW        private 10.0.11.0/24 · 10.0.12.0/24 ──► NAT GW
  ├─ EKS control-plane ENIs                        └─ EKS worker primary ENIs (VPC CNI, prefix delegation)
  └─ jumphost mgmt ENI (EICE)

  BNK_EXT 10.0.10.0/24  (every pattern)            BNK_INT 10.0.20.0/24  (dual-interface only)
  ├─ TMM ext ENI · SelfIP .240 · Gateway VIP .100   └─ TMM int ENI · SelfIP .240
  │  (VIP = secondary IP set by the cne-controller
  │   in AWS cloud mode; no BGP needed for it)
  ├─ jumphost data ENI — the test client
  └─ [bnk.bgp] Route Server endpoint ── tcp/179 + udp/3784 ──► TMM .240   (optional, hand-built)

  client ──► VIP 10.0.10.100 ──► TMM ──► backend pods over the CNI          (all patterns)
                                    └──► int-vlan → off-cluster backends   (dual-interface)
```

```
  example          pattern           TMM ENIs   backend path                   what sits on top                      bnk.bgp
  ───────────────  ────────────────  ─────────  ─────────────────────────────  ────────────────────────────────────  ───────
  full-cluster     dual-interface    ext + int  CNI (+ int-vlan)               jumphost; demo: / bigipVE: opt-in     yes
  external-only    external-only †   ext        CNI                            jumphost; † sriov-external swap       yes
  egress-demo      external-only     ext        CNI; pod egress → VXLAN → TMM  jumphost; F5SPKEgress toggle          yes
  ai-rig           external-only     ext        CNI → GPU node group           LB controller; SageMaker endpoint     yes
  demo-ai          dual-interface    ext + int  CNI (+ int-vlan) → GPU ng      demo; jumphost; SageMaker; LB ctrl    yes
  agentcore-demo   dual-interface    ext + int  CNI (+ int-vlan)               LB controller; Route 53; AgentCore    yes
  local-zone       manifests only    —          —                              apply to a cluster you already have   —
```

`bnk.bgp: true` only opens the doors (security group + `F5SPKVlan`); the Route
Server, its endpoint and the routing CRs are added by hand after `up`. Each
example ships a `bgp-route-server.yaml` and the procedure is
[`docs/BGP-ROUTE-SERVER.md`](../docs/BGP-ROUTE-SERVER.md).

## Conventions

Every `cluster.yaml` here follows the same rules:

- **`pattern:`** picks the data-path topology — `external-only`,
  `dual-interface` (alias `host-device`), or `sriov-external`. See the pattern
  table in the [root README](../README.md).
- **F5 credentials** are referenced as `./cne_pull_64.json` (FAR pull
  credentials) and `./license.jwt` (subscription JWT), each marked
  `# REPLACE with your …`. Both filename patterns are gitignored repo-wide, so
  your real credentials can never be committed by accident.
- **Relative paths resolve against the directory holding the `cluster.yaml`**,
  not your shell's working directory. `./cne_pull_64.json` in
  `examples/full-cluster/cluster.yaml` means
  `examples/full-cluster/cne_pull_64.json` no matter where you invoke
  `awsbnkctl` from. Use an absolute path to keep credentials elsewhere.
- **`forge:` is enabled in every example**, pointing at `localhost:8000`. Forge
  is the web UI for the `*bnkctl` tools —
  [f5devcentral/bnk-forge](https://github.com/f5devcentral/bnk-forge). If you
  don't run one, nothing breaks: Phase 09 soft-fails, writes a pending link,
  warns, and `up` still exits 0. Set `enabled: false` to skip it.
- **Secrets are never written to YAML.** Passwords and tokens come from the
  environment: `AWSBNKCTL_FORGE_PASSWORD`, `AWSBNKCTL_BIGIP_PASSWORD`,
  `HF_TOKEN`.
- **`metadata.name` is load-bearing.** It becomes the
  `awsbnkctl:cluster=<name>` tag on every AWS resource and the directory name
  under `.awsbnkctl/`. Pick something unique in your account so tag-based
  discovery on `down` never collides with another cluster.
- **`metadata.region` is always explicit** — `awsbnkctl` never guesses a region.
- **`bnk.bgp: true` in every deployable example.** It opens TCP 179 / UDP 3784
  from the external subnet on the data-plane security group and the external
  `F5SPKVlan`, and nothing else — no peer exists until you build a Route Server
  and apply the example's `bgp-route-server.yaml`. Harmless when you don't.
- **`cluster.kubernetesVersion` is 1.34 or newer.** 1.34 is both the mandated
  floor and the default when the key is omitted; `validate` rejects anything
  lower before making an AWS call. BNK 2.3 installs cleanly up to 1.35 — 1.36+
  gets a warning, because the apiserver there rejects two core BNK CRDs. See
  [the version policy](../docs/ARCHITECTURE.md#kubernetes-version-policy).

## Before you run one

```bash
# Intent only, no AWS calls
awsbnkctl validate examples/<name>/cluster.yaml

# Plan against real AWS, no mutations
awsbnkctl up --config examples/<name>/cluster.yaml --dry-run
```
