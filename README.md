# awsbnkctl

A single-binary CLI for deploying F5 BIG-IP Next for Kubernetes (BNK) onto AWS EKS, managing its S3-backed supply chain, and validating the deployment with built-in connectivity, DNS, and throughput tests.

> **Status:** **Sprint 6 complete; v0.9-rc ready; v1.0 awaits spike.** All six AWS-retarget sprints (Sprint 0 identity rewrite → Sprint 5 book retarget → Sprint 6 hardening + security audit + release-artefact prep) have landed per [`docs/PLAN.md`](docs/PLAN.md). The v0.9 release candidate is structurally complete — `awsbnkctl up` / `down` / `test` / `doctor` work end-to-end, the embedded HCL stands up an EKS cluster + SR-IOV self-managed node group + S3 supply chain + IRSA, goreleaser produces six binary archives (linux/macOS/windows × amd64/arm64), the book ships at the AWS shape, `gosec` and the secrets scan are clean, and the v0.9-rc tag is ready to cut. The first stable **`v1.0`** tag waits on the operator-run PRD 07 spike (validating SR-IOV VFs on a real EKS `c5n.4xlarge` node) — see [`docs/PLAN.md`](docs/PLAN.md) §"What's deferred to post-v1.0". Until the spike clears, the binaries ship without the "officially supported on this account" tag; anyone with operator-run spike validation can cut v1.0 immediately. Forked from [`jgruberf5/roksbnkctl`](https://github.com/jgruberf5/roksbnkctl) (IBM Cloud ROKS); the roksbnkctl source is preserved on `upstream/main`, AWS-specific work lands on `main`.

## Why fork roksbnkctl instead of starting clean

Three of the five Terraform modules in roksbnkctl — `cert_manager`, `flo`, `cne_instance`, `license` — are pure Kubernetes manifests parameterised by namespace, image, and cert chain. They port unchanged. Only the cluster module, the object-store module, and the workload-identity wiring are IBM-Cloud-specific. The Go CLI scaffolding (cobra, terraform-exec wrapper, client-go k8s wrapper, miekg/dns probe, doctor framework, four-agent sprint pattern) is reusable as-is. Net new work concentrates in:

1. `internal/aws/` to replace `internal/ibm/` + `internal/cos/` (aws-sdk-go-v2)
2. `terraform/modules/eks_cluster/` to replace `terraform/modules/roks_cluster/`
3. S3 supply chain (or ECR mirror) to replace IBM Cloud Object Storage
4. IRSA / IAM OIDC to replace IBM Trusted Profiles
5. Self-managed node groups on ENA-enabled instance types (`c5n`, `m5n`) + SR-IOV CNI DaemonSet + Multus, for BNK data-plane SR-IOV requirements

## Quick start

```bash
# 1. Install the binary
go install github.com/JLCode-tech/awsbnkctl/cmd/awsbnkctl@latest

# 2. Interactive setup — region, VPC, EKS version, node-group shape.
awsbnkctl init

# 3. Make AWS credentials available (standard chain: env, profile, SSO, IRSA, IMDS).
export AWS_PROFILE=...

# 4. Plan + confirm + apply + auto-fetch kubeconfig.
awsbnkctl up

# 5. Run the built-in DNS + connectivity + throughput tests.
awsbnkctl test

# 6. Tear it all down.
awsbnkctl down
```

Same 4-command lifecycle as roksbnkctl: `init` → `up` → `test` → `down`. The canonical user guide lives at the published book: <https://JLCode-tech.github.io/awsbnkctl/book/>.

## Target architecture

| roksbnkctl uses (IBM Cloud) | awsbnkctl substitute | Terraform provider / module |
|---|---|---|
| ROKS cluster | EKS cluster + **self-managed** node group (SR-IOV-capable) | `terraform-aws-modules/eks/aws` + custom launch template |
| IBM Cloud VPC + Transit Gateway | VPC + (optional) Transit Gateway / VPC Lattice | `terraform-aws-modules/vpc` |
| IBM COS bucket (FAR + license artefacts) | S3 bucket, server-side encrypted | `aws_s3_bucket` + `aws_s3_object` |
| IBM Trusted Profile (workload identity) | IRSA — IAM role for service account | `aws_iam_role` + OIDC provider |
| `cert_manager` / `flo` / `cne_instance` / `license` modules | **Identical** — pure K8s manifests | `helm` / `kubernetes` providers |
| `ibmcloud` CLI in a tools image | `aws-sdk-go-v2` directly (no shell-out) | — |

### Data-plane decision (load-bearing)

BNK requires SR-IOV. EKS managed node groups don't expose SR-IOV cleanly, so awsbnkctl uses **self-managed node groups** on ENA-enabled instance types (`c5n.*`, `m5n.*`, `m5dn.*`) with a custom launch template, plus a Multus + SR-IOV CNI + SR-IOV device plugin DaemonSet stack on top of the standard AWS VPC CNI. This is the biggest open design surface; the trade-off vs. managed node groups (more user-managed AMI lifecycle in exchange for actual SR-IOV) is documented in [`docs/prd/07-EKS-CLUSTER-SRIOV.md`](docs/prd/07-EKS-CLUSTER-SRIOV.md) (drafted in Sprint 0; finalised against spike findings in Sprint 1).

## Prerequisites (target state)

`terraform` (1.5+) on `PATH` will be the only required host install. `awsbnkctl doctor` will report green on a stock dev box with terraform alone — every other tool the binary needs (`kubectl`, `aws`, `iperf3`, `dig`) is internalised:

| Surface | Internalised path |
|---|---|
| `kubectl` | `client-go` — `awsbnkctl k get/apply/describe/delete/logs/exec/port-forward` |
| `aws` | `aws-sdk-go-v2` — no shell-out, no bundled image |
| `iperf3` | Bundled tools image — `--backend k8s` runs the throughput probe as a one-shot Job |
| `dig` | `miekg/dns` — `awsbnkctl test dns` |

## What's in this repo

```
awsbnkctl/
├── cmd/awsbnkctl/             # binary entry point (renamed from cmd/roksbnkctl/ in Sprint 0)
├── internal/                  # Go packages (cli, tf, aws, k8s, cred, exec, test, doctor, …)
├── terraform/                 # the HCL deployment — embedded into the binary at build time
├── tools/                     # vendored tool images + cobra-md / tfvars-md reference generators
├── book/                      # mdBook sources — AWS-retargeted in Sprint 5; published at GitHub Pages
├── docs/                      # PLAN.md (sprint roadmap), prd/ (per-feature design specs)
├── agents/                    # four-role sprint pattern: architect / staff / validator / tech-writer
├── prompts/                   # checked-in per-sprint task briefs (auditable + reproducible)
└── scripts/                   # e2e test runners
```

The Terraform that drives the deployment is embedded into the binary at build time — every tagged release ships a matched CLI + HCL pair, eliminating skew between binary and TF.

## Development model

awsbnkctl uses the **four-role parallel-agent sprint pattern** inherited from roksbnkctl. See [`agents/README.md`](agents/README.md) for the role definitions (architect / staff / validator / tech-writer) and [`prompts/README.md`](prompts/README.md) for the sprint-kickoff playbook. Each sprint's task briefs are checked in verbatim under `prompts/sprint<N>/` for auditability and reproducibility.

The sprint roadmap lives in [`docs/PLAN.md`](docs/PLAN.md). Per-feature design rationale lives under [`docs/prd/`](docs/prd/).

## Relationship to roksbnkctl

awsbnkctl is a hard fork of [`jgruberf5/roksbnkctl`](https://github.com/jgruberf5/roksbnkctl). The `upstream` git remote points at the source repo so improvements can be cherry-picked across (`git fetch upstream && git log upstream/main ^main`). The shared surface — cobra scaffolding, Terraform driver, k8s client wrapper, DNS / connectivity / throughput tests, doctor framework, four-agent sprint pattern, mdBook documentation — should stay close. AWS-specific surface (cluster, storage, IAM, data-plane) diverges by design.

The awsbnkctl book — published at <https://JLCode-tech.github.io/awsbnkctl/book/> — is the canonical learning resource as of Sprint 5. The roksbnkctl book at <https://jgruberf5.github.io/roksbnkctl/book/> stays useful as the ROKS-shaped counterpart for the shared concepts (workspaces, execution backends, cred chain, internalised kubectl), but the AWS-shaped surface (cluster, storage, IAM, data plane) is now documented end-to-end in the awsbnkctl book.

## What this is *not*

- Not a Terraform authoring tool. The HCL under [`./terraform/`](./terraform/) is the source of truth for the deployment shape.
- Not a general-purpose AWS CLI — `aws` covers that. awsbnkctl's scope on AWS is the BNK supply chain: EKS for the cluster, S3 (or ECR) for prerequisite artefacts, IAM for what BNK consumes.
- Not a general-purpose Kubernetes CLI — `kubectl` covers that. The internalised `awsbnkctl k *` verbs make their workspace context easy to load without a host install.
- Not an arbitrary workload deployer. BNK is the workload; the iperf3 / nginx test fixtures exist only to validate it.

## Pointers

- **[`MIGRATING.md`](MIGRATING.md)** — for users coming from manual EKS + BNK deployment or from `roksbnkctl` on IBM Cloud.
- **[`CHANGELOG.md`](CHANGELOG.md)** — per-release change log; currently tracks the fork point and Sprint 0 prep.
- **[`docs/PLAN.md`](docs/PLAN.md)** — sprint-by-sprint roadmap.
- **[`docs/prd/`](docs/prd/)** — per-feature design rationale.
- **[`CONTRIBUTING.md`](CONTRIBUTING.md)** — how to contribute, run tests, add a chapter, and cut a release.
- Failing that, file issues at <https://github.com/JLCode-tech/awsbnkctl/issues>.

## License

[MIT](LICENSE), inherited from roksbnkctl.
