# Credentials and the AWS resolver chain

`awsbnkctl` handles four kinds of secrets: an AWS credential, a kubeconfig, an SSH private key, and the Terraform state file. Each has a different threat model, a different lookup chain, and a different rule for "what's safe to commit to a workspace".

This chapter is the user-facing distillation of [PRD 04 — credential propagation](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md). PRD 04 is the design surface for developers extending the credential system; this chapter is the operational surface for users who need to know "where does my key live, and how does the tool find it".

## The four secrets in scope

| Credential | Used by | Resolved from |
|---|---|---|
| **AWS credentials** | `aws` CLI (optional), terraform's AWS provider, AWS SDK calls | AWS standard chain: env → shared-config profile → SSO cached token → IMDS → container task role → web-identity token |
| **kubeconfig** | `kubectl` passthrough, `awsbnkctl k get/apply/...`, terraform's k8s + helm providers | `KUBECONFIG` env → `~/.kube/config` (kubectl-style) |
| **SSH private key** | The SSH client backing `--on` and the `ssh:<target>` execution backend | Per-target: file path, ssh-agent, or `tf-output:<name>` |
| **Terraform state** | The `terraform-exec` calls inside `awsbnkctl up`/`apply`/`destroy` | Workspace `state/terraform.tfstate` (filesystem only) |

Each has its own discovery rules. Walk them in turn.

## The AWS standard credential chain

The single most-used credential. Resolved by `internal/aws/client.go::NewClients`, which wraps `aws-sdk-go-v2`'s `config.LoadDefaultConfig` — exactly the chain the `aws` CLI and terraform's AWS provider walk. The chain walks these sources in order, and the first to yield a usable credential wins:

```
1. Environment variables
   AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY (+ optional AWS_SESSION_TOKEN)
   AWS_REGION

2. Shared-config files (~/.aws/credentials, ~/.aws/config)
   Profile from $AWS_PROFILE, or "default" if unset

3. SSO cached token (~/.aws/sso/cache/<hash>.json)
   When the active profile is sso_session-based

4. EC2 IMDSv2 (the instance role of the host)
   Used automatically when running on an EC2 instance with an attached role

5. ECS / EKS container task role
   AWS_CONTAINER_CREDENTIALS_RELATIVE_URI in env, surfaced by the task scheduler

6. Web identity token (OIDC)
   AWS_WEB_IDENTITY_TOKEN_FILE + AWS_ROLE_ARN — the GitHub Actions OIDC path
```

The first source that yields a non-expired credential wins. The SDK reports the **source** that won via a string like `EnvConfigCredentials`, `SharedConfigCredentials`, `SSOProviderCredentials`, `IRSACredentials`, etc. `awsbnkctl doctor` surfaces this as the `aws creds` row's `resolved via <source>` text so you can see which link is actually winning when multiple are configured.

### Source 1 — Environment variables

The most explicit path. If you've gone to the trouble of setting `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`, you've made a deliberate choice. Pre-existing CI pipelines, `direnv` setups, and one-off operator overrides all live here.

```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_REGION="us-west-2"
# optional, for short-lived role-assumption credentials:
export AWS_SESSION_TOKEN="..."
```

### Source 2 — Shared-config profile

The everyday path on a developer laptop. `aws configure` writes `~/.aws/credentials` (the secret bits) and `~/.aws/config` (the region + role-chaining metadata). Each named profile is a self-contained credential.

```ini
# ~/.aws/credentials
[default]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[awsbnkctl-dev]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

# ~/.aws/config
[default]
region = us-west-2

[profile awsbnkctl-dev]
region = us-west-2
# Optional role-chaining:
# role_arn = arn:aws:iam::123456789012:role/dev-role
# source_profile = default
```

Pin the profile per shell:

```bash
export AWS_PROFILE=awsbnkctl-dev
```

…or per workspace via `aws.profile` in `config.yaml` ([Chapter 12](./12-workspace-config.md)).

### Source 3 — SSO cached token

The recommended path for AWS organisations using IAM Identity Center (formerly AWS SSO). The `aws sso login` flow opens a browser, completes the device-code dance, and caches a short-lived token under `~/.aws/sso/cache/<hash>.json`. Subsequent SDK calls exchange that token for STS credentials transparently.

```bash
# In ~/.aws/config:
# [profile dev-sso]
# sso_session = my-org
# sso_account_id = 123456789012
# sso_role_name = DeveloperAccess
# region = us-west-2

aws sso login --profile dev-sso
export AWS_PROFILE=dev-sso
awsbnkctl doctor
# ✓ aws creds   resolved via SSO (profile=dev-sso, session=my-org)
```

The cached token typically lives 1-8 hours; when it expires, `aws sts get-caller-identity` returns `ExpiredToken` and you re-run `aws sso login`.

### Source 4 — EC2 instance role (IMDS)

When `awsbnkctl` runs on an EC2 instance with an attached IAM role, the SDK pulls short-lived credentials from the Instance Metadata Service v2 (IMDSv2) at `http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>`. No host configuration needed — the role is the instance's identity.

This is the right path for the **bastion EC2 instance** awsbnkctl auto-provisions: when you run `awsbnkctl --on jumphost up`, the bastion's instance role provides credentials to the `terraform apply` running over SSH.

### Source 5 — Container task role (ECS / EKS)

When `awsbnkctl` runs inside an ECS task or EKS pod with a task-role / IRSA-bound service account, the SDK reads the credentials endpoint published in the task's env (`AWS_CONTAINER_CREDENTIALS_RELATIVE_URI`). This is how the in-cluster ops pod ([Chapter 19](./19-in-cluster-ops-pod.md)) authenticates to AWS without any static keys in its spec.

### Source 6 — Web-identity token (OIDC)

The path for **GitHub Actions** (and any other OIDC-issuing CI). GitHub mints a JWT representing the workflow; AWS STS exchanges it for credentials via `sts:AssumeRoleWithWebIdentity`. The SDK reads `AWS_WEB_IDENTITY_TOKEN_FILE` (path to the JWT) + `AWS_ROLE_ARN` (the role to assume) from env.

Example GitHub Actions snippet:

```yaml
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: arn:aws:iam::123456789012:role/awsbnkctl-ci
    aws-region: us-west-2
```

After the action runs, `AWS_WEB_IDENTITY_TOKEN_FILE` + `AWS_ROLE_ARN` + `AWS_REGION` are set in the workflow env; `awsbnkctl` resolves credentials transparently.

## Where the AWS chain lives in the tree

`internal/aws/` houses the AWS standard chain. The key types:

- **`internal/aws/client.go::NewClients`** — wraps `config.LoadDefaultConfig` and constructs the typed AWS service clients (`sts`, `ec2`, `eks`, `s3`, `iam`) for the rest of the codebase to reach for.
- **`internal/aws/client.go::CredentialsConfigured`** — returns the resolved provider source string (e.g., `"EnvConfigCredentials"`, `"SharedConfigCredentials"`, `"SSOProviderCredentials"`) — used by `awsbnkctl doctor` for the `aws creds` row's detail text.
- **`internal/aws/sts.go::Clients.CallerIdentity`** — wraps `sts:GetCallerIdentity` so doctor and the cluster-register flow can verify the chain end-to-end.

In-cluster IRSA credentials are auto-injected by the EKS-managed pod-identity webhook — not by any awsbnkctl code. When the FLO pod (or the ops pod) lands on a node, the webhook intercepts the pod admission and projects an OIDC token + the env vars (`AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE`) the SDK needs. The `internal/aws/` chain in the pod's process picks them up automatically via Source 6 of the standard chain.

The historical `internal/cred/` package (carried from the upstream `roksbnkctl` codebase for back-compat-naming-only) is deprecated and scheduled for deletion. No production caller materialises a non-empty value from it; the AWS standard chain is the only credential path in v0.9 and beyond.

## kubeconfig discovery

Different chain, different rules. `awsbnkctl` discovers the kubeconfig the same way `kubectl` does — two sources, in this order:

```
1. KUBECONFIG environment variable (first existing path in a colon-separated list)
2. ~/.kube/config
```

This is the kubectl-standard discovery chain. Whatever you've already taught `kubectl` to read, `awsbnkctl` reads too.

`cluster up`'s post-apply step writes the admin kubeconfig to `~/.kube/config` (mode `0600`) by default — so the second source in the chain is also the destination of the tool's own output, and the same `KUBECONFIG`-overrides-everything rule applies. If `KUBECONFIG` is set when `cluster up` runs, the generated kubeconfig lands at that path instead.

The kubeconfig EKS produces references the IAM authenticator (`aws-iam-authenticator`-equivalent built into `kubectl` since v1.24) — when `kubectl` makes an API call, it shells out to `aws eks get-token --cluster-name <name>` (or the equivalent SDK call when `awsbnkctl k` is used) to mint a short-lived token. The token is signed with the AWS credentials from the standard chain above; whichever profile is active when `kubectl` runs is the identity the cluster sees.

### When the file is missing

If neither source yields a kubeconfig, commands that need one error with:

```
error: no kubeconfig: KUBECONFIG env not set, ~/.kube/config not present.
       Run `awsbnkctl cluster up`, `awsbnkctl cluster register <name>`,
       or set KUBECONFIG (e.g. `aws eks update-kubeconfig --name <cluster>`).
```

The remediation message tells you which path to take. `cluster register <name>` is the path for an existing cluster you want to adopt without re-creating it (see [Chapter 9](./09-registering-existing-cluster.md)).

### File permissions

`cluster up` writes `~/.kube/config` `chmod 0600` (owner read/write only). It contains the cluster CA certificate + the authenticator invocation. Don't commit it, don't email it, don't `cat` it in screen-shared sessions. (Note: unlike static bearer tokens, the kubeconfig itself doesn't contain a long-lived credential — the bearer token is minted on each call. But the CA cert + the cluster endpoint are still inventory you don't want leaked.)

## SSH private keys

Per-target, not per-workspace. Each entry under `targets:` in `config.yaml` declares exactly one of:

| Source | Form | Notes |
|---|---|---|
| **File** | `key_path: ~/.ssh/id_ed25519` | Standard OpenSSH key formats. Tilde expansion honoured. |
| **Agent** | `key_source: agent` | Talks to ssh-agent over `$SSH_AUTH_SOCK`. Linux/macOS only at v1.0; Windows ssh-agent named-pipe support is on the v1.x roadmap. |
| **TF output** | `key_source: tf-output:bastion_shared_key` | Reads from terraform state at connect time; never written to disk separately. |

The `tf-output:` form is the auto-discovered bastion path — the upstream HCL provisions a `tls_private_key` resource per cluster create, marks it `sensitive`, and surfaces it as a terraform output. `awsbnkctl` reads the output via `terraform output -raw` at SSH-connect time, never persists it, and the key only exists in TF state plus in memory during a connect.

[Chapter 15 — SSH targets](./15-ssh-targets.md) is the deep reference for the `targets:` block; this chapter just notes the credential-side framing.

## Terraform state

`~/.awsbnkctl/<workspace>/state/terraform.tfstate` is the workspace's terraform state file. It contains:

- AWS resource identifiers (cluster ARN, OIDC provider ARN, bucket name, IAM role ARNs)
- Generated TLS private keys (the bastion shared key referenced above)
- Sensitive outputs that the modules expose

It does **not** contain long-lived AWS access keys or secret access keys — those are sourced from the standard chain at apply time, not embedded into state. STS-issued session tokens are also short-lived and not durably persisted.

That said, the state file is still **sensitive**: an attacker with read access can learn your cluster identity, your IRSA role ARNs, your bucket names, and the bastion's SSH private key. The file mode is `0600`; the parent directory is `0700`. Backup the workspace dir intact, never commit it to git, treat compromise of the state file as compromise of every secret it contains.

There is no separate "TF state credential" — the file's filesystem ACL is the only access control. Remote state (`backend "s3"`) is on the v1.x roadmap; at v0.9 the local file is the only path.

## What's safe to commit vs not

A short rule:

```
SAFE TO COMMIT:    nothing in ~/.awsbnkctl/<workspace>/
NOT SAFE:          everything in ~/.awsbnkctl/<workspace>/
```

The longer version, by file:

| Path | Commit? | Why |
|---|---|---|
| `~/.awsbnkctl/<ws>/config.yaml` | **No** | Documents your AWS account, region, cluster name, instance types — useful inventory for an attacker. No credentials in this file (rejected at load), but the inventory is enough. |
| `~/.awsbnkctl/<ws>/state/kubeconfig` | **No** | Cluster CA + authenticator invocation; not directly a secret, but tied to the cluster. |
| `~/.awsbnkctl/<ws>/state/terraform.tfstate` | **No** | The bastion private key, every ARN, every output. |
| `~/.awsbnkctl/<ws>/state/terraform.tfvars` | **No** | Generated; references no secrets directly but documents resource layout. |
| `~/.awsbnkctl/<ws>/terraform.tfvars.user` | **Maybe** | If you've kept secrets out (no inline keys), it's just config. Audit before committing. |
| `~/.awsbnkctl/<ws>/cluster-outputs.json` | **No** | Cluster identity + supply-chain bucket name. Not directly a secret but tied to the workspace. |
| `~/.awsbnkctl/known_hosts` | **Yes (if you want)** | Host-key fingerprints; not a secret. Same threat model as OpenSSH's `~/.ssh/known_hosts`. |

The simplest policy: a `.gitignore` that excludes the entire `~/.awsbnkctl/` tree. If you really want to share a workspace skeleton with a colleague, send the `config.yaml` and let them re-run `awsbnkctl init` against their own AWS account.

## Backend-specific cred propagation

The credential-propagation rules differ per backend. All four backends ship at v0.9:

| Backend | Where creds live | Mechanism |
|---|---|---|
| `local` | The user's environment | `os/exec` inherits parent env (AWS_PROFILE, AWS_REGION, AWS_ACCESS_KEY_ID, etc.) |
| `docker` | Caller's env, propagated by reference | `docker run --env AWS_PROFILE -v ~/.aws:/root/.aws:ro` — shared-config files bind-mounted read-only; the SDK in the container reads them the same way the host SDK does |
| `k8s` | IRSA-bound ServiceAccount | The ops pod's SA is annotated `eks.amazonaws.com/role-arn=<role>`; the EKS pod-identity webhook injects the OIDC token + env vars; no static keys ever land in the pod spec |
| `ssh` | Remote env or instance role | `ssh -o SetEnv=AWS_PROFILE=...` for env-based; the bastion's own EC2 instance role for the everyday case (no env needed) |

Each backend's "where creds live" surface is summarised in [Chapter 17 — Execution backends](./17-execution-backends.md); the design rationale is in [PRD 04](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md).

The user-facing invariant across all four: you put the credentials into one of the AWS standard chain's sources, and `awsbnkctl` figures out the rest. You don't have to learn four different credential APIs to use four different backends.

## IRSA — the in-cluster credential path

The resolver chain above governs **host-side** AWS credentials — what `awsbnkctl up` sees on the workspace machine when it shells out to `terraform apply`. There's a separate, **in-cluster** credential shape used by BNK's FLO operator and the awsbnkctl ops pod: **IRSA** (IAM Roles for Service Accounts).

The chain in cluster:

1. EKS issues a projected OIDC token to the Pod's ServiceAccount.
2. The pod-identity webhook injects `AWS_WEB_IDENTITY_TOKEN_FILE` + `AWS_ROLE_ARN` into the Pod's env when it admits the Pod.
3. The AWS SDK in the Pod's process (the awsbnkctl binary, the FLO operator, etc.) walks the standard chain; Source 6 (web-identity) wins because the env vars are populated.
4. STS exchanges the OIDC token for short-lived AWS credentials (`sts:AssumeRoleWithWebIdentity`), scoped to the IAM role.
5. The Pod's process reads S3, calls EKS APIs, whatever — all under the IAM role's policy.

No static `AWS_ACCESS_KEY_ID` ever lands in a Kubernetes Secret or pod spec. The trust chain is `<flo ServiceAccount> → <IRSA IAM role> → <EKS OIDC provider> → <IAM>`, with the supply-chain bucket policy scoped to the IRSA role ARN.

[Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) is the deep reference for the IRSA trust chain and the bucket-policy shape that makes it work.

## The redactor

`awsbnkctl` writes a fair amount to its own logs (stdout, stderr) — terraform plan output, SDK error traces, debug dumps. Anywhere we can plausibly print an AWS secret access key (because a downstream tool printed it, because an error message included it, because a debug trace dumped a struct), the redactor masks it before the bytes leave the binary.

What gets redacted:

- The `AWS_SECRET_ACCESS_KEY` value, anywhere it appears in `Stdout` or `Stderr` of an exec backend's `RunOpts`. Replaced with `[REDACTED]`.
- The `AWS_SESSION_TOKEN` value (the same shape — short-lived but still sensitive).
- The same values in `awsbnkctl`'s own log output.

What does **not** get redacted:

- Output captured by callers via `-o yaml`/`-o json` for resources that legitimately contain secrets (e.g., a `Secret` returned from `awsbnkctl k get`). The redactor doesn't know about Kubernetes resource semantics.
- Output from a tool you ran outside `awsbnkctl` (e.g., piping to `tee` after invoking `terraform` directly). The redactor only sees bytes that pass through the exec backend's `Stdout`/`Stderr` writers.
- The terraform state file. State is on-disk; the redactor is an in-memory stream filter. (As noted above, state doesn't contain long-lived secret access keys anyway — but the bastion private key is in there.)

The implementation is `internal/exec/redact.go` — a wrapping `io.Writer` with byte-comparison redaction and cross-write prefix buffering (so a secret split across two `Write` calls still gets masked). The matcher uses the resolved secret values verbatim (known strings at run-time) rather than a generic "looks like an AWS key" pattern, to avoid false positives on legitimate output.

PRD 04's acceptance criteria require that the secret access key never appears in `docker inspect`, `ps -ef`, `kubectl get pods/events -o yaml`, or `kubectl describe pod`. The redactor is the defence-in-depth layer; the per-backend cred-propagation rules (especially the docker bind-mount of `~/.aws` rather than env-by-value, and the IRSA path for k8s) are the primary controls.

## Cross-references

- [PRD 04 — credential propagation across execution backends](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md) — the full design.
- [Chapter 12 — Workspace config](./12-workspace-config.md) — the `aws:` block schema.
- [Chapter 13 — Terraform variables](./13-terraform-variables.md) — why AWS credentials don't go in tfvars.
- [Chapter 15 — SSH targets](./15-ssh-targets.md) — the SSH key sources.
- [Chapter 17 — Execution backends](./17-execution-backends.md) — backend-specific cred mechanics.
- [Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) — IRSA, the in-cluster cred shape FLO uses to read S3.
