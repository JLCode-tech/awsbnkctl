# awsbnkctl Provisioning Phases

`awsbnkctl up` executes 41 deterministic phases in order via the AWS SDK and Kubernetes API: 39 always run, `sagemaker-lmi` and `demo-stage` only when enabled. The phases are divided into four logical stages. Each phase is idempotent and records its outcome in the local `state.env`.

`up` flags: `--dry-run` prints the plan without AWS calls, `--auto` skips confirmations, `--no-kubeconfig` leaves the admin kubeconfig unwritten, `--register-with-forge` registers the cluster with Forge after a successful apply, `--skip-activation-poll` makes phase 25 return at once, and `--demo` stages the demo assets (`DEMO_MODE` in state; needs `testing.jumphost.enabled`).

`awsbnkctl down` walks the same four stages in reverse, but its step list (`runPhasedDown` in `internal/cli/lifecycle.go`) is not a strict mirror of `up`:

- On a demo cluster (`DEMO_MODE=true` in state), every registered demo use-case's `Cleanup` hook runs first, while the kubeconfig is still valid.
- Stage 4 starts with `otel-certs` (`Phase15OTELCertsDown`) and `lb-controller` (`Phase14bLBControllerDown`) — the LB controller must go before `flo-helm` and before `irsa-oidc` so its IAM role is gone before its OIDC provider — then continues in reverse: `activation-poll` (25), `pod-manager-heal` (24c), `dssm-overlay` (24b), `cwc-heal` (24), `spk-vlan-gateway-class` (23b), `license` (23), `cne-instance` (22), `irsa-sa` (21), `sriov-dataplane` (20b), `nads` (20), `cloud-network-mapping` (19), `flo-helm` (14), `k8s-foundation` (12).
- Stage 3 runs `ebs-csi-hugepages` (11b), `nvidia-device-plugin` (11c) and `sagemaker-lmi` while the API server is still reachable, then `kubeconfig` (11), `irsa-oidc` (18), `demo-stage` (17d), `iface-discovery` (17c), `bigip-ve` (17e), `jumphost` (17b), `secondary-enis` (17), `tmm-node-label` (16), `node-group` (10).
- Stage 2 adds a down-only step, `forge-benchmark-cleanup` (`Phase09bBenchmarkDown`), before `forge-register` (09), `metrics-server` (08c, clears state only), `vpc-cni-prefix` (08b) and `eks-cluster` (08).
- Stage 1 runs `iam` (07), `route-tables` (06), `nat` (05), `igw` (04), `subnets` (03), `vpc` (02).

`preflight` (00), `postflight` (13), `tmm-log-stream` (24d) and `bigip-onboard` (17f) have no down step — destroying the BIG-IP VE in `bigip-ve` removes everything onboarding created. `--keep-forge-link` skips the forge unregister and `--keep-irsa` keeps the OIDC provider and the IRSA role.

## STAGE 1 — VPC · subnets · IGW · NAT · IAM

- **`preflight`** (`Phase00Preflight`): Validates credentials (SSO sentinel, `sts:GetCallerIdentity`) and, for BNK patterns, the node instance-type floors (ENI count, ≥16 vCPU, ≥64 GiB, `desiredSize` ≥3); creates `.awsbnkctl/<cluster>/` and records the cluster name and region.
- **`vpc`** (`Phase02VPC`): Creates the AWS VPC with DNS hostnames and resolution enabled.
- **`subnets`** (`Phase03Subnets`): Creates the public and private subnets across the configured AZs, plus the TMM data-path subnets — `subnet-bnk-ext` for every BNK pattern and `subnet-bnk-int` only for `dual-interface`.
- **`igw`** (`Phase04IGW`): Attaches an Internet Gateway to the VPC.
- **`nat`** (`Phase05NAT`): Allocates Elastic IPs and creates NAT Gateways for private subnet egress.
- **`route-tables`** (`Phase06RouteTables`): Configures routing tables for public (IGW) and private (NAT) subnets, and associates the BNK data-path subnets (`subnet-bnk-ext`, `subnet-bnk-int`) with the private table so TMM SelfIPs can egress through NAT and Route Server propagations apply to them.
- **`iam`** (`Phase07IAM`): Creates the EKS cluster IAM role, the node group IAM role, and the BNK data-plane security group `SG_BNK_DATA`. With `bnk.bgp: true` it also admits TCP 179 (BGP) and UDP 3784 (BFD) from the external data-path subnet, so a Route Server endpoint there can peer with TMM.

## STAGE 2 — EKS control plane

- **`eks-cluster`** (`Phase08EKSCluster`): Provisions the EKS control plane and waits for it to become ACTIVE; records the cluster's service range as `EKS_SERVICE_CIDR`, which the CNEInstance passes to TMM as `TMM_K8S_ROUTES`.
- **`forge-register`** (`Phase09ForgeRegister`): Registers the cluster with Forge over MCP (REST fallback, three attempts). Skipped unless `forge.enabled`; a failure writes a `pending` `forge_link.json` and never fails the run. `up --register-with-forge` runs the same registration after the apply instead.
- **`vpc-cni-prefix`** (`Phase08bVPCCNIPrefix`): Configures VPC-CNI prefix delegation BEFORE the node group boots to prevent secondary ENI asymmetric-drop bugs.
- **`metrics-server`** (`Phase08cMetricsServer`): Creates the metrics-server EKS managed add-on so `metrics.k8s.io` serves pod and node CPU/memory to `kubectl top` and the Forge fleet view; `bnk heal -f` adds it to existing clusters.

Every cluster-side repair the phases apply (Multus token watch, metrics-server, TMM log stream, pod-manager, cwc, dSSM probe overlay, controller EndpointSlice RBAC, controller IRSA, `TMM_K8S_ROUTES`, the `awsbnkctl-test` probe namespace) is one of the ten entries in the heal registry (`internal/aws/phases/heal.go`): `awsbnkctl bnk heal` runs the fixes on an existing cluster and `awsbnkctl doctor --backend k8s` prints the detections.

## STAGE 3 — Nodes · kubeconfig · ENIs · jumphost

- **`node-group`** (`Phase10NodeGroup`): Provisions the EKS managed node group and waits for nodes to join the cluster. On `down`, after the node group is gone it also deletes the EBS volumes the CSI driver provisioned for the cluster's PVCs (tag `kubernetes.io/cluster/<name>`), which nothing else reclaims once the namespaces and addon are deleted.
- **`kubeconfig`** (`Phase11Kubeconfig`): Generates and saves the admin kubeconfig via the AWS SDK.
- **`tmm-node-label`** (`Phase16TMMNodeLabel`): Labels the specific worker node targeted for TMM scheduling.
- **`nvidia-device-plugin`** (`Phase11cNvidiaDevicePlugin`): Deploys the NVIDIA device plugin (GPU node groups only).
- **`sagemaker-lmi`** (`PhaseSageMakerUp`): Provisions a SageMaker LMI endpoint (opt-in).
- **`secondary-enis`** (`Phase17SecondaryENIs`): Creates secondary ENIs for the data plane (internal/external) attached directly to the worker node and records the nominal TMM self IPs (`<subnet>.240`); on BNK 2.4 the address is allocated by the F5 IPAM controller, so Phase 23b assigns it on the ENI.
- **`jumphost`** (`Phase17bJumphost`): Opt-in via `testing.jumphost.enabled`. Provisions an EC2 jumphost, reached through an EC2 Instance Connect Endpoint, for tests and API access. Waits up to 10 minutes for the EC2 Instance Connect Endpoint, fails fast with the AWS state message when it reaches `create-failed`, and logs describe errors instead of hiding them behind the timeout.
- **`bigip-ve`** (`Phase17eBigIPVE`): Provisions an optional BIG-IP Virtual Edition instance for proxy tests (opt-in).
- **`iface-discovery`** (`Phase17cIfaceDiscovery`): Runs a host-netns probe on the TMM node and records the Linux names + PCI addresses of the data-path ENIs (`EXTERNAL_IFNAME`/`INTERNAL_IFNAME`) and of the node's primary ENI (`NODE_PRIMARY_IFNAME`, consumed by `egress-snat` as the pseudo-CNI `nodeInterfaceName`). The data-path pair is matched once (TMM later owns those NICs); the primary is re-resolved on re-run if missing from state.
- **`demo-stage`** (`Phase17dDemoStage`): Demo clusters only (`demo.enabled` or `up --demo`). Pre-stages demo client assets (grpcurl, python scripts) on the jumphost.
- **`irsa-oidc`** (`Phase18IRSAOIDC`): Configures the OIDC provider for IAM Roles for Service Accounts (IRSA).

## STAGE 4 — BNK supply chain · activation

- **`ebs-csi-hugepages`** (`Phase11bEBSCSIHugepages`): Installs the aws-ebs-csi-driver EKS add-on, records the StorageClass BNK requests (`bnk.storageClassName`, default `gp2`), applies the hugepages DaemonSet (2Mi hugepages plus proxy ARP on the CNI veths) and waits for the nodes to advertise the capacity.
- **`k8s-foundation`** (`Phase12K8sFoundation`): Creates the BNK namespaces and the FAR pull secrets, applies cert-manager v1.21.1, installs Multus v4.2.4 with the kubeconfig token watch, and creates the `awsbnkctl-test` namespace the `test --backend k8s` probe Jobs run in.
- **`flo-helm`** (`Phase14FLOHelm`): Deploys the F5 Lifecycle Operator (FLO) via Helm.
- **`lb-controller`** (`Phase14bLBController`): Installs the AWS Load Balancer Controller (opt-in). On `down`, if `LB_CONTROLLER_POLICY_ARN` is missing from state (a previous `down` failed mid-phase, e.g. on an expired SSO token), the deterministic policy `<cluster>-lb-controller-iam-policy` is looked up by name (account from an ARN in state or `sts:GetCallerIdentity`) and deleted; a failed `DeletePolicy` keeps the key in state so the next `down` retries.
- **`otel-certs`** (`Phase15OTELCerts`): Deploys OpenTelemetry certificates for observability.
- **`cloud-network-mapping`** (`Phase19CloudNetworkMapping`): Creates the CloudNetworkMapping ConfigMap required by FLO.
- **`nads`** (`Phase20NADs`): Creates NetworkAttachmentDefinitions for host-device integration in the cluster.
- **`sriov-dataplane`** (`Phase20bSriovDataplane`): Configures vfio node-prep and SR-IOV device plugins (if enabled).
- **`cne-instance`** (`Phase22CNEInstance`): Applies the CNEInstance custom resource to trigger the BNK installation. The TMM env carries `TMM_K8S_ROUTES=<EKS_SERVICE_CIDR>` (the f5-tmm chart's `add_k8s_routes`): the Kubernetes service range TMM keeps reachable over eth0 once the `Infra` default route is programmed; without it DNS, dSSM (iRule `table`, persistence) and log forwarding fail inside the TMM pod.
- **`irsa-sa`** (`Phase21IRSASA`): Reads the ServiceAccount the FLO-created `f5-cne-controller` Deployment runs as, scopes the IRSA role trust policy to it, annotates it with `eks.amazonaws.com/role-arn`, and rollout-restarts the controller if its pods lack the injected credentials. Runs after `cne-instance` because the SA name is release-specific and only known once FLO has created it.
- **`license`** (`Phase23License`): Waits for the License CRD and applies the BNK license to activate the instance.
- **`spk-vlan-gateway-class`** (`Phase23bSPKVlanGatewayClass`): Adds a ClusterRole/Binding so the cne-controller can `get` EndpointSlices (FLO 2.30 grants only list/watch; the 2.4 chart grants get), applies the GatewayClass, waits for it to be Accepted, then applies the BNK 2.4 `Infra` CR (external/internal VLANs with /27 self-IP pools around the nominal `.240`, the VIP listener pool, the egress tunnel defaults and two static routes: the VPC via the tunnel VLAN's gateway, `0.0.0.0/0` via the external gateway), waits for `Programmed=True`, then puts the self IP the F5 IPAM controller allocated on the ENI as a secondary IP (AWS only delivers traffic for addresses the ENI owns) and records it as `TMM_EXT_SELFIP` / `TMM_INT_SELFIP`. The pools are /27s like F5's 2.4 example because the F5 IPAM controller splits every pool into per-device blocks and rejects single addresses and /30s. The controller only reconciles Infra once a GatewayClass exists. Waits for the cne-controller to be available first, because its pod serves the F5 validating webhook. The 2.3 `F5SPKVlan` / `F5SPKStaticRoute` CRs are not applied on 2.4 (the Infra CR consolidates them and the controller ignores them in GatewaySettings mode); `down` deletes everything, including F5SPKVlan CRs left by older builds. With `bnk.bgp: true` nothing extra is rendered: the Infra CRD has no per-VLAN allowed-services (the 2.3 F5SPKVlan carried tcp:179/udp:3784) and the routing container peers from the external self IP by default, so the phase just logs that and phase 07's security-group rule is the only cluster-side change. The BGP stanza itself goes in the ZebOS ConfigMap `f5-tmm-dynamic-routing-template` (FLO 2.30 installs 2.4.0 with the ZebOS routing container; the GlobalRoutingConfig/RoutingTemplate CRs are the OcNOS path and fail with "No heartbeat from grpc-svc-f5dr"), see `docs/BGP-ROUTE-SERVER.md`.
- **`cwc-heal`** (`Phase24CWCHeal`): Applies best-effort DNS warmup healing for the CWC pod.
- **`dssm-overlay`** (`Phase24bDSSMInsecureOverlay`): Overlays `--insecure` flags to fix strict TLS probe failures in DSSM.
- **`tmm-log-stream`** (`Phase24dTMMLogStream`): Turns on the `@type stdout` store in F5's `f5-toda-fluentd-custom` ConfigMap and bounces the fluentd pod once, so the TMM lines the f5-fluentbit sidecar only forwards (governance records included) reach `awsbnkctl logs tmm` and the collectors. Idempotent.
- **`pod-manager-heal`** (`Phase24cPodManagerHeal`): Restarts f5-tmm-pod-manager to break cold-start kube-proxy loops.
- **`activation-poll`** (`Phase25ActivationPoll`): Polls CNEInstance and License status for up to 9 minutes (30 s, then 18 × 30 s) until ACTIVE, then requires the shared readiness verdict (Infra, Gateways, controller, TMM).
- **`bigip-onboard`** (`Phase17fBigIPOnboard`): Drives tmsh and declarative onboarding through the jumphost to configure the BIG-IP VE instance.
- **`postflight`** (`Phase13Postflight`): Final verification of FLO, OTEL, and deployment activation state.
