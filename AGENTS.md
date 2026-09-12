# AGENTS.md — awsbnkctl Developer & Operator Guide for AI Agents

> This file is the root knowledge base for AI coding agents and automated tools working on or operating `awsbnkctl`.

---

## 1. System Overview

`awsbnkctl` is a single-binary CLI written in Go for deploying and operating **F5 BIG-IP Next for Kubernetes (BNK)** on AWS EKS. It eliminates Terraform and external `kubectl` dependencies by using the AWS Go SDK v2 and client-go directly.

### Core Tenets
1. **Single-Binary Delivery** — Zero required host binaries besides optional `aws` CLI for SSO and EICE jumphost tunnels.
2. **Deterministic Phased State Machine** — Exactly 39 phase files / `stage()` calls (`internal/cli/lifecycle.go`) executed sequentially with full idempotency and resume-safety. Two phases are conditional: `sagemaker-lmi` (only with `ai.sagemaker.enabled`) and `demo-stage` (only in demo mode).
3. **AWS Tags as Source of Truth** — Cloud resources are tagged with `awsbnkctl:cluster`, `awsbnkctl:component`, and `awsbnkctl:managed` (`internal/aws/tags`), enabling reliable reconstruction of local workspace state.
4. **End-to-End Traffic Scenarios** — 15 built-in automated test scenarios validate data-plane VIPs and routing policies from inside the VPC.

---

## 2. Pinned Ecosystem Versions

- **BNK**: `2.4.0` (default); the 2.3.0–2.3.3 builds via `bnk.manifestVersion` in `cluster.yaml` — the release table in `internal/manifest/manifest.go` (`KnownReleases`) pairs each with its FLO chart
- **CNE Release Manifest**: `2.4.0` (`internal/manifest/manifest.go` `DefaultManifestVersion`; F5 docs write it `2.4.0-3.3175.0+0.0.380`, the chart says `2.4.0`); e.g. `2.3.3-3.2598.3-0.0.509` as an operator override
- **Kubernetes (EKS)**: floor `1.34` (`intent.MinKubernetesVersion`), default `1.35` (`intent.DefaultKubernetesVersion`); `1.35` is the newest tested minor, `1.36+` warns
- **cert-manager**: `v1.21.1` (`intent.EmbeddedCertManagerVersion`), embedded upstream YAML in `internal/k8s/manifests/cert-manager/` applied via client-go (not Helm)
- **FLO Chart**: `v2.30.0-0.5.2` (`intent.DefaultFLOVersion`, paired per release in `manifest.KnownReleases`)
- **Go**: `1.26` (`go.mod`); **AWS SDK for Go v2**: `github.com/aws/aws-sdk-go-v2 v1.42.0`

---

## 3. CLI Command Taxonomy

27 top-level commands (`awsbnkctl --help` is authoritative):

| Category | Commands |
|---|---|
| **Lifecycle** | `init`, `validate <path>`, `up -f`, `down -f`, `status`, `doctor`, `topology -f`, `version` |
| **Validation Tests** | `test [suite]` with `test {connectivity, dns, throughput, traffic, list}` and `test hosts {add, clear, list, remove}` |
| **Data Plane Scenarios** | `scenarios {list, run, clean}` — 15 registered scenarios (the core-dump one is `core-file-collection`) |
| **Walkthrough & Demos** | `demo {list, run, clean, preview}` — demos: `diameter`, `http2`, `bigip-cis`, `ingress-migration` |
| **Kubernetes Passthrough** | `k {apply, delete, describe, exec, get, logs, port-forward}`; top-level aliases `get <resource>` and `logs <component>` (flo, cis, cert-manager, cneinstance) |
| **BNK Runtime & Manifests** | `bnk resync`, `manifest probe` |
| **AI & Benchmarking** | `benchmark {setup, run, daemon, list, status}` (bare `benchmark` = `benchmark run`) |
| **Fleet & Forge** | `forge {register, status, unregister, cleanup, benchmark}`; `forge register` flags: `--cluster-name`, `--kubeconfig`, `--project-name`, `--scan` |
| **Agentic Workflow** | `agent` (list CLIs), `agent init`, `agent <cli>` where cli is exactly one of `claude`, `gemini`, `aider`, `openai`, `pi`, `opencode`; `journal {add, list, report}` (no `--format` flag) |
| **Workspaces & Targets** | `workspaces {current, delete, list, new, use}`, `targets {add, list, remove, show}` |
| **Maintenance** | `install`, `self update`, `completion <shell>`, `help` |

There is no `mcp` command: awsbnkctl is only an MCP *client* to BNK Forge (`internal/forge`).

---

## 4. Verification and Pre-Push Quality Gates

Before committing any code or opening a PR, ensure all four gates pass cleanly:

```bash
gofmt -l internal cmd             # Must be empty (0 formatted diffs)
go vet ./internal/... ./cmd/...   # Must pass
go tool staticcheck ./internal/... ./cmd/... # Must pass with 0 errors
gosec ./...                       # Must pass with 0 findings
go test -race ./internal/... ./cmd/... # Must pass
```

### Dry-Run Plan Regression Guard
```bash
AWSBNKCTL_SKIP_AUTH=1 go run ./cmd/awsbnkctl up -f examples/full-cluster/cluster.yaml --dry-run
AWSBNKCTL_SKIP_AUTH=1 go run ./cmd/awsbnkctl down -f examples/full-cluster/cluster.yaml --dry-run
```

---

## 5. Security & Secret Discipline

- **Never commit secrets**: F5 FAR pull secrets, JWT tokens, AWS access keys, or private SSH keys.
- **Permissions**: Keep directory permissions `0o750` and file permissions `0o600`.
- **Gosec Justifications**: Any `#nosec` annotation must specify the rule ID and reason.
