# Contributing to awsbnkctl

Thank you for your interest in contributing to **awsbnkctl**! This document provides everything you need to get set up locally, run tests, and successfully ship your changes.

---

## Prerequisites

This project is tested on Linux and macOS hosts.

**Required:**
- **Go 1.26+** (Check `go.mod` for the exact source of truth)
- **git**, **make**, **docker**
- Standard dev utilities: `jq`, `unzip`, `gnupg`, `openssh-client`, `python3`, and `helm 3` (for chart operations)

**Required for live runs that drive traffic:**
- `aws` CLI and `ssh` — `internal/jumphost` shells out to
  `aws ec2-instance-connect open-tunnel` piped into `ssh` to curl the VIP from
  inside the VPC. Every scenario and demo that validates the data plane goes
  through this path. Not needed for unit tests or dry-runs.

**NOT Required:**
- `terraform` (awsbnkctl uses the AWS SDK directly)
- `kubectl` (Internalized via `client-go`)
- `goreleaser` (Run by the release workflow, not locally)
- `staticcheck` (Declared as a `tool` directive in `go.mod`; run it with `go tool staticcheck`)

---

## Building Locally

You can easily build the binary from the root directory:

```bash
go build -o awsbnkctl ./cmd/awsbnkctl
./awsbnkctl --help
```

*Tip: Check the `Makefile` for common workflow targets.*

---

## Testing

### Running Tests
The unit suite runs entirely without external dependencies. Always run these locally before pushing your code — CI enforces all four, and `gofmt` plus `staticcheck` catch things `go build`, `go vet` and `go test` do not.

```bash
gofmt -l .        # Must be empty
go vet ./...      # Must be clean
go tool staticcheck ./...  # Must be clean (tool directive in go.mod; no separate install)
go test ./...     # Must pass
```

> [!TIP]
> **If you have built the AgentCore demo's Node dependencies, `./...` breaks.**
> `examples/agentcore-demo/agent/**/node_modules/` ships Go template files named
> `%name%.template.go`, which the Go toolchain rejects as invalid input file
> names. All four commands above then fail on files that have nothing to do with
> your change. Those directories are gitignored, so CI and a fresh clone are
> unaffected — but locally, scope the gates to the real packages:
>
> ```bash
> gofmt -l internal cmd
> go vet ./internal/... ./cmd/...
> go tool staticcheck ./internal/... ./cmd/...
> go test ./internal/... ./cmd/...
> ```

### Credential-Free Dry-Run
The `AWSBNKCTL_SKIP_AUTH=1` environment variable lets `up --dry-run` and
`down --dry-run` execute the full phase graph without authenticating to AWS.
This is the primary local regression guard for the dry-run path:

```bash
AWSBNKCTL_SKIP_AUTH=1 ./awsbnkctl up -f examples/full-cluster/cluster.yaml --dry-run
AWSBNKCTL_SKIP_AUTH=1 ./awsbnkctl down -f examples/full-cluster/cluster.yaml --dry-run
```

It is intentionally rejected for live runs; `AWSBNKCTL_SKIP_AUTH=1` is only
valid together with `--dry-run`.

### Integration Tiers

The jobs in `.github/workflows/ci.yml` run on every PR and on pushes to `main` / `staging`:

| Tier | CI job | What it exercises |
|---|---|---|
| **Unit** | `test` | `go vet`, `gofmt`, `go tool staticcheck`, `go test ./...`, `go build` — pure Go packages with fakes for external IO |
| **`testcontainers` (sshd)** | `integration` | `internal/exec` SSH backend against a containerised sshd (Linux only) |
| **Docker backend** | `docker-backend` | `internal/exec` docker backend against a local `busybox` container |
| **AWS-SDK mocked** | `aws-mocked` | `internal/aws` helpers against mocked aws-sdk-go-v2 clients — no live AWS |
| **K8s backend (`kind`)** | `k8s-backend` | `internal/exec` k8s backend and `internal/cli` ops lifecycle against an ephemeral `helm/kind-action` cluster; tests self-skip if no kubeconfig is reachable |
| **Dry-run smoke** | `test-dryrun` | Builds the binary and runs `awsbnkctl test {connectivity,dns,throughput} --dry-run` against a materialised fake workspace |
| **Live e2e** | `e2e-full.yml` | Real AWS account + real EKS cluster — on demand only |

*Note: Integration tests are gated by the `integration` build tag so they don't run by default. To run them:*
```bash
go test -tags integration ./...
```

### Live End-to-End (e2e) Tests
The full e2e tier spins up a real EKS cluster and tears it down. **It costs real money** and takes ~25 minutes per cycle. Use it sparingly against a sandbox account.

```bash
export AWS_PROFILE=my-profile
aws sso login --profile $AWS_PROFILE
./scripts/e2e-test-full.sh
```

---

## Code Style & Guidelines

- **Surgical changes:** Touch only what the change requires; match the surrounding style.
- **Clarity over cleverness:** Prefer maintainable code over impressive one-liners.
- **Comment WHY, not WHAT:** Avoid comments explaining what the code does, document *why* when non-obvious.
- **Complete implementations:** If you can't finish a code path in a PR, document the limitation and gate the surface.

For deeper architectural context, read the [Architecture Guide](docs/ARCHITECTURE.md).

---

## Adding a New Phase

If you're extending the provisioning graph:
1. Read the existing phase you're closest to in shape (e.g. `phase17_secondary_enis.go`).
2. Add a new file `phaseNN_<name>.go` and its corresponding test.
3. Wire it into `internal/cli/lifecycle.go:runPhasedUp` and the inverse in `runPhasedDown` at the correct ordering.
4. Make sure the phase is **idempotent** on healthy re-runs.
5. Update `docs/ARCHITECTURE.md` if the phase changes the model.

---

## Adding a Scenario or Demo

- **Scenario:** Create `internal/scenarios/<name>/` implementing the `scenarios.Scenario` interface. Self-register via `init()`.
- **Demo:** Create `internal/demo/<name>/` implementing the same interface. Self-register via `init()`. Each demo owns a dedicated VIP.

Ensure you include a `VerifyDeps` struct with a `TestVerifyCallOrder` test, an idempotent `Cleanup`, and templated embedded manifests via `//go:embed`.

---

## Releasing

Releases are cut by `release-please`, not by pushing tags. `.github/workflows/release.yml`
runs on every push to `main`: `release-please` maintains an open Release PR that
aggregates merged Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `test:`)
and bumps `CHANGELOG.md`; merging that PR creates the `vX.Y.Z` tag and GitHub
release, and `goreleaser` then builds and attaches the binaries. Do not push
tags by hand. See [docs/RELEASE.md](docs/RELEASE.md) for the full process.

---

## Reporting Issues

Open a GitHub issue. For bugs, please include:
- `awsbnkctl --version`
- The minimal `cluster.yaml` (redacted)
- The full stderr output
- Whether the issue reproduces on a fresh `up` or specific state

Thank you for helping us improve **awsbnkctl**!
