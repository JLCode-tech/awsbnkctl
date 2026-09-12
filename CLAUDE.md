# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) and AI coding agents working with the `awsbnkctl` repository.

## What this binary is

`awsbnkctl` is a single-binary Go CLI that drives a full F5 BIG-IP Next for Kubernetes (BNK) 2.4 deployment onto an AWS EKS cluster with secondary Elastic Network Interfaces (ENIs) dedicated to the Traffic Management Microkernel (TMM). The default CNE release manifest is `2.4.0` (BNK 2.4.0; `internal/manifest/manifest.go`); operators pin an older 2.3.x build per cluster via `bnk.manifestVersion` in `cluster.yaml`, and `manifest.KnownReleases` pairs every supported build with the FLO chart F5 shipped with it.

It executes a deterministic 39-phase provisioning lifecycle (two phases, `sagemaker-lmi` and `demo-stage`, are conditional) implemented directly in Go using the AWS SDK for Go v2 and client-go — **no Terraform, no host `kubectl`, no host `helm`**. cert-manager `v1.21.1` is applied from embedded upstream YAML via client-go; the EKS floor is Kubernetes `1.34` (`intent.MinKubernetesVersion`), the default is `1.35` (`intent.DefaultKubernetesVersion`, the minor F5 lists for BNK 2.3.x and 2.4.0), 1.36+ warns.

## Key Commands & Development Workflows

### Build & Run
```bash
make build               # Builds binary to bin/awsbnkctl
./bin/awsbnkctl --help
./bin/awsbnkctl version
```

### Validation & Test Gates (Required Before Every Commit)
```bash
gofmt -l internal cmd    # Must produce 0 output
go vet ./internal/... ./cmd/...
go tool staticcheck ./internal/... ./cmd/...
gosec ./...              # Security analyzer (0 findings)
go test -race ./internal/... ./cmd/...   # -race needs cgo (CGO_ENABLED=1 + a C toolchain); fall back to plain `go test` where cgo is unavailable
```

### Dry-Run Testing Without AWS Credentials
```bash
AWSBNKCTL_SKIP_AUTH=1 ./bin/awsbnkctl up -f examples/full-cluster/cluster.yaml --dry-run
AWSBNKCTL_SKIP_AUTH=1 ./bin/awsbnkctl down -f examples/full-cluster/cluster.yaml --dry-run
```

## Architecture & Codebase Map

```
cmd/awsbnkctl/           # CLI entrypoint (main.go)
internal/
├── aws/                 # aws-sdk-go-v2 wrappers: VPC, EKS, EC2, IAM, S3, STS, Service Quotas; tags/ and state/ subpackages
│   └── phases/          # Exactly 39 ordered provisioning and teardown phases (phaseNN_*.go), orchestrated by internal/cli/lifecycle.go
├── bnkconst/            # BNK-wide constants shared across packages
├── cli/                 # Cobra command tree — every command lives here; Version/Commit/BuildDate vars in root.go are stamped via -ldflags
├── config/              # Workspace paths ($AWSBNKCTL_HOME) and global config — NOT the cluster.yaml schema
├── demo/                # Demo use-case registry + narration (diameter, http2, bigip-cis, ingress-migration)
├── doctor/              # Prerequisite checks behind `awsbnkctl doctor`
├── embedded/            # Agentic-mode scaffolding (AGENTS.md, personas/, journal/) shipped in the binary
├── exec/                # Execution backends: local, docker, k8s, ssh:<target>
├── forge/               # BNK Forge client (MCP preferred, REST fallback) — register/unregister/benchmark
├── intent/              # cluster.yaml schema (v1), strict loader, validation, pinned defaults (K8s floor, FLO, cert-manager)
├── jumphost/            # SSH-via-EICE probe utilities for the test jumphost
├── k8s/                 # Embedded client-go wrapper (k verbs); manifests/ (cert-manager YAML) and render/ (CNEInstance, Infra etc.)
├── manifest/            # F5 release-manifest (BOM) fetch/probe; DefaultManifestVersion lives here
├── remote/              # Embedded SSH client and target plumbing
├── scenarios/           # 15 end-to-end validation scenarios (HTTP, L4, gRPC, AI, CWC, core files)
├── test/                # connectivity / dns / throughput probe runners behind `awsbnkctl test`
├── topology/            # Data-path topology model + ASCII/mermaid renderers
└── ui/                  # Terminal output primitives (spinners, progress bars, colour)
pkg/bnk/                 # Exported BNK runtime helpers (TMM pool-member resync, watch)
```

There is no `internal/bnk` and no `internal/version` package.

## Agentic Mode & Personas

`awsbnkctl` includes built-in agent scaffolding and operational logging:
- `awsbnkctl agent init` — Scaffolds `AGENTS.md`, `personas/`, and `journal/`.
- `awsbnkctl agent <cli>` — Prints the invocation to launch one of `claude`, `gemini`, `aider`, `openai`, `pi`, `opencode` against the workspace (no other names are accepted).
- `awsbnkctl journal {add, list, report}` — Maintains an append-only markdown log of operational decisions and execution events.
- There is **no** `awsbnkctl mcp` command and no embedded MCP server. The binary is only an MCP *client* to BNK Forge (`internal/forge`, used by `forge register`, `up --register-with-forge`, and `benchmark`).

## Coding Standards & Rules
1. **File and directory permissions**: Keep directory permissions `0o750` and file write permissions `0o600`.
2. **Security**: Ensure `#nosec` annotations are applied with justifications where appropriate (e.g., `#nosec G304`, `#nosec G204`).
3. **No hardcoded secrets**: Never commit AWS credentials, FAR pull tokens, or JWT license strings.
