# The in-cluster ops pod

The k8s execution backend has two execution patterns: a **long-lived ops pod** for ad-hoc commands, and **one-shot Jobs** for throughput tests, DNS probes, and other per-invocation workloads. [Chapter 17 §"K8s backend"](./17-execution-backends.md#k8s-backend) covered the **interface mechanics** — how `Backend.Run` dispatches into either pattern.

This chapter is the reference for the **pod itself**: what `awsbnkctl ops install` deploys, what RBAC it grants, where credentials live, how to rotate them, and how to debug when something goes wrong.

If you've never run `awsbnkctl ops install`, you can read this chapter front-to-back; otherwise the [§ Operability](#operability) section near the end is the troubleshooting jump-off point.

## What the ops pod is

A long-lived pod in the `awsbnkctl-ops` namespace, running an image bundled with the tools `awsbnkctl` may want to invoke cluster-side: `aws` CLI plus `kubectl` as a fallback, with `oc` and `terraform` reserved for future iterations.

The pod sits idle waiting for `kubectl exec` calls. Each `awsbnkctl exec -- --backend k8s …` invocation routes through `client-go`'s SPDY executor, runs the wrapped tool inside the existing pod, streams stdout/stderr back, and returns the exit code. No pod create/start latency between invocations — a session of twenty `aws` commands pays the startup cost once.

Compared to the one-shot Job pattern (used for `iperf3` and the DNS probe), the ops pod trades a bit of resource-usage idle-state for substantially lower per-call latency. It's the right shape when you want to debug interactively or run many small commands.

## `awsbnkctl ops install`

Idempotent setup. Run once per cluster; re-run any time you want to refresh the image, rotate the API key Secret, or recover from a partial uninstall.

```bash
awsbnkctl ops install
```

What it does, step by step:

### 1. Create the namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: awsbnkctl-ops
  labels:
    app.kubernetes.io/name: awsbnkctl
    app.kubernetes.io/component: ops-pod
```

The `awsbnkctl-ops` namespace is dedicated to the long-lived pod. Separate from `awsbnkctl-test` (where one-shot Jobs run) so RBAC can be scoped per namespace — see [§ RBAC](#rbac-the-clusterrole-rules) below.

### 2. Create the ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: awsbnkctl-ops
  namespace: awsbnkctl-ops
```

The pod runs as this SA. Its projected token is auto-mounted at `/var/run/secrets/kubernetes.io/serviceaccount/`, which is what the bundled `kubectl` uses for in-cluster authentication. The AWS APIs key (a separate credential) reaches the pod through a Kubernetes Secret — see [§ Credential propagation](#credential-propagation) below.

### 3. Create the ClusterRole + ClusterRoleBinding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: awsbnkctl-ops
rules:
- apiGroups: [""]
  resources: ["pods", "pods/exec", "pods/log"]
  verbs:     ["get", "list", "watch", "create", "delete"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs:     ["get", "list", "watch", "create", "delete"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs:     ["get", "list"]
  resourceNames: ["awsbnkctl-ibm-creds"]
- apiGroups: [""]
  resources: ["services"]
  verbs:     ["get", "list", "create", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs:     ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs:     ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: awsbnkctl-ops
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: awsbnkctl-ops
subjects:
- kind: ServiceAccount
  name: awsbnkctl-ops
  namespace: awsbnkctl-ops
```

The full manifest lives at `internal/exec/k8s_install.yaml` (embedded into the binary). [§ RBAC](#rbac-the-clusterrole-rules) walks through what each rule is for.

### 4. Create or update the credential Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: awsbnkctl-ibm-creds
  namespace: awsbnkctl-ops
  annotations:
    helm.sh/resource-policy: keep            # don't sweep on accidental destroy
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: <resolved-key-value>
```

The key value comes from the workspace's resolver chain (env → keychain → config-b64 → prompt) — see [Chapter 14](./14-credentials-resolver.md) for the resolution order. The Secret carries two keys (`AWS_ACCESS_KEY_ID` and the legacy alias `AWS_SECRET_ACCESS_KEY`) both populated from the same resolved value, so older `aws` CLI versions that look for the `IC_` name find it.

If the Secret already exists (re-running `ops install` after a key rotation), `awsbnkctl` does a client-side Get + Update: the Secret's `data` is overwritten with the freshly resolved value, the `awsbnkctl.io/rotated-at` annotation is stamped with the current timestamp, and the rest of the Secret's metadata is left untouched. `awsbnkctl ops show` surfaces `last cred rotation: <timestamp>` by reading that annotation.

### 5. Create the Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: awsbnkctl-ops
  namespace: awsbnkctl-ops
  labels:
    app: awsbnkctl-ops
    awsbnkctl.io/managed: "true"
spec:
  serviceAccountName: awsbnkctl-ops
  restartPolicy: Always
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: tools
    image: ${OPS_IMAGE}                  # resolved from awsbnkctl's version at install time
    imagePullPolicy: IfNotPresent
    command: ["sleep", "infinity"]
    envFrom:
    - secretRef:
        name: awsbnkctl-ibm-creds
    securityContext:
      allowPrivilegeEscalation: false
      runAsNonRoot: true
      capabilities:
        drop: ["ALL"]
    resources:
      requests: { cpu: 50m,  memory: 64Mi }
      limits:   { cpu: 500m, memory: 256Mi }
```

Three details to call out:

- **`command: ["sleep", "infinity"]`** — the pod's own command. Each `Backend.Run` invocation issues a `kubectl exec` against this idle process, which means the pod's main process never exits as long as the pod is healthy.
- **`securityContext` is set explicitly** for OpenShift's `restricted-v2` SCC. Pod-level `runAsNonRoot` + `seccompProfile.type: RuntimeDefault`; container-level `allowPrivilegeEscalation: false` + `capabilities.drop: [ALL]` + `runAsNonRoot` — the same fields the iperf3 server pod sets, for the same reason.
- **`envFrom: secretRef`** — the API key reaches the pod's env without ever touching the pod manifest's argv or `env:` block. `kubectl describe pod awsbnkctl-ops` shows the secret reference name but not the value, per [PRD 04 §"In-cluster pod"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#in-cluster-pod-k8s-backend).

### 6. Wait for readiness

`awsbnkctl ops install` waits for `Pod.Status.Phase == Running` and the container's `Ready` condition before returning. Default timeout is 60 seconds; longer for clusters with slow image pulls (the ghcr.io image is ~80 MiB). Failures surface a `kubectl describe pod awsbnkctl-ops` excerpt for context.

## Trusted-profile flow (v1.2+)

The static-key Secret described above is the v1.0.x / v1.1.x path. In `v1.2.0` it becomes the **fallback**: the default `ops install` invocation auto-provisions an AWS IAM **IRSA role** linked to the ops pod's ServiceAccount, and the static API key no longer needs to land in any Kubernetes Secret. [PRD 04 §"Resolved in Sprint 9" → "Trusted-profile auto-provisioning (k8s backend)"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#trusted-profile-auto-provisioning-k8s-backend) is the design reference; this section is the operational walkthrough.

> **v1.2.0 partial closure — read this first.** Sprint 9 ships the **provisioning side** of the trusted-profile flow: profile creation, compute-resource binding to your cluster's OIDC issuer URL, SA annotation, and the Secret-rendered-with-empty-data manifest under `--trusted-profile=auto` success. **The runtime side — the in-pod `aws sso login` wrap that primes stateful subcommands (`iam`, `ks`, …) — still uses `--apikey "$AWS_ACCESS_KEY_ID"` in v1.2.0**, unchanged from v1.0.x. Under `--trusted-profile=auto` success the Secret carries empty data by design, so the in-pod wrap will fail with `missing API key` when you actually exercise `awsbnkctl --backend k8s aws <subcommand>`. Full closure (in-pod wrap switches to `aws sso login --trusted-profile-id "$IAM_PROFILE_ID"` when the SA is annotated) is Sprint 10 work — tracked in [staff Issue 2](https://github.com/JLCode-tech/awsbnkctl/blob/main/issues/issue_sprint9_staff.md). For v1.2.0, the security-side win is real but partial: no static API key sits at rest in any Secret in etcd under auto-success. If you need the runtime wrap to actually work today, pass `--trusted-profile=off` and the v1.0.x static-key path applies unchanged.

### `awsbnkctl ops install --trusted-profile=auto`

`--trusted-profile=auto` is the **default** as of v1.2 — running `awsbnkctl ops install` with no flag picks the auto path. Naming the flag explicitly is useful in scripts that pin behaviour or in docs that want to read unambiguously:

```bash
$ awsbnkctl ops install --trusted-profile=auto
✓ Provisioned IAM IRSA role arn:aws:iam::123456789012:role/awsbnkctl-ops-sandbox-eks (iam-Profile-9f2…)
✓ created namespace awsbnkctl-ops
✓ created sa awsbnkctl-ops/awsbnkctl-ops
✓ created secret awsbnkctl-ops/awsbnkctl-ibm-creds
✓ created clusterrole awsbnkctl-ops
✓ created crb awsbnkctl-ops
✓ created pod awsbnkctl-ops/awsbnkctl-ops
→ Waiting for ops pod to be Ready (60s timeout)
✓ Ops pod is Ready (IRSA role arn:aws:iam::123456789012:role/awsbnkctl-ops-sandbox-eks)
```

Re-runs against an existing install emit `updated <kind> …` / `<kind> … exists` instead of `created` for each resource that already matches the desired state. The trusted-profile provisioning line above is the single line `internal/cli/ops.go` emits for the whole AWS IAM-side flow (perm probe + profile create + compute-resource link + SA annotation) — the work happens silently inside `resolveTrustedProfileForInstall`; the one line you see is the receipt.

What just happened, in order (the binary doesn't narrate these steps but they're what's actually going on):

1. **IAM perm probe.** `ops install` calls AWS IAM Identity to confirm the resolved API key has `iam-identity` perms. On `403`, the flag value drives the next step: `auto` falls back to the static-key Secret with a warning (see §"`--trusted-profile=auto` falling back" below); `on` errors out with a non-zero exit.
2. **Profile creation.** Names the profile `arn:aws:iam::<account>:role/awsbnkctl-ops-<workspace>` so multiple workspaces against the same AWS account don't race for a single shared name. The compute-resource link binds the profile to your cluster's OIDC issuer URL + the `awsbnkctl-ops/awsbnkctl-ops` ServiceAccount specifically — other SAs on the same cluster can't assume the profile.
3. **Policy attachment.** v1.2 ships with no default policies attached — the profile inherits whatever IAM policies your account has set up for IRSA roles in general (typically nothing, until you grant). A future cycle will surface `aws.irsa_role_policies` as a workspace-config block; tracked under v1.x deferred. If you need the profile to actually authorise specific actions (Container Registry pulls, Cloud Object Storage reads), grant the policies via AWS Console or `aws iam attach-role-policy` after `ops install` returns.
4. **SA annotation.** The ServiceAccount gets `eks.amazonaws.com/role-arn: arn:aws:iam::<account>:role/awsbnkctl-ops-<workspace>` plus the `awsbnkctl.io/trusted-profile-managed: "true"` marker that signals `ops uninstall` to delete the profile during cleanup.
5. **Pod creation.** The pod's container always has `envFrom: secretRef: awsbnkctl-ibm-creds`; what changes between modes is the Secret's contents. Under `--trusted-profile=auto` success the Secret is created with **empty data** — `AWS_ACCESS_KEY_ID` is the empty string. The Sprint 10 conditional-login-wrap closure (see the v1.2.0 partial-closure admonition at the top of this section) will switch the in-pod `aws sso login` to `--trusted-profile-id` when the SA carries the annotation; until then, exercising stateful `aws` subcommands inside the pod fails with `missing API key`. The provisioning side is real (no static key at rest in any Secret); the runtime side ships in Sprint 10.

### Verifying the profile is in use

The ServiceAccount carries the truth-of-record annotation:

```bash
$ oc get serviceaccount awsbnkctl-ops -n awsbnkctl-ops -o yaml
# or, kubectl-equivalent via the bundled passthrough:
$ awsbnkctl k get sa awsbnkctl-ops -n awsbnkctl-ops -o yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/awsbnkctl-ops-sandbox-eks            # ← the profile name
    awsbnkctl.io/trusted-profile-managed: "true"                             # ← ops uninstall will delete it
    awsbnkctl.io/provisioned-at: "2026-05-13T14:08:33Z"
  name: awsbnkctl-ops
  namespace: awsbnkctl-ops
```

End-to-end smoke test of the runtime cred flow:

```bash
$ awsbnkctl --backend k8s aws sts get-caller-identity
```

> **Heads up — Sprint 10 carry-over.** Under v1.2.0 + `--trusted-profile=auto` success, the command above will return `failed to authenticate: missing API key`. The in-pod `aws sso login` wrap still uses `--apikey "$AWS_ACCESS_KEY_ID"` (unchanged from v1.0.x); the Secret carries empty data under auto-success by design; the wrap fails. The runtime-cred-flow closure ships in Sprint 10 (see the v1.2.0 partial-closure admonition at the top of this section). For v1.2.0, exercise the smoke test under `--trusted-profile=off` instead (the static-key path works as in v1.0.x):
>
> ```bash
> $ awsbnkctl ops install --trusted-profile=off
> $ awsbnkctl --backend k8s aws sts get-caller-identity
> IAM token:  Bearer eyJ…
> ```
>
> Once Sprint 10 lands, the `--trusted-profile=auto` smoke test returns the token directly — fresh-each-call from the pod's SDK trading the projected SA token. The cluster's OIDC issuer URL needs ~30-60 seconds to propagate through AWS IAM after `ops install` returns; if your auto-mode smoke test errors with `failed to assume IRSA role` post-Sprint-10, retry after that window.

### `--trusted-profile=auto` falling back

`auto` falls back to the v1.0.x static-key Secret when any of three pre-conditions for trusted-profile provisioning aren't met. The warning prints first (the fallback decision is made before any cluster-side resource is applied), then the rest of the install proceeds with the v1.0.x static-key shape:

```
$ awsbnkctl ops install
warning: IAM perm 'iam-identity' missing; using static-key Secret. Pass `--trusted-profile=off` to silence.
✓ created namespace awsbnkctl-ops
✓ created sa awsbnkctl-ops/awsbnkctl-ops
✓ created secret awsbnkctl-ops/awsbnkctl-ibm-creds
✓ created clusterrole awsbnkctl-ops
✓ created crb awsbnkctl-ops
✓ created pod awsbnkctl-ops/awsbnkctl-ops
→ Waiting for ops pod to be Ready (60s timeout)
✓ Ops pod is Ready (static-key Secret)
```

The three warning shapes (in source order — `internal/cli/ops.go` `resolveTrustedProfileForInstall`):

| Trigger | Warning |
|---|---|
| Workspace has no registered cluster yet (`cluster-outputs.json` missing — run `cluster up` or `cluster register` first) | `warning: trusted-profile mode 'auto' needs a registered cluster (<err>); falling back to static-key Secret. Pass `--trusted-profile=off` to silence.` |
| Registered cluster lookup against the AWS APIs failed (network, key auth, cluster deleted out-of-band) | `warning: trusted-profile mode 'auto' couldn't look up cluster (<err>); falling back to static-key Secret. Pass `--trusted-profile=off` to silence.` |
| API key lacks IAM `iam-identity` permission (the most common fallback) | `warning: IAM perm 'iam-identity' missing; using static-key Secret. Pass `--trusted-profile=off` to silence.` |

All three are non-fatal; the install completes and the pod works exactly as it did in v1.0.x. The warnings are terse on purpose — the actionable detail belongs in this chapter, not in every stderr line. Three ways to clear them permanently:

- **Run `cluster up` or `cluster register`** first if the warning names the missing cluster registration. `ops install` re-run after the registration completes will detect the cluster and switch to the trusted-profile path.
- **Ask your IAM admin** to grant `iam-identity` Operator role on the API key (or use a different key that already has it) if the warning names the missing IAM perm. Re-run `ops install` — the install detects the changed perm posture on re-run and replaces the static-key Secret with a trusted-profile binding.
- **Opt out** via `--trusted-profile=off` (next subsection) if you don't want the warning every install.

### `--trusted-profile=off`

Explicit opt-out. Skips the IAM perm check entirely and provisions the v1.0.x static-key Secret:

```bash
$ awsbnkctl ops install --trusted-profile=off
✓ created namespace awsbnkctl-ops
✓ created sa awsbnkctl-ops/awsbnkctl-ops
✓ created secret awsbnkctl-ops/awsbnkctl-ibm-creds
✓ created clusterrole awsbnkctl-ops
✓ created crb awsbnkctl-ops
✓ created pod awsbnkctl-ops/awsbnkctl-ops
→ Waiting for ops pod to be Ready (60s timeout)
✓ Ops pod is Ready (static-key Secret)
```

Use cases:

- **Reproducing v1.0.x behaviour exactly** — for byte-for-byte parity tests against an older deployment, or for scripts whose assertions match the v1.0.x `ops show` output verbatim.
- **Air-gapped clusters** that can't reach the AWS IAM API at runtime — without that connectivity, the pod can't trade its projected SA token for an IAM token, so the trusted-profile path is non-functional regardless of perms.
- **Cred rotation runbooks** that already automate the static-key path and aren't yet ready to switch to the projected-token model.

The third value, `--trusted-profile=on`, is the inverse — it forces the trusted-profile path and refuses to fall back on perm-missing, returning a non-zero exit with the same warning text. Use it in CI to surface IAM-perm regressions explicitly.

### Cleanup on `ops uninstall`

`awsbnkctl ops uninstall --confirm` honors the `awsbnkctl.io/trusted-profile-managed: "true"` annotation on the SA and deletes the AWS IRSA role alongside the cluster-side objects:

```bash
$ awsbnkctl ops uninstall --confirm
✓ deleted IRSA role arn:aws:iam::123456789012:role/awsbnkctl-ops-sandbox-eks
✓ deleted pod awsbnkctl-ops
✓ deleted secret awsbnkctl-ibm-creds
✓ deleted serviceaccount awsbnkctl-ops
✓ deleted clusterrolebinding awsbnkctl-ops
✓ deleted clusterrole awsbnkctl-ops
✓ deleted namespace awsbnkctl-ops
✓ deleted namespace awsbnkctl-test
```

The IRSA role is deleted **first**, before the cluster-side objects, so even if the cluster API becomes unreachable mid-uninstall the AWS-side state isn't left orphaned. The Secret is always deleted regardless of mode (it's always rendered by `ops install`, just with empty data under `--trusted-profile=auto` success).

The trusted-profile delete is **best-effort** — if the calling user's API key has lost `iam-identity` perms in the meantime (or the key itself has rotated and the new key doesn't have those perms), the cluster-side objects still delete and a warning line is printed instructing the user to delete the profile manually via the AWS console. The annotation remains correct documentation of what was provisioned; `awsbnkctl ops install` on a fresh cluster will pick a fresh profile name unconditionally so an orphaned profile from a prior install doesn't collide.

`--trusted-profile=off` installs leave no IRSA role to clean up — `ops uninstall --confirm` just deletes the cluster-side objects + the static-key Secret as it did in v1.0.x.

## `awsbnkctl ops show`

Reports current state without making any changes:

```bash
$ awsbnkctl ops show
namespace:    awsbnkctl-ops
pod:          awsbnkctl-ops
phase:        Running
ready:        true
image:        ghcr.io/JLCode-tech/awsbnkctl-tools-aws:v0.9.0
rbac subject: system:serviceaccount:awsbnkctl-ops:awsbnkctl-ops
secret:       awsbnkctl-ibm-creds (rotated 2026-05-10T11:03:17Z)
```

What each line surfaces:

1. **Pod phase + readiness** — `Running` + `true` is green; anything else means the pod is unhealthy and `Backend.Run` calls will fail. The container count is exactly one (`tools`); `ready: true` is a single bool, not a `2/2`-style ratio.
2. **Image** — the `:v…` tag matches the `awsbnkctl` release the image was published with (resolved at install time from the binary's version; see [Chapter 17 §`:dev` tag resolution](./17-execution-backends.md#dev-tag-resolution)). Mismatched against your `awsbnkctl --version` means re-running `ops install` will pull the matching image.
3. **RBAC subject** — the SA the pod runs as. `kubectl describe clusterrole awsbnkctl-ops` prints the full ruleset (the ClusterRoleBinding is named the same as the role).
4. **Secret line** — the cred Secret's name + the `awsbnkctl.io/rotated-at` annotation that `ops install` stamps each time the Secret is applied. If the Secret is missing entirely, the line reads `secret: (missing: …)`.

The current output is a fixed six-line key/value block; a structured `--output json` mode is on the v1.x roadmap once `ops show` grows additional fields (image-id hash, env-hash reconciliation against the live pod, etc.). See [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deliberately deferred to post-v1.0".

## `awsbnkctl ops uninstall`

Full removal. Run when decommissioning the cluster, or when you want a clean re-install. The command is a **destructive-action gate**: by default it prints a preview of what *would* be deleted and exits successfully; the actual deletion only runs with `--confirm`.

```bash
$ awsbnkctl ops uninstall
Would delete (re-run with --confirm to proceed):
  - Pod        awsbnkctl-ops/awsbnkctl-ops
  - Secret     awsbnkctl-ops/awsbnkctl-ibm-creds
  - ServiceAccount awsbnkctl-ops/awsbnkctl-ops
  - ClusterRole/ClusterRoleBinding awsbnkctl-ops
  - Namespace  awsbnkctl-ops
  - Namespace  awsbnkctl-test

$ awsbnkctl ops uninstall --confirm
✓ deleted pod awsbnkctl-ops
✓ deleted secret awsbnkctl-ibm-creds
✓ deleted serviceaccount awsbnkctl-ops
✓ deleted clusterrolebinding awsbnkctl-ops
✓ deleted clusterrole awsbnkctl-ops
✓ deleted namespace awsbnkctl-ops
✓ deleted namespace awsbnkctl-test
```

Note that the cluster-scoped objects (ClusterRole, ClusterRoleBinding) get cleaned too — they're not garbage-collected by namespace deletion since they live above the namespace. `awsbnkctl ops uninstall --confirm` makes this explicit so a stale `awsbnkctl-ops` ClusterRole can't outlive a namespace removed via `kubectl delete ns`.

Both managed namespaces (`awsbnkctl-ops` and `awsbnkctl-test`) are deleted. The `awsbnkctl-test` namespace is where one-shot Job pods (iperf3 client, future probes) land — by the time you're running `uninstall` you've already concluded those test workloads are finished, so removing the namespace alongside the ops-pod surface keeps the cluster clean.

When to run `uninstall`:

- **Cluster decommission** — the cluster is going away, clean up cluster-scoped objects before destroying it.
- **Cred rotation when paranoid** — the rotation story (next section) doesn't require uninstall, but if you're worried about old secrets persisting in etcd snapshots, an uninstall + re-install regenerates the Secret cleanly.
- **Image upgrade with a major manifest change** — if the embedded `k8s_install.yaml` evolves (new RBAC rule, security-context tweak), `uninstall` + `install` is the cleanest way to apply.

## RBAC: the ClusterRole rules

The full ClusterRole rule set (transcribed from `internal/exec/k8s_install.yaml`):

| API group | Resources | Verbs | Why |
|---|---|---|---|
| `batch` | `jobs` | get, list, watch, create, delete | One-shot Job lifecycle (iperf3 client, future probes). The backend creates the Job, watches it, reads logs, deletes it. |
| `""` (core) | `pods` | get, list, watch | The backend lists/watches pods to find Job-spawned pods + to wait for the ops pod's Ready state. **No `create`/`delete`** — pods are owned by their Jobs (or by `ops install`'s user-side privilege), not by the pod's SA. |
| `""` (core) | `pods/log` | get, list | Log streaming from one-shot Job pods (the bytes the wrapped tool wrote to stdout/stderr). |
| `""` (core) | `pods/exec` | create, get | `kubectl exec` is a `create` against the `pods/exec` subresource — the SPDY-channel verb the long-lived ops-pod path uses. |
| `""` (core) | `secrets` (named `awsbnkctl-ibm-creds`) | get | The pod reads the cred Secret directly only if a future workflow opts to (kubelet's projection of `envFrom: secretRef` runs as kubelet, not as this SA). The `resourceNames` filter keeps the SA from reading any other Secret in the namespace — least-privilege per [PRD 04 §"In-cluster pod"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#in-cluster-pod-k8s-backend). |

Notably **not** granted:

- **`pods` create / delete** — the SA can list and watch pods but can't create or delete them. Pod lifecycle is mediated by Jobs (which the SA does manage) and by the ops pod itself (which `ops install` creates with the user's privilege, not the SA's).
- **`secrets` create / update / delete / list** — the pod never writes Secrets, and can't even list to discover which Secrets exist in the namespace. The install-time Secret creation is done by the user invoking `ops install` (whose kubeconfig has cluster-admin or comparable), not by the pod's SA. Combined with the `resourceNames` filter on `get`, this is the tightest practical surface that still lets the pod consume its own cred.
- **`services`, `deployments`, `namespaces`** — the SA can't touch these at all. The iperf3 server fixture (when the throughput test runs) is provisioned by `awsbnkctl test throughput` running on the caller's side using the user's kubeconfig, not by anything inside the ops pod.
- **`clusterroles`, `clusterrolebindings`** — the pod never modifies its own RBAC.
- **`*` cluster-admin** — explicitly avoided. The pod has exactly the verbs it needs and nothing else.

This matches [PRD 04 §"Least privilege per backend"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#cross-backend-principles) and [PRD 03 §"K8s"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/03-EXECUTION-BACKENDS.md#k8s-internalexeck8sgo): the ops pod is a powerful tool but its blast radius is bounded.

To audit the rules on a running cluster:

```bash
kubectl describe clusterrole awsbnkctl-ops
kubectl auth can-i --as=system:serviceaccount:awsbnkctl-ops:awsbnkctl-ops \
  '*' '*' --all-namespaces       # should print mostly "no"
```

## Credential propagation

> **v1.2+ note.** What follows is the **static-key** propagation path. As of v1.2 it's the fallback rather than the default — `--trusted-profile=auto` installs assume an AWS IRSA role via the pod's projected SA token and the static API key never lands in a Kubernetes Secret. See [§"Trusted-profile flow (v1.2+)"](#trusted-profile-flow-v12) above for that path. The hop-by-hop description below still describes what happens under `--trusted-profile=off` (and under the `auto`-fallback when IAM perms don't allow the trusted-profile path).

The `AWS_ACCESS_KEY_ID` reaches the wrapped tool in three hops:

```
resolver chain (env → keychain → config-b64 → prompt)
       ↓                        on the laptop, at `awsbnkctl ops install` time
  Kubernetes Secret awsbnkctl-ibm-creds                in awsbnkctl-ops namespace
       ↓                        applied by `ops install` via kubectl-equivalent
  Pod env (AWS_ACCESS_KEY_ID=…)                          via `envFrom: secretRef`
       ↓                        kubelet reads Secret, sets env on container start
  Wrapped tool (`aws sts get-caller-identity`)            reads from os.Getenv
```

Three properties this gives you:

1. **The key never appears in argv.** `kubectl describe pod awsbnkctl-ops` shows `envFrom: secretRef: name: awsbnkctl-ibm-creds`, not the value. `kubectl get pod awsbnkctl-ops -o yaml` shows the same.
2. **The key never appears in the pod's own logs.** The wrapped tool uses the env var; the env var name (not value) is what the pod's startup logs print.
3. **The redactor is the defense-in-depth backstop.** If the wrapped tool ever prints the value (e.g., `aws --debug`), the SPDY stream from the pod is wrapped through `internal/exec/redact.go` before reaching the caller's stdout — same as the local + docker backends.

The Secret carries two keys today — `AWS_ACCESS_KEY_ID` and the legacy `AWS_SECRET_ACCESS_KEY` alias older `aws` versions accept — both populated from the same resolved value. Names are stable; embedded in `internal/exec/k8s_install.yaml`. Future cluster-side credentials (an AWS access key, a GCP service-account JSON) will add new keys to the same Secret rather than spinning up new Secrets, simplifying RBAC.

## Rotation: rotating the API key

> **v1.2+ note.** Under `--trusted-profile=auto` / `=on` (default), there's nothing to rotate — the ops pod's IAM tokens are short-lived and the AWS IAM endpoint refreshes them transparently each time the SDK trades the projected SA token. Key rotation only matters when the install ran with `--trusted-profile=off` or fell back to the static-key Secret because the resolved key lacked `iam-identity` perms. The procedure below covers that static-key case.

When the AWS APIs key changes (key rotation, account takeover, key compromise), you need to update the cluster-side Secret. The flow:

```bash
# 1. Update the local resolver chain — pick whichever source you populated
#    initially (the chain order is: env > keychain > config-b64; see chapter 14):
export AWS_ACCESS_KEY_ID=<new-key>             # env (one-shot)
# or update the keychain entry directly: `keyring` / `secret-tool` / Keychain.app
# or edit ~/.awsbnkctl/<workspace>/config.yaml's api_key_b64 field

# 2. Re-run ops install — this re-resolves the key, updates the cluster
#    Secret, and rolls the pod
awsbnkctl ops install
```

What `ops install` does on re-run:

- The Secret `awsbnkctl-ibm-creds` is updated with the new value via a client-side Get + Update (the existing Secret's `data` is overwritten, the `awsbnkctl.io/rotated-at` annotation is refreshed, the rest of the metadata is left alone).
- The pod's env, however, **is set at container-start time** — kubelet reads the Secret value when the pod is created, not on every Secret update. So an updated Secret doesn't propagate to the running pod's env until the pod is recreated.
- `ops install` therefore deletes and recreates the bare ops pod after the Secret update. New pod → kubelet reads the updated Secret → env contains the new value. (Re-creation takes a few seconds for the image cache hit; up to ~30 seconds on a cold cluster.)

The ops pod is a **bare Pod**, not a Deployment or DaemonSet, so `kubectl rollout restart` won't work on it (`rollout restart` only operates on controller resources). The canonical way to force a fresh pod is `awsbnkctl ops install` (idempotent — it'll handle the delete-and-recreate). If you really want to do it by hand:

```bash
kubectl delete pod awsbnkctl-ops -n awsbnkctl-ops
# then re-run `awsbnkctl ops install` to create the replacement;
# the bare pod has no controller, so nothing else will recreate it.
```

`awsbnkctl ops show` will report `phase: Pending` briefly during recreation, then `Running` + `ready: true` once kubelet finishes projecting the updated Secret.

## Operability

Things to know when something's wrong.

### Where pod logs go

```bash
awsbnkctl k logs -n awsbnkctl-ops awsbnkctl-ops
# or
kubectl logs -n awsbnkctl-ops awsbnkctl-ops
```

The pod's main process is `sleep infinity`, so the log is mostly empty. Each `kubectl exec` invocation runs in its own ephemeral process — those processes' stdout/stderr go back through the SPDY channel to the caller, **not** into the pod's log. So `kubectl logs` is helpful for debugging pod startup (image pull failures, SCC denials, OOMKills) but not for "what did `aws sts get-caller-identity` actually print" — that's just the caller's stdout.

For a paper trail of recent invocations, capture `awsbnkctl exec -- --backend k8s … 2>&1 | tee /tmp/aws-eks.log` on the calling side.

### Debugging a stuck `ops install`

`ops install` waits up to 60 seconds for the pod to become Ready. If it times out:

```bash
awsbnkctl k describe -n awsbnkctl-ops pod/awsbnkctl-ops
awsbnkctl k get -n awsbnkctl-ops events --sort-by=.lastTimestamp | tail -20
```

Common causes:

| Symptom | Cause | Fix |
|---|---|---|
| `ImagePullBackOff` | ghcr.io rate limit, or image tag doesn't exist | check `awsbnkctl --version`, ensure ghcr.io is reachable from the cluster |
| `CreateContainerConfigError` referencing the Secret | Secret was deleted between Secret apply and Pod create (race) | re-run `awsbnkctl ops install` (idempotent) |
| `RunContainerError` with SCC denial | the cluster's PodSecurity admission rejected the manifest | `kubectl get events` will name the missing field; usually means an OpenShift cluster expects the `restricted-v2` profile and a manifest field is wrong — file an issue with the event message |
| Pod stuck in `Pending` with no Events | cluster is at capacity / out of CPU | scale the cluster or trim resources; the pod requests `50m` CPU + `128Mi` mem, very small |

### Cluster API outage during `ops install`

If the kube-apiserver becomes unreachable mid-install (transient cloud-provider issue, kubeconfig expired, network partition), `ops install` fails fast at whichever step hit the apiserver:

```
✓ applied namespace awsbnkctl-ops
applying secret awsbnkctl-ops/awsbnkctl-ibm-creds       ... ERROR: Get "https://...": dial tcp: i/o timeout
```

The install is **partial** at that point — earlier steps succeeded, later steps didn't. `ops install` is idempotent, so just re-run once the apiserver is back; the steps that already completed are no-ops the second time, the steps that didn't will run.

If the apiserver is permanently gone (cluster destroyed): `ops uninstall` will fail the same way, since it also needs the apiserver. In that case the cluster-scoped objects (ClusterRole, ClusterRoleBinding) become orphans you can clean up manually if you ever rebuild the cluster, or ignore if you're done with this cluster's identity entirely.

### Verifying the install end-to-end

A one-liner sanity check:

```bash
awsbnkctl exec --backend k8s iam oauth-tokens
```

If the SA/Secret/RBAC/Pod chain is healthy, this prints a fresh OAuth token. If it errors, the error message names which link in the chain broke (pod not found, Secret missing, exec denied, AWS CLI exit non-zero).

[Chapter 26 — Troubleshooting](./26-troubleshooting.md) covers the broader "ops pod is unhappy" failure modes alongside other end-user troubleshooting.

## Cross-references

- [PRD 03 — pluggable execution backends, §"K8s"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/03-EXECUTION-BACKENDS.md#k8s-internalexeck8sgo) — the ops-pod design rationale.
- [PRD 04 — credential propagation, §"In-cluster pod"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#in-cluster-pod-k8s-backend) — Secret-based propagation rules.
- [Chapter 14 — Credentials and the resolver chain](./14-credentials-resolver.md) — where the `AWS_ACCESS_KEY_ID` value comes from before it lands in the Secret.
- [Chapter 17 §"K8s backend"](./17-execution-backends.md#k8s-backend) — the interface mechanics this chapter complements.
- [Chapter 18 — Choosing a backend per tool](./18-choosing-backend.md) — when `--backend k8s` is the right call.
- `internal/exec/k8s_install.yaml` — the embedded RBAC manifests: <https://github.com/JLCode-tech/awsbnkctl/blob/main/internal/exec/k8s_install.yaml>
- `internal/cli/ops.go` — the `awsbnkctl ops install/show/uninstall` command implementation: <https://github.com/JLCode-tech/awsbnkctl/blob/main/internal/cli/ops.go>
