# Examples

Each directory holds a `cluster.yaml` that `awsbnkctl up` can build, plus a
README that says what it shows, what it costs and how to tear it down. Copy
one, point the two F5 credential paths at your own files, and run it.
`local-zone` is the exception: reference manifests only, no `cluster.yaml`.

| Example | Pattern | ~US$/hr | What it shows |
| --- | --- | --- | --- |
| [`full-cluster`](full-cluster/) | `dual-interface` (two edits give `external-only` or `sriov-external`) | 3 | **Start here.** The reference cluster: VPC, TMM data-path subnets, EKS, BNK, jumphost. Uncomment `demo:` for the protocol demos, `bigipVE:` for the BIG-IP migration demo |
| [`egress-demo`](egress-demo/) | `external-only` | 3 | A pod's outbound traffic flips to a BNK-controlled identity with a firewall ACL by applying one CR |
| [`demo-ai`](demo-ai/) | `dual-interface` | 12 (6 as the lean rig) | `full-cluster` plus a GPU node group and a disposable SageMaker endpoint: the protocol demos and AI inference in one cluster |
| [`agentcore-demo`](agentcore-demo/) | `dual-interface` | 4 | An Amazon Bedrock AgentCore agent calls an MCP tool through BNK; BNK authenticates, rate-limits and logs every call |
| [`local-zone`](local-zone/) | none | 0 | Telco/edge CRs (SCTP, Diameter, HTTP/2, SNAT) from an AWS Local Zone trial that did **not** reach a working data path; kept as a reference |

Costs are rough `ap-southeast-2` on-demand rates for the whole footprint,
excluding data transfer and EBS. Nothing here scales to zero, so tear clusters
down when you stop.

## Network layout

Every example builds the same VPC. The pattern decides whether TMM gets one
data-path ENI or two.

```
  public 10.0.1.0/24 · 10.0.2.0/24 ──► IGW        private 10.0.11.0/24 · 10.0.12.0/24 ──► NAT GW
  ├─ EKS control-plane ENIs                        ├─ EKS worker primary ENIs (VPC CNI)
  └─ jumphost mgmt ENI (EICE)                      └─ BNK_EXT / BNK_INT route here too (TMM egress via NAT)

  BNK_EXT 10.0.10.0/24  (every pattern)            BNK_INT 10.0.20.0/24  (dual-interface only)
  ├─ TMM external ENI · SelfIP .240 · VIP .100     └─ TMM internal ENI · SelfIP .240
  ├─ jumphost data ENI — the test client
  └─ optional Route Server endpoint ──tcp/179, udp/3784──► TMM .240   (bnk.bgp: true opens the ports)

  client ──► VIP 10.0.10.100 ──► TMM ──► backend pods over the CNI
```

The Gateway VIP is a secondary IP on the TMM external ENI, assigned by the BNK
controller, so anything in the VPC reaches it without BGP. Scenario and demo
VIPs are allocated from `.100` upward; the plan is in
[`docs/SCENARIOS.md`](../docs/SCENARIOS.md#7-vip-plan).

## Rules every `cluster.yaml` follows

- **Credentials are files you supply.** `bnk.farArchive` and `bnk.jwt` point at
  `./cne_pull_64.json` and `./license.jwt`, relative to the `cluster.yaml`
  directory. Both names are gitignored, so they cannot be committed by accident.
- **`metadata.name` must be unique in your account.** It tags every AWS resource
  and names the state directory `.awsbnkctl/<name>/`; `down` finds resources by
  that tag even if the state directory is gone.
- **Secrets never go in YAML.** `AWSBNKCTL_FORGE_PASSWORD`,
  `AWSBNKCTL_BIGIP_PASSWORD` and `HF_TOKEN` come from the environment.
- **`forge:` is on** and points at `localhost:8000`. Without a Forge instance the
  registration phase warns and `up` still succeeds.
- **`bnk.bgp: true` is on.** It only opens the BGP/BFD ports; nothing peers until
  you build a Route Server ([`docs/BGP-ROUTE-SERVER.md`](../docs/BGP-ROUTE-SERVER.md)).

## Before you build one

```bash
awsbnkctl validate examples/<name>/cluster.yaml                       # intent only
AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up -f examples/<name>/cluster.yaml --dry-run   # the plan, no mutations
```

After `up`, every example can run the full scenario suite:
`awsbnkctl scenarios run --all -f examples/<name>/cluster.yaml`. What each
scenario needs, and which examples give it a real data path, is in
[`docs/SCENARIOS.md`](../docs/SCENARIOS.md#6-where-each-scenario-runs).
