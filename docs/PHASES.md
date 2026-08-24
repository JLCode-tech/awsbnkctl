# awsbnkctl Provisioning Phases

`awsbnkctl up` executes 39 deterministic phases in order via the AWS SDK and Kubernetes API. These phases are divided into four logical stages. Each phase is idempotent and records its outcome in the local `state.env`.

The `awsbnkctl down` command runs these phases in exact reverse order to cleanly destroy the environment.

## STAGE 1 — VPC · subnets · IGW · NAT · IAM

- **`preflight`** (`Phase00Preflight`): Validates AWS credentials, EULA acceptance, region, and existing state before mutations begin.
- **`vpc`** (`Phase02VPC`): Creates the AWS VPC with DNS hostnames and resolution enabled.
- **`subnets`** (`Phase03Subnets`): Creates the management, public, internal, and external subnets across multiple AZs.
- **`igw`** (`Phase04IGW`): Attaches an Internet Gateway to the VPC.
- **`nat`** (`Phase05NAT`): Allocates Elastic IPs and creates NAT Gateways for private subnet egress.
- **`route-tables`** (`Phase06RouteTables`): Configures routing tables for public (IGW) and private (NAT) subnets.
- **`iam`** (`Phase07IAM`): Creates the EKS cluster IAM role and the node group IAM role.

## STAGE 2 — EKS control plane

- **`eks-cluster`** (`Phase08EKSCluster`): Provisions the EKS control plane and waits for it to become ACTIVE.
- **`forge-register`** (`Phase09ForgeRegister`): Registers the newly created EKS cluster with the Forge platform over MCP.
- **`vpc-cni-prefix`** (`Phase08bVPCCNIPrefix`): Configures VPC-CNI prefix delegation BEFORE the node group boots to prevent secondary ENI asymmetric-drop bugs.

## STAGE 3 — Nodes · kubeconfig · ENIs · jumphost

- **`node-group`** (`Phase10NodeGroup`): Provisions the EKS managed node group and waits for nodes to join the cluster.
- **`kubeconfig`** (`Phase11Kubeconfig`): Generates and saves the admin kubeconfig via the AWS SDK.
- **`tmm-node-label`** (`Phase16TMMNodeLabel`): Labels the specific worker node targeted for TMM scheduling.
- **`nvidia-device-plugin`** (`Phase11cNvidiaDevicePlugin`): Deploys the NVIDIA device plugin (GPU node groups only).
- **`sagemaker-lmi`** (`PhaseSageMakerUp`): Provisions a SageMaker LMI endpoint (opt-in).
- **`secondary-enis`** (`Phase17SecondaryENIs`): Creates secondary ENIs for the data plane (internal/external) attached directly to the worker node.
- **`jumphost`** (`Phase17bJumphost`): Provisions a secure EC2 jumphost for internal testing and API access.
- **`bigip-ve`** (`Phase17eBigIPVE`): Provisions an optional BIG-IP Virtual Edition instance for proxy tests (opt-in).
- **`iface-discovery`** (`Phase17cIfaceDiscovery`): Discovers and records interface details (MACs, device indices) for data-plane networking.
- **`demo-stage`** (`Phase17dDemoStage`): Pre-stages demo client assets (grpcurl, python scripts) on the jumphost.
- **`irsa-oidc`** (`Phase18IRSAOIDC`): Configures the OIDC provider for IAM Roles for Service Accounts (IRSA).

## STAGE 4 — BNK supply chain · activation

- **`ebs-csi-hugepages`** (`Phase11bEBSCSIHugepages`): Deploys the EBS CSI managed addon, gp3 StorageClass, and configures node hugepages.
- **`k8s-foundation`** (`Phase12K8sFoundation`): Deploys foundational cluster components including cert-manager and the Multus CNI.
- **`flo-helm`** (`Phase14FLOHelm`): Deploys the F5 Lifecycle Operator (FLO) via Helm.
- **`lb-controller`** (`Phase14bLBController`): Installs the AWS Load Balancer Controller (opt-in).
- **`otel-certs`** (`Phase15OTELCerts`): Deploys OpenTelemetry certificates for observability.
- **`cloud-network-mapping`** (`Phase19CloudNetworkMapping`): Creates the CloudNetworkMapping ConfigMap required by FLO.
- **`nads`** (`Phase20NADs`): Creates NetworkAttachmentDefinitions for host-device integration in the cluster.
- **`sriov-dataplane`** (`Phase20bSriovDataplane`): Configures vfio node-prep and SR-IOV device plugins (if enabled).
- **`irsa-sa`** (`Phase21IRSASA`): Pre-creates IRSA ServiceAccounts with role annotations for AWS integration.
- **`cne-instance`** (`Phase22CNEInstance`): Applies the CNEInstance custom resource to trigger the BNK installation.
- **`license`** (`Phase23License`): Waits for the License CRD and applies the BNK license to activate the instance.
- **`spk-vlan-gateway-class`** (`Phase23bSPKVlanGatewayClass`): Configures F5SPKVlan and GatewayClass data-plane plumbing.
- **`cwc-heal`** (`Phase24CWCHeal`): Applies best-effort DNS warmup healing for the CWC pod.
- **`dssm-overlay`** (`Phase24bDSSMInsecureOverlay`): Overlays `--insecure` flags to fix strict TLS probe failures in DSSM.
- **`pod-manager-heal`** (`Phase24cPodManagerHeal`): Restarts f5-tmm-pod-manager to break cold-start kube-proxy loops.
- **`activation-poll`** (`Phase25ActivationPoll`): Polls CNEInstance and License status for up to 20 minutes until ACTIVE.
- **`bigip-onboard`** (`Phase17fBigIPOnboard`): Drives tmsh and declarative onboarding through the jumphost to configure the BIG-IP VE instance.
- **`postflight`** (`Phase13Postflight`): Final verification of FLO, OTEL, and deployment activation state.
