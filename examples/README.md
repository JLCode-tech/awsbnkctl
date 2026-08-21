# Examples

Each directory is a self-contained topology: a `cluster.yaml` intent file that
`awsbnkctl up` can provision, plus a README explaining what it demonstrates and
what it costs. Copy one, point the two F5 credential paths at your own files, and
run it. (`local-zone` is the exception — reference manifests, no `cluster.yaml`.)

| Example | Pattern | Approx. $/hr | What it demonstrates |
| --- | --- | --- | --- |
| [`full-cluster`](full-cluster/) | `dual-interface` (`host-device`) | ~3 | The complete reference stack: VPC, both TMM data-path subnets, EKS, BNK 2.3, jumphost. Uncomment `demo:` for the protocol walkthroughs, `bigipVE:` for the BIG-IP migration story |
| [`external-only`](external-only/) | `external-only`, or `sriov-external` | ~3 | Single-interface TMM reaching pods over the CNI — no internal VLAN. One-line `pattern:` swap gets the experimental SR-IOV / `vfio-pci` DPDK data path |
| [`egress-demo`](egress-demo/) | `external-only` | ~3 | Transparent egress and an egress firewall ACL, flipped on and off by applying one CR |
| [`ai-rig`](ai-rig/) | `external-only` | ~6 | BNK fronting GPU inference, with an optional disposable SageMaker endpoint |
| [`demo-ai`](demo-ai/) | `dual-interface` (`host-device`) | ~12 | `full-cluster` and `ai-rig` composed into one cluster: all protocol demos plus managed inference |
| [`local-zone`](local-zone/) | n/a — no `cluster.yaml` | n/a | Reference telco/edge custom resources (SCTP, Diameter, HTTP/2, SNAT pool) to apply to an existing cluster |

Six directories, not one per permutation. Where two topologies differed by a
single field, they are one file with the alternative documented in place —
`full-cluster` carries demo mode and the BIG-IP appliance as commented blocks,
and `external-only` carries the SR-IOV pattern as a one-line swap. That keeps
the variants from drifting apart, which is how the old `sriov-external` file
ended up describing the wrong pattern in three of its comments.

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
- **`cluster.kubernetesVersion` is 1.32 or newer.** 1.32 is both the mandated
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
