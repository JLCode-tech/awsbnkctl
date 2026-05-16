# S3 (and optional ECR) supply chain

BIG-IP Next for Kubernetes (BNK) pulls three artefacts at deploy time: an **F5 Application Runtime (FAR) pull-key archive** that lets the FLO operator authenticate against `dev-registry.f5.com` to fetch container images, a **subscription JWT** that proves a valid F5 entitlement, and a **CA cert chain** for the cluster's internal mTLS (cert-manager handles the last one; not in scope here). The first two are what BNK calls the **supply chain**, and `awsbnkctl` stages them in an S3 bucket so FLO can read them at apply time.

On the upstream `awsbnkctl` fork the same artefacts lived in IBM Cloud Object Storage (COS), with the FLO service account bound to an IBM Trusted Profile. On AWS the equivalent shape is **S3 + IRSA** — IAM Roles for Service Accounts. EKS issues an OIDC token to the FLO pod, AWS STS exchanges it for short-lived AWS credentials, and FLO reads the bucket through the resulting role. No static `AWS_ACCESS_KEY_ID` ever appears in a Kubernetes Secret or pod spec.

This chapter walks through how the bucket gets provisioned, how the IRSA trust chain hangs together, how `awsbnkctl init` stages the artefacts, the optional ECR-mirror story for air-gapped customers, and how to rotate the artefacts on day 2 without re-running `awsbnkctl up`. Cross-links: [PRD 07](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md) for the cluster + OIDC provider that this chapter consumes, [PRD 08](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md) for the design surface this chapter operationalises, and [Chapter 14](./14-credentials-resolver.md) for how AWS credentials get resolved at the host level (a separate concern from the in-cluster IRSA path).

## What's in the supply chain

Two objects, both small (a few kilobytes), both confidential:

| Object | What it is | Consumed by |
|---|---|---|
| `far-auth.tar.gz` | FAR repository pull credentials — the F5-internal artefact key that lets FLO download FAR container images from `dev-registry.f5.com` | `flo` module at install time |
| `subscription.jwt` | BNK subscription JWT — the licence the `CNEInstance` reconciler validates against F5's entitlement service | `flo` and `license` modules |

The exact object keys are configurable via Terraform variables; the defaults above match the `s3_supply_chain` module's `aws_s3_object` resource names. Other artefacts (a future schematic JSON, additional CA bundles, per-environment overrides) can sit in the same bucket alongside these two — the bucket policy and IRSA permissions scope to the whole bucket prefix, not to individual object keys, so adding new objects doesn't require any IAM churn.

Both objects are **end-user-supplied**. F5 distributes the FAR archive and the subscription JWT separately (subscription portal, sales-engineering hand-off, etc.); `awsbnkctl` doesn't fetch them. The operator points `awsbnkctl init` at local copies of both files at workspace creation time.

## The S3 bucket shape

The `terraform/modules/s3_supply_chain/` module provisions one bucket per workspace:

```
awsbnkctl-<workspace>-<random-suffix>
├── far-auth.tar.gz
└── subscription.jwt
```

The random suffix is a four-byte `random_id` Terraform resource — bucket names are globally unique across AWS, so a deterministic name (e.g. just `awsbnkctl-prod`) would collide as soon as a second customer tried to deploy. The `<workspace>-<random-suffix>` pattern keeps fresh deployments friction-free while letting operators in governance-bound environments override the name explicitly via `bucket_name_override`.

**Encryption.** The bucket is SSE-KMS-encrypted with a **customer-managed key** (CMK) created in the workspace's region. The default is CMK rather than SSE-S3 (AWS-managed) because the subscription JWT is sensitive enough to want CloudTrail-auditable decrypt operations and per-key rotation policy. The CMK costs about $1/month per region; v1.x evaluates SSE-S3 as a doc'd cost-sensitive alternative.

**Bucket policy.** The policy scopes `s3:GetObject` to the FLO IRSA role ARN. Bucket-policy semantics make a scoped `Allow` sufficient on its own (S3 is default-deny), but v1.0 ships an **explicit `Deny`** on every other principal as defence-in-depth against future hand-edits in the AWS console:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowFLOReadOnly",
      "Effect": "Allow",
      "Principal": { "AWS": "<flo_role_arn>" },
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::<bucket>/*"
    },
    {
      "Sid": "DenyEveryoneElse",
      "Effect": "Deny",
      "NotPrincipal": { "AWS": "<flo_role_arn>" },
      "Action": "s3:*",
      "Resource": [
        "arn:aws:s3:::<bucket>",
        "arn:aws:s3:::<bucket>/*"
      ],
      "Condition": {
        "StringNotLike": {
          "aws:PrincipalArn": "arn:aws:iam::<account-id>:role/aws-service-role/*"
        }
      }
    }
  ]
}
```

The `StringNotLike` on `aws-service-role/*` keeps AWS service-linked roles (replication, Inspector, etc.) functional; without it, a `NotPrincipal`-based Deny breaks any future service that wants read access. Operators who don't run any S3 service integrations can drop the condition; v1.x makes that configurable.

**Public access.** All four S3 Block-Public-Access flags are on: `BlockPublicAcls`, `IgnorePublicAcls`, `BlockPublicPolicy`, `RestrictPublicBuckets`. The supply-chain bucket is never reachable from the public internet.

**Versioning.** Off by default. The artefacts are operator-rotated (see [Day-2 ops](#day-2-ops-rotating-the-far-archive--jwt)) and the rotation flow overwrites the existing keys; bucket versioning would accumulate stale-but-still-decryptable copies. Operators who want a rotation audit trail enable versioning via `var.enable_bucket_versioning = true` and accept the storage cost.

## IRSA trust chain

This is the load-bearing piece — the trust hop FLO uses to read the bucket without a static key. Four resources collaborate:

```
┌──────────────────────────────────────────────────────────────┐
│ EKS cluster (PRD 07 — eks_cluster module output)             │
│                                                              │
│   ┌────────────────────────────────────────────────────┐     │
│   │ IAM OIDC provider                                  │     │
│   │   issuer:  https://oidc.eks.<region>.amazonaws.com │     │
│   │            /id/<cluster-OIDC-suffix>               │     │
│   │   thumb:   <SHA-1 of issuer cert>                  │     │
│   └────────────────────────────────────────────────────┘     │
│                          ▲                                   │
│                          │  Trust hop 1:                     │
│                          │  STS AssumeRoleWithWebIdentity    │
│                          │  using the projected SA token     │
│                          │                                   │
│   ┌──────────────────────┴─────────────────────────────┐     │
│   │ IAM role  awsbnkctl-<ws>-flo-supply-reader         │     │
│   │   trust policy: federated principal = OIDC ARN     │     │
│   │   trust condition:                                 │     │
│   │     <issuer>:sub = system:serviceaccount:          │     │
│   │       flo-system:flo-controller                    │     │
│   │     <issuer>:aud = sts.amazonaws.com               │     │
│   │   permission policy: s3:GetObject on supply bucket │     │
│   │                       + kms:Decrypt on the CMK     │     │
│   └────────────────────────────────────────────────────┘     │
│                          ▲                                   │
│                          │  Trust hop 2:                     │
│                          │  ServiceAccount annotation        │
│                          │  eks.amazonaws.com/role-arn = …   │
│                          │                                   │
│   ┌──────────────────────┴─────────────────────────────┐     │
│   │ ServiceAccount  flo-system/flo-controller          │     │
│   │   (created by the flo module, Sprint 3)            │     │
│   └────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────┘
```

The hops in plain English:

1. **PRD 07's `eks_cluster` module creates the IAM OIDC provider.** That's the federated identity provider AWS uses to trust tokens issued by the EKS control plane. The `oidc_provider_arn` + `cluster_oidc_issuer_url` outputs flow into Sprint 2's IRSA module.
2. **The `iam_irsa` module creates the IAM role** with a trust policy whose `Principal.Federated` is the OIDC provider ARN, and whose condition keys lock down which Kubernetes service account is allowed to assume the role. The condition uses `<issuer>:sub` to pin to a specific namespace + SA name (default `flo-system:flo-controller`) and `<issuer>:aud` to pin the audience claim (`sts.amazonaws.com` — the EKS pod-identity webhook sets this).
3. **The `flo` module (Sprint 3) creates the ServiceAccount** in `flo-system` with an `eks.amazonaws.com/role-arn` annotation pointing at the IRSA role ARN.
4. **At pod start, the EKS pod-identity webhook** injects two env vars (`AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE`) and a projected SA token volume into the FLO pod. The aws-sdk-go-v2 client inside FLO sees those env vars, calls `sts:AssumeRoleWithWebIdentity` with the token, gets back short-lived (one-hour-ish) AWS credentials, and reads the bucket.

The token is auto-rotated by the webhook every hour; FLO re-assumes when its credentials expire. No static key ever lands on disk in the cluster.

The condition keys are the part most likely to bite — a typo in the namespace or SA name produces a silent `AccessDenied` from STS rather than a clear "this SA isn't trusted" error. The `iam_irsa` module's `flo_namespace` + `flo_service_account_name` variables default to `flo-system` / `flo-controller` to match the Sprint 3 `flo` module's defaults; an operator who overrides one must override both.

## Uploading via `awsbnkctl init`

The end-to-end provisioning flow from a fresh dev box, after AWS credentials are set up and the cluster from chapter 8 is `Ready`:

```bash
$ awsbnkctl init
Workspace name [default]: prod
AWS region [us-east-1]:
EKS cluster name [auto-detect from workspace]: prod-bnk
FAR archive local path: /Users/op/Downloads/f5cne-far-auth-2.3.0.tar.gz
Subscription JWT local path: /Users/op/Downloads/f5cne-subscription-2026Q2.jwt
FLO namespace [flo-system]:
FLO service account name [flo-controller]:
  ✓ workspace written to ~/.awsbnkctl/prod/
  ✓ FAR archive path recorded: /Users/op/Downloads/f5cne-far-auth-2.3.0.tar.gz
  ✓ JWT path recorded: /Users/op/Downloads/f5cne-subscription-2026Q2.jwt
  • next: awsbnkctl up   (provisions bucket + uploads + IRSA)
```

`init` does **not** upload the artefacts itself. It writes the local paths into the workspace's generated `terraform.tfvars` (as `far_auth_file_local_path` and `jwt_file_local_path`) and stops. The actual upload happens during `awsbnkctl up`, which calls `terraform apply` against the `s3_supply_chain` module; the `aws_s3_object` resources stream the local files into S3 with `etag` computed from the file contents.

This split keeps the supply chain **reproducible from `terraform apply` alone**. Anyone who can read the workspace state and the local files can re-create the supply-chain state without re-running `init`; the bucket isn't a side effect of an imperative wizard step.

The `internal/aws/s3.go` Go helpers (`PutObject`, `HeadObject`, `GetObject`) exist for two adjacent jobs: `doctor` probes that head the bucket to verify the FLO role can actually read it, and a v1.x "rotate one object without re-running `terraform apply`" path. They don't run at init time.

## ECR mirror (optional)

Customers running EKS in fully air-gapped accounts — no outbound HTTPS to `dev-registry.f5.com` — can't have FLO pull FAR container images at apply time. The fall-back is to mirror the FAR images into the customer's own Elastic Container Registry (ECR) before `awsbnkctl up` runs, and rewrite FLO's image references to point at ECR.

`awsbnkctl` ships an optional `terraform/modules/ecr_mirror/` module, gated on `var.enable_ecr_mirror`:

```hcl
module "ecr_mirror" {
  source = "./modules/ecr_mirror"
  count  = var.enable_ecr_mirror ? 1 : 0

  region               = var.aws_region
  workspace_name       = var.workspace_name
  far_auth_local_path  = var.far_auth_file_local_path
  ecr_repository_names = var.far_image_names   # populated by a small helper
}
```

The module body creates one `aws_ecr_repository` per FAR image (typically a dozen for a BNK 2.3 release), then runs `skopeo copy` via a `null_resource` `local-exec` provisioner using the project's tools-image:

```bash
skopeo copy \
  --src-authfile=<far-auth.tar.gz extracted> \
  docker://dev-registry.f5.com/<image>:<tag> \
  docker://<account>.dkr.ecr.<region>.amazonaws.com/<image>:<tag>
```

`skopeo` is the right tool here because it's a single static binary, supports both Docker and OCI image formats, and doesn't require a running Docker daemon on the workspace host. The tools-image's Dockerfile pins `skopeo` to a known-good version (see `tools/docker/aws/Dockerfile`).

ECR mirror is a **v1.0 stretch** feature — the Sprint 2 task brief explicitly allows deferring it to a Sprint 3 follow-up if the bandwidth isn't there. v1.0 ships with the module wired but defaulted off (`enable_ecr_mirror = false`); v1.x makes it first-class for air-gapped customers and adds an automated FAR-image-sync workflow on `awsbnkctl up` re-runs.

When the mirror is enabled, FLO's image references get rewritten by the `flo` module (Sprint 3) to point at the ECR URIs instead of `dev-registry.f5.com`. The IRSA role gets a second permission policy attached — `ecr:GetDownloadUrlForLayer`, `ecr:BatchGetImage`, `ecr:GetAuthorizationToken` on the mirrored repos — so the same workload identity that reads the supply bucket also pulls the images.

## Day-2 ops: rotating the FAR archive / JWT

Three rotation moments. All three are operator-driven; nothing rotates automatically.

### Rotating the subscription JWT

The most common rotation. When a trial JWT expires or a production JWT arrives:

```bash
# Place the new JWT alongside the existing path
$ cp ~/Downloads/f5cne-subscription-2026Q3.jwt /Users/op/keys/subscription.jwt

# Re-run apply; the aws_s3_object etag changes, so Terraform sees the
# file change and re-uploads. Nothing else in the stack changes.
$ awsbnkctl up
  ...
  aws_s3_object.subscription_jwt: Modifying... [id=…]
  aws_s3_object.subscription_jwt: Modifications complete after 1s
  ...

# Force FLO to re-read the licence — delete the License CR; FLO's
# reconciler re-creates it from the new JWT within ~60-90 seconds.
$ awsbnkctl k delete license -n flo-system --all
```

If the operator points `subscription.jwt` at a fresh path (different filename), update `jwt_file_local_path` in `terraform.tfvars` before re-running `up`. Same flow either way — the file's content drives the etag, not the path.

### Rotating the FAR archive

When F5 rotates the FAR pull credentials (rare — annual-ish):

```bash
# Replace the local file
$ cp ~/Downloads/f5cne-far-auth-2026.tar.gz /Users/op/keys/far-auth.tar.gz

# Re-apply
$ awsbnkctl up

# Force the FLO pods to restart so they re-pull with the new credentials
$ awsbnkctl k delete pod -n flo-system -l app=flo
```

The pod-restart step is necessary because FLO caches the FAR auth in memory after the first successful pull; without the restart, the new credentials only get picked up at the next natural pod reschedule (could be hours).

### Rotating the IRSA role

The role is itself a Terraform resource; rotating its trust policy or permissions is an `awsbnkctl up` after an edit to the `iam_irsa` module's inputs. The role ARN doesn't change unless the role is destroyed and recreated, so FLO's ServiceAccount annotation stays valid across rotations. There's no separate "credential" inside the IRSA role to rotate — the role is the credential, and STS hands out short-lived tokens against it on demand.

If a customer wants to rotate the CMK that encrypts the bucket: it's a separate `aws_kms_key` resource; either set `var.kms_key_arn` to an existing CMK to swap, or let the module create a new one and re-key the bucket. Both flows are `awsbnkctl up`-driven; no out-of-band kubectl needed.

## Verifying the supply chain end-to-end

After `awsbnkctl up`, four sanity checks confirm the path works:

```bash
# 1. The bucket exists and is encrypted
$ aws s3api get-bucket-encryption --bucket $(terraform -chdir=.awsbnkctl/prod/state output -raw s3_bucket_name)
{ "ServerSideEncryptionConfiguration": { "Rules": [...SSE-KMS...] } }

# 2. The artefacts are uploaded
$ aws s3 ls s3://$(terraform … output -raw s3_bucket_name)/
2026-05-16 10:14:12       2412 far-auth.tar.gz
2026-05-16 10:14:13       1857 subscription.jwt

# 3. The IRSA role exists and is annotated on the FLO ServiceAccount
$ awsbnkctl k get sa flo-controller -n flo-system -o yaml | grep role-arn
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/awsbnkctl-prod-flo-supply-reader

# 4. The FLO pod can actually read the bucket — doctor probes this
$ awsbnkctl doctor
  ...
  ✓  aws s3 supply-chain bucket reachable
  ✓  aws iam:GetOpenIDConnectProvider — OIDC provider matches eks_cluster output
  ✓  aws irsa flo role assumable from flo-system/flo-controller
```

If the doctor row for IRSA assumability fails, the most common cause is a mismatch between the `iam_irsa` module's `flo_namespace` / `flo_service_account_name` defaults and what the Sprint 3 `flo` module actually creates. Compare the trust policy's `<issuer>:sub` value (visible via `aws iam get-role --role-name ...`) against `kubectl get sa -n <ns>` and the ServiceAccount annotation.

## Cross-references

- [PRD 07 — EKS cluster + SR-IOV node group](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md) — supplies the OIDC issuer URL + provider ARN this chapter consumes.
- [PRD 08 — S3 supply chain + IRSA workload identity](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md) — the design surface this chapter operationalises.
- [Chapter 12 — Workspace config](./12-workspace-config.md) — where `far_auth_file_local_path` + `jwt_file_local_path` land in the workspace YAML.
- [Chapter 13 — Terraform variables](./13-terraform-variables.md) — the auto-regenerated tfvar reference covering `s3_supply_chain` + `iam_irsa` + `ecr_mirror`.
- [Chapter 14 — Credentials and the AWS resolver chain](./14-credentials-resolver.md) — the host-side AWS credential chain (env / profile / instance role / SSO). IRSA is the in-cluster cred shape; the host-side chain is what `awsbnkctl up` itself uses to apply Terraform.
- [Chapter 24 — Day-2 ops](./24-day-2-ops.md) — `awsbnkctl logs flo` and `awsbnkctl k describe cneinstance` are the post-rotation verification surface.
- [Chapter 26 — Troubleshooting](./26-troubleshooting.md) — see [§"AWS credentials + auth"](./26-troubleshooting.md#aws-credentials--auth) for the `AccessDenied`-on-IRSA-assume diagnostic walkthrough (the most common supply-chain failure shape — `<issuer>:sub` typo in the trust policy condition keys, propagation lag between bucket-policy edit and the next FLO read, "FLO can't reach the bucket but doctor says it can" SA-annotation drift).
- [AWS IRSA docs](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) — canonical reference for the federated-identity flow.
