# Sprint 6 — staff engineer issues

Sprint 6 is the final sprint of the AWS retarget — the v1.0-candidate
cut. Staff scope: close the Sprint 5 tech-writer Issue 1 BLOCKER (the
embedded `internal/exec/k8s_install.yaml` ops-pod manifest had survived
the Sprint 5 IBM-residue sweep entirely intact), run the security audit
(gosec + govulncheck + secrets scan), verify goreleaser produces the
expected 6-binary archive set, and gate on a green Go build/test pass.

SPIKE DEFERRAL carries — no live AWS exercise; the YAML retarget is
validated by spec-check + Go build/test, the doctor IRSA probes
exercise their happy paths on a real cluster only.

**Closed in this sprint:** Sprint 5 tech-writer Issue 1 BLOCKER (the
manifest); two minor follow-on cleanups (unused `K8sOpsSecretName`
const; legacy `ROKSBNKCTL_K8S_LONG_LIVED` sentinel env var name); one
dep bump fix (govulncheck — `golang.org/x/net` HTTP/2 vulns).

**Issues filed:** 3 (1 medium accepted-as-shipped on gosec
file/dir-perm warnings; 1 low informational on the residual gosec
HIGH-severity findings cluster; 1 informational deferral on the in-pod
`AWS_REGION` injection that needs the ops-install CLI command to land).

---

## ✅ Closed in this sprint

### k8s_install.yaml IRSA retarget (Sprint 5 tech-writer Issue 1 BLOCKER)

**Status**: closed

The embedded ops-install manifest (`internal/exec/k8s_install.yaml`)
was the single largest piece of un-retargeted IBM-shape carry-over in
the v0.9 surface. Sprint 5's "297 → 1 hit" headline counted only `.go`
files; the YAML was outside that grep. Closed end-to-end this sprint:

**Manifest changes:**

- Dropped `Namespace roksbnkctl-ops` → renamed to `awsbnkctl-ops`;
  same for `roksbnkctl-test` → `awsbnkctl-test`. Label keys
  retargeted (`roksbnkctl.io/managed` → `awsbnkctl.io/managed`,
  `roksbnkctl.io/test` → `awsbnkctl.io/test`).
- Dropped the standalone `Secret roksbnkctl-ibm-creds` document
  entirely. No static credential lands in any Kubernetes Secret on
  the AWS path. PRD 04 §"In-cluster identity" §"IRSA replaces Trusted
  Profile" makes this the only shape; there is no `--trusted-profile=
  {auto,on,off}` flag on the AWS surface.
- ServiceAccount `awsbnkctl-ops/awsbnkctl-ops` carries the
  `eks.amazonaws.com/role-arn: "${OPS_IRSA_ROLE_ARN}"` annotation
  (replaces the `iam.cloud.ibm.com/trusted-profile: …` IBM annotation).
  The EKS pod-identity webhook reads this at pod-admission time and
  injects `AWS_ROLE_ARN` + `AWS_WEB_IDENTITY_TOKEN_FILE` + a projected
  SA token at `/var/run/secrets/eks.amazonaws.com/serviceaccount/token`.
  aws-sdk-go-v2 inside the pod handles the
  `sts:AssumeRoleWithWebIdentity` exchange automatically.
- Pod container `envFrom: secretRef: roksbnkctl-ibm-creds` dropped.
  The container now carries only `env: HOME=/tmp` (the well-known
  non-root-UID `$HOME` workaround) — IRSA injection handles the
  AWS-side env entirely.
- ClusterRole `secrets:get` with `resourceNames:
  ["roksbnkctl-ibm-creds"]` replaced with `secrets:{create, delete,
  patch}` in `awsbnkctl-test` only (the per-Job ephemeral files Secret
  the k8s backend's `runAsJob` path materialises at /work — see
  `internal/exec/k8s.go::runAsJob`). No cluster-wide Secret reads.
- Comments retargeted at PRD 04 §"In-cluster identity" instead of
  PRD 03/04 §"K8s" §"Resolved in Sprint 9" §"Trusted-profile
  auto-provisioning".

**Template tokens (called out for the integrator):**

The template surface changed too. The old shape used:

- `${IBMCLOUD_API_KEY_B64}` — gone. The Secret is gone.
- `${ROTATED_AT}` — gone. The Secret was the only thing that carried
  this annotation; `ops show`'s "last cred rotation" line disappears
  on the IRSA path (the equivalent on AWS is the IRSA role's trust
  policy revision, which IAM tracks server-side).

The new shape uses:

- `${OPS_IRSA_ROLE_ARN}` — required; the IAM role ARN provisioned by
  `terraform/modules/iam_irsa` (PRD 08) for the ops pod's
  ServiceAccount.
- `${OPS_IMAGE}` — unchanged; the awsbnkctl tools image ref, resolved
  from `internal/cli.Version` at install time.

The CLI-layer template substitution code is not yet present (there is
no `awsbnkctl ops install` command — only the embedded manifest and
the doctor probes that assume it has been applied; see Issue 3). When
the integrator wires the CLI command in a later sprint, the
substitution should resolve `${OPS_IRSA_ROLE_ARN}` from the workspace's
`cluster-outputs.json` (the `terraform/modules/iam_irsa` output
`ops_role_arn` lands there).

**Verification:**

- `grep -iE 'ibm|roks|cos|ibmcloud' internal/exec/k8s_install.yaml` →
  0 hits.
- `go build ./...` clean.
- `go vet ./...` clean.
- `go test ./internal/exec/...` ok.

### k8s_install.go embed docstring + legacy const cleanup

**Status**: closed

Two minor follow-ons from the YAML retarget:

- `internal/exec/k8s_install.go` docstring retargeted at the new
  template tokens (`${OPS_IRSA_ROLE_ARN}` + `${OPS_IMAGE}`) and the
  IRSA shape; the old `${ROTATED_AT}` doc-line removed.
- `internal/exec/k8s.go` constant `K8sOpsSecretName = "awsbnkctl-ibm-creds"`
  was unused after Sprint 5's `.go` IBM-residue sweep (no caller
  references it). Dropped along with its docstring; the const block
  comment retargets the explanation at the IRSA shape.
- `internal/exec/k8s.go` long-lived-exec sentinel env var renamed
  `ROKSBNKCTL_K8S_LONG_LIVED=1` → `AWSBNKCTL_K8S_LONG_LIVED=1`. The
  sentinel is an internal-only encoding (callers don't see it; it's
  stripped from the env passed to the in-pod container), so the
  rename is mechanical with no compatibility impact.

### govulncheck — `golang.org/x/net` HTTP/2 vulns

**Status**: closed

govulncheck on the Sprint 5 dep set flagged two HIGH-severity
vulnerabilities in `golang.org/x/net@v0.50.0`:

- **GO-2026-4918** — Infinite loop in HTTP/2 transport when given a
  bad `SETTINGS_MAX_FRAME_SIZE`. Fixed in `golang.org/x/net@v0.53.0`.
- **GO-2026-4559** — Sending certain HTTP/2 frames can cause a server
  to panic. Fixed in `golang.org/x/net@v0.51.0`.

Both reachable from `internal/tf/source.go`'s
`tf.ResolveLatestRelease → http.Client.Do` path, the
`internal/k8s/apply.go` discovery client, and various error-stringer
paths in `internal/test/` and `internal/doctor/`. Bumped via
`go get golang.org/x/net@v0.53.0` + `go mod tidy`. Transitive bumps
land on `golang.org/x/crypto v0.50.0`, `golang.org/x/sys v0.43.0`,
`golang.org/x/term v0.42.0`, `golang.org/x/text v0.36.0`,
`golang.org/x/mod v0.34.0`, `golang.org/x/tools v0.43.0`,
`golang.org/x/sync v0.20.0`.

Post-bump `govulncheck ./...` returns **"No vulnerabilities found."**

### .goreleaser.yml — fork-name residue swept

**Status**: closed

Two surviving fork-name strings cleaned up:

- `release.header.Long` (line 83): "Single-binary CLI for deploying
  F5 BIG-IP Next for Kubernetes on **IBM Cloud ROKS**." → "… on
  **AWS EKS**."
- `brews.*.description` (commented-out tap block, line 122): same
  IBM-Cloud-ROKS → AWS-EKS swap inside the commented placeholder so
  the future tap wire-up doesn't carry the residue forward.

All `roksbnkctl` / `jgruberf5` / `gruber` references in
`.goreleaser.yml`: 0 hits post-sweep. The
`github.com/JLCode-tech/awsbnkctl/...` ldflags + the
`JLCode-tech.github.io/awsbnkctl/book/` book URL + the `name_template`
+ the build matrix all read clean.

### goreleaser snapshot — 6 binaries produced

**Status**: closed

`goreleaser build --snapshot --clean` produces:

```
dist/awsbnkctl_darwin_amd64_v1/awsbnkctl
dist/awsbnkctl_darwin_arm64_v8.0/awsbnkctl
dist/awsbnkctl_linux_amd64_v1/awsbnkctl
dist/awsbnkctl_linux_arm64_v8.0/awsbnkctl
dist/awsbnkctl_windows_amd64_v1/awsbnkctl.exe
dist/awsbnkctl_windows_arm64_v8.0/awsbnkctl.exe
```

Six binaries: linux × {amd64, arm64}, darwin × {amd64, arm64}, windows
× {amd64, arm64} — exactly the matrix the v1.0 release plan calls for.
Build wall-clock 6m21s on the local sandbox (cold Go build cache).

Disk-space note for the integrator: the cross-compile sweep can blow
through the available temp-dir space on a constrained box. On the
local sandbox the run failed once with `no space left on device` until
`go clean -cache` freed ~10 GiB. The release CI runner has 14 GiB free
on first-run and a fresh module cache, so this won't hit there, but
operators reproducing locally should run `goreleaser` from a working
dir with at least 8 GiB free.

### Build-green gate

**Status**: closed

- `go build ./...` clean
- `go vet ./...` clean
- `gofmt -l internal/ cmd/ tools/` zero diffs
- `go test ./...` — 10 packages ok, 0 failures (full suite,
  cache-warm pass: aws 0.7s, cli 1.6s, config 3.6s, doctor 21.4s,
  exec 7.0s, k8s 5.1s, remote 3.5s, test 3.2s, tf 4.1s, refgen 6+5s)
- `govulncheck ./...` — no vulnerabilities
- secrets scan (`git grep -nE 'AKIA[0-9A-Z]{16}|aws_secret.*=.*[A-Za-z0-9/+=]{30,}|-----BEGIN.*PRIVATE KEY-----'`)
  — zero hits across source / config / yaml / json / toml (excluding
  the IBM-era terraform `modules/license/*_k8sconfig/` cache files
  that the integrator deletes on first AWS apply; those carry the
  upstream operator's `j.gruber@f5.com` k8sconfig fragments but no
  AWS credentials, and they regenerate against the awsbnkctl workspace
  on first `terraform init`).

---

## Issue 1 (MEDIUM — gosec G301 / G306 file-mode warnings on user-config writes — accepted as shipped)

**Severity**: medium
**Status**: open (accepted as shipped; doc rationale below)

`gosec ./...` flags 13 sites under G301 (directory permissions ≥ 0750)
and 7 sites under G306 (WriteFile permissions ≥ 0600) — the bulk of
the gosec "issues 55" total. All sites write **user-readable**
configuration files (workspace config, cluster outputs, tfvars
renders, the install binary itself) where 0o755 dirs + 0o644 files
are the expected XDG / dotfile convention.

**Sample sites flagged:**

- `internal/config/workspace.go:291,298,305` — `~/.config/awsbnkctl/<ws>/`
  dir + workspace yaml. Users routinely `cat`, `vim`, `git add` these.
- `internal/config/global.go:53,60` — `~/.config/awsbnkctl/config.yaml`.
- `internal/config/cluster_outputs.go:78,86` — terraform outputs JSON.
- `internal/cli/tfvars.go:59,91` — `tfvars` rendered to user-specified
  paths.
- `internal/cli/install.go:80` — the destination for `awsbnkctl install`
  (`~/.local/bin/awsbnkctl`); the binary must be o+rx so other users
  on the box can exec it.
- `internal/tf/terraform.go:58,62,72,82,112` + `internal/tf/fetch.go:80,
  101,107,151,206,210` — terraform staging dir + tfvars override
  + the unpacked terraform source tree (provider plugins must be
  world-readable for terraform to dlopen them).

**Why these are accepted, not silenced:**

None of these files carry credentials in the v1.0 shape. Credentials
resolve via the AWS standard chain (env → profile → SSO → IRSA per
PRD 04 §"Resolved in Sprint 3") — there's no static-key Secret to
mode-0600. The IBM-era `~/.config/awsbnkctl/<ws>/keys/` private-key
files **are** correctly mode-0600 today (`internal/remote/keys.go`
writes them via `os.WriteFile(..., 0o600)`) and gosec doesn't flag
those.

**Recommendation:** capture the gosec posture in a `.gosec.yaml`
config that explicitly excludes G301/G306 from the staff-owned write
sites, OR rule them out per-file with `#nosec G306 -- user-readable
config` annotations. Both are noise-reduction work without a
correctness payoff; defer either treatment to the v1.0.1 polish
cycle if CI starts gating on gosec exit code (Sprint 6 validator's
audit will surface whether that gate lands now or later).

## Issue 2 (LOW — informational — gosec residual HIGH-severity findings cluster)

**Severity**: low (informational)
**Status**: open (filed for validator review)

After the v0.50→v0.53 `golang.org/x/net` bump and the YAML retarget,
gosec's HIGH-severity bucket contains 10 findings. Each one is a
**known design call** rather than a regression; documenting them
here so the validator audit + the v1.0 release notes can cross-
reference:

| # | Site | Rule | Nature |
|---|---|---|---|
| 1 | `internal/aws/eks.go:107` | G101 (hardcoded creds) | False positive — `const EKSAuthTokenPrefix = "k8s-aws-v1."` is the public EKS bearer-token prefix from aws-iam-authenticator. |
| 2 | `internal/test/connectivity.go:25` | G402 (TLS InsecureSkipVerify) | Already annotated `//nolint:gosec`; gated by an explicit caller-passed flag, off by default. Feature documented in `book/src/20-connectivity-testing.md`. |
| 3 | `internal/remote/keys.go:93` | G704 (SSRF taint) | SSH `agent.Dial` against the user-supplied `$SSH_AUTH_SOCK`; that's the only correct value for it. |
| 4 | `internal/remote/agent.go:23` | G704 (SSRF taint) | Same — `net.Dial` against `$SSH_AUTH_SOCK`. |
| 5 | `internal/k8s/client.go:78` | G703 (path traversal) | `os.ReadFile` on the user's kubeconfig path; that's by design (`KUBECONFIG=…`). |
| 6 | `internal/cli/tfvars.go:91` | G703 (path traversal) | User-supplied `--output` path on a CLI write; intentional. |
| 7 | `internal/tf/fetch.go:213` | G115 (int64→uint32) | Tar entry mode bits, range-bounded by the tar format spec to fit uint32 by definition. Could add a bounded conversion + `&0o777` mask for belt-and-braces but the surface is read-only post-extraction. |
| 8 | `internal/exec/k8s.go:376` | G118 (goroutine context) | Cleanup goroutine for the per-Job ephemeral Secret on context-cancel; uses `context.Background()` deliberately because the request context is the one that just cancelled (we still want the cleanup to run on the way out). Documented in the surrounding comment. |
| 9 | `internal/exec/docker.go:291` | G118 (goroutine context) | Same pattern in the docker backend's cleanup goroutine. |
| 10 | `internal/k8s/apply.go:129` | G122 (symlink TOCTOU) | `filepath.WalkDir` over the unpacked terraform tree; the tree itself is in a staff-controlled dir under the workspace's state path (no untrusted symlinks possible without an explicit attacker-controlled write to the workspace dir). |

**Recommendation:** validator catalogues these against the CI gosec
gate (if/when one is wired in Sprint 6 validator surface); none
warrant a code change for v1.0.

## Issue 3 (INFORMATIONAL — `awsbnkctl ops install` CLI command not yet wired; doctor probes assume manifest is applied out-of-band)

**Severity**: informational
**Status**: open (cross-sprint, integrator scope)

The retargeted `internal/exec/k8s_install.yaml` is embedded into the
binary and exposed via `exec.K8sInstallYAML()`, but **no `awsbnkctl
ops install` cobra command consumes it**. The `internal/cli/install.go`
file ships the `install` verb that copies the running binary onto
`$PATH`; there is no `ops` cobra group, no `install` subverb under it.

This affects two surfaces:

- **doctor**: `internal/cli/doctor_backend.go` § "IRSA-shape ops-pod
  check" assumes the manifest has been applied (probes the
  `awsbnkctl-ops/awsbnkctl-ops` ServiceAccount for the
  `eks.amazonaws.com/role-arn` annotation and `kubectl exec`'s the
  ops pod for `printenv AWS_WEB_IDENTITY_TOKEN_FILE`). On a fresh
  cluster the probe fires "ops namespace missing — run `awsbnkctl
  ops install`" — but the command the error message names doesn't
  exist.
- **chapter 19**: Sprint 5 tech-writer Issue 1 flagged the book
  chapter still references `awsbnkctl ops install --trusted-profile=
  auto`-shape sample output. The architect's chapter 19 retarget (in
  parallel this sprint) annotates the chapter as describing the
  designed-but-not-yet-shipped command; the YAML retarget gets the
  designed shape onto disk; the CLI verb that consumes the YAML and
  drives the apply is the missing piece.

**Out of scope for this sprint** per the prompt — the staff brief
gates on the YAML retarget + audit + goreleaser, not on net-new CLI
surface. The verb implementation is roughly:

1. New `internal/cli/ops.go` (or `ops_install.go`) that creates the
   `ops` cobra group + `install` subverb.
2. Resolve `${OPS_IRSA_ROLE_ARN}` from the workspace's
   `cluster-outputs.json` (terraform's `iam_irsa` module output
   `ops_role_arn`).
3. Resolve `${OPS_IMAGE}` from `internal/cli.Version`.
4. Substitute via `strings.NewReplacer(…).Replace(K8sInstallYAML())`.
5. Apply documents one at a time via `internal/k8s/apply.go`'s
   `ApplyOptions.Run` (which already handles the split-on-`---` +
   bail-on-first-error semantics).
6. Wait for `Pod.Status.Phase == Running` + Ready condition on the
   ops pod, 60-second default timeout.
7. Print `✓ created namespace …` / `✓ updated …` per applied
   document (idempotent on re-runs).

**Recommendation**: file for v1.0.1 (post-v1.0-cut polish). Until
then, operators bootstrap the ops pod manually via `kubectl apply -f
<(awsbnkctl ops dump-yaml)` or equivalent, and the doctor probes
gate on its presence. The validator's Sprint 6 audit may upgrade this
to a v1.0-cut blocker — the staff scope here is to **land the YAML
retarget so the CLI command, when wired, applies the right shape**.

---

## Verification snapshot (sprint close)

| Gate | Status | Detail |
|---|---|---|
| `grep -iE 'ibm\|roks\|cos\|ibmcloud' internal/exec/k8s_install.yaml` | ✅ | 0 hits |
| `grep -iE 'roksbnkctl\|jgruberf5\|gruber' .goreleaser.yml` | ✅ | 0 hits |
| `go build ./...` | ✅ | clean |
| `go vet ./...` | ✅ | clean |
| `gofmt -l internal/ cmd/ tools/` | ✅ | zero diffs |
| `go test ./...` | ✅ | 10 packages ok, 0 failures |
| `gosec ./...` | ⚠ | 55 findings; 10 HIGH all known-design / false-positive (Issue 2); 20 G301+G306 user-config perms accepted as shipped (Issue 1); zero net-new findings vs Sprint 5 baseline |
| `govulncheck ./...` | ✅ | "No vulnerabilities found." (post `x/net@v0.53.0` bump) |
| secrets scan | ✅ | 0 static creds / private keys in source/yaml/json/toml |
| `goreleaser build --snapshot --clean` | ✅ | 6 binaries produced (linux/darwin/windows × amd64/arm64); wall-clock 6m21s cold |

## Sibling cross-references

- **Sprint 5 tech-writer Issue 1 (BLOCKER)** — closed end-to-end on
  the YAML side; the architect's chapter-19 retarget closes the prose
  side in parallel this sprint.
- **Sprint 5 staff Issue 2** (manifest IBM-Secret retire deferred) —
  closed; the deferral landed cleanly.
- **Sprint 5 architect** chapter-19 scope: the YAML now ships the
  shape the chapter prose can describe truthfully; the architect's
  pass against the new shape needs the new template tokens
  (`${OPS_IRSA_ROLE_ARN}` + `${OPS_IMAGE}`) called out, the secret
  references deleted, and the trust chain re-drawn against IRSA.
- **PRD 04 §"In-cluster identity"** — the canonical design ref the
  retargeted YAML now matches.
- **PRD 08** — provides the `terraform/modules/iam_irsa` output
  (`ops_role_arn`) that the CLI command will eventually resolve
  `${OPS_IRSA_ROLE_ARN}` from at install time.

## SPIKE DEFERRAL carry

The YAML retarget is validated by spec-check + Go build/test only.
The doctor IRSA probes (`probeOpsPodIRSA` + the `eks.amazonaws.com/
role-arn` annotation lookup) exercise their happy paths only on a
real cluster with the EKS pod-identity webhook running — that
exercise gates on the operator-run PRD 07 spike per the SPIKE
DEFERRAL framing the whole AWS retarget operates under. v1.0
candidate ships with the YAML structurally correct; v1.0 tag waits
on the spike to confirm IRSA-shape end-to-end against a live EKS
cluster.
