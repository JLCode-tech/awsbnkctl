# Architecture Guide

`awsbnkctl` is a single Go binary that provisions **F5 BIG-IP Next for Kubernetes (BNK)** onto AWS EKS. It communicates directly with AWS via the AWS SDK for Go, using a sequence of imperative, idempotent **phases** dictated by a single `cluster.yaml` intent file.

This document outlines the core architecture: the provisioning model, the intent format, the lifecycle, state management, and network patterns.

---

## 1. Design Philosophy

The tool is built around four core commitments:

| Concept | Approach |
|---|---|
| **SDKs Only** | AWS SDK for Go v2 (`internal/aws/`), client-go (`internal/k8s/`) and the Helm Go SDK (phase 14). No `aws`, `kubectl`, `helm` or Terraform on the host. |
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
  name: full-cluster          # lowercase letters, digits, hyphens; starts with a letter, 2-40 chars
  region: ap-southeast-2

pattern: dual-interface       # external-only | dual-interface | sriov-external

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

bnk:                          # Supply-chain credentials and release
  farArchive: ./cne_pull_64.json
  jwt: ./license.jwt
  manifestVersion: "2.4.0"    # default; a 2.3.x build can be pinned here
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
cluster.kubernetesVersion "1.33" is below the mandated floor 1.34: everything below it is past the end of EKS standard support (1.31, 1.32 and 1.33 all expired between 2025-11 and 2026-07), which forces extended-support pricing and is not exercised in CI; set 1.34 or newer (1.35 is the highest BNK 2.3 installs on)
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

1. **Network & IAM:** VPC, subnets, IGW, NAT, route tables, IAM roles and the data-plane security group.
2. **EKS Control Plane:** EKS cluster, optional Forge registration, VPC CNI prefix delegation, the metrics-server add-on.
3. **Nodes & Data Path:** Node group, kubeconfig, TMM node label, GPU and SageMaker opt-ins, secondary ENIs, optional jumphost and BIG-IP VE, interface discovery, demo staging, OIDC/IRSA.
4. **BNK Install & Activation:** EBS CSI and hugepages, cert-manager and Multus, FLO via the Helm SDK, optional LB controller, OTEL certs, network mapping and NADs, the CNEInstance with license, GatewayClass and Infra, the cluster-side repairs (cwc, dSSM probe, TMM log stream, pod-manager), the activation poll, optional BIG-IP onboarding, postflight.

The ordered list of all 41 phases is in [`PHASES.md`](PHASES.md).

### Cluster-side repairs

The fixes the phases apply to a running cluster (Multus token watch, metrics-server, TMM log stream, pod-manager, cwc, dSSM probe, controller EndpointSlice RBAC, controller IRSA, `TMM_K8S_ROUTES`, the `awsbnkctl-test` namespace) live in one registry, `internal/aws/phases/heal.go`. `awsbnkctl bnk heal` runs them on any 2.3 or 2.4 cluster and `awsbnkctl doctor --backend k8s` prints the detections. `bnk upgrade` and `bnk migrate-2.4` move a 2.3 cluster to 2.4 (section 7).

### `awsbnkctl down`

`down` walks the stages in reverse but is not a strict mirror: demo `Cleanup` hooks run first, `otel-certs` and `lb-controller` are removed before FLO, and a down-only `forge-benchmark-cleanup` step runs before the Forge unregister. Resources that are already gone are skipped. Flags: `--yes`, `--keep-forge-link`, `--keep-irsa`. The exact order is in [`PHASES.md`](PHASES.md).

---

## 4. State Management

`awsbnkctl` uses a dual-state approach:

1. **AWS Tags (Single Source of Truth):**
   - `awsbnkctl:cluster` = `<metadata.name>`
   - `awsbnkctl:component` = e.g., `vpc`, `subnet-public`
   - `awsbnkctl:managed` = `true`

2. **Local ID Cache (`state.env`):**
   A simple `KEY=VALUE` file stored in `.awsbnkctl/<cluster-name>/state.env`. AWS resources are re-discoverable from the `awsbnkctl:cluster` tag, so `down` reclaims the infrastructure without the cache. The Kubernetes-side steps need `KUBECONFIG_PATH` from state.env, and a few IAM objects are found by their deterministic name.

---

## 5. BNK Interface Patterns

The `pattern:` field determines how TMM (Traffic Management Microkernel) interfaces with the network.

| `pattern:` | Topology | Binding | Internal Subnet | Min ENIs |
|---|---|---|---|---|
| `external-only` | External only | `host-device` | No | 2 |
| `dual-interface` | External + Internal | `host-device` | Yes | 3 |
| `sriov-external` | External only | `sriov / vfio-pci` | No | 2 (Experimental) |

*Note: `host-device` is treated as a legacy alias for `dual-interface`. BNK patterns also need `desiredSize` ≥3 and an instance type with ≥16 vCPU and ≥64 GiB; `preflight` checks this.*

---

## 6. Codebase Organization

| Component | Location |
|---|---|
| CLI commands and wiring | `internal/cli/` |
| Provisioning phases and the heal registry | `internal/aws/phases/` (`heal.go`) |
| Intent (`cluster.yaml`) schema and validation | `internal/intent/` |
| Kubernetes client, `k` verbs, rendered CRs, embedded manifests, 2.3→2.4 migration, readiness scan, MCP session persistence | `internal/k8s/` (`render/`, `manifests/`, `migrate/`, `bnkscan/`, the MCP session package) |
| F5 release manifests (BOM), default version | `internal/manifest/` |
| Forge client (MCP first, REST fallback), benchmarks, telemetry | `internal/forge/`, `internal/genai/` |
| Execution backends: local, docker, k8s (one-shot Jobs in `awsbnkctl-test`), ssh | `internal/exec/`, `internal/remote/` |
| Scenarios, demos, tests, doctor, topology | `internal/scenarios/`, `internal/demo/`, `internal/test/`, `internal/doctor/`, `internal/topology/` |
| State and tagging | `internal/aws/state/`, `internal/aws/tags/` |
| Exported BNK runtime helpers | `pkg/bnk/` |
| Runnable examples | `examples/` |

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

A cluster built on a 2.3.x manifest moves to 2.4 in place with `awsbnkctl bnk
upgrade` (FLO chart + CNEInstance) followed by `awsbnkctl bnk migrate-2.4`
(F5SPKVlan, F5SPKEgress, F5BnkGateway and the policies into Infra,
GatewaySettings, EgressGateway, SecPolicy and NetPolicy); see
[`UPGRADE-2.4.md`](UPGRADE-2.4.md).

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
to every build; the 2.4.0 controller takes the TMM VLANs and the Gateway listener
context from the new `Infra` CR and ignores `F5BnkGateway` (live 2026-09-11: every
Gateway logged "Pre-computed 0 device contexts" and TMM got no virtual server), so
phase 23b applies an `Infra` CR instead of `F5SPKVlan` (the 2.4 release notes: Infra consolidates F5SPKVlan, F5SPKStaticRoute, VRF and VxLAN; in GatewaySettings mode the controller ignores those CRs), and every scenario ships a
`GatewaySettings` that its Gateway references through `infrastructure.parametersRef`;
the Infra also carries the TMM self-IP pools (tagged with the data-path availability zone,
which is what lets the controller count the zone's self IPs and activate the TMM), the VIP
listener pool, the egress tunnel defaults and the two static routes egress needs;
the TMM container gets `TMM_K8S_ROUTES=<EKS service range>` (the f5-tmm chart's `add_k8s_routes`, live 2026-09-14): once
the Infra default route exists, TMM would otherwise send the cluster service range out the external VLAN and lose DNS,
dSSM (iRule `table` commands time out and reset the connection, persistence profiles) and log forwarding inside its pod;
F5's chart derives the value from kubeadm-config or the OpenShift Network CR, EKS has neither, so phase 08 records
`serviceIpv4Cidr` as `EKS_SERVICE_CIDR` and the CNEInstance passes it;
the controller container gets `USE_GATEWAY_SETTINGS=true` from the CNEInstance, because the
2.4.0 f5ingress binary only starts the Infra and GatewaySettings reconcilers when that
env flag is set and neither FLO 2.30 nor the f5ingress chart sets it;
the controller also gets `MAX_ACTIVE_TMM_REPLICAS=32` (the chart default) because FLO 2.30 renders the
Deployment without it and the 2.4 controller then keeps every TMM in standby, and phase 23b adds
a ClusterRole/Binding for `get` on EndpointSlices, which the FLO-generated ClusterRole lacks;
the VLAN self IPs are allocated by the F5 IPAM controller from /27 pools
(`<subnet>.224`–`.254` around the nominal `.240`; the controller splits a pool into per-device
blocks and rejected both a single address and a /30), so phase 23b now assigns the allocated
address on the ENI and records it, where 2.3 pinned `.240` in phase 17;
`k8s.f5net.com/v3` F5SPKEgress and `k8s.f5net.com/v1` F5SPKStaticRoute are still installed
but only reach the validation webhook in GatewaySettings mode (live 2026-09-12: TMM egress
counters stayed at 0 with them), so egress runs on the 2.4 model: `EgressGateway`
(`gateway.k8s.f5.com/v1alpha1`) + `GatewaySettings` `egressConfigs` + the Infra `egressDefaults`
and `staticRoutes`; `k8s.f5net.com/v1alpha1` RoutingTemplate / GlobalRoutingConfig are still watched; the Gateway API extension group moved from
`gateway.k8s.f5net.com` to `gateway.k8s.f5.com` (L4Route `v1`, NetPolicy and
SecPolicy `v1alpha1`, formerly BNKNetPolicy and BNKSecPolicy), and the 2.4.0
controller no longer watches the old group, so 2.3.x manifests for those kinds do
not work on 2.4.0; the routing
container is still ZebOS (the TMM chart default), so `ZEBOS_STATE=legacy` is set as
in F5's 2.4 examples; and the `format: int32` + `maximum: 4294967295` defect that
blocks Kubernetes 1.36 is still present in the 2.4.0 CRDs, so the 1.36 warning
stands.
