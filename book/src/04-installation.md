# Installation

This chapter gets an `awsbnkctl` binary onto your machine and verifies it works. Two install paths are covered: build-from-source (native Go, the canonical path until release artefacts ship) and build-with-Docker (no host Go required).

Pre-built binaries are attached to every [GitHub Release](https://github.com/JLCode-tech/awsbnkctl/releases) (Linux, macOS, Windows × amd64, arm64). The book also ships as an offline PDF (`awsbnkctl-book-<tag>.pdf`) on the same release page. A Homebrew tap is on the v1.x roadmap; until then macOS users grab the binary from the release page or build from source.

## Prerequisites

- **Linux or macOS** for the day-to-day developer experience. Windows compiles cleanly but interactive features (TTY-bound SSH shell, ssh-agent integration) are not first-class on Windows yet.
- **Git** to clone the repository (only if building from source — not needed if you grab a pre-built binary).
- **Go 1.25 or newer** if you want a native build. If you don't have Go (or have an older version), use the Docker-based build or a pre-built release binary.
- **Terraform >= 1.5 on PATH** at runtime — required for `awsbnkctl up` / `plan` / `apply` / `down`.
- **Helm 3 on PATH** at runtime — required during `awsbnkctl up`. The bundled terraform modules (`cert_manager`, `flo`, `cne_instance`) use `null_resource` + `local-exec` provisioners that shell out to `helm upgrade --install`; without `helm` the apply errors out with `exit status 127 — helm: not found`.

The remaining tools (`aws` CLI, `kubectl`, `iperf3`, `docker`) are optional. `awsbnkctl` talks to AWS via the embedded aws-sdk-go-v2 — it does **not** shell out to the `aws` CLI for any production path, so a host `aws` install is only useful for ad-hoc inspection, troubleshooting, or running `aws eks update-kubeconfig` outside the workspace context. `kubectl` is similarly optional: the everyday verbs (`get`, `apply`, `describe`, `delete`, `logs`, `exec`, `port-forward`) are internalised under `awsbnkctl k` via `client-go`.

You do not need Docker installed to *use* `awsbnkctl` with the default `local` backend. Docker is required only if you opt in to `--backend docker` for `terraform` or other tools. The k8s and ssh backends are alternatives that need neither host Docker nor host Go.

## Installing prerequisites

Install paths per platform. `terraform` and `helm` are strictly required for v1.0; the rest are optional, install only what you need.

### macOS — Homebrew

```bash
brew install terraform               # required
brew install helm                    # required — terraform `local-exec` provisioner shells out to `helm`
brew install awscli                  # optional — only for ad-hoc inspection / kubeconfig refresh
brew install kubectl                 # optional — only for `awsbnkctl kubectl …` passthrough (`awsbnkctl k *` is internalised)
brew install iperf3                  # optional — only for `--backend local`/`--backend ssh:<t>` throughput tests
```

### Linux — Ubuntu / Debian

```bash
# terraform — required
wget -qO- https://apt.releases.hashicorp.com/gpg \
  | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] \
https://apt.releases.hashicorp.com $(lsb_release -cs) main" \
  | sudo tee /etc/apt/sources.list.d/hashicorp.list
sudo apt-get update && sudo apt-get install -y terraform

# helm 3 — required (terraform's null_resource + local-exec provisioner for cert_manager / flo / cne_instance shells out to `helm`)
curl https://baltocdn.com/helm/signing.asc \
  | sudo gpg --dearmor -o /usr/share/keyrings/helm.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] \
https://baltocdn.com/helm/stable/debian/ all main" \
  | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update && sudo apt-get install -y helm

# aws CLI — optional, for ad-hoc inspection / kubeconfig refresh
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip && sudo ./aws/install

# kubectl — optional, only for `awsbnkctl kubectl <args>` passthrough (`awsbnkctl k *` is internalised and needs no host install)
sudo snap install kubectl --classic
# or via direct download:
# curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
# chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# iperf3 — optional, only for `--backend local` / `--backend ssh:<t>` throughput tests
sudo apt-get install -y iperf3
```

Instructions above target Ubuntu and Debian. For other Linux distributions (RHEL, Fedora, Arch, openSUSE, Alpine, …), a quick online search for "install terraform on _&lt;your distro&gt;_" — and the same pattern for `aws-cli`, `kubectl`, and `iperf3` — yields the equivalent commands. HashiCorp ships an RPM repo at <https://rpm.releases.hashicorp.com> covering RHEL/Fedora, and most distributions package `kubectl` and `iperf3` in their official repos; AWS ships an x86_64 zip and an arm64 zip at the URLs documented on the [AWS CLI installation page](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).

### Windows — Chocolatey

```powershell
choco install terraform
choco install kubernetes-helm  # required — terraform local-exec provisioner shells out to `helm`
choco install awscli           # optional
choco install kubernetes-cli   # optional, provides kubectl
choco install iperf3           # optional
```

Or via [Scoop](https://scoop.sh/):

```powershell
scoop install terraform helm awscli kubernetes-cli iperf3
```

Windows TTY-bound SSH features (the `awsbnkctl shell --on <target>` interactive path) have known limitations on Windows; file-based SSH keys + non-interactive commands work, but `ssh-agent` named-pipe integration is a v1.x item. See [`docs/PLAN.md`](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/PLAN.md) §"What's deliberately deferred to post-v1.0".

## Path A — native build (requires Go 1.25+)

If `go version` reports `1.25` or newer, this is the simplest path:

```bash
git clone https://github.com/JLCode-tech/awsbnkctl.git
cd awsbnkctl

go mod tidy                          # first time only — populates go.sum
make build                           # → bin/awsbnkctl

# Install via awsbnkctl itself (recommended — copies into ~/.local/bin):
./bin/awsbnkctl install
```

That's the whole thing. The `install` subcommand is idempotent and copies the running binary into a directory on your `PATH`. Default destination is `~/.local/bin/awsbnkctl`.

Make targets you'll use most often:

```
make build      # go build -ldflags ... -o bin/awsbnkctl ./cmd/awsbnkctl
make test       # go test ./...
make vet        # go vet ./...
make tidy       # go mod tidy
make clean      # rm -rf bin/
```

If `make build` fails, the most likely cause is **Go too old**. The module declares `go 1.25.0` in `go.mod` (forced by transitive deps from the SSH/integration test layers); older versions error out with `go: module requires Go 1.25`. Either upgrade Go or fall back to the Docker path below.

## Path B — Docker-based build (no host Go required)

This path is ideal for sealed CI workstations, custom VM images, or anywhere installing Go on the host is awkward. The official `golang:1.25-alpine` image has everything needed; the build artefact lands in `./bin/` owned by your host user.

```bash
git clone https://github.com/JLCode-tech/awsbnkctl.git
cd awsbnkctl

docker run --rm -v "$PWD:/work" -w /work \
  --user "$(id -u):$(id -g)" -e HOME=/tmp \
  golang:1.25-alpine sh -c 'go mod tidy && go build -o bin/awsbnkctl ./cmd/awsbnkctl'

./bin/awsbnkctl install
```

Anatomy of the docker invocation:

| Flag | Why |
|---|---|
| `-v "$PWD:/work"` | Bind-mount the repo into the container at `/work`. |
| `-w /work` | Container working directory matches the mount. |
| `--user "$(id -u):$(id -g)"` | Output binary is owned by your host user, not root. |
| `-e HOME=/tmp` | Go writes its module cache under `$HOME`; `/tmp` is writable by any user. |
| `golang:1.25-alpine` | Pinned major version; matches `go.mod`'s minimum. |

### Cross-compile via Docker

Set `GOOS` / `GOARCH` env vars in the same `docker run` to produce binaries for other platforms:

```bash
# macOS arm64 (Apple Silicon)
docker run --rm -v "$PWD:/work" -w /work \
  --user "$(id -u):$(id -g)" -e HOME=/tmp \
  -e GOOS=darwin -e GOARCH=arm64 \
  golang:1.25-alpine sh -c 'go mod tidy && go build -o bin/awsbnkctl-darwin-arm64 ./cmd/awsbnkctl'

# Windows amd64 (compile-only; not tested at runtime)
docker run --rm -v "$PWD:/work" -w /work \
  --user "$(id -u):$(id -g)" -e HOME=/tmp \
  -e GOOS=windows -e GOARCH=amd64 \
  golang:1.25-alpine sh -c 'go mod tidy && go build -o bin/awsbnkctl.exe ./cmd/awsbnkctl'
```

Each binary is statically linked (Alpine + `CGO_ENABLED=0` is the cross-compile default) so the produced file has no runtime library dependencies.

## The `install` subcommand

```bash
awsbnkctl install [--dir PATH] [--force]
```

`install` copies the running binary into a directory on `PATH`. Defaults:

- **Destination**: `~/.local/bin/awsbnkctl` — this directory is on the default `PATH` for most modern Linux and macOS user environments and does not require sudo.
- **Mode**: `0755`.
- **Idempotent**: if the running binary is already at the destination, no-op (no error).

Override the destination with `--dir`:

```bash
./bin/awsbnkctl install --dir ~/bin
sudo ./bin/awsbnkctl install --dir /usr/local/bin
```

`--force` overwrites an existing file at the destination. Without it, `install` refuses if the destination is a different binary.

If `~/.local/bin` is not on your `PATH`, add it. On bash:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
exec $SHELL -l
```

On zsh, swap `~/.bashrc` for `~/.zshrc`.

## Verifying the install

Two quick checks: version (proves the binary runs) and `doctor` (proves the runtime environment is set up for actual work).

### `awsbnkctl version`

```bash
awsbnkctl version
```

Sample output:

```
awsbnkctl v0.9.0 (commit abc1234, built 2026-05-10T14:22:08Z)
Docs: https://JLCode-tech.github.io/awsbnkctl/book/
```

The version string is populated via `-ldflags` at build time; `make build VERSION=v0.9.0` injects an explicit tag. A bare `make build` produces something like `dev (commit abc1234, built ...)`.

### `awsbnkctl doctor`

```bash
awsbnkctl doctor
```

`doctor` runs the prereq + credentials report. Sample output on a healthy machine looks like this (yours will differ depending on which optional binaries you have installed and whether you've initialised a workspace):

```
✓  terraform         /usr/bin/terraform (Terraform v1.15.2)                                   (required for `awsbnkctl up`)
✓  helm              /usr/local/bin/helm (v3.20.2)                                            (required for `awsbnkctl up`; terraform `local-exec` provisioners shell out to helm)
⚠  iperf3            not on PATH                                                              (needed for `awsbnkctl test throughput --backend local`)
✓  kubectl           /usr/local/bin/kubectl (clientVersion: ...)                              (optional; `awsbnkctl kubectl` passthrough)
✓  aws cli           /usr/local/bin/aws (aws-cli/2.15.0)                                      (optional; awsbnkctl uses the SDK directly)
✓  kubeconfig        /home/you/.kube/config                                                   (needed for cluster-side ops)
✓  workspace         default                                                                  (per-environment config + state)
✓  aws creds         resolved via shared-config (profile=default)                             (auth for terraform + AWS SDK calls)
✓  aws sts           OK (account: 123456789012, arn: arn:aws:iam::123456789012:user/you)      (verifies the cred chain via sts:GetCallerIdentity)
```

Each row is `<status> <name> <detail> <why we care>`. Failures are red `✗` and exit non-zero; warnings are yellow `⚠` and don't fail the run. `terraform` and `helm` are the hard-required checks at v1.0 — the rest are either optional passthroughs or specific to test suites. [Chapter 5](./05-doctor.md) walks through what each check is verifying and how to fix common failures.

## OS support matrix

| OS | Native build | Docker build | Cross-compile target | Runtime status |
|---|---|---|---|---|
| Linux (amd64, arm64) | yes | yes | yes | first-class |
| macOS (amd64, arm64) | yes | yes | yes | first-class |
| Windows (amd64, arm64) | yes | yes | yes | compile-only; `awsbnkctl shell --on` and `awsbnkctl exec --on jumphost` PTY behaviour limited |

"First-class" means the v1.0 acceptance criteria are validated on those platforms; "compile-only" means the binary builds and runs but interactive features (notably TTY-bound SSH) have known limitations and are not part of the v1.0 release gate.

## Required prerequisites — `terraform` and `helm` at v1.0

The v1.0 cluster lifecycle needs two binaries on `PATH`:

- **`terraform` (>= 1.5)** — hard-required for any cluster lifecycle command (`up`, `down`, `plan`, `apply`).
- **`helm` (3.x)** — hard-required during `awsbnkctl up`. The bundled terraform modules (`cert_manager`, `flo`, `cne_instance`) use `null_resource` + `local-exec` provisioners that shell out to `helm upgrade --install`. Without it, the apply fails with `exit status 127 — helm: not found`.

Optional binaries — only needed for the corresponding passthrough or fallback path:

- **`iperf3`** — only needed for `--backend local` and `--backend ssh:<target>` throughput modes. The default `--backend k8s` runs iperf3 entirely in cluster (no host binary needed).
- **`kubectl`** — only needed for the `awsbnkctl kubectl <args...>` passthrough. The everyday verbs (`get`, `apply`, `describe`, `delete`, `logs`, `exec`, `port-forward`) are internalised under `awsbnkctl k` and need no host binary.
- **`aws` CLI** — only useful for ad-hoc inspection (`aws sts get-caller-identity`, `aws eks describe-cluster`). `awsbnkctl` talks to AWS via the embedded aws-sdk-go-v2; no production code path shells out to `aws`.
- **`docker`** — only needed for `--backend docker`. Optional; the `k8s` and `ssh` backends are alternatives if docker isn't available.

Run `awsbnkctl doctor` to see exactly what your environment is missing for the workflow you intend to run.

## Updating

`git pull && make build` is the source-build update mechanism (or re-run the Docker build for the containerised path).

`awsbnkctl self update` upgrades from a tagged GitHub release. Use it once you've installed an initial release binary:

```bash
awsbnkctl self update
# Checks https://github.com/JLCode-tech/awsbnkctl/releases/latest, downloads
# the matching asset for your OS+arch, verifies the checksum, swaps the
# binary atomically.
```

## Migrating from roksbnkctl (upstream fork)

The fork relationship is documented in [Chapter 32 — Extending awsbnkctl](./32-extending-roksbnkctl.md). For most users coming from `roksbnkctl`, the migration is straightforward: install `awsbnkctl` (this chapter), run `awsbnkctl init -w <name>` to create a new AWS-shaped workspace, and `awsbnkctl up` against an AWS account. The `~/.roksbnkctl/` directory is independent — `awsbnkctl` reads from `~/.awsbnkctl/`, so the two tools coexist on the same host without clobbering each other's state. There is no automatic state-migration command; the upstream tool's workspaces describe IBM Cloud / ROKS resources that don't translate to AWS / EKS.

## Next

With a working binary on PATH, [Chapter 5 — Doctor](./05-doctor.md) explains what every doctor check is looking at, [Chapter 6 — Workspaces](./06-workspaces.md) explains the `~/.awsbnkctl/<workspace>/` layout, and [Chapter 7 — Quick start](./07-quick-start.md) walks the 4-command lifecycle end-to-end.
