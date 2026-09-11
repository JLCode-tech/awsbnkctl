# awsbnkctl

![BNK](https://img.shields.io/badge/BNK-2.4.0-0a3a5c)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.34--1.35-326ce5?logo=kubernetes&logoColor=white)
![AWS EKS](https://img.shields.io/badge/AWS-EKS-ff9900?logo=amazon-aws&logoColor=white)
[![CI](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml/badge.svg)](https://github.com/JLCode-tech/awsbnkctl/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/JLCode-tech/awsbnkctl?label=download)](https://github.com/JLCode-tech/awsbnkctl/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`awsbnkctl` deploys **F5 BIG-IP Next for Kubernetes (BNK)** on AWS EKS from one
`cluster.yaml`. It builds the VPC, the EKS cluster, the dedicated TMM data-plane
ENIs and the BNK stack itself, then validates the result with built-in traffic
scenarios and tears it all down again. One static binary; no Terraform, no host
`kubectl`, no host `helm`.

You need an AWS account, an F5 pull key and licence JWT for `repo.f5.com`, and
about **US$3 per hour** while the reference cluster is up.

## Install

Download the archive for your platform from
[the latest release](https://github.com/JLCode-tech/awsbnkctl/releases/latest)
and put the binary on your `PATH`:

```bash
VERSION=1.4.0
OS=darwin    # or linux
ARCH=arm64   # or amd64
curl -fsSL "https://github.com/JLCode-tech/awsbnkctl/releases/download/v${VERSION}/awsbnkctl_${VERSION}_${OS}_${ARCH}.tar.gz" | tar -xz
sudo mv awsbnkctl /usr/local/bin/
awsbnkctl version
```

`awsbnkctl self update` upgrades in place. To build from source run `make build`
with Go 1.26 or newer; the binary lands in `bin/awsbnkctl`.

## Quick start

Every cluster starts from an example. `full-cluster` is the reference; the
others are demos built on the same shape (see [`examples/`](examples/)).

```bash
# 1. Copy the reference example. Everything you run reads cluster.yaml from here.
cp -r examples/full-cluster my-cluster && cd my-cluster

# 2. Edit cluster.yaml — three things:
#      metadata.name           a name unique in your AWS account (it tags every resource)
#      metadata.region + azs   if not ap-southeast-2
#      bnk.farArchive / jwt    paths to your F5 pull key and licence JWT (never committed)

# 3. Check the file, then the plan. Neither touches AWS.
awsbnkctl validate cluster.yaml
AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up -f cluster.yaml --dry-run

# 4. Build it (about 25 minutes), prove traffic flows, tear it down.
awsbnkctl up -f cluster.yaml
awsbnkctl scenarios run http-routing-e2e -f cluster.yaml
awsbnkctl down -f cluster.yaml --yes
```

`up` writes its state to `.awsbnkctl/<name>/` under the directory you run it
from, so run `scenarios`, `status` and `down` from the same place. `doctor`
checks AWS credentials and local prerequisites before you start.

## What gets built

```
  public subnets ──► IGW          private subnets ──► NAT GW
  ├─ EKS control plane            └─ EKS workers (3 × m6i.4xlarge, VPC CNI)
  └─ jumphost (test client)

  BNK_EXT subnet                  BNK_INT subnet (dual-interface only)
  └─ TMM external ENI             └─ TMM internal ENI
     SelfIP .240, Gateway VIP .100

  client ──► VIP ──► TMM ──► backend pods
```

The `pattern` field picks the TMM interface layout:

| Pattern | TMM ENIs | Use it for |
|---|---|---|
| `dual-interface` (alias `host-device`) | external + internal | the reference cluster; backends off-cluster as well as pods |
| `external-only` | external | single-arm ingress; the pattern transparent egress needs |
| `sriov-external` | external, DPDK over `vfio-pci` | line-rate experiments; **experimental** |

The reference `cluster.yaml` is dual-interface and documents the two edits that
switch it to either single-interface pattern.

## Versions

| Component | Version | Change it with |
|---|---|---|
| BNK release manifest | `2.4.0` (BNK 2.4; the 2.3.0–2.3.3 builds also supported) | `bnk.manifestVersion` — the matching FLO chart follows automatically |
| Kubernetes (EKS) | default `1.35`, floor `1.34`, 1.36+ warns | `cluster.kubernetesVersion` |
| cert-manager | `v1.21.1`, embedded upstream YAML | `intent.EmbeddedCertManagerVersion` |
| FLO chart | paired with the manifest (`v2.30.0-0.5.2` for 2.4.0) | `addons.flo.version` |
| Go | 1.26 | `go.mod` |

Four of those pins expire on a calendar. Check them before a new deployment:

| Check | Current | Changes when | Then |
|---|---|---|---|
| EKS standard support for the Kubernetes floor | `1.34` | **2026-12-02** | raise `intent.MinKubernetesVersion` |
| cert-manager support window | `v1.21.1` (K8s 1.33–1.36) | cert-manager **1.23** releases | embed the newest supported minor |
| Newest BNK manifest | `2.4.0` | F5 publishes a build (`awsbnkctl manifest probe`) | add a row to `manifest.KnownReleases` |
| BNK CRDs on Kubernetes 1.36+ | `format: int32` + `maximum: 4294967295` still in 2.4.0 | a future BNK manifest fixes it | drop the 1.36 warning |

The reasoning behind each pin is in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Scenarios

`awsbnkctl scenarios run --all -f cluster.yaml` runs 15 validation scenarios
against a live cluster: HTTP and gRPC routing, weighted splits, L4 TCP/UDP,
multi-VIP, external backends, cluster-wide watch, admin-access RBAC, AI token
counting and semantic caching, GPU inference, transparent egress and core-file
collection. Most drive real traffic from the jumphost; the ones that only assert
control-plane state are rated Amber and say so. `scenarios list` prints the rating of each, and
[`docs/SCENARIOS.md`](docs/SCENARIOS.md) has what every scenario needs and the
VIP each one owns.

`awsbnkctl demo run --all` adds four narrated protocol demos (Diameter, HTTP/2,
ingress migration, BIG-IP CIS) on a cluster built with `demo.enabled: true`.

## Commands you will actually use

| Command | What it does |
|---|---|
| `validate <cluster.yaml>` | parse and check the intent, no AWS calls |
| `up -f <cluster.yaml>` | build everything; `--dry-run` prints the plan, `--demo` marks a demo cluster |
| `status -f <cluster.yaml>` | what exists, what state it is in |
| `scenarios run <name\|--all> -f <cluster.yaml>` | run validation scenarios |
| `k get\|apply\|logs\|exec …` | kubectl equivalents built in, no host kubectl |
| `down -f <cluster.yaml> --yes` | remove everything `up` created, by tag, even if local state is lost |
| `doctor` | check credentials and prerequisites |
| `manifest probe [version]` | list what a BNK release manifest ships |

Every command, every flag and every environment variable is in
[`docs/COMMANDS.md`](docs/COMMANDS.md).

## Further reading

- [`examples/`](examples/) — the five examples, what each costs and demonstrates.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — the intent format, the 39 phases, patterns, version policy.
- [`docs/SCENARIOS.md`](docs/SCENARIOS.md) — every scenario, its prerequisites and VIP.
- [`docs/BGP-ROUTE-SERVER.md`](docs/BGP-ROUTE-SERVER.md) — peering TMM with an AWS Route Server.
- [`docs/FORGE_INTEGRATION.md`](docs/FORGE_INTEGRATION.md) — registering clusters with [BNK Forge](https://github.com/f5devcentral/bnk-forge).
- `awsbnkctl agent init` scaffolds `AGENTS.md`, personas and a journal for coding-agent sessions; the binary embeds no LLM and no MCP server.

## The `*bnkctl` family

`awsbnkctl` is one of a family of single-binary CLIs that deploy BNK on different platforms:

| Tool | Platform | Data plane | Repository |
|---|---|---|---|
| **`awsbnkctl`** | AWS EKS | secondary ENIs (host-device or SR-IOV DPDK), Multus | [JLCode-tech/awsbnkctl](https://github.com/JLCode-tech/awsbnkctl) |
| **`gkebnkctl`** | GCP GKE | secondary VPC interfaces (host-device), Multus | [JLCode-tech/gkebnkctl](https://github.com/JLCode-tech/gkebnkctl) |
| **`roksbnkctl`** | IBM Cloud ROKS / OpenShift | IBM VPC secondary subnets, Calico or OVN-Kubernetes | [jgruberf5/roksbnkctl](https://github.com/jgruberf5/roksbnkctl) |
| **`ocibnkctl`** | local OCI containers / k3s | container netns virtio demo mode, Anycast BGP | [mwiget/ocibnkctl](https://github.com/mwiget/ocibnkctl) |
| **`bnkctl-index`** | BNK Forge catalogue | packages every `*bnkctl` as a Forge runner module | [mwiget/bnkctl-index](https://github.com/mwiget/bnkctl-index) |

## Contributing and licence

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the development workflow and the
gates every change must pass. MIT licence, © 2026 JLCode-tech.
