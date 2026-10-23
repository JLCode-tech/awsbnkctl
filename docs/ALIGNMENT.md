# *bnkctl Family Alignment & Feature Parity

`awsbnkctl` adheres strictly to the unified architectural conventions and UX patterns established across the `*bnkctl` tool family:

## Tool Family Map

| Tool | Cloud / Platform | Kubernetes Engine | Datapath Technology | Auth Mechanism |
| :--- | :--- | :--- | :--- | :--- |
| **`awsbnkctl`** | AWS | Amazon EKS | AWS Multi-NIC / Secondary ENI (`host-device` / SR-IOV DPDK) | AWS IAM / STS |
| **`gkebnkctl`** | Google Cloud | Google GKE | GKE Datapath V2 Multi-NIC (`host-device` / PCI passthrough) | Google Application Default Credentials (ADC) |
| **`azrbnkctl`** | Microsoft Azure | Azure Kubernetes Service (AKS) | Azure Accelerated Networking (SR-IOV / DPDK) | Azure `DefaultAzureCredential` |
| **`roksbnkctl`** | IBM Cloud | Red Hat OpenShift (ROKS) | SRIOV Network Operator | IBM Cloud IAM API Key |
| **`ocibnkctl`** | Oracle Cloud | OCI Container Engine / K3s | Multus CNI / SR-IOV | OCI Instance / API Key Auth |

---

## CLI Command Parity Matrix

All top-level commands across the `*bnkctl` family:

| Subcommand | Purpose | `awsbnkctl` | `gkebnkctl` | `azrbnkctl` | `roksbnkctl` | `ocibnkctl` |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: |
| `init` | Generate starter `cluster.yaml` intent file | ✓ | ✓ | ✓ | ✓ | ✓ |
| `validate` | Validate `cluster.yaml` schema, CIDRs, and version gates | ✓ | ✓ | ✓ | ✓ | ✓ |
| `up` | Provision cloud + K8s + BNK 2.4 stack | ✓ | ✓ | ✓ | ✓ | ✓ |
| `down` | Destroy cluster and managed network resources | ✓ | ✓ | ✓ | ✓ | ✓ |
| `status` | Inspect cloud / K8s / BNK / Forge readiness | ✓ | ✓ | ✓ | ✓ | ✓ |
| `logs` | Stream TMM, controller, CWC, FLO, or governance logs | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get` | Shorthand alias for `k get` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `k` | Embedded `client-go` Kubernetes CLI (`get`, `apply`, `describe`, `delete`, `logs`, `exec`, `port-forward`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `doctor` | Preflight + live cluster readiness and heal diagnostics | ✓ | ✓ | ✓ | ✓ | ✓ |
| `test` | Data-plane verification (`connectivity`, `dns`, `throughput` / `egress`, `traffic`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `scenarios` | End-to-end BNK 2.4 traffic & F5 BNK AI Service Plane scenarios (`list`, `run`, `clean`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `demo` | Interactive protocol & architecture walkthroughs (`list`, `run`, `clean`, `preview`) | ✓ | — | — | — | — |
| `benchmark` | Run `aiperf` benchmarks from the in-VPC/VNet jumphost against BNK VIPs | ✓ | ✓ | ✓ | — | — |
| `bnk` | Day-2 BNK operations (`heal`, `resync`, `upgrade`, `migrate-2.4`, `mcp-session`) | ✓ | ✓ | ✓ | — | — |
| `forge` | [BNK Forge](https://github.com/JLCode-tech/bnk-forge) integration (`register`, `status`, `unregister`, `cleanup`, `scan`, `telemetry`, `benchmark`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `topology` | Discover and print live BNK data-plane / node / ENI / VF topology | ✓ | ✓ | ✓ | ✓ | ✓ |
| `manifest` | Inspect and probe F5 FAR release manifests (`probe`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `journal` | Append-only operational journal (`add`, `list`, `report`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `agent` | Generate AI coding assistant instructions (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `workspaces` | Manage multiple local cluster workspaces (`list`, `current`, `new`, `use`, `delete`) | ✓ | — | — | — | — |
| `targets` | Manage remote SSH / Kubernetes execution targets (`scan`, `add`, `list`, `remove`, `show`) | ✓ | — | — | — | — |
| `install` | Install binary into `$PATH` (`/usr/local/bin` or `~/.local/bin`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `self` | Self-inspection and in-place GitHub Release binary upgrade (`update`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `version` | Print version, commit, and build date | ✓ | ✓ | ✓ | ✓ | ✓ |
| `completion` | Generate shell completion scripts (`bash`, `zsh`, `fish`, `powershell`) | ✓ | ✓ | ✓ | ✓ | ✓ |
| `help` | Built-in command help | ✓ | ✓ | ✓ | ✓ | ✓ |

---

## Technical Invariants
1. **Kubernetes Version Floor**: Every `*bnkctl` tool mandates Kubernetes version >= `1.32.0` (`1.34`–`1.35` on EKS).
2. **In-Process Dynamic SSA**: All Kubernetes operations run via `client-go` Server-Side Apply with FieldManager `<cli>`.
3. **Pure Cloud SDKs**: No shell out to cloud CLIs (`aws`, `gcloud`, `az`, `ibmcloud`, `oci`) for cluster infrastructure lifecycle.
4. **Offline & Dry-Run Simulation**: Dry-run verification (`--dry-run` + auth bypass) guarantees testability in zero-credential CI environments.
