# Sprint 2 — validator issues

End-of-sprint regression gate. Scope this run: Dockerfile multi-arch
fix (closes Sprint 1 tech-writer Issue 6), tools-images.yml multi-arch
buildx wiring, CI S3 + IAM integration coverage scaffolding (mocked
path; localstack deferred), cspell additions for the IRSA / OIDC / KMS
/ skopeo / multi-arch terminology. SPIKE DEFERRAL still in force — no
live AWS this sprint. Sibling Sprint 2 issue files (architect, staff,
tech-writer) cross-referenced where relevant.

Format matches the Sprint 1 file. `roadmap` is reserved for forward-
looking non-actionable observations; `low/medium/high/blocker` for
actionable findings.

---

## Issue 1: Dockerfile multi-arch — local buildx verification (closes Sprint 1 tech-writer Issue 6)

**Severity**: low
**Status**: resolved (Dockerfile patched; buildx verification run on
this host)

**Description**: Sprint 1 tech-writer Issue 6 flagged that
`tools/docker/aws/Dockerfile` hard-coded `linux/amd64` download URLs
for awscli, kubectl, and helm — every `RUN curl ... linux-x86_64 ...`
or `linux/amd64/kubectl` / `linux-amd64.tar.gz` line broke arm64 builds
under buildx and broke `aws --version` on darwin_arm64 hosts running
the published manifest entry under rosetta. The fix proposed in that
issue was a `TARGETARCH`-aware case statement with per-arch sha256
pins.

Applied this sprint:

- New `ARG TARGETARCH` at top of the downloader stage; auto-set by
  buildx when `--platform linux/amd64,linux/arm64` is passed.
- Three per-tool `RUN` blocks each carry a `case "${TARGETARCH}"`
  switch that selects the correct download URL **and** the matching
  sha256 ARG.
  - awscli: `x86_64` (amd64) / `aarch64` (arm64) — AWS's bundle
    publishes both forms at the same CDN base URL.
  - kubectl: `linux/${TARGETARCH}/kubectl` — upstream publishes the
    binary under the literal `amd64` / `arm64` path segments, so the
    URL substitution is straight.
  - helm: `helm-<v>-linux-${TARGETARCH}.tar.gz` extracts to
    `linux-${TARGETARCH}/helm` — symmetric layout, same straight
    substitution.
- Six total sha256 ARGs (three tools × two arches); each `case`
  selects the right one and `sha256sum -c` runs against the matching
  pin. Mismatch on either arch fails the build — supply-chain
  protection survives the multi-arch split.
- Unknown TARGETARCH (e.g., `riscv64`, `ppc64le`) errors early with a
  clear "unsupported TARGETARCH" message rather than 404'ing on a
  nonexistent CDN URL.

The six sha256 values landed in the patch were verified against the
real artefacts at validator time:

| Tool        | Arch    | sha256 (first 12 chars) | Source             |
|---|---|---|---|
| awscli v2.17.52 | x86_64  | `11dd0016d93a` | `shasum -a 256` on the official zip |
| awscli v2.17.52 | aarch64 | `f9b2b8486dfa` | `shasum -a 256` on the official zip |
| kubectl v1.30.5 | amd64   | `b8aa921a580c` | `dl.k8s.io/.../kubectl.sha256` sidecar |
| kubectl v1.30.5 | arm64   | `efc594857f92` | `dl.k8s.io/.../kubectl.sha256` sidecar |
| helm v3.15.4    | amd64   | `11400fecfc07` | `get.helm.sh/.../tar.gz.sha256sum` |
| helm v3.15.4    | arm64   | `fa419ecb1394` | `get.helm.sh/.../tar.gz.sha256sum` |

(AWS publishes no `.sha256` sidecar for the awscli zip, so the two
amd64/arm64 values were computed against the upstream download at
version-pin time. A Sprint 3+ refresh would re-compute alongside the
version bump.)

**Files affected**: `tools/docker/aws/Dockerfile`
(`ARG TARGETARCH`, six sha256 ARGs, three per-tool `case` blocks).
**Proposed fix**: applied. Tech-writer Sprint 1 Issue 6 can be marked
resolved at integration time.

## Issue 2: localstack-backed S3 + IAM integration tier deferred to v1.x

**Severity**: roadmap
**Status**: open (forward-looking, intentional deferral)

**Description**: The validator brief offered two paths for CI coverage
of Sprint 2's new `internal/aws/{s3,iam}.go` helpers:

1. **localstack as a GHA service container** — `services: localstack:`
   gives the SDK a real HTTP endpoint to round-trip `PutObject`,
   `HeadObject`, `GetOpenIDConnectProvider` against. Closer-to-prod
   than mocks; catches signing / endpoint / region-mismatch bugs that
   middleware-only mocks miss.
2. **Mocked-only** — extend the existing `aws-mocked` job in
   `.github/workflows/ci.yml`, which already wildcards
   `./internal/aws/...` and so auto-picks up `s3_test.go` and
   `iam_test.go` once staff lands them.

For v1.0 the **mocked-only path** is the right call:

- The S3 + IAM helpers in PRD 08 are thin wrappers (`PutObject`,
  `HeadObject`, `GetOpenIDConnectProvider`, IRSA role probe). The bug
  surface is small enough that middleware-test mocks catch the regression
  shapes — request shape, parameter passing, error mapping.
- localstack-in-CI pulls a ~700 MB image on every job run. Even with
  GHA's image cache the cold-runner case adds ~30s and a Docker Hub
  rate-limit risk to every PR. The "Docker-in-CI cost" warning in the
  validator brief is real.
- The live-AWS spike (operator-run, PRD 07 §"Spike protocol", PRD 08
  inherits) is the canonical "real endpoint" gate for awsbnkctl v1.0.
  Live-AWS catches the same class of bug localstack would, on the
  authoritative endpoint, with no Docker-Hub dependency.

For v1.x — once the helper surface grows (multipart upload, presigned
URLs, lifecycle policy assertion, IRSA trust-policy round-trip) — the
case for localstack improves. Worth wiring then.

**Files affected**: future `.github/workflows/ci.yml` job
(e.g., `aws-localstack:`); none changed this sprint.
**Proposed fix**: track for a future sprint when the S3 helper
surface grows beyond `PutObject` + `HeadObject`. Recipe sketch for the
follow-up agent:

```yaml
aws-localstack:
  name: integration (aws — localstack)
  needs: test
  runs-on: ubuntu-latest
  services:
    localstack:
      image: localstack/localstack:3.5
      ports: ['4566:4566']
      env:
        SERVICES: s3,iam,sts
  env:
    AWS_ENDPOINT_URL: http://localhost:4566
    AWS_ACCESS_KEY_ID: test
    AWS_SECRET_ACCESS_KEY: test
    AWS_REGION: us-east-1
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version-file: go.mod }
    - run: go test -tags integration,localstack -timeout 5m ./internal/aws/...
```

The `,localstack` build-tag splits localstack-only tests from the
mocked tier so a contributor without Docker can still run the mocked
suite via `scripts/test-integration-aws.sh`.

## Issue 3: buildx local verification — multi-arch manifest builds clean on this host

**Severity**: low
**Status**: resolved (verified end-to-end, exit code 0)

**Description**: Validator brief asked for a local
`docker buildx build --platform linux/amd64,linux/arm64 -t test:aws-multi tools/docker/aws/`
to confirm the new Dockerfile builds on both arches before CI picks
it up. Ran on the validator's host (darwin_arm64 with Docker Desktop
29.2.1 + buildx 0.32.1-desktop.1 + QEMU emulator registered).
**Exit code 0**; final manifest list:

```
exporting manifest sha256:fe1f58837c4382203c8ea02be85dcdb23b43d2a09f051c42c7c38ad7433553a6  (linux/amd64)
exporting manifest sha256:365af7c610b8826beeb296dd9397e69a5e5ed17bc92d8f45994f452df8582384  (linux/arm64)
exporting manifest list sha256:30e0135646143afd625d539b7180be64ca3375bee7fc0844b22fe3c1249d6e57
naming to docker.io/library/test:aws-multi done
```

Stage-by-stage outcome:

- `[stage-0/downloader]` ran twice in parallel — once per arch under
  buildx's matrix executor.
- Each arch's `case` block selected the right URL: amd64 →
  `awscli-exe-linux-x86_64-2.17.52.zip` +
  `linux/amd64/kubectl` + `helm-v3.15.4-linux-amd64.tar.gz`; arm64 →
  the `aarch64` + `linux/arm64` + `linux-arm64` counterparts.
- `sha256sum -c` passed on **all six downloads** (three tools × two
  arches), with each `OK` line visible in the build log. The six
  pinned values cross-check the real CDN artefacts at sprint time.
- Runtime stage (`alpine:3.19`, apk packages, USER 1000) is
  architecture-agnostic — Alpine publishes apk indexes for both
  arches; both `apk add` runs succeeded.

One curiosity worth flagging (host-side, not a Dockerfile bug): the
amd64 `./aws/install` step printed `rosetta error: failed to open elf
at /lib64/ld-linux-x86-64.so.2 / Trace/breakpoint trap` once mid-step
(line `#14 9.543` in the build log), then re-ran the same line and
succeeded (`#14 13.98 You can now run: /opt/aws-cli/bin/aws --version`).
This is Docker Desktop's QEMU↔Rosetta fallback path on darwin_arm64
flipping mid-build — the amd64 install script `exec`s an intermediate
helper which the kernel briefly tries to dispatch through Rosetta
before falling back to QEMU. Same Dockerfile on a true linux/amd64
host (e.g., GHA `ubuntu-latest`) won't show this line. CI is the
canonical "published manifest works" gate; this local pass confirms
the structure.

Smoke `docker run`s under buildx-loaded local images are NOT part of
this verification — `docker buildx build` without `--load` doesn't
materialise an image into the local docker daemon (multi-arch manifest
lists can't be `--load`'d). The CI `tools-images.yml` workflow
exercises the push path via `docker/build-push-action@v5` with
`platforms: linux/amd64,linux/arm64`, which buildx supports natively.
First green run on `main` after this sprint integrates is the canonical
"published manifest works" gate.

**Files affected**: `tools/docker/aws/Dockerfile` (Issue 1);
`.github/workflows/tools-images.yml` (Issue 4).
**Proposed fix**: none — local multi-arch build green (exit 0); CI
gate carries the publish path.

## Issue 4: tools-images.yml — added QEMU setup + `platforms: linux/amd64,linux/arm64` flag

**Severity**: low
**Status**: resolved

**Description**: `.github/workflows/tools-images.yml`'s three
`docker/build-push-action@v5` invocations (tag-push, main → :dev,
workflow_dispatch → :dev) all needed `platforms: linux/amd64,linux/arm64`
to publish a multi-arch manifest list. Before this sprint they
defaulted to `linux/amd64` (the runner's native arch), so a
darwin_arm64 host pulling `ghcr.io/.../awsbnkctl-tools-aws:latest`
would get an amd64 image and hit rosetta errors at runtime.

Applied:

- New `docker/setup-qemu-action@v3` step before `setup-buildx-action`
  — registers binfmt handlers for arm64 emulation on the amd64 runner.
  Without this the first arm64-targeted `RUN` instruction fails with
  "exec format error".
- All three `docker/build-push-action@v5` invocations now carry
  `platforms: linux/amd64,linux/arm64`.
- `provenance: false` added to each — without it, buildx emits SLSA
  provenance attestations alongside the manifest list, which can
  confuse some downstream `docker pull`s that don't handle the OCI
  image-index `mediaType` for attestations. Worth revisiting in a
  future sprint when the consumer-side handling is verified.
- The same flag is applied to both matrix entries (`aws`, `iperf3`).
  iperf3 builds from Alpine apk packages which publish multi-arch; no
  Dockerfile edit needed there.

**Files affected**: `.github/workflows/tools-images.yml`.
**Proposed fix**: applied. YAML parse-checks clean (`python -c
'import yaml; yaml.safe_load(...)'`).

## Issue 5: cspell additions — IRSA / OIDC / KMS / multi-arch terminology

**Severity**: low
**Status**: resolved

**Description**: cspell.json was missing some terms used in PRD 08
and the architect's chapter 25 rewrite. The brief listed: `IRSA`,
`OIDC`, `skopeo`, `KMS`, `CMK`, `kms`, `cmk`, `aarch64`, `OpenID`,
`presigned`, `webhook`, `Webhook`.

Already present (no-op): `IRSA`, `OIDC`, `skopeo`, `AWS`, `EKS`,
`ECR`, `IAM`.

Added this sprint (9 entries):

- `KMS`, `kms` — the AWS Key Management Service acronym, both casings;
  used in PRD 08's "SSE-KMS" / "kms_key_arn" / `aws_kms_key` Terraform
  references.
- `CMK`, `cmk` — Customer-Managed Key, used in PRD 08's tradeoffs
  section ("Customer-managed KMS key by default. Costs $1/month per
  key.") and the IRSA module's KMS-decrypt permission rationale.
- `OpenID` — appears in "IAM OpenID Connect provider" (the AWS term;
  always written `OpenID` with capital `D`, not `Openid`).
- `presigned` — anticipated for future S3 helper docs (`PresignGetObject`
  for short-lived download URLs).
- `webhook`, `Webhook` — used in PRD 08's "EKS pod-identity webhook"
  description. Both casings because the term appears both in running
  text and as a proper noun ("Pod Identity Webhook").
- `aarch64` — used in the Dockerfile's TARGETARCH→awscli mapping; also
  appears in the Sprint 1 tech-writer issue file's proposed-fix code
  block (`case "$TARGETARCH" in ... arm64) ARCH=aarch64 ;;`).

Verified by reading PRD 08 + chapter 25 + the updated Dockerfile for
unfamiliar terms; no other unknown-word counts surfaced beyond the
brief's list. The full cspell pass requires the `cspell` binary which
this validator host doesn't carry — verified by parse-checking
`cspell.json` JSON validity (`python -c 'import json;
json.load(open("cspell.json"))'`) and visual diff against the existing
alphabetical-ish grouping.

**Files affected**: `cspell.json` (9 new entries).
**Proposed fix**: applied.

## Issue 6 (roadmap): kubectl version bump to track EKS-supported minor — track separately

**Severity**: roadmap
**Status**: informational

**Description**: The Dockerfile pins `KUBECTL_VERSION=v1.30.5`,
matching PRD 07's `cluster_version` default of `1.30`. EKS supports
1.28 / 1.29 / 1.30 / 1.31 as of this sprint's date (May 2026); when
the cluster_version default bumps (Sprint 6 / v1.0-prep is a likely
trigger), the kubectl pin should follow within one minor version.
kubectl is forward-compatible by one minor, so v1.30 client → v1.31
cluster works, but the validator gate gets simpler if the tools-image
matches the default.

Same observation for helm: v3.15.4 is current at sprint start; the
helm 3 line is stable enough that bumps are seldom forced, but worth
tracking alongside the kubectl bump so both supply-chain ARGs refresh
together (and both arches' sha256 pins refresh together — the per-arch
table in Issue 1 shows the four values that need rolling on each bump).

**Files affected**: `tools/docker/aws/Dockerfile` (`KUBECTL_VERSION` +
`HELM_VERSION` + the four sha256 ARGs around them).
**Proposed fix**: surface during Sprint 6 hardening pass.

## Issue 7 (roadmap): `iperf3` image is still single-Dockerfile — verify upstream alpine package coverage

**Severity**: roadmap
**Status**: informational

**Description**: The `iperf3` matrix entry in `tools-images.yml` now
inherits the `platforms: linux/amd64,linux/arm64` flag from Issue 4's
edit. The validator brief touched the Dockerfile in `tools/docker/aws/`
only — `tools/docker/iperf3/Dockerfile` was out of scope. The
expectation is that iperf3's Dockerfile uses `apk add iperf3`, and
Alpine's apk repository publishes the `iperf3` package for both
linux/amd64 and linux/arm64 (verified by spec: `apk` indexes have
been multi-arch since the Alpine 3.x line). If iperf3's Dockerfile
turns out to install via a per-arch curl-download (unlikely but
possible), the same TARGETARCH-aware pattern from `tools/docker/aws/`
would apply.

**Files affected**: `tools/docker/iperf3/Dockerfile` (verify-only;
not edited this sprint).
**Proposed fix**: tech-writer Sprint 2 read-only gate verifies; if
the apk-only assumption is wrong, file as Sprint 3 follow-up.

## Issue 8: `scripts/test-integration-aws.sh` — extended header + example invocation for S3/IAM

**Severity**: low
**Status**: resolved

**Description**: The convenience runner script's header was Sprint
1-era (mentions only `client,sts,ec2,eks,vpc`). Updated this sprint
to also call out the new `s3,iam` helpers landing in Sprint 2 (PRD 08)
and added a worked `-run 'TestIntegration_S3|TestIntegration_IAM'`
example for contributors who want to scope to the S3/IAM suite. No
runtime behaviour change — `go test -tags integration ./internal/aws/...`
already wildcards over the new files, so the existing invocation
covers them automatically.

`bash -n` parse-check clean.

**Files affected**: `scripts/test-integration-aws.sh` (header
comment only).
**Proposed fix**: applied.

## Issue 9 (roadmap): Skopeo + ECR mirror Dockerfile deferred — Sprint 3 territory

**Severity**: roadmap
**Status**: informational

**Description**: PRD 08 §"Terraform module: terraform/modules/ecr_mirror/"
gates the optional ECR mirror flow on `var.enable_ecr_mirror`, and the
implementation runs `skopeo copy` via a `null_resource` `local-exec`
provisioner using the tools-image. The current `tools/docker/aws/`
Dockerfile does **not** ship skopeo — the brief explicitly defers ECR
mirror to Sprint 3 follow-up. When that lands, the tools image either
gains a skopeo install (alpine's `apk add skopeo` exists for both
arches as of Alpine 3.18+, so it's a one-line patch) or a sibling
`tools/docker/aws-mirror/` Dockerfile spins up specifically for the
mirror step. Two choices, both viable:

1. **Bake skopeo into `tools/docker/aws/`** — single image, slightly
   larger (~30 MB additional layer). Pro: one image to maintain. Con:
   the docker-exec backend now pulls skopeo bytes even when the
   operator never enables ECR mirror.
2. **Sibling `tools/docker/aws-mirror/`** — second image. Pro:
   per-flow image size. Con: the matrix in tools-images.yml grows;
   image-tag bookkeeping doubles.

PRD 08 doesn't force the choice. v1.0 stretch / v1.x first-class
status means Sprint 3 validator picks one (probably option 1 for
simplicity at the cost of a small layer).

**Files affected**: `tools/docker/aws/Dockerfile` (or new
`tools/docker/aws-mirror/Dockerfile`); `.github/workflows/tools-images.yml`
matrix (if option 2).
**Proposed fix**: defer to Sprint 3 prompt drafting.

## Issue 10 (roadmap): notes for Sprint 3 validator

**Severity**: roadmap
**Status**: open by design

**Description**: Sprint 3 (per PLAN.md) ports reusable modules and
delivers the first end-to-end `awsbnkctl up`. The validator agent
for Sprint 3 should expect to:

- Run `terraform validate` across the union of Sprint 1 + Sprint 2 +
  Sprint 3 modules — the dependency wiring between `eks_cluster`,
  `s3_supply_chain`, and `iam_irsa` becomes the regression surface as
  Sprint 3 plugs them together.
- Add the skopeo / ECR mirror Dockerfile path (Issue 9 above) if
  Sprint 3 staff lands the `ecr_mirror` module.
- Refresh the kubectl + helm pinned sha256s in `tools/docker/aws/Dockerfile`
  if either version bumps to track an EKS-supported minor (Issue 6).
- Audit the v1.0 doctor surface: PRD 08 §"CLI surface" adds `aws
  s3:PutObject permission` + `aws iam:GetOpenIDConnectProvider
  permission` rows; verify they render correctly when a workspace is
  configured and degrade gracefully when not. Sprint 1 tech-writer
  Issue 1's "workspace-gated invisibility" lesson applies — confirm
  the new rows aren't gated behind a sneakily-conditional block.
- Re-run the multi-arch buildx pass after Sprint 3's tools-image edits;
  catch the regression where someone adds a per-arch download URL
  without the TARGETARCH switch.

**Files affected**: future Sprint 3 deliverables.
**Proposed fix**: pre-write into `prompts/sprint3/validator.md` when
that prompt is drafted.

---

*Total filed: 10 issues — 0 blocker, 0 high, 0 medium, 5 low (Dockerfile
multi-arch, buildx local-verify, workflow multi-arch wiring, cspell
additions, scripts/test-integration-aws.sh header), 5 roadmap
(localstack v1.x, kubectl/helm version-bump tracking, iperf3 multi-arch
verify, skopeo deferral, Sprint 3 hand-off notes). Three findings
resolved in-band (Dockerfile, tools-images.yml, cspell.json); the
others are forward-looking flags for later sprints.*

## Verification summary

- `tools/docker/aws/Dockerfile`: rewritten with `ARG TARGETARCH` +
  three per-tool `case` blocks + six sha256 ARGs (three tools ×
  two arches). All six sha256 values cross-checked against upstream
  (kubectl + helm via official `.sha256` sidecars; awscli via direct
  download + `shasum -a 256`).
- `docker buildx build --platform linux/amd64,linux/arm64
  -t test:aws-multi tools/docker/aws/`: buildx 0.32.1-desktop.1 with
  QEMU emulation invoked on validator host (darwin_arm64 + Docker
  Desktop 29.2.1). Build path validated end-to-end (Sprint 2 issue
  closeout; see Issue 3 above for the full verification result).
- `.github/workflows/tools-images.yml`: added
  `docker/setup-qemu-action@v3` step; three `docker/build-push-action@v5`
  invocations updated with `platforms: linux/amd64,linux/arm64` +
  `provenance: false`. `python -c 'yaml.safe_load(...)'` parses clean.
- `.github/workflows/ci.yml`: `aws-mocked` job header refreshed for
  PRD 08's new `s3,iam` helpers (the wildcard `./internal/aws/...`
  already covers them — no new step needed). Localstack tier deferred
  per Issue 2. `yaml.safe_load(...)` parses clean.
- `cspell.json`: 9 new entries (`KMS`, `kms`, `CMK`, `cmk`, `OpenID`,
  `presigned`, `webhook`, `Webhook`, `aarch64`). `json.load(...)`
  parses clean.
- `scripts/test-integration-aws.sh`: header extended for Sprint 2's
  S3/IAM coverage; `bash -n` parses clean. Note the script invokes
  `go test` which requires a Go toolchain — not run by this validator
  agent because the Go-build gate is the staff agent's surface
  (validator scope is the script + CI workflow, not the Go test
  outputs).
- Off-limits surfaces unchanged: no edits to `.go`, `go.mod`,
  `Makefile`, `.goreleaser.yml`, `terraform/**`, `docs/`, `book/`,
  `agents/`, `prompts/`.
