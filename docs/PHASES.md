# awsbnkctl Provisioning Phases

`awsbnkctl up` executes 39 deterministic phases in order via the AWS SDK and Kubernetes API. These phases are divided into four logical stages. Each phase is idempotent and records its outcome in the local `state.env`.

`awsbnkctl down` walks the same four stages in reverse, but its step list (`runPhasedDown` in `internal/cli/lifecycle.go`) is not a strict mirror of `up`:

- Before any infrastructure step, every registered demo use-case's `Cleanup` hook runs while the kubeconfig is still valid.
- Stage 4 starts with `otel-certs` (`Phase15OTELCertsDown`) and `lb-controller` (`Phase14bLBControllerDown`) — the LB controller must go before `flo-helm` and before `irsa-oidc` so its IAM role is gone before its OIDC provider — then continues in reverse: `activation-poll` (25), `pod-manager-heal` (24c), `dssm-overlay` (24b), `cwc-heal` (24), `spk-vlan-gateway-class` (23b), `license` (23), `cne-instance` (22), `irsa-sa` (21), `sriov-dataplane` (20b), `nads` (20), `cloud-network-mapping` (19), `flo-helm` (14), `k8s-foundation` (12).
- Stage 3 runs `ebs-csi-hugepages` (11b), `nvidia-device-plugin` (11c) and `sagemaker-lmi` while the API server is still reachable, then `kubeconfig` (11), `irsa-oidc` (18), `demo-stage` (17d), `iface-discovery` (17c), `bigip-ve` (17e), `jumphost` (17b), `secondary-enis` (17), `tmm-node-label` (16), `node-group` (10).
- Stage 2 adds a down-only step, `forge-benchmark-cleanup` (`Phase09bBenchmarkDown`), before `forge-register` (09), `vpc-cni-prefix` (08b) and `eks-cluster` (08).
- Stage 1 runs `iam` (07), `route-tables` (06), `nat` (05), `igw` (04), `subnets` (03), `vpc` (02).

`preflight` (00), `postflight` (13) and `bigip-onboard` (17f) have no down step — destroying the BIG-IP VE in `bigip-ve` removes everything onboarding created. `--keep-forge-link` skips the forge unregister and `--keep-irsa` preserves the OIDC provider.

## STAGE 1 — VPC · subnets · IGW · NAT · IAM

- **`preflight`** (`Phase00Preflight`): Validates AWS credentials, EULA acceptance, region, and existing state before mutations begin.
- **`vpc`** (`Phase02VPC`): Creates the AWS VPC with DNS hostnames and resolution enabled.
- **`subnets`** (`Phase03Subnets`): Creates the public and private subnets across the configured AZs, plus the TMM data-path subnets — `subnet-bnk-ext` for every BNK pattern and `subnet-bnk-int` only for `dual-interface`.
- **`igw`** (`Phase04IGW`): Attaches an Internet Gateway to the VPC.
- **`nat`** (`Phase05NAT`): Allocates Elastic IPs and creates NAT Gateways for private subnet egress.
- **`route-tables`** (`Phase06RouteTables`): Configures routing tables for public (IGW) and private (NAT) subnets, and associates the BNK data-path subnets (`subnet-bnk-ext`, `subnet-bnk-int`) with the private table so TMM SelfIPs can egress through NAT and Route Server propagations apply to them.
- **`iam`** (`Phase07IAM`): Creates the EKS cluster IAM role, the node group IAM role, and the BNK data-plane security group `SG_BNK_DATA`. With `bnk.bgp: true` it also admits TCP 179 (BGP) and UDP 3784 (BFD) from the external data-path subnet, so a Route Server endpoint there can peer with TMM.

## STAGE 2 — EKS control plane

- **`eks-cluster`** (`Phase08EKSCluster`): Provisions the EKS control plane and waits for it to become ACTIVE.
- **`forge-register`** (`Phase09ForgeRegister`): Registers the newly created EKS cluster with the Forge platform over MCP.
- **`vpc-cni-prefix`** (`Phase08bVPCCNIPrefix`): Configures VPC-CNI prefix delegation BEFORE the node group boots to prevent secondary ENI asymmetric-drop bugs.

## STAGE 3 — Nodes · kubeconfig · ENIs · jumphost

- **`node-group`** (`Phase10NodeGroup`): Provisions the EKS managed node group and waits for nodes to join the cluster. On `down`, after the node group is gone it also deletes the EBS volumes the CSI driver provisioned for the cluster's PVCs (tag `kubernetes.io/cluster/<name>`), which nothing else reclaims once the namespaces and addon are deleted.
- **`kubeconfig`** (`Phase11Kubeconfig`): Generates and saves the admin kubeconfig via the AWS SDK.
- **`tmm-node-label`** (`Phase16TMMNodeLabel`): Labels the specific worker node targeted for TMM scheduling.
- **`nvidia-device-plugin`** (`Phase11cNvidiaDevicePlugin`): Deploys the NVIDIA device plugin (GPU node groups only).
- **`sagemaker-lmi`** (`PhaseSageMakerUp`): Provisions a SageMaker LMI endpoint (opt-in).
- **`secondary-enis`** (`Phase17SecondaryENIs`): Creates secondary ENIs for the data plane (internal/external) attached directly to the worker node and records the nominal TMM self IPs (`<subnet>.240`); on BNK 2.4 the address is allocated by the F5 IPAM controller, so Phase 23b assigns it on the ENI.
- **`jumphost`** (`Phase17bJumphost`): Provisions a secure EC2 jumphost for internal testing and API access.
- **`bigip-ve`** (`Phase17eBigIPVE`): Provisions an optional BIG-IP Virtual Edition instance for proxy tests (opt-in).
- **`iface-discovery`** (`Phase17cIfaceDiscovery`): Runs a host-netns probe on the TMM node and records the Linux names + PCI addresses of the data-path ENIs (`EXTERNAL_IFNAME`/`INTERNAL_IFNAME`) and of the node's primary ENI (`NODE_PRIMARY_IFNAME`, consumed by `egress-snat` as the pseudo-CNI `nodeInterfaceName`). The data-path pair is matched once (TMM later owns those NICs); the primary is re-resolved on re-run if missing from state.
- **`demo-stage`** (`Phase17dDemoStage`): Pre-stages demo client assets (grpcurl, python scripts) on the jumphost.
- **`irsa-oidc`** (`Phase18IRSAOIDC`): Configures the OIDC provider for IAM Roles for Service Accounts (IRSA).

## STAGE 4 — BNK supply chain · activation

- **`ebs-csi-hugepages`** (`Phase11bEBSCSIHugepages`): Deploys the EBS CSI managed addon, gp3 StorageClass, and configures node hugepages and proxy ARP.
- **`k8s-foundation`** (`Phase12K8sFoundation`): Deploys foundational cluster components including cert-manager and the Multus CNI.
- **`flo-helm`** (`Phase14FLOHelm`): Deploys the F5 Lifecycle Operator (FLO) via Helm.
- **`lb-controller`** (`Phase14bLBController`): Installs the AWS Load Balancer Controller (opt-in).
- **`otel-certs`** (`Phase15OTELCerts`): Deploys OpenTelemetry certificates for observability.
- **`cloud-network-mapping`** (`Phase19CloudNetworkMapping`): Creates the CloudNetworkMapping ConfigMap required by FLO.
- **`nads`** (`Phase20NADs`): Creates NetworkAttachmentDefinitions for host-device integration in the cluster.
- **`sriov-dataplane`** (`Phase20bSriovDataplane`): Configures vfio node-prep and SR-IOV device plugins (if enabled).
- **`cne-instance`** (`Phase22CNEInstance`): Applies the CNEInstance custom resource to trigger the BNK installation.
- **`irsa-sa`** (`Phase21IRSASA`): Reads the ServiceAccount the FLO-created `f5-cne-controller` Deployment runs as, scopes the IRSA role trust policy to it, annotates it with `eks.amazonaws.com/role-arn`, and rollout-restarts the controller if its pods lack the injected credentials. Runs after `cne-instance` because the SA name is release-specific and only known once FLO has created it.
- **`license`** (`Phase23License`): Waits for the License CRD and applies the BNK license to activate the instance.
- **`spk-vlan-gateway-class`** (`Phase23bSPKVlanGatewayClass`): Adds a ClusterRole/Binding so the cne-controller can `get` EndpointSlices (FLO 2.30 grants only list/watch; the 2.4 chart grants get), applies the GatewayClass, waits for it to be Accepted, then applies the BNK 2.4 `Infra` CR (external/internal VLANs with /27 self-IP pools around the nominal `.240`, the VIP listener pool, the egress tunnel defaults and two static routes: the VPC via the tunnel VLAN's gateway, `0.0.0.0/0` via the external gateway), waits for `Programmed=True`, then puts the self IP the F5 IPAM controller allocated on the ENI as a secondary IP (AWS only delivers traffic for addresses the ENI owns) and records it as `TMM_EXT_SELFIP` / `TMM_INT_SELFIP`. The pools are /27s like F5's 2.4 example because the F5 IPAM controller splits every pool into per-device blocks and rejects single addresses and /30s. The controller only reconciles Infra once a GatewayClass exists. Waits for the cne-controller to be available first, because its pod serves the F5 validating webhook. The 2.3 `F5SPKVlan` / `F5SPKStaticRoute` CRs are not applied on 2.4 (the Infra CR consolidates them and the controller ignores them in GatewaySettings mode); `down` deletes everything, including F5SPKVlan CRs left by older builds.
- **`cwc-heal`** (`Phase24CWCHeal`): Applies best-effort DNS warmup healing for the CWC pod.
- **`dssm-overlay`** (`Phase24bDSSMInsecureOverlay`): Overlays `--insecure` flags to fix strict TLS probe failures in DSSM.
- **`pod-manager-heal`** (`Phase24cPodManagerHeal`): Restarts f5-tmm-pod-manager to break cold-start kube-proxy loops.
- **`activation-poll`** (`Phase25ActivationPoll`): Polls CNEInstance and License status for up to 20 minutes until ACTIVE.
- **`bigip-onboard`** (`Phase17fBigIPOnboard`): Drives tmsh and declarative onboarding through the jumphost to configure the BIG-IP VE instance.
- **`postflight`** (`Phase13Postflight`): Final verification of FLO, OTEL, and deployment activation state.
