# AGENTS.md — awsbnkctl Developer & Operator Guide for AI Agents

> This file is the root knowledge base for AI coding agents and automated tools working on or operating `awsbnkctl`.

---

## 1. System Overview

`awsbnkctl` is a single-binary CLI written in Go for deploying and operating **F5 BIG-IP Next for Kubernetes (BNK)** on AWS EKS. It eliminates Terraform and external `kubectl` dependencies by using the AWS Go SDK v2 and client-go directly.

### Core Tenets
1. **Single-Binary Delivery** — Zero required host binaries besides optional `aws` CLI for SSO and EICE jumphost tunnels.
2. **Deterministic Phased State Machine** — Exactly 39 numbered phases executed sequentially with full idempotency and resume-safety.
3. **AWS Tags as Source of Truth** — Cloud resources are tagged with `awsbnkctl.f5.com/*` tags, enabling reliable reconstruction of local workspace state.
4. **End-to-End Traffic Scenarios** — 15 built-in automated test scenarios validate data-plane VIPs and routing policies from inside the VPC.

---

## 2. Pinned Ecosystem Versions

- **BNK**: `2.3.2`
- **CNE Release Manifest**: `2.3.2-3.2598.3-0.0.392`
- **Kubernetes (EKS)**: `1.32` to `1.35` (default `1.32`)
- **cert-manager**: `v1.16.2`
- **FLO Chart**: `v2.21+`

---

## 3. CLI Command Taxonomy

| Category | Commands |
|---|---|
| **Lifecycle** | `init`, `validate`, `up`, `down`, `status`, `doctor` |
| **Data Plane Scenarios** | `scenarios {list, run, clean}` |
| **AI & Benchmarking** | `benchmark {setup, run, list, status}` |
| **Walkthrough & Demos** | `demo {list, run, clean}`, `topology` |
| **Kubernetes Passthrough** | `k {get, apply, describe, delete, logs, exec, port-forward}` |
| **Agentic Workflow** | `agent {init, claude, gemini, chatgpt, aider}`, `journal {add, list, report}` |
| **Registry & Manifests** | `manifest probe [version]` |
| **Fleet & Forge** | `forge {register, status, unregister, benchmark}` |
| **Maintenance** | `self update`, `version` |

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
