# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) and AI coding agents working with the `awsbnkctl` repository.

## What this binary is

`awsbnkctl` is a single-binary Go CLI that drives a full F5 BIG-IP Next for Kubernetes (BNK) 2.3.2 deployment onto an AWS EKS cluster with secondary Elastic Network Interfaces (ENIs) dedicated to the Traffic Management Microkernel (TMM).

It executes a deterministic 39-phase provisioning lifecycle implemented directly in Go using the AWS SDK (v2) and client-go — **no Terraform, no host `kubectl`, no host `helm`**.

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
go test -race ./internal/... ./cmd/...
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
├── aws/                 # AWS SDK v2 client, IAM, VPC, EKS, EC2, ENIs
│   └── phases/          # Exactly 39 ordered provisioning and teardown phases
├── bnk/                 # BNK manifests, FAR client, CNE & FLO templates, licensing
├── cli/                 # Cobra CLI commands (up, down, validate, doctor, scenarios, agent, journal)
├── config/              # cluster.yaml schema, validation, default settings
├── k8s/                 # Embedded client-go wrapper for K8s API (k subcommand)
├── jumphost/            # EICE (EC2 Instance Connect Endpoint) tunnel & SSH curling
├── scenarios/           # 15 automated validation scenarios (HTTP, L4, gRPC, AI, CWC)
├── topology/            # ASCII data-plane visualizer
└── version/             # Build version, commit, date, and pinned BNK version metadata
```

## Agentic Mode & Personas

`awsbnkctl` includes built-in agent scaffolding and operational logging:
- `awsbnkctl agent init` — Scaffolds `AGENTS.md`, `personas/`, and `journal/`.
- `awsbnkctl agent <cli>` — Outputs optimized command invocations for Claude, Gemini, Aider, OpenAI, etc.
- `awsbnkctl journal {add, list, report}` — Maintains an append-only markdown log of operational decisions and execution events.
- `awsbnkctl mcp serve` — Serves an embedded Model Context Protocol (MCP) server for IDEs and desktop agents.

## Coding Standards & Rules
1. **File and directory permissions**: Keep directory permissions `0o750` and file write permissions `0o600`.
2. **Security**: Ensure `#nosec` annotations are applied with justifications where appropriate (e.g., `#nosec G304`, `#nosec G204`).
3. **No hardcoded secrets**: Never commit AWS credentials, FAR pull tokens, or JWT license strings.
