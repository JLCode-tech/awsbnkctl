# Architecture Guide

`awsbnkctl` is a single Go binary that provisions **F5 BIG-IP Next for Kubernetes (BNK)** onto AWS EKS. It communicates directly with AWS via the AWS SDK for Go, using a sequence of imperative, idempotent **phases** dictated by a single `cluster.yaml` intent file.

This document outlines the core architecture: the provisioning model, the intent format, the lifecycle, state management, and network patterns.

---

## 1. Design Philosophy

The tool is built around four core commitments:

| Concept | Approach |
|---|---|
| **AWS SDK Only** | Direct use of the AWS SDK for Go. No shelling out to the `aws` CLI. All AWS interactions live under `internal/aws/`. |
| **Phased Provisioning** | Imperative, sequential phases. No complex reconciler framework. Each phase is a readable, log-able Go function. |
| **Declarative Intent** | A structured `cluster.yaml` is mapped directly to AWS calls without intermediate variable layers. |
| **Tag-Driven State** | AWS resource tags are the ultimate source of truth. A local `state.env` cache simply accelerates re-runs. |

> [!NOTE] 
> There is **no Terraform, no tfstate, and no external IaC engine**. The binary itself makes AWS API calls and tags everything it creates. This ensures a clean, linear failure surface.

---

## 2. The `cluster.yaml` Intent

A cluster is defined by one Kubernetes-style YAML document. 

Here is an overview of the core structure:

```yaml
apiVersion: awsbnkctl/v1
kind: Cluster

metadata:
  name: full-cluster          # Must be lowercase alphanumeric (2-40 chars)
  region: ap-southeast-2

pattern: host-device          # Selects the data-path variant

network:
  vpcCidr: 10.0.0.0/16
  azs: [ap-southeast-2a, ap-southeast-2b]
  subnets:
    public:  [{cidr: 10.0.1.0/24, az: ap-southeast-2a}, ...]
    private: [{cidr: 10.0.11.0/24, az: ap-southeast-2a}, ...]
  dataPath:                   
    external: {cidr: 10.0.10.0/24, az: ap-southeast-2a}   
    internal: {cidr: 10.0.20.0/24, az: ap-southeast-2a}   
  natGateways: 1              

cluster:                      
  kubernetesVersion: "1.35"   # default when omitted; 1.34 is the mandated floor
  nodeGroups:
    - name: default
      instanceType: m6i.4xlarge
      desiredSize: 3

bnk:                          # Supply-chain credentials
  farArchive: ./cne_pull_64.json
  jwt: ./license.jwt
```

**Key Points:**
- **Strict Validation:** Typos are caught immediately. Unknown fields cause a validation error.
- **Explicit AZs:** `network.azs` is explicit to ensure reproducible deployments.
- **`metadata.name`:** Becomes the AWS resource tag (`awsbnkctl:cluster`) and the local state folder name.
- **`kubernetesVersion` has a mandated floor of 1.34** — see below.

### Kubernetes version policy

`cluster.kubernetesVersion` must be **1.34 or newer**. When the key is omitted the
default is **1.35** (`intent.DefaultKubernetesVersion`): that is the minor F5's
BNK 2.3.x and 2.4.0 release notes list for host Kubernetes, and the one every
example pins. 1.34 is only our EKS standard-support floor. Anything lower is
rejected by `validate`, before any AWS call:

```
cluster.kubernetesVersion "1.33" is below the mandated floor 1.34: everything
below it is past the end of EKS standard support (1.31, 1.32 and 1.33 all
reached end of standard support) and is not exercised in CI; set 1.34 or newer
```

Two reasons for the floor. It tracks EKS *standard* support: as of 2026-08,
1.31 (EOL 2025-11-26), 1.32 (EOL 2026-03-23) and 1.33 (EOL 2026-07-29) are all
on extended support, so a new cluster on them starts out costing more for no
benefit and is blocked from the addon versions the BNK stack expects; and
nothing below 1.34 is exercised by CI any more, so allowing it would ship an
untested path. The floor is time-sensitive by design — 1.34 leaves standard
support on 2026-12-02, at which point it should be raised again.

There is a soft **upper** bound as well. BNK 2.3 and 2.4 are known to install cleanly up to
**1.35**. From 1.36 the apiserver rejects the `f5-spk-pools` and HSL CRDs, whose
integer fields declare `format: int32` alongside `maximum: 4294967295` — a value
that does not fit in an int32. Those CRDs are core to BNK, so the install fails at
CRD apply; turning telemetry off does not avoid them. `validate` warns rather than
errors, because the fix belongs in a future BNK manifest and because the CRDs
arrive from the FAR archive at run time, where we cannot inspect them in advance.

The floor and the tested ceiling live in one place each —
`intent.MinKubernetesVersion` and `maxTestedKubernetesMinor` in
`internal/intent/cluster.go`. `TestExampleConfigs_MeetVersionFloor` keeps every
published example above the floor.

---

## 3. The Phased Lifecycle

Provisioning is an ordered sequence of phases. Each phase checks authentication, calls the SDK, creates/reads resources, tags them, and writes to `state.env`.

### `awsbnkctl up`

The `up` command runs four conceptual stages:

1. **Network & IAM:** VPC, subnets, IGW, NAT, Route Tables, and IAM roles.
2. **EKS Control Plane:** Deploys EKS cluster and configures VPC CNI prefix delegation.
3. **Nodes & Data Path:** Node group, kubeconfig, TMM labels, host-device secondary ENIs, optional test jumphost, and OIDC/IRSA.
4. **BNK Install & Activation:** EBS CSI, cert-manager, FLO via Helm, OTEL certs, network mappings, data-plane plumbing, and final activation polling.

### `awsbnkctl down`

The `down` command runs in reverse. It cleans up Kubernetes objects first, then AWS resources, gracefully handling items that are "already gone."

---

## 4. State Management

`awsbnkctl` uses a dual-state approach:

1. **AWS Tags (Single Source of Truth):**
   - `awsbnkctl:cluster` = `<metadata.name>`
   - `awsbnkctl:component` = e.g., `vpc`, `subnet-public`
   - `awsbnkctl:managed` = `true`

2. **Local ID Cache (`state.env`):**
   A simple `KEY=VALUE` file stored in `.awsbnkctl/<cluster-name>/state.env`. This cache speeds up destruction (`down`) but the tool can fully recover and clean up a cluster just by reading AWS tags if the cache is lost.

---

## 5. BNK Interface Patterns

The `pattern:` field determines how TMM (Traffic Management Microkernel) interfaces with the network.

| `pattern:` | Topology | Binding | Internal Subnet | Min ENIs |
|---|---|---|---|---|
| `external-only` | External only | `host-device` | No | 2 |
| `dual-interface` | External + Internal | `host-device` | Yes | 3 |
| `sriov-external` | External only | `sriov / vfio-pci` | No | 2 (Experimental) |

*Note: `host-device` is treated as a legacy alias for `dual-interface`.*

---

## 6. Codebase Organization

| Component | Location |
|---|---|
| CLI Commands & Wiring | `internal/cli/` |
| AWS SDK Phases | `internal/aws/phases/` |
| Intent Validation | `internal/intent/` |
| Kubernetes Apply logic | `internal/k8s/` |
| State & Tagging | `internal/aws/state/`, `internal/aws/tags/` |
| Runnable Examples | `examples/` |

## 7. BNK release policy

`awsbnkctl` targets the **current BNK line** F5 has published on `repo.f5.com` —
`manifest.DefaultManifestVersion`, currently `2.4.0`. Every example and every
scenario is written against that default. The 2.3.x builds stay deployable: set
`bnk.manifestVersion` in `cluster.yaml` and the rest of the stack follows.

F5 writes the 2.4.0 manifest as `2.4.0-3.3175.0+0.0.380` in its docs. The chart
on `repo.f5.com` is tagged both `2.4.0` and `2.4.0-3.3175.0-0.0.380`, and the file
inside is `bigip-k8s-manifest-2.4.0.yaml` with `version: 2.4.0`. FLO matches the
CNEInstance `manifestVersion` against that file, so awsbnkctl uses `2.4.0`.

| Release manifest | FLO chart it ships with | Status |
| --- | --- | --- |
| `2.4.0` | `v2.30.0-0.5.2` | default |
| `2.3.3-3.2598.3-0.0.509` | `v2.21.13-0.0.64` | supported (last 2.3.x; the `release-2.3` branch tracks it) |
| `2.3.2-3.2598.3-0.0.392` | `v2.21.13-0.0.58` | supported (live-validated on `bnk-singapore-pe`) |
| `2.3.1-3.2598.3-0.0.304` | `v2.21.13-0.0.53` | supported |
| `2.3.0-3.2598.3-0.0.170` | `v2.21.13-0.0.28` | supported (original release awsbnkctl was built against) |

The table is `manifest.KnownReleases`. It exists because the F5 Lifecycle
Operator is installed by Phase 14 *before* the release manifest is available in
the cluster, so the operator/manifest pairing has to be known up front.
`Cluster.FLOVersion()` resolves it: an explicit `addons.flo.version` wins, then
the chart paired with `bnk.manifestVersion`, then the default release's chart
(with a `validate` warning when the manifest is a build the table does not
know). Add a row when F5 publishes a new build; the tags are listed by
`awsbnkctl manifest probe`.

What was checked against the 2.4.0 charts and CRD installer (2026-09-11): the
`CNEInstance` CRD only gained fields since 2.3.3, so the embedded template applies
to every build; the F5 CRDs the scenarios and examples use are all installed and
watched by the 2.4.0 controller (`k8s.f5net.com/v1` for F5BnkGateway and
F5SPKVlan, `k8s.f5net.com/v3` for F5SPKEgress, `k8s.f5net.com/v1alpha1` for
RoutingTemplate / GlobalRoutingConfig); the Gateway API extension group moved from
`gateway.k8s.f5net.com` to `gateway.k8s.f5.com` (L4Route `v1`, NetPolicy and
SecPolicy `v1alpha1`, formerly BNKNetPolicy and BNKSecPolicy), and the 2.4.0
controller no longer watches the old group, so 2.3.x manifests for those kinds do
not work on 2.4.0; 2.4.0 adds `Infra`, `GatewaySettings` and `EgressGateway`
(`gateway.k8s.f5.com/v1alpha1`), which awsbnkctl does not use yet; the routing
container is still ZebOS (the TMM chart default), so `ZEBOS_STATE=legacy` is set as
in F5's 2.4 examples; and the `format: int32` + `maximum: 4294967295` defect that
blocks Kubernetes 1.36 is still present in the 2.4.0 CRDs, so the 1.36 warning
stands.
