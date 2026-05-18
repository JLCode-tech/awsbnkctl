# Forge MCP Integration — Plan

**Status:** P1 implemented — MCP-only (Option B). REST fallback eliminated by `bnk-forge#114`.
**Owner:** awsbnkctl maintainers
**Companion repo:** `bnk-forge-v2` (localhost dev at `http://localhost:8000`; MCP at `http://localhost:8081/mcp/`)
**Last updated:** 2026-05-18

## 1 · What we're trying to do

After `awsbnkctl up` finishes provisioning AWS infra + EKS + BNK, hand the resulting deployment over to **bnk-forge** so the operator can manage and observe it from forge's UI. The handoff happens over forge's **MCP server** (with REST fallback for surfaces that aren't yet exposed as MCP tools).

Concretely, after a successful `up`, awsbnkctl will:

1. Create (or attach to) a forge **project** that represents the workspace.
2. Register the **EKS cluster** under that project so forge has a `cluster_id` it can address.
3. Adopt the Terraform **state bundles** awsbnkctl produced as forge **project-modules**, so forge sees the deployment topology and can plan/apply/destroy in-place.
4. Trigger a forge **scan** to seed BNK / namespace / Helm-release state.

From that point forward, forge owns the day-2 view (health, upgrades, license, recovery, drift), and awsbnkctl steps back to a CLI / bootstrapping role.

---

## 2 · Forge MCP surface — what's available today

Read directly from `bnk-forge-v2/mcp-server/tools/mcp_tool_catalog.json` (69 tools, ≥`pre-2.10.71`).

| Module | Tools | What we need from it |
|---|---|---|
| `bnk_operations` (19) | `bnk_health`, `bnk_gateway_topology`, `bnk_data`, `bnk_upgrade_*`, `bnk_license_status`, `bnk_recovery_*` | Post-handoff day-2: forge reads BNK directly once cluster is registered. |
| `cluster_management` (14) | `list_clusters`, `get_cluster`, `scan_cluster`, `test_cluster_connectivity`, `list_namespaces`, `list_resources`, `get_pod_logs`, … | After registration: forge surfaces the cluster + scans for capabilities. |
| `iac_operations` (15) | `list_projects`, `get_project`, `list_project_modules`, `project_plan`, `project_apply`, `project_destroy`, `deployment_history`, `list_stacks`, `deploy_stack` | Run + observe the IaC lifecycle once awsbnkctl's state is adopted. |
| `helm` (11) | `helm_list_releases`, `helm_get_release`, `helm_get_values`, `helm_install`, `helm_upgrade`, `helm_rollback`, `helm_uninstall` | Verify FLO / CIS / CNEInstance helm releases that awsbnkctl installed. |
| `config_management` (4) | `config_export`, `config_diff`, `config_import`, `config_promote` | Snapshot BNK config; promote between environments later. |
| `system` (6) | `system_health`, `system_version`, `system_settings`, `audit_log`, `system_queue_metrics`, `list_users` | Pre-flight; verify forge is reachable + at compatible version. |

### Gaps (mutating REST endpoints not yet in MCP catalog)

These are the create-paths we need but which haven't been promoted to MCP tools yet. We'll call them as REST in v1 and propose MCP-catalog additions as a follow-up PR against `bnk-forge-v2`.

| Operation | REST endpoint | MCP catalog status |
|---|---|---|
| Create project | `POST /api/projects` | not exposed |
| Create cluster under project | `POST /api/projects/{project_id}/k8s/clusters` | not exposed |
| Auto-detect EKS clusters from project modules | `POST /api/projects/{project_id}/k8s/clusters/detect-eks` | not exposed |
| Create project-module (adopt external TF state) | `POST /api/project-modules/...` | not exposed |
| Login (token) | `POST /api/auth/login` | n/a (out-of-band) |

---

## 3 · Data model mapping (awsbnkctl → forge)

```
awsbnkctl workspace                    forge project
├── terraform state (root)         →   project-module: "root" (adopted external state)
├── terraform/modules/eks               project-module: "eks-cluster"
├── terraform/modules/sriov-nodegroup   project-module: "eks-sriov-nodes"
├── terraform/modules/irsa-flo          project-module: "flo-irsa"
├── terraform/modules/irsa-cis          project-module: "cis-irsa"
├── terraform/modules/irsa-ops          project-module: "ops-irsa"
├── terraform/modules/ecr_mirror?       project-module: "ecr-mirror" (gated on var.enable_ecr_mirror)
├── terraform/modules/s3-supply-chain   project-module: "s3-supply-chain"
└── EKS cluster (live)             →   cluster (under project)
                                       ├── helm releases: flo, cis, cert-manager, cneinstance
                                       └── BNK CRDs (discovered by forge scan)
```

**Project name convention:** `awsbnkctl-<workspace-name>` (e.g. `awsbnkctl-default`, `awsbnkctl-aws-syd-test`).

**Cluster slug:** `<aws-region>-<eks-cluster-name>` (e.g. `us-east-1-bnk-prod`).

---

## 4 · End-to-end handoff sequence

The integration runs as the final step of `awsbnkctl up` (gated behind a new flag — see §6). All calls go to `${AWSBNKCTL_FORGE_URL:-http://localhost:8000}`.

```
┌─────────────────────────────────────────────────────────────────────┐
│  awsbnkctl up  (existing flow: TF apply → EKS → IRSA → BNK helm)    │
│       │                                                              │
│       ▼                                                              │
│  awsbnkctl forge register   ← NEW subcommand (also runnable          │
│       │                       standalone post-`up`)                  │
│       ▼                                                              │
│  1. POST /api/auth/login                  →  bearer token            │
│  2. POST /api/projects                    →  project_id              │
│  3. POST /api/project-modules (×N)        →  one per TF module,      │
│         body includes adopted state bundle   adopting state from     │
│                                              workspace/state-bundle  │
│  4. POST /api/projects/{id}/k8s/clusters  →  cluster_id              │
│         (or /detect-eks if forge can autodiscover from modules)      │
│  5. MCP scan_cluster(cluster_id)          →  populate namespaces +   │
│                                              helm releases + BNK     │
│  6. MCP bnk_health(cluster_id)            →  smoke-test the handoff  │
│  7. Write forge_link.json to workspace dir:                          │
│         { project_id, cluster_id, forge_url, registered_at }         │
└─────────────────────────────────────────────────────────────────────┘
```

After step 7, the operator opens forge at `http://localhost:8000/projects/<id>` and sees the freshly-deployed stack with live state.

---

## 5 · How awsbnkctl talks MCP

Two options. **Recommendation: Option A** (lower complexity for v1).

### Option A · awsbnkctl as plain HTTP client (recommended for v1)

Forge's MCP server is a thin facade over the REST backend (`mcp-server/src/bnk_forge_mcp/client.py` proxies tool calls to the same REST routes). So in practice awsbnkctl can hit the REST routes directly with a bearer token, without speaking MCP protocol at all.

- **Pro:** no MCP client dependency in Go; one HTTP client covers both "REST gaps" and "tools that happen to also exist in MCP".
- **Pro:** identical to how forge's own frontend talks to its backend.
- **Con:** doesn't exercise the MCP transport, so we don't catch MCP-only regressions.

### Option B · awsbnkctl as MCP client

Bring a Go MCP client library (e.g. `github.com/modelcontextprotocol/go-sdk` once stable, or a hand-rolled JSON-RPC over stdio/HTTP). Call MCP tools by name where available; REST-fallback for gaps.

- **Pro:** future-proof — if forge promotes more endpoints to MCP, we get them for free by switching the call site from REST to MCP-tool-name.
- **Pro:** lets ops chains (Claude Code, other MCP clients) reuse the same tool definitions awsbnkctl uses.
- **Con:** MCP Go ecosystem is still pre-1.0; adds a moving dependency.
- **Con:** auth flow over MCP is less well-defined than REST + bearer.

**Decision criterion:** if forge ships an MCP `register_cluster` tool in the next release, switch to Option B. Until then, Option A.

---

## 6 · CLI surface (awsbnkctl side)

New subcommand, three verbs:

```
awsbnkctl forge register     # one-shot: register current workspace into forge
awsbnkctl forge status       # show forge_link.json + ping MCP/REST
awsbnkctl forge unregister   # remove cluster + project from forge (idempotent)
```

Flags:

```
--forge-url string        forge base URL (default $AWSBNKCTL_FORGE_URL, fallback http://localhost:8000)
--forge-user string       username (default admin)
--forge-password-env      env var to read the password from (default AWSBNKCTL_FORGE_PASSWORD)
--forge-token string      pre-obtained bearer token (bypasses login)
--project-name string     forge project name (default awsbnkctl-<workspace>)
--dry-run                 print the planned API calls without executing
```

**Auto-trigger:** `awsbnkctl up` accepts `--register-with-forge` (also `workspace.yaml: forge.auto_register: true`) to run `forge register` as the final step. Default is **off** in v1 — opt-in to keep the existing offline flow intact.

---

## 7 · State + auth handling

- **Bearer token:** obtained once per `awsbnkctl forge` invocation via `POST /api/auth/login` with `{username, password}` (returns `{token: "..."}`). Token is held in-memory only — never written to disk. Tokens currently expire after ~hours; awsbnkctl re-logs in for each invocation.
- **Workspace link:** `<workspace-dir>/forge_link.json` records `{project_id, cluster_id, forge_url, registered_at, forge_version}`. This is what `forge status` and `forge unregister` read. Versioned alongside `terraform.tfstate` bundles.
- **TF state adoption:** awsbnkctl posts each module's `terraform.tfstate` to forge as an "external state import" (forge's `project_module` schema supports this — verify exact body shape before coding). This is read-only for forge; awsbnkctl remains the owner of the apply path in v1.
- **Idempotency:** all writes are keyed by `(project_name, cluster_slug)`. Re-running `forge register` on an already-registered workspace updates rather than duplicating.

---

## 8 · Phasing

| Phase | Scope | Deliverable | Status |
|---|---|---|---|
| **P0 — design** *(this doc)* | Define mapping, flow, CLI surface | This file | ✅ 2026-05-18 |
| **P3 — MCP catalog gap-fill** | PR against `bnk-forge-v2` adding the missing endpoints | `bnk-forge#114` (initial 4 + Tier A–E = 62 tools; new `cloud_auth` governed module) | ✅ 2026-05-18 (PR open against `staging`) |
| **P1 — MCP integration (Option B, skipped REST)** | `awsbnkctl forge {register, status, unregister}` over MCP transport | New `internal/forge/` + `internal/forge/mcp/` Go packages; `internal/cli/forge.go` Cobra subcommand | ✅ 2026-05-18 (`feat/forge-mcp-integration` branch) |
| **P2 — auto-register on `up`** | `--register-with-forge` flag + workspace config | `awsbnkctl up --register-with-forge` calls `forge register` after a successful apply (no-op in `--dry-run`). Module-as-project-module adoption deferred to v0.2 (see below). | ✅ 2026-05-18 (`feat/forge-up-auto-register` branch — flag wiring; module adoption blocked on catalog) |
| **P5 — drift / observability hooks** | `awsbnkctl status` learns to query forge; `awsbnkctl doctor` gains a forge-reachability row | Status output shows forge-side health alongside local TF state | ⏳ deferred |

### P2 — TF-module-adoption gap

`create_project_module` requires a `module_library_id` that points at forge's module catalog. awsbnkctl's TF modules live in the repo's `terraform/modules/` tree — they're not registered in forge's catalog. Two paths to close the gap, both follow-up work:

1. **Catalog awsbnkctl's modules in `bnk-forge-modules`** so they have IDs forge can reference. Sequential: catalog change → forge release → awsbnkctl adoption code can look up IDs via `list_module_catalog`.
2. **Add a new MCP tool for "register external project module"** that accepts a `path_in_project` + `state_blob` without requiring a catalog ID. This is what the original plan §4 step 3 envisioned. Needs a new PR against `bnk-forge-v2`.

Cluster registration via `create_cluster` works without this, so P2's primary value (one-command handoff) ships today.

P0 + P3 + P1 shipped in one session — Option A (REST) was leapfrogged because PR #114 made the MCP surface complete enough to skip it entirely.

P1 + P2 are the meat of the integration. P3-P5 are polish / future work that doesn't block the operator workflow.

---

## 9 · Testing strategy

- **Unit tests** (`internal/forge/*_test.go`): mock REST client; verify request bodies, idempotency keys, error mapping.
- **`forge register --dry-run`**: prints the planned API calls (`POST /api/projects {...}`, etc.) without executing — covered by a golden test.
- **Localhost E2E** (`scripts/forge-e2e.sh`): given a running `bnk-forge-v2` localhost stack (`make install` in that repo) + a dry-run `awsbnkctl up` workspace, exercise `forge register` end-to-end and assert forge's `/api/projects/{id}` reflects the expected shape. Skipped in CI unless `FORGE_E2E=1` is set.
- **PRD 07 spike integration**: after the operator runs the real-AWS spike and lands a real cluster, `forge register` becomes part of the spike checklist (extends `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Spike protocol — day 3").

---

## 10 · Open questions

These don't block writing the code, but worth resolving before P1 lands.

1. **State adoption shape.** Does `POST /api/project-modules` accept a `terraform.tfstate` blob inline, or does forge expect a URL/path to fetch? *Verify by reading `services/project_module_service.py` in forge.*
2. **Cluster identity collision.** What happens if two awsbnkctl workspaces register the same EKS cluster name under different projects? *Probably an error from forge's unique constraint — define UX.*
3. **EKS auth flow.** Forge needs an EKS-compatible kubeconfig. Does awsbnkctl push its `aws eks update-kubeconfig` result up, or does forge `refresh-kubeconfig` it independently (which requires AWS creds inside forge)? *Lean toward the former — push, don't fetch.*
4. **Multi-tenant credentials.** Localhost forge runs as admin/changeme. In a shared / prod forge, awsbnkctl needs scoped creds. Define credential provisioning UX (likely: forge issues a service-account API key per workspace).
5. **`forge unregister` semantics.** Does it `project_destroy_all` + delete cluster, or does it only sever the link and leave forge's records intact? Default to "sever link, keep records" (safer); add `--purge` for full delete.

---

## 11 · Out of scope (explicit)

- **Migrating forge to manage the *apply* path.** awsbnkctl keeps owning `up` / `down` in v1. Forge observes; it doesn't run TF for us.
- **Multi-cloud parity.** This plan is AWS-only. The cross-cloud abstraction lives in forge, not here.
- **Replacing `awsbnkctl test` with forge-side tests.** Some overlap (forge has `test_cluster_connectivity`), but `awsbnkctl test dns` / `awsbnkctl test bandwidth` stay in the CLI because they need workspace-local context.
- **MCP server *exposed by awsbnkctl*.** That would be Option B-prime: awsbnkctl-mcp surfaces awsbnkctl's own commands as MCP tools. Worth doing eventually (e.g. `claude` agents could orchestrate awsbnkctl + forge over MCP), but not required for the forge integration this doc describes.

---

## 12 · References

- Forge MCP catalog: `bnk-forge-v2/mcp-server/tools/mcp_tool_catalog.json` (69 tools)
- Forge MCP server source: `bnk-forge-v2/mcp-server/src/bnk_forge_mcp/`
- Forge cluster routes: `bnk-forge-v2/backend/routes/k8s/clusters.py`
- Forge IaC routes: `bnk-forge-v2/backend/routes/` (projects, project-modules, stacks)
- Localhost dev creds: `admin` / `changeme` at `POST http://localhost:8000/api/auth/login`
- awsbnkctl CLI lifecycle: `docs/PRD.md`, `docs/prd/00-OVERVIEW.md`
- PRD 07 spike protocol (where day-3 will eventually call `forge register`): `docs/prd/07-EKS-CLUSTER-SRIOV.md`
