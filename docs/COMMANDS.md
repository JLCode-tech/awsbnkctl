# Command reference

Every command accepts `--help`. Global flags on every command: `-w/--workspace`,
`-o/--output text|json`, `-v/--verbose`, `-q/--quiet`, `--no-color`,
`--backend local|docker|k8s|ssh:<target>`, `--on <target>`, `--bootstrap`,
`--insecure-host-key`.

## Lifecycle

| Command | Description |
|---|---|
| `init` | Interactive AWS setup; collects region, VPC, subnets, FAR archive and JWT, writes the workspace config (`--dry-run` skips the S3 upload) |
| `validate <path>` | Parse and validate a `cluster.yaml` (no AWS API calls) |
| `up -f <config>` | Provision the EKS cluster and BNK stack (flags: `--dry-run`, `--auto`, `--demo`, `--no-kubeconfig`, `--register-with-forge`, `--skip-activation-poll`) |
| `down -f <config>` | Destroy everything `up` provisioned (flags: `--dry-run`, `--yes`, `--auto`, `--keep-irsa`, `--keep-forge-link`) |
| `status` | Summary of the workspace: cluster, components, deploy state (`-f` locates the phased path's `state.env`) |
| `doctor` | Check prerequisites and report missing pieces (`--backend k8s|ssh:<target>`, `--target <name>` add per-backend probes) |
| `topology -f <config>` | Render the data-path topology as `--format ascii` or `mermaid` |
| `version` | Print version, commit and build date |

`--demo` on `up` marks the cluster as a demo deployment: it writes `DEMO_MODE`
to `state.env`, tags every resource `awsbnkctl:demo=true`, pre-stages the demo
clients on the jumphost, and requires `testing.jumphost.enabled: true`.

## Validation and demos

| Command | Description |
|---|---|
| `test [suite]` | Run deployment validation tests (default: all; `--dry-run` prints the probe plan, `--insecure` skips TLS validation) |
| `test connectivity` | HTTP/HTTPS reachability against configured hosts |
| `test dns` | DNS resolution probe (single-vantage, GSLB-compare, or workspace-driven) |
| `test throughput` | iperf3 throughput; deploys the server pod automatically |
| `test traffic` | Drive HTTP traffic through TMM from the test jumphost (alias for `scenarios run http-routing-e2e`) |
| `test list` | List available test suites |
| `test hosts {add,clear,list,remove}` | Manage `test.connectivity.extra_hosts` in the workspace config |
| `scenarios list` | Print registered scenarios with their rating |
| `scenarios run <name>` | Run a scenario (or `--all`); `--vip` overrides the base VIP, `--synthetic` runs `ai-inference-e2e` without a GPU |
| `scenarios clean <name>` | Invoke a scenario's Cleanup hook |
| `demo list` | Print registered demo use-cases and Green scenarios |
| `demo run <name>` | Run a demo use-case (or `--all`); requires a demo cluster |
| `demo clean <name>` | Invoke a demo use-case's Cleanup hook (or `--all`) |
| `demo preview` | Play the up/down animation locally (no AWS) |

## Kubernetes and BNK runtime

| Command | Description |
|---|---|
| `k apply` | Server-side apply YAML/JSON manifests, directories, or kustomize bases |
| `k delete` | Delete resources by name or label selector |
| `k describe` | Detailed resource info (events, conditions, related objects) |
| `k exec` | Exec into a pod via SPDY |
| `k get` | Get one or more resources |
| `k logs` | Stream pod logs |
| `k port-forward` | Forward local ports to a pod via SPDY |
| `get <resource> [name]` | Top-level alias of `k get` (`-n`, `-A`, `-l`, `-o yaml|json|wide|name|jsonpath=…`) |
| `logs <component>` | Tail logs for a BNK component (`flo`, `cis`, `cert-manager`, `cneinstance`); `-f`, `--since`, `--tail`, `--previous`, `-c` |
| `bnk resync` | Force the F5 cne-controller to re-resolve stale TMM pool members |
| `manifest probe [version]` | Pull a BNK release manifest from `repo.f5.com` with the helm SDK (no host helm) and print its charts and images (`--all`, `--far <path>`) |

## AI benchmarking and BNK Forge

| Command | Description |
|---|---|
| `benchmark` | Runs the default `benchmark run` workflow |
| `benchmark setup` | Prepare the jumphost (aiperf) and register the benchmark agent and target in Forge |
| `benchmark run` | Drive an aiperf run, preset (`--scenarios`), native Forge scenario sweep (`--scenario`), or proxy shootout (`--proxies`) |
| `benchmark list` | List native Forge scenarios and smoke presets |
| `benchmark status` | Check the benchmark environment, jumphost and Forge linkage |
| `benchmark daemon` | Run the persistent Forge benchmark agent daemon |
| `forge register` | Register the workspace's EKS cluster with Forge (idempotent); `--cluster-name`, `--kubeconfig`, `--project-name`, `--scan` |
| `forge status` | Show this workspace's Forge registration state |
| `forge unregister` | Remove this workspace's Forge registration |
| `forge cleanup` | Delete all awsbnkctl benchmark artifacts from Forge for a workspace |
| `forge benchmark` | Alias for `benchmark run` |

`awsbnkctl up --register-with-forge` registers after a successful apply; `down`
unregisters unless `--keep-forge-link` is passed. The binary is an MCP *client*
to Forge; it does not ship an MCP server. See
[`FORGE_INTEGRATION.md`](FORGE_INTEGRATION.md).

## Agentic workflow

| Command | Description |
|---|---|
| `agent` | List supported coding-agent CLIs and this workspace's default |
| `agent init` | Scaffold `AGENTS.md`, `personas/` and `journal/` into the workspace |
| `agent <cli>` | Print the invocation to launch `claude`, `gemini`, `aider`, `openai`, `pi` or `opencode` against the workspace |
| `journal add <note>` | Append a note to today's journal entry |
| `journal list` | List journal entries with one-line summaries |
| `journal report` | Assemble `report.md` from `decisions.md` and the journal timeline |

The binary embeds no LLM; bring your own coding-agent CLI.

## Workspaces, targets and maintenance

| Command | Description |
|---|---|
| `workspaces list` | List workspaces and their states |
| `workspaces current` | Print the current workspace name |
| `workspaces new <name>` | Create an empty workspace skeleton; run `init -w <name>` to populate |
| `workspaces use <name>` | Set the current workspace pointer |
| `workspaces delete <name>` | Delete a workspace (refuses if state is non-empty unless `--force`) |
| `targets {add,list,remove,show}` | Manage the SSH targets used by `--on` / `--backend ssh:<target>` |
| `install` | Copy the running binary into a directory on `PATH` (`--dir`, `--force`) |
| `self update` | Pull the latest release matching the host OS/arch |
| `completion <shell>` | Generate the shell completion script |
| `help [command]` | Help about any command |

## Environment variables

| Variable | Effect |
|---|---|
| `AWSBNKCTL_SKIP_AUTH=1` | Skip AWS credential resolution; only valid together with `--dry-run` on `up` / `down` |
| `AWSBNKCTL_HOME` | Override the workspace/state root directory (legacy alias `ROKSBNKCTL_HOME` is still honoured) |
| `AWSBNKCTL_FORGE_URL` | BNK Forge REST base URL (overrides `forge.url`; default `http://localhost:8000`) |
| `AWSBNKCTL_FORGE_MCP_URL` | BNK Forge MCP endpoint (overrides `forge.mcpUrl`; default `http://localhost:8081/mcp/`) |
| `AWSBNKCTL_FORGE_USERNAME` / `AWSBNKCTL_FORGE_PASSWORD` | Forge credentials; the password is never read from YAML in production use |
| `AWSBNKCTL_FORGE_PROJECT` | Forge project name to register into (overrides `forge.projectName`) |
| `AWSBNKCTL_FORGE_ENVIRONMENT` | Forge project environment, e.g. `dev`, `staging`, `prod` (overrides `forge.environment`) |
| `AWSBNKCTL_BIGIP_PASSWORD` | BIG-IP VE admin password for the `bigipVE` onboarding phase and the `bigip-cis` demo |
| `HF_TOKEN` | Hugging Face token for gated models (`ai.sagemaker`, `ai-inference-e2e`) |
| `AWSBNKCTL_GPU_AZ_DENY` | Extra GPU instance-type AZ deny entries, format `region:az1,az2;region2:az3` |
| `AWSBNKCTL_DOCTOR_SERVICE_QUOTAS=1` | Opt `doctor` into the AWS Service Quotas checks |
| `AWSBNKCTL_SSH_TARGET` / `AWSBNKCTL_K8S_LONG_LIVED` | Internal sentinels set when re-dispatching to an `ssh:<target>` or `k8s` backend; not meant to be set by operators |

## Two `cluster.yaml` blocks worth knowing

- **BGP peering** — `bnk.bgp: true` (alias `bnk.dynamicRouting: true`) admits
  TCP 179 / UDP 3784 from the external data-path subnet into the data-plane
  security group. That is the whole cluster side on BNK 2.4: the `Infra` CR that
  defines the external VLAN has no per-VLAN allowed-services list (the 2.3
  F5SPKVlan did) and the routing container peers from the external self IP by
  default. Every
  example sets it. The Route Server and the ZebOS BGP ConfigMap (2.4.0 runs the
  ZebOS routing container, so the routing CRs do not apply) are yours to add:
  [`BGP-ROUTE-SERVER.md`](BGP-ROUTE-SERVER.md).
- **Shared Forge project** — `forge.projectName` registers the cluster into an
  existing Forge project instead of the auto-created `awsbnkctl-<cluster>` one.
  With a non-default name set, `down` unregisters the cluster but does not purge
  the project.

The full schema is `internal/intent/cluster.go`; the repository layout is in
[`CLAUDE.md`](../CLAUDE.md).
