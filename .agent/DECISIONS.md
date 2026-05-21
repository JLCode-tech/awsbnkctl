# Architecture Decision Records

> Decisions are permanent — never delete, only supersede.

Last Updated: 2026-05-21

---

## Accepted Decisions

### D-001 — Remove Terraform; provisioning is strict Go SDK with imperative phases (2026-05-21)

**Context.** Terraform caused four real, compounding pains: TF-binary distribution overhead, `.tfstate` fragility, two control planes for the same AWS account (TF + bash/SDK), and slow iteration on the apply→fix→reapply cycle. The aws-gpu-setup PoC (`/Users/j.lucia/Code/aws-gpu-setup/`) proved an imperative awscli + YAML method works for the same job at much higher iteration velocity.

**Decision.** Remove Terraform from awsbnkctl. Provisioning becomes **strict Go SDK only** (no shell-out to `aws` or `kubectl`), organized as sequential imperative **phase functions** ported from aws-gpu-setup's `up.sh` / `down.sh` shape — NOT a reconciler framework.

**Consequences.**
- Delete: `terraform/`, `internal/tf/`, `embedded.go` (TF embed), `install_build_dependencies.sh`, `internal/cli/tfvars.go`.
- `internal/aws/` grows to cover what TF did (already has typed wrappers for ec2/eks/iam/s3/vpc/sts).
- Single-binary distribution restored. forge MCP integration keeps typed Go contracts.
- Adopts the *method* from aws-gpu-setup; does **not** vendor or subprocess the bash.
- Current TF stack and TF-created clusters: ignored (no in-flight clusters depend on it; manual cleanup by operator).

**See:** [`docs/POST_TERRAFORM_DIRECTION.md`](../docs/POST_TERRAFORM_DIRECTION.md) — full buildable spec.

---

### D-002 — State model: AWS tags as truth + local IDs cache as convenience (2026-05-21)

**Context.** With TF state gone, awsbnkctl needs a way to know "what did I create" for status and destroy. Two real options: a local state file (recreates the `.tfstate` problem in a new format) or AWS tags + discovery (matches the existing "AWS is truth" thesis from [`docs/FORGE_MCP_INTEGRATION.md`](../docs/FORGE_MCP_INTEGRATION.md)).

**Decision.** **AWS tags are the source of truth.** Every awsbnkctl-created resource carries `awsbnkctl:cluster=<name>` plus component/pattern tags. A small `.awsbnkctl/<cluster>/state.env` file caches resource IDs for speed, but it is **rebuildable from tags at any time** and tag-discovery is the fallback when the cache is missing or corrupt.

**Consequences.**
- `status`, `doctor`, `inspect` query AWS directly by tag, never read forge or rely solely on the cache.
- `down` reads the cache first; on missing/corrupt cache, falls back to tag-discovery — never silently no-ops.
- `state.env` is in `.gitignore`. Lossy by design.
- Tag scheme: `awsbnkctl:cluster`, `awsbnkctl:component`, `awsbnkctl:pattern`, `awsbnkctl:managed=true` + `Name`. Plus any `cluster.yaml: tags:` / `metadata.labels:` merged in.

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §9–10.

---

### D-003 — Intent format: `cluster.yaml` (structured, canonical) (2026-05-21)

**Context.** With TF gone, awsbnkctl needs a declarative intent surface. aws-gpu-setup uses flat `vars.env`; dpubnkctl uses YAML examples as named topologies. Strict Go SDK removes the bash-source benefit of vars.env. Forge MCP wants typed I/O.

**Decision.** **`cluster.yaml`** — Kubernetes-style (`apiVersion: awsbnkctl/v1`, `kind: Cluster`), structured, Go-struct-validated, forge-readable. One file per cluster. No `vars.env` emit; bash-source compatibility is not a goal under strict Go SDK.

**Consequences.**
- `internal/intent/cluster.go` defines the schema. Unknown fields are errors in v1.
- `metadata.name` is the value of `awsbnkctl:cluster=<name>` tag and the directory name under `.awsbnkctl/`. Must match EKS cluster name regex.
- `network.azs` is explicit (no auto-pick) for reproducibility.
- `pattern: <name>` selects k8s manifest variant directory (see D-004).
- Schema evolution: `apiVersion: awsbnkctl/v1` reserves room for v2 without breakage.

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §5.

---

### D-004 — K8s manifests live in variant directories selected by `cluster.yaml: pattern:` (2026-05-21)

**Context.** Cluster-side YAMLs differ across deployment patterns — currently host-device ENI, planned next: SR-IOV to TMM. Switching patterns must NOT require a Go SDK code change. aws-gpu-setup keeps these in `manifests/*.yaml` flat; dpubnkctl uses examples/ as named topologies.

**Decision.** **Variant directories** under `internal/k8s/manifests/`:
- `internal/k8s/manifests/shared/` — pattern-agnostic (cert chain, license CR, pull secrets, otel)
- `internal/k8s/manifests/host-device/` — current pattern
- `internal/k8s/manifests/sr-iov-tmm/` — planned

`cluster.yaml: pattern: <name>` selects the directory. Adding a third pattern = add a directory. Manifests are embedded via `go:embed`, rendered with Go `text/template` (NOT envsubst), applied via existing `internal/k8s/apply.go` (already client-go-based, already strict).

**Consequences.**
- Operator/agent changes pattern by editing one field in `cluster.yaml` — no recompile.
- Apply order: `shared/` first (alphabetical within), then `<pattern>/` (alphabetical within).
- Namespace stamped with `awsbnkctl.io/pattern: <pattern>` label after apply, so forge GUI can surface it without a bnk-forge schema change.
- Templating uses typed struct from `cluster.yaml`, not env vars.

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §11.

---

### D-005 — SSO sentinel pattern from aws-gpu-setup ported to Go middleware (2026-05-21)

**Context.** aws-gpu-setup's `lib/lab-core.sh` solves a hard mid-run failure mode: SSO token expiry during a multi-phase script causes downstream phases to silently no-op (because `... || true` swallows the error), producing a false-positive "DONE" exit. The fix in bash: `aws_q` wrapper writes a sentinel file on auth errors; `check_auth_or_die` at every phase banner reads it and hard-exits.

**Decision.** **Port the sentinel pattern into a Go SDK middleware** (`internal/aws/awsmw/sso.go`). Every AWS client constructed by awsbnkctl is wrapped with `WithSSOWatch`, which inspects deserialized errors and sets a process-level `atomic.Bool` on `ExpiredToken` / `InvalidClientTokenId` / `UnauthorizedException` / SSO-session-expired codes. Every phase function begins with `CheckAuthOrDie(profile)`, which hard-exits with the exact `aws sso login --profile <X>` hint.

**Consequences.**
- No silent no-op cascade on mid-run SSO expiry.
- Up-front protection too: `sts.GetCallerIdentity` in phase 00.
- Unit-testable by injecting fake auth errors via SDK middleware.
- Operators get an actionable error message, not a confusing AccessDenied stack.

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §8.

---

### D-006 — Forge register fires on EKS-Active (not at end of `up`); soft-fail with retry (2026-05-21)

**Context.** [`docs/FORGE_MCP_INTEGRATION.md`](../docs/FORGE_MCP_INTEGRATION.md) defines the forge handoff model (peer-read; awsbnkctl creates project + cluster records, forge reads AWS on its own creds). Two questions it left unresolved: *when* in the up flow register fires, and what happens if forge is unreachable.

**Decision.**
- **Register fires on EKS-Active, before BNK install** — not at end of `up`. Forge's own scan polling then surfaces BNK-install progress in the GUI during the longest phase. STS-presigned bootstrap kubeconfig has 15-min TTL, plenty of headroom.
- **Soft-fail with retry** (3 attempts, exponential backoff). On final failure, AWS infra stays up; `up` exits 0 with a warning; `forge-link.json` written with `status: pending`; operator runs `awsbnkctl forge register` later. AWS infra is the expensive thing; do not roll it back for a localhost dev-server hiccup.
- **`down` calls forge `unregister` by default**; `--keep-forge-link` flag preserves the project record. Matches the `--keep-iam` / `--keep-keypair` flag family.

**Consequences.**
- Operators watching forge see the cluster appear within ~10 min of `up` start, not after ~20+ min.
- `internal/cli/up.go` calls `internal/forge/register.go` between phase 08 (EKS active) and phase 09 (node group).
- Pattern variant (`host-device`, `sr-iov-tmm`) is exposed to forge via namespace label `awsbnkctl.io/pattern: <name>` — no bnk-forge schema change.

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §3.

---

### D-007 — Tracer-bullet first slice: cluster.yaml → tagged VPC + subnets only (2026-05-21)

**Context.** D-001 through D-006 commit to a large architectural shift. Doing it as one big-bang rewrite would mean weeks with no shippable work. Module-by-module migration over the old TF code adds coexistence cost. User direction: greenfield build, no migration tax.

**Decision.** **Tracer-bullet first slice** = `cluster.yaml` → tagged VPC + subnets + IGW + NAT + RTs via Go SDK, symmetric `up`/`down`, IDs cache write, idempotent re-run. **No EKS, no IAM, no k8s, no forge.** Smallest deliverable that exercises every architectural commitment: intent format, SDK shape, tag scheme, IDs cache, auth sentinel, post-condition waits, idempotency, symmetric destroy.

**Consequences.**
- ~10-file PR scope: intent loader, middleware, tags, state, 5 phase files, up/down CLI, example, tests.
- Acceptance criteria spelled out in spec §13. Critical ones: idempotent re-run, tag-discovery fallback when cache deleted, SSO mid-run expiry triggers hard exit with the right hint.
- Subsequent slices stack on top: IAM (slice 2), EKS + node group (slice 3), forge register (slice 4), k8s + variants (slice 5), polish (slice 6).

**See:** `docs/POST_TERRAFORM_DIRECTION.md` §13.

---

## Superseded Decisions

(None)
