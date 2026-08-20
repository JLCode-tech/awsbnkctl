# Examples

Each directory is a self-contained topology: a `cluster.yaml` intent file that
`awsbnkctl up` can provision, plus a README explaining what it demonstrates and
what it costs. Copy one, point the two F5 credential paths at your own files, and
run it.

| Example | Pattern | Approx. $/hr | What it demonstrates |
| --- | --- | --- | --- |
| [`full-cluster`](full-cluster/) | `dual-interface` (`host-device`) | ~3 | The complete reference stack: VPC, both TMM data-path subnets, EKS, BNK 2.3, jumphost |
| [`external-only`](external-only/) | `external-only` | ~3 | Single-interface TMM reaching pods over the CNI — no internal VLAN |
| [`sriov-external`](sriov-external/) | `sriov-external` (experimental) | ~3 | TMM driving the data plane with DPDK over an ENA bound to `vfio-pci` |
| [`demo`](demo/) | `dual-interface` (`host-device`) | ~3 | `full-cluster` marked as a demo, plus the curated protocol walkthroughs (`awsbnkctl demo run`) |
| [`egress-demo`](egress-demo/) | `external-only` | ~3 | Transparent egress and an egress firewall ACL, flipped on and off by applying one CR |
| [`ai-rig`](ai-rig/) | `external-only` | ~6 | BNK fronting GPU inference, with an optional disposable SageMaker endpoint |
| [`demo-ai`](demo-ai/) | `dual-interface` (`host-device`) | ~12 | `demo` and `ai-rig` composed into one cluster: all protocol demos plus managed inference |
| [`local-zone`](local-zone/) | n/a — no `cluster.yaml` | n/a | Reference telco/edge custom resources (SCTP, Diameter, HTTP/2, SNAT pool) to apply to an existing cluster |

Costs are rough `ap-southeast-2` on-demand estimates for the whole footprint while
it is up, excluding data transfer and EBS. None of these topologies scales to
zero — an idle cluster bills the same as a busy one, so tear them down. Each
README has the breakdown and the `down` command.

There is also `agentcore-demo/`, which is under active development; see its own
README rather than this table.

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
  `examples/demo/cluster.yaml` means `examples/demo/cne_pull_64.json` no matter
  where you invoke `awsbnkctl` from. Use an absolute path to keep credentials
  elsewhere.
- **Secrets are never written to YAML.** Passwords and tokens come from the
  environment: `AWSBNKCTL_FORGE_PASSWORD`, `AWSBNKCTL_BIGIP_PASSWORD`,
  `HF_TOKEN`.
- **`metadata.name` is load-bearing.** It becomes the
  `awsbnkctl:cluster=<name>` tag on every AWS resource and the directory name
  under `.awsbnkctl/`. Pick something unique in your account so tag-based
  discovery on `down` never collides with another cluster.
- **`metadata.region` is always explicit** — `awsbnkctl` never guesses a region.

## Before you run one

```bash
# Intent only, no AWS calls
awsbnkctl validate examples/<name>/cluster.yaml

# Plan against real AWS, no mutations
awsbnkctl up --config examples/<name>/cluster.yaml --dry-run
```
