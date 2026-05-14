# Migrating to awsbnkctl

This guide covers moving an existing F5 BIG-IP Next for Kubernetes (BNK) on AWS deployment workflow over to `awsbnkctl`. It also covers in-tree upgrades between awsbnkctl versions and the migration path for teams coming from `roksbnkctl` on IBM Cloud.

Once the book (`book/src/`) is retargeted at AWS, it will be the canonical learning path for new users; this file is a focused reference for users who already have an environment, an automation pipeline, or a workspace from a previous version.

> **Status:** scaffolding. awsbnkctl has no shipped release yet — the sections below describe the migration *target* so the implementation knows what to honour. Each section will be tightened as the corresponding sprint lands.

## From manual EKS + BNK deployment

If you currently stand BNK up on EKS by hand — `terraform apply` against your own HCL, `aws eks update-kubeconfig`, `kubectl apply -f cert-manager.yaml`, hand-rolled FLO values, manually-uploaded FAR pull keys — awsbnkctl is a drop-in replacement for that workflow. It does not replace the Terraform itself; it embeds a vetted Terraform tree (cluster + cert-manager + FLO + CNEInstance + license + testing) and drives it with idempotent lifecycle verbs.

| Old (hand-rolled) | New (`awsbnkctl`) |
|---|---|
| Hand-edited `terraform.tfvars` | `awsbnkctl init` (interactive wizard writes `~/.awsbnkctl/<workspace>/config.yaml` and a derived `terraform.tfvars`) |
| `aws configure` / `AWS_PROFILE=…` | AWS credentials resolved via the standard chain (env → profile → instance role → SSO); see [Chapter 14 — Credentials resolver](./book/src/14-credentials-resolver.md) once written |
| `cd terraform && terraform init && terraform plan && terraform apply` | `awsbnkctl up` (runs init + plan + apply; idempotent and resumable) |
| `aws eks update-kubeconfig --name <cluster> --region <region>` | Auto-fetched post-apply; landed at `~/.awsbnkctl/<workspace>/state/kubeconfig` and pointed to via `KUBECONFIG` in `awsbnkctl shell` |
| Manual `iperf3` install on jumphost + manual port-forward | `awsbnkctl test throughput` (bundled image, k8s Job by default; no host install required) |
| Manual `dig` + comparing answers across vantages | `awsbnkctl test dns` (multi-vantage probe) |
| `terraform destroy` | `awsbnkctl down` |

## From `roksbnkctl` on IBM Cloud

awsbnkctl is a hard fork of roksbnkctl. The user-facing CLI surface (`init`, `up`, `test`, `down`, `doctor`, `k`, `self`) is intentionally preserved. The migration is mostly a swap of cloud-side primitives.

| roksbnkctl concept | awsbnkctl equivalent |
|---|---|
| ROKS cluster | EKS cluster (self-managed node group for SR-IOV) |
| `IBMCLOUD_API_KEY` env / keychain entry | `AWS_PROFILE` / `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` / `AWS_SESSION_TOKEN` |
| IBM Cloud Object Storage (FAR pull keys + license artefacts) | S3 bucket (server-side encrypted) — or ECR mirror for FAR images |
| IBM Trusted Profile (workload identity for FLO) | IRSA — IAM role for the FLO service account, bound via the EKS OIDC provider |
| OpenShift `oc` verbs (`roksbnkctl k …` aliased to `oc` where useful) | `awsbnkctl k …` — pure `kubectl` semantics, no OpenShift-specific verbs |
| `roksbnkctl ibmcloud …` passthrough | Dropped. AWS API calls are made directly via `aws-sdk-go-v2`; no shell-out CLI passthrough is planned. |

A `roksbnkctl` workspace **cannot** be migrated automatically — the underlying cluster and storage primitives differ. The recommended path is:

1. `roksbnkctl test` against your existing ROKS workspace to capture a baseline.
2. `awsbnkctl init` a fresh AWS workspace, accepting defaults where the AWS shape differs (instance types, node group min/max, VPC CIDR).
3. `awsbnkctl up` and `awsbnkctl test` against the new EKS workspace.
4. Cut over BNK clients (DNS, GSLB pointers) once the AWS workspace passes its test suite.
5. `roksbnkctl down` on the old workspace once cutover is verified.

## Between awsbnkctl versions

Per-version migration notes will land here as releases are cut. Until v0.1.0 ships, there is nothing to migrate between.

## Cross-references

The following book chapters will document the underlying mechanics referenced above once they are retargeted at AWS:

- Chapter 6 — Workspaces
- Chapter 12 — Workspace config
- Chapter 13 — Terraform variables
- Chapter 14 — Credentials resolver
- Chapter 17 — Execution backends

Until then, the equivalent chapters in the [roksbnkctl book](https://jgruberf5.github.io/roksbnkctl/book/) describe the same mechanics for the shared surface (workspaces, cred chain, execution backends, internalised kubectl). Swap "ROKS" → "EKS" and "COS" → "S3" mentally while reading.
