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
  kubernetesVersion: "1.32"   # mandated floor, and the default when omitted
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
- **`kubernetesVersion` has a mandated floor of 1.32** — see below.

### Kubernetes version policy

`cluster.kubernetesVersion` must be **1.32 or newer**. It is also the default when
the key is omitted. Anything lower is rejected by `validate`, before any AWS call:

```
cluster.kubernetesVersion "1.30" is below the mandated floor 1.32: 1.30/1.31 are
at or past the end of EKS standard support and are not exercised in CI; set 1.32
or newer
```

Two reasons for the floor. EKS moves 1.30 and 1.31 onto extended support, so a new
cluster on them starts out costing more for no benefit; and nothing below 1.32 is
exercised by CI any more, so allowing it would ship an untested path.

There is a soft **upper** bound as well. BNK 2.3 is known to install cleanly up to
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
