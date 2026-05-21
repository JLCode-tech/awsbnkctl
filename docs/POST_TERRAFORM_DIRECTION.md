# Post-Terraform Direction — Implementation Spec

**Status:** Approved 2026-05-21 (via grill-me session). Not yet implemented.
**Owner:** awsbnkctl maintainers
**Last updated:** 2026-05-21
**Companion docs:** [`docs/FORGE_MCP_INTEGRATION.md`](FORGE_MCP_INTEGRATION.md) (forge handoff already designed; this doc realigns the rest of the tool to the same model)

---

## 0 · TL;DR

Terraform was the wrong implementation choice for awsbnkctl. The aws-gpu-setup PoC (`/Users/j.lucia/Code/aws-gpu-setup/`) proved that an imperative awscli + YAML method delivers the same result with dramatically faster iteration. We adopt the **method**, not the code — porting the imperative phased shape into Go against the AWS SDK, keeping the Go binary as the typed contract surface for forge MCP and the single-binary distribution story.

**Bottom line:**
- Delete `terraform/`, `internal/tf/`, `embedded.go` (TF embed), `install_build_dependencies.sh`.
- Provisioning becomes sequential phase functions in Go, using only the AWS SDK (no `aws` CLI exec).
- Intent is a structured `cluster.yaml` per cluster.
- K8s manifests live in **variant directories** (`internal/k8s/manifests/<pattern>/`) selected by `cluster.yaml: pattern: …`.
- State = AWS tags (truth) + a small `.awsbnkctl/<cluster>/state.env` IDs cache (rebuildable from tags).
- Forge handoff stays as designed in `FORGE_MCP_INTEGRATION.md`, with one timing change: register on EKS-Active, not at the very end.

---

## 1 · Why (the four pains TF caused)

All four mattered — that's why the move is wholesale, not patch:

| Pain | Symptom | Fix in new model |
|---|---|---|
| **Distribution / binary fetch** | `internal/tf/fetch.go`, version pinning, embedded modules, `install_build_dependencies.sh` | TF binary stops shipping. Single Go binary. |
| **State-file fragility** | `terraform.tfstate` drift, `terraform.applied.tfvars` dance, lock contention, manual `state rm` recovery | AWS tags are truth. Local cache is regenerable. |
| **Two control planes** | TF for infra + bash/SDK for runtime = two mental models, divergent failure surfaces | One Go program, one mental model. |
| **Iteration speed** | Plan/apply cycles slow; errors opaque; hard for AI agents (forge/MCP) to reason about TF graph | Imperative, sequential, log-able. Mirror aws-gpu-setup's velocity. |

---

## 2 · Architectural commitments (locked)

| Layer | Decision |
|---|---|
| AWS writes + reads | **Strict Go SDK only.** No exec to `aws` CLI. `internal/aws/` grows to cover what TF did. |
| Provisioning shape | **Imperative phased functions** ported from aws-gpu-setup's `up.sh` / `down.sh`. NOT a reconciler framework. |
| Auth | **SSO sentinel pattern** from `aws-gpu-setup/lib/lab-core.sh`'s `aws_q` + `check_auth_or_die`, ported into a Go middleware wrapper around the SDK client. Up-front `sts.GetCallerIdentity` + mid-run phase-boundary checks. Hard-exit on `ExpiredToken` / `InvalidClientTokenId` with the exact `aws sso login --profile <X>` hint. |
| State | **AWS tags (truth) + `.awsbnkctl/<cluster>/state.env` IDs cache.** Required tag on every resource: `awsbnkctl:cluster=<name>`. Cache is rebuildable from tags. Loss of cache → tag-discovery fallback. |
| Intent | **`cluster.yaml`** — structured, Go-struct-validated, forge-MCP-readable. No vars.env emit. |
| K8s manifests | **Variant directories.** `internal/k8s/manifests/<pattern>/*.yaml` (e.g. `host-device/`, `sr-iov-tmm/`). `cluster.yaml: pattern: <name>` selects. Adding a third pattern = add a directory. Shared manifests in `internal/k8s/manifests/shared/`. |
| K8s apply path | client-go via existing `internal/k8s/apply.go`. Already strict (no kubectl exec). Confirmed by grep of `internal/k8s/*.go` — uses `k8s.io/apimachinery`, `k8s.io/cli-runtime`. |
| Idempotency | Per-call "tolerate already-gone" (swallow `NotFound` / `AlreadyExists`). Post-condition waits (`WaitUntilReady`) only where the AWS object has a delay-to-usable (EKS cluster active, node group active, NAT EIP unassociation, ENI detach). |
| Subcommand surface | `plan` dies. `up --dry-run` replaces it. |
| Destroy | Single interactive confirm + flags: `--keep-iam`, `--keep-keypair`, `--skip-bnk`, `--keep-forge-link`, `--yes`/`-y`. Idempotent reverse-order. |

---

## 3 · Forge integration timing (delta from existing plan)

Forge integration model is **locked in [`FORGE_MCP_INTEGRATION.md`](FORGE_MCP_INTEGRATION.md)** (peer-read; AWS is truth; awsbnkctl never asks forge for cluster state). This doc only changes two things:

| Aspect | Decision |
|---|---|
| **Register timing** | Fire on EKS-Active, **before BNK install**, not at end of `up`. Forge's own poll surfaces BNK-install progress in the GUI during the longest phase. Bootstrap STS-presigned kubeconfig has 15-min TTL, plenty of headroom. |
| **Failure semantics** | **Soft-fail with retry** (3 attempts, exponential backoff). AWS infra stays up. Link file `forge-link.json` marked `pending`. Operator runs `awsbnkctl forge register` to recover. Rationale: don't roll back expensive AWS infra for a localhost dev-server hiccup. |
| **Down + forge** | `awsbnkctl down` calls `forge unregister` **by default**. `--keep-forge-link` preserves the project record. Matches the `--keep-iam` / `--keep-keypair` flag family. |
| **Pattern variant visibility** | Stamp namespace label/annotation: `awsbnkctl.io/pattern: host-device`. No bnk-forge schema change required. Forge GUI can surface it later if it grows the feature. |

### Implementation status (slice 4, 2026-05-21)

Phase 09 (`Phase09ForgeRegister` / `Phase09ForgeRegisterDown`) is **implemented** in
`internal/aws/phases/phase09_forge_register.go` and wired into `runPhasedUp` /
`runPhasedDown` in `internal/cli/lifecycle.go`.

Key implementation notes:
- **MCP-first, REST fallback.** `forge.Register()` (MCP) is tried first. On a catalog-gap
  error (`IsMCPCatalogGapErr` in `internal/forge/rest.go`) the phase retries via
  `forge.RegisterREST()` which speaks directly to the forge REST API. This mirrors the
  kindbnkctl precedent (D-009): MCP is preferred but REST is the canonical fallback, not
  exceptional.
- **REST fallback credentials.** Hardcoded `admin/changeme` matching the localhost
  bnk-forge dev stack. Real forge auth is out of scope for slice 4.
- **Soft-fail retry loop.** 3 iterations × exponential backoff (1s → 3s → 9s). Uses
  `select { case <-time.After(d): case <-ctx.Done() }` so `ctx` cancellation propagates.
- **`Link.Status` field.** `internal/forge/link.go` `Link` struct gains a `Status` field
  (`"registered"` or `"pending"`). Empty string = `"registered"` for backward compat.
  `IsRegistered()` helper method used by Phase09 idempotency check.
- **`--keep-forge-link` flag.** Added to `downCmd` only; bound to `flagKeepForgeLink`
  (single-owner per the cobra anti-pattern comment in lifecycle.go).
- **`Clients.ForgeClient`.** Added to `internal/aws/phases/clients.go`. Populated via
  `AttachForgeClient(enabled, mcpURL)` called in `runPhasedUp` / `runPhasedDown` after
  `NewClients`, keeping the existing constructor signature unchanged.

---

## 4 · Repository layout — what changes

### Delete

- `terraform/` (entire directory — 1,480 LOC across 9 modules)
- `internal/tf/` (terraform binary fetcher, wrapper, vars handling)
- `embedded.go` (TF embed; recheck — may be repurposed for manifest embed)
- `install_build_dependencies.sh` (TF binary install)
- `internal/cli/tfvars.go` (TF-vars CLI command)
- TF-binary fetch logic in scripts/

### Keep (with growth)

- `internal/aws/` — already has typed wrappers for ec2, eks, iam, s3, vpc, servicequotas, sts. Grows to cover what TF did. **Stays the SDK-only contract surface.**
- `internal/k8s/` — already client-go-based and strict. No structural change.
- `internal/forge/` — no structural change; timing change in caller.
- `internal/cli/` — `up`, `down`, `status`, `doctor`, `inspect`, `init`, `forge`, `k`, `install`, `test` survive; `plan` is removed.
- `internal/config/`, `internal/exec/`, `internal/remote/`, `internal/ui/`, `internal/test/` — unaffected.

### New

- `internal/intent/` — `cluster.yaml` schema (Go struct), loader, validator.
- `internal/aws/awsmw/` — SDK middleware: SSO sentinel wrapper, structured logger, retry policy.
- `internal/aws/phases/` — phased provisioning functions: `Phase01Preflight`, `Phase02VPC`, `Phase03Subnets`, etc. Each is a top-level function that mutates a `*State` (in-memory) and tags every resource it creates.
- `internal/aws/tags/` — tag constants + helpers (`Required(name)`, `Component(name)`, `Pattern(name)`).
- `internal/aws/state/` — IDs cache reader/writer (`state.env`), tag-discovery fallback.
- `internal/k8s/manifests/<pattern>/*.yaml` — variant manifests. Embedded via `go:embed`.
- `internal/k8s/manifests/shared/*.yaml` — pattern-agnostic manifests (cert chain, license CR).
- `internal/k8s/render/` — Go `text/template` rendering against the cluster intent struct.
- `docs/POST_TERRAFORM_DIRECTION.md` — this file.

### Operator-visible files at runtime

- `cluster.yaml` — user-authored intent (typically in repo root or `examples/<topology>/`).
- `.awsbnkctl/<cluster-name>/state.env` — IDs cache (in `.gitignore`).
- `.awsbnkctl/<cluster-name>/forge-link.json` — forge project + cluster IDs.
- `.awsbnkctl/<cluster-name>/kubeconfig` — generated by `awsbnkctl` after EKS-Active.

---

## 5 · `cluster.yaml` schema (v1)

Kubernetes-style for familiarity; the `apiVersion` lets us evolve the schema later without breaking older intents.

```yaml
apiVersion: awsbnkctl/v1
kind: Cluster
metadata:
  name: jl-gpu-lab                  # used as the awsbnkctl:cluster=<name> tag value
  region: ap-southeast-2
  labels:                           # optional, propagated to AWS resource tags
    owner: jarrod
    purpose: gpu-lab

network:
  vpcCidr: 10.0.0.0/16
  azs:                              # explicit AZ list for reproducibility
    - ap-southeast-2a
    - ap-southeast-2b
  subnets:
    public:
      - cidr: 10.0.1.0/24
        az: ap-southeast-2a
      - cidr: 10.0.2.0/24
        az: ap-southeast-2b
    private:
      - cidr: 10.0.11.0/24
        az: ap-southeast-2a
      - cidr: 10.0.12.0/24
        az: ap-southeast-2b
  natGateways: 1                    # 1 (cost-optimized) or per-az (HA)

cluster:                            # OPTIONAL for slices 1+2 (network + IAM only)
                                    # REQUIRED for slices 3+ (EKS phases 08/10/11)
  kubernetesVersion: "1.30"         # default "1.30" if omitted
  nodeGroups:
    - name: default                 # required; forms <cluster>-ng-<name>; lowercase alphanumeric + hyphens
      instanceType: t3.medium       # default t3.medium
      desiredSize: 1                # default 1
      minSize: 1                    # default 1
      maxSize: 2                    # default 2
      diskSize: 50                  # GiB; default 50
      labels:                       # optional Kubernetes node labels
        node-role: worker
    # For GPU workloads (slice 5+):
    # - name: gpu
    #   instanceType: p5.48xlarge
    #   desiredSize: 1
    #   minSize: 0
    #   maxSize: 2
    #   diskSize: 200
    #   labels:
    #     node-role: gpu

pattern: host-device                # selects internal/k8s/manifests/host-device/
                                    # alternatives: sr-iov-tmm (planned)

addons:                             # OMITTED for tracer-bullet slice
  flo:
    enabled: true
    version: "1.5.0"
  license:
    jwt: file://./license.jwt
  cneInstance:
    cisVersion: "3.9.0"

tags:                               # additional tags applied to all AWS resources
  cost-center: "RnD-AI"
```

Field-level notes:
- `metadata.name` is **load-bearing**: it becomes the value of the `awsbnkctl:cluster=<name>` tag and the directory name under `.awsbnkctl/`. Must match regex `^[a-z][a-z0-9-]{0,38}[a-z0-9]$` (EKS cluster name rules).
- `network.azs` is **explicit by design** — we will not "pick N AZs from the region" for you, because that produces non-deterministic infra across runs.
- `cluster` block is **optional for slices 1+2** (network + IAM only); **required for slice 3+** (phases 08/10/11 error clearly if it's absent).
- `cluster.nodeGroups` must be **non-empty** when the `cluster` block is present.
- `cluster.nodeGroups[].name` must match `^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$` (lowercase alphanumeric + hyphens).
- `pattern` is **required** when the cluster.yaml drives k8s manifests. For the tracer-bullet slice (VPC + subnets only), it is ignored.
- Unknown fields are **errors**, not warnings, in v1.

---

## 6 · Phase ordering — `up`

Sequential phases, each a Go function. Each phase calls `checkAuthOrDie()` at its start (the SSO sentinel pattern), uses `internal/aws/awsmw`-wrapped SDK clients, and writes IDs to `state.env` as it creates resources.

```
up
├─ 00. preflight (Phase00Preflight)
│   ├─ load cluster.yaml + validate schema
│   ├─ sts.GetCallerIdentity (auth check, account verification)
│   ├─ ensure .awsbnkctl/<name>/ exists; load state.env if present
│   └─ servicequotas spot-check (vCPU, EIPs, NAT GW count for region)
├─ 02. vpc (Phase02VPC)               ◄── slice 1 (shipped)
├─ 03. subnets (Phase03Subnets)       ◄── slice 1
├─ 04. igw (Phase04IGW)               ◄── slice 1
├─ 05. nat gateway + EIP (Phase05NAT) ◄── slice 1
├─ 06. route tables (Phase06RouteTables) ◄── slice 1
├─ 07. iam: cluster role, node instance role, node instance profile (Phase07IAM) ◄── slice 2 (shipped)
├─ 08. eks cluster (Phase08EKSCluster)       ◄── slice 3 (shipped)
│       wait until ACTIVE (~10 min); capture endpoint/CA/OIDC URL/security group
├─ 09. forge register (slice 4: RESERVED — NOT YET IMPLEMENTED)
│       fires on EKS-Active, before node group; forge sees cluster while node group + BNK install proceed
├─ 10. eks node group (Phase10NodeGroup)     ◄── slice 3 (shipped)
│       wait until ACTIVE (~7 min); subnets: public only; AMI: AL2_x86_64
├─ 11. kubeconfig (Phase11Kubeconfig)        ◄── slice 3 (shipped)
│       write .awsbnkctl/<name>/kubeconfig (exec-auth via `aws eks get-token`)
│       ◄── CURRENT IMPLEMENTATION ENDS HERE
├─ 12. ecr mirror + s3 supply chain
├─ 13. k8s: apply manifests/shared/, then manifests/<pattern>/
│       (cert chain → license CR → CNEInstance → FLO)
└─ 14. postflight smoke + optional forge scan_cluster
```

> **Phase numbering note:** Phase 01 is reserved; the network phases are 02–06; IAM is 07.
> The original spec had IAM at §6.06 — the actual code uses 07 to leave room for future
> additions between network and IAM without renumbering existing phases.

Each phase function signature:

```go
type Phase func(ctx context.Context, intent *intent.Cluster, st *state.State) error
```

`State` holds:
- All known IDs (VPC ID, subnet IDs, EKS cluster ARN, …)
- Persisted to `state.env` on every successful phase
- Reloaded on re-run for idempotency

---

## 7 · Phase ordering — `down`

Reverse of `up`, with destructive guardrails:

```
down
├─ 00. preflight (interactive confirm unless --yes)
│   ├─ sts.GetCallerIdentity
│   ├─ load state.env; if missing, tag-discovery / name-based fallback
│   └─ unless --keep-forge-link: forge unregister
│
│   ◄── Future slices (slice 3+) insert here in reverse order
│
├─ 07. iam down (Phase07IAMDown) ◄── slice 2 (shipped)
│       remove role from profile → delete profile → detach + delete inline
│       policies on node role → delete node role → same for cluster role
├─ 06. route tables down (Phase06RouteTablesDown) ◄── slice 1
├─ 05. nat gateway + EIP down (Phase05NATDown)    ◄── slice 1
├─ 04. igw down (Phase04IGWDown)                  ◄── slice 1
├─ 03. subnets down (Phase03SubnetsDown)          ◄── slice 1
└─ 02. vpc down (Phase02VPCDown)                  ◄── slice 1
```

**Idempotency:** every phase tolerates "already gone" by swallowing the relevant AWS error codes:

| Service | "already gone" codes to swallow |
|---|---|
| ec2 | `InvalidVpcID.NotFound`, `InvalidSubnetID.NotFound`, `InvalidRouteTableID.NotFound`, `InvalidInternetGatewayID.NotFound`, `InvalidNatGatewayID.NotFound`, `InvalidAllocationID.NotFound`, `InvalidNetworkInterfaceID.NotFound` |
| eks | `ResourceNotFoundException` |
| iam | `NoSuchEntity` |
| s3 | `NoSuchBucket` |
| ecr | `RepositoryNotFoundException` |

**Post-condition waits** (port from aws-gpu-setup's `lib/lab-core.sh: wait_gone`):
- NAT GW deletion → wait for `State == deleted`
- EIP unassociation → wait for `AssociationId == ""` before release
- ENI detach → poll until ENI is gone before deleting its SG
- EKS cluster delete → wait until `DescribeCluster` returns `ResourceNotFoundException`

---

## 8 · SSO auth sentinel — Go port

aws-gpu-setup's pattern in `lib/lab-core.sh`:
1. Every `aws` CLI call goes through `aws_q`, which captures stderr to a tempfile and grep's for token-expiry strings.
2. On match, `aws_q` writes a sentinel file (`/tmp/aws-gpu-setup.auth-fail.$$`) and returns the error.
3. Every phase begins with `banner`, which calls `check_auth_or_die`, which reads the sentinel and hard-exits with the `aws sso login` hint.

Go port (sketch):

```go
// internal/aws/awsmw/sso.go
package awsmw

var authFail atomic.Bool

// Middleware wraps every SDK client. On ExpiredToken-class errors it sets
// authFail and returns the original error. Phase code calls CheckAuthOrDie()
// at the top of every phase function.
func WithSSOWatch(stack *middleware.Stack) error {
    return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("SSOWatch",
        func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
            middleware.DeserializeOutput, middleware.Metadata, error,
        ) {
            out, md, err := next.HandleDeserialize(ctx, in)
            if err != nil && isAuthError(err) {
                authFail.Store(true)
            }
            return out, md, err
        }), middleware.After)
}

func isAuthError(err error) bool {
    var ae smithy.APIError
    if !errors.As(err, &ae) { return false }
    switch ae.ErrorCode() {
    case "ExpiredToken", "ExpiredTokenException",
         "InvalidClientTokenId", "UnauthorizedException",
         "InvalidIdentityToken", "AccessDeniedException":
        return true
    }
    return strings.Contains(ae.ErrorMessage(), "SSO session")
}

func CheckAuthOrDie(profile string) {
    if !authFail.Load() { return }
    fmt.Fprintln(os.Stderr, "")
    fmt.Fprintln(os.Stderr, "FATAL: AWS auth failure detected mid-run — refusing to continue.")
    fmt.Fprintln(os.Stderr, "  Re-authenticate, then re-run:")
    fmt.Fprintf(os.Stderr,  "    aws sso login --profile %s\n", profile)
    os.Exit(99)
}
```

Apply via `config.WithAPIOptions` when constructing each AWS service client. Test by injecting a fake `ExpiredToken` error in unit tests.

---

## 9 · Tag scheme

Every AWS resource created by awsbnkctl carries:

| Key | Value | Purpose |
|---|---|---|
| `awsbnkctl:cluster` | `<cluster.metadata.name>` | **Required.** Identifies the cluster a resource belongs to. Drives `down` discovery. |
| `awsbnkctl:component` | `vpc`, `subnet-public`, `subnet-private`, `igw`, `nat`, `rtb`, `eks`, `nodegroup`, `iam-role`, `s3`, `ecr` | Per-resource category. Lets `inspect` produce structured output. |
| `awsbnkctl:pattern` | `host-device`, `sr-iov-tmm`, … | The data-path pattern used. Stamped on cluster-level resources. |
| `awsbnkctl:managed` | `true` | Marker for any future bulk-listing tool. |
| `Name` | `<cluster.metadata.name>-<component>` | Human-readable AWS console label. |

Any additional tags from `cluster.yaml: tags:` and `metadata.labels:` are merged in. Operator-applied tags on awsbnkctl-created resources are preserved across `up` re-runs (we only update the four `awsbnkctl:*` keys + `Name`).

---

## 10 · IDs cache file format

Path: `.awsbnkctl/<cluster-name>/state.env`. Format: shell-source-compatible `KEY=VALUE` (vars.env parity for human grep, even though Go reads it).

```
# Generated by awsbnkctl on 2026-05-21T15:32:01Z — do not edit by hand
CLUSTER_NAME=jl-gpu-lab
AWS_REGION=ap-southeast-2
VPC_ID=vpc-0abc123def456
IGW_ID=igw-0123abc
PUBLIC_SUBNETS=subnet-0aaa,subnet-0bbb
PRIVATE_SUBNETS=subnet-0ccc,subnet-0ddd
NAT_GW_ID=nat-0xyz
NAT_EIP_ALLOC=eipalloc-0qqq
PUBLIC_RTB=rtb-0pub
PRIVATE_RTB=rtb-0priv
EKS_CLUSTER_ARN=arn:aws:eks:ap-southeast-2:…
EKS_OIDC_URL=https://oidc.eks.ap-southeast-2.amazonaws.com/id/…
NODEGROUP_ARN=arn:aws:eks:…:nodegroup/…
FORGE_PROJECT_ID=42
FORGE_CLUSTER_ID=99
```

**Rules:**
- Written by `up` after every successful phase (so a mid-run failure leaves a valid partial cache).
- Read by `down` first; on missing/corrupt, fall back to tag-discovery.
- In `.gitignore` — never committed.
- `awsbnkctl inspect` prints the cache plus a tag-list diff (drift report).

---

## 11 · Variant manifest layout

```
internal/k8s/manifests/
├── shared/
│   ├── bnk-cert-chain.yaml
│   ├── far-pull-secret.yaml
│   ├── license-cr.yaml
│   └── otel-certs.yaml
├── host-device/
│   ├── cloud-network-mapping.yaml
│   ├── cneinstance.yaml
│   ├── f5spkvlan.yaml
│   ├── flo-values.yaml
│   ├── gatewayclass.yaml
│   └── network-attachment-defs.yaml
└── sr-iov-tmm/                 # planned, mirrors host-device with SR-IOV plumbing
    └── … (same filenames; different bodies)
```

- Embedded via `go:embed all:manifests` in `internal/k8s/render/embed.go`.
- Each YAML is a Go `text/template` rendered against a typed struct derived from `cluster.yaml`. Placeholders use Go's `{{ .Field }}` syntax (NOT envsubst — we have a strict Go SDK).
- Apply order: walk `shared/` first (alphabetical within), then walk `<pattern>/` (alphabetical within).
- Apply via existing `internal/k8s/apply.go` (client-go server-side apply).
- After apply, stamp the namespace with `awsbnkctl.io/pattern: <pattern>` label so forge GUI can surface it.

---

## 12 · Subcommand surface (post-TF)

| Command | Purpose | Notes |
|---|---|---|
| `awsbnkctl init` | Drop a `cluster.yaml` skeleton + `.awsbnkctl/` workspace dir | Optionally drop AGENTS.md (dpubnkctl pattern) |
| `awsbnkctl up [--config cluster.yaml] [--dry-run] [--phase N]` | Run all phases, or up to phase N, or print what would happen | `--phase N` lets you resume / debug |
| `awsbnkctl down [--yes] [--keep-iam] [--keep-keypair] [--skip-bnk] [--keep-forge-link] [--purge]` | Reverse-order destroy | `--purge` also removes `.awsbnkctl/<name>/` after success |
| `awsbnkctl status` | Tag-driven AWS query → table of resources for this cluster | NEVER asks forge |
| `awsbnkctl doctor` | Deeper health check: AWS + cluster reachability + BNK CR status | Uses Go SDK + client-go directly |
| `awsbnkctl inspect` | Print state.env + tag-list diff | Drift report |
| `awsbnkctl forge register / unregister / status` | Existing forge commands | Caller of `internal/forge/register.go` |
| `awsbnkctl k <kubectl-equivalent>` | Existing kubectl multiplexer | Unchanged |
| `awsbnkctl install / test` | Existing install + test commands | Unchanged |

**Removed:**
- `awsbnkctl plan` — fold into `up --dry-run`
- `awsbnkctl tfvars` — gone with TF
- `awsbnkctl cluster` (the old TF cluster subcommands) — superseded by `up`/`down`/`status`

---

## 13 · Tracer-bullet first slice (smallest deliverable)

**Scope:** `cluster.yaml` → tagged VPC + subnets + IGW + NAT + RTs via Go SDK, symmetric `up`/`down`, IDs cache write, idempotent re-run. **No EKS, no IAM, no k8s, no forge.**

**Deliverable PR contents:**

1. `internal/intent/cluster.go` — Cluster struct + YAML loader + validation.
2. `internal/aws/awsmw/sso.go` — auth sentinel middleware + `CheckAuthOrDie`.
3. `internal/aws/tags/tags.go` — tag constants + helpers.
4. `internal/aws/state/state.go` — `state.env` reader/writer + tag-discovery fallback.
5. `internal/aws/phases/phase00_preflight.go` — preflight phase.
6. `internal/aws/phases/phase02_vpc.go` through `phase05_routetables.go` — the five network phases.
7. `internal/cli/up.go` — replaces TF up; loads intent, runs phases through 05.
8. `internal/cli/down.go` — destroys phases 05→01 in reverse.
9. `examples/tracer/cluster.yaml` — minimal working example.
10. `internal/aws/phases/*_test.go` — unit tests with mocked SDK clients (one per phase + idempotency test that runs `up` twice).

**Acceptance criteria:**
- `awsbnkctl up --config examples/tracer/cluster.yaml` provisions a tagged VPC + subnets + IGW + NAT in a real account.
- Re-running `up` is a no-op (every phase reports "already exists, skipping").
- `.awsbnkctl/tracer/state.env` exists with all expected IDs.
- `aws ec2 describe-vpcs --filter Name=tag:awsbnkctl:cluster,Values=tracer` lists the VPC.
- Deleting `.awsbnkctl/tracer/state.env` and running `down` still works (tag-discovery fallback).
- `awsbnkctl down --yes` removes everything; second `down` is a no-op.
- Mid-run SSO expiry produces the auth-sentinel hard-exit with the `aws sso login` hint, not silent no-op.

**Out of scope for the tracer slice (subsequent slices):**
- IAM roles (slice 2)
- EKS cluster + node group (slice 3)
- forge register call (slice 4)
- k8s manifests + variants (slice 5)
- `inspect` / `doctor` / `status` polish (slice 6)

---

## 14 · Test strategy (sketch, deferred for own session)

Two layers planned, not yet detailed:
- **Unit:** SDK mocks via `aws-sdk-go-v2`'s middleware injection. Every phase function has a unit test that simulates "already exists," "create succeeds," "auth expired mid-run." Pattern matches the existing `internal/aws/*_test.go`.
- **Integration:** Real-account harness, region-scoped, cluster-name-prefixed (`tracer-ci-<sha>`). Skipped without `AWSBNKCTL_INTEGRATION=1` + valid SSO session. Tear-down in `t.Cleanup`. Aws-gpu-setup's `tests/` directory worth surveying for prior art.

---

## 15 · What we explicitly are NOT doing

- **Not** wrapping aws-gpu-setup as a subprocess. We adopt the *method*; we don't ship the bash.
- **Not** building a reconciler abstraction (`Reconcile()` interface, dependency graph engine). Sequential phase functions are sufficient and far easier to debug.
- **Not** adding a `--native` feature flag to coexist TF and Go paths. Greenfield build per user direction; current TF clusters cleaned up manually.
- **Not** changing the forge MCP integration model. Existing `docs/FORGE_MCP_INTEGRATION.md` stands; only the *timing* of register changes.
- **Not** introducing two-factor `--confirm-cluster` typo protection. The single confirm + per-cluster `.awsbnkctl/<name>/` directory is enough.

---

## 16 · Open questions (resolve before slice ships)

- `cluster.yaml`: should `network.azs` default from `metadata.region` (auto-pick first N) or stay strictly explicit? Current spec says explicit; revisit if it bites.
- IRSA OIDC URL: passed via `*State` struct between phases — confirm the EKS Go SDK returns the URL in `DescribeCluster.Identity.Oidc.Issuer` (it does) and that's our source.
- Helm SDK vs raw client-go for FLO / cert-manager install: deferred to k8s-side slice. Default lean is raw `unstructured.Unstructured` + Server-Side Apply via existing `internal/k8s/apply.go`. Revisit if FLO requires Helm-specific upgrade semantics.
- `awsbnkctl init` content: should it drop AGENTS.md + persona files (dpubnkctl style)? Likely yes — confirm when building `init`.

---

## 17 · Acknowledgements

- The aws-gpu-setup PoC (`/Users/j.lucia/Code/aws-gpu-setup/`) is the proof that an imperative awscli + YAML method works for this problem. Treat that repo as a reference implementation we are porting (not vendoring) into Go.
- The dpubnkctl architectural direction (mwiget/dpubnkctl) is the long-term polestar. Patterns to keep adopting as awsbnkctl evolves: PoC-as-repo state model, examples/ as named topologies, AGENTS.md numbered-gotcha catalog, validate command, journal/decisions.md.
