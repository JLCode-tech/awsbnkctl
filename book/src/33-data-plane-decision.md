# The data-plane decision: SR-IOV on EKS

This is the chapter where awsbnkctl's load-bearing technical choice is explained: how BIG-IP Next for Kubernetes (BNK) gets a high-performance data plane on Amazon EKS when EKS doesn't expose SR-IOV out of the box.

If you only ever consume `awsbnkctl up`, you can skip this chapter — the tool makes the right calls for you. If you're sizing a cluster, planning an AMI upgrade, or trying to understand why awsbnkctl ships a *self-managed* node group when AWS offers a *managed* one, read on. The design rationale here is the same rationale you would have walked yourself through to arrive at the same answer; consolidating it once means you don't have to.

For the depth-level reference — every input variable, every spike step, every fail-mode mitigation — see [PRD 07 — EKS cluster + self-managed SR-IOV node group](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md). The chapter you're reading carries the narrative; the PRD carries the contract.

## BNK's SR-IOV requirement

BNK's data plane is the F5 Traffic Management Microkernel (TMM), packaged to run as Kubernetes pods. TMM does what TMM has always done: terminate connections, apply L4/L7 policy, push packets at line rate. The "line rate" part is the requirement that drives every decision in this chapter.

To hit line rate inside a Kubernetes pod, TMM needs a network interface that bypasses the kernel software stack and lets the userspace data plane talk directly to hardware queues. The standard way to deliver that on Linux is **SR-IOV** — Single-Root I/O Virtualisation — which lets a single physical NIC expose multiple *virtual functions* (VFs), each of which the kernel surfaces as an independent network device that a pod can claim as its own.

BNK 2.x is validated against clusters where worker nodes expose VFs as schedulable Kubernetes resources. The reconciler that brings a `CNEInstance` (BNK's umbrella resource) to a `Ready` state will assign a VF to TMM on whichever node TMM lands; without VFs in the allocatable resource list, that reconciliation fails and the data plane never comes up. So "what does a worker node look like in awsbnkctl's cluster?" decomposes to "how does a worker node expose SR-IOV VFs that the Kubernetes scheduler can hand out?"

On bare-metal deployments of BNK — the `dpubnkctl` lineage that this fork's siblings target — the answer is straightforward: stock Mellanox NICs in SR-IOV mode, with the standard `sriov-network-device-plugin` advertising the VFs. On IBM Cloud, the sibling fork `roksbnkctl` has it equally easy: ROKS exposes a SR-IOV-capable worker pool that the cluster API can provision on request. **AWS EKS exposes neither.** That is the gap this chapter walks through.

## The AWS networking primitives

Three AWS concepts matter here, and getting the relationships right is half the battle.

**Elastic Network Adapter (ENA)** is AWS's SR-IOV implementation. On any modern EC2 instance type, the primary NIC is an ENA device, and the underlying hypervisor (Nitro) exposes SR-IOV virtual functions to the guest. ENA is "real SR-IOV" in the kernel sense — same `iommu`, same VFIO machinery, same `/sys/class/net/<eth>/device/sriov_*` shape — but it is not Mellanox SR-IOV. The driver, the firmware, the queue semantics, the offload set are all AWS-specific. The open question in this chapter is *whether BNK accepts an ENA VF as a substitute for the Mellanox VF it was validated against*, and the answer is the load-bearing one for awsbnkctl's first release.

**Elastic Fabric Adapter (EFA)** is ENA's HPC-targeted sibling. It surfaces a libfabric kernel-bypass interface on top of the same SR-IOV plumbing, suited to MPI workloads where every microsecond of latency matters. EFA is only available on a narrow set of instance families (`c5n.18xlarge`, `c5n.metal`, `p4d.*`, `hpc6a.*`) and brings constraints — same-AZ-only collective ops, narrower driver support — that awsbnkctl does not need. The cluster awsbnkctl builds uses **ENA, not EFA**; the EFA story is reserved for a hypothetical v1.x "high-throughput tier" once the ENA path is in production.

**Elastic Network Interfaces (ENIs)** are the L2 attachment point: each ENI is a virtual NIC bound to a subnet, with one or more private IPs, optional public IPs, security groups, and so on. An EC2 instance gets one primary ENI on launch and can attach additional ENIs up to its instance-type limit. ENIs are what the AWS VPC CNI hands out to pods — one secondary ENI per pod when running in "Pod ENI" mode. ENIs are *not* the same thing as SR-IOV VFs: a VF is a hardware queue exposed by the NIC, an ENI is a logical subnet attachment with its own IP address. A given ENA NIC carries multiple ENIs *and* multiple SR-IOV VFs simultaneously — they're orthogonal axes.

The cluster awsbnkctl builds uses **ENIs for pod networking** (via AWS VPC CNI) and **ENA SR-IOV VFs for the BNK data plane** (via Multus + SR-IOV CNI). The two stacks share the same physical NIC but address different layers of the abstraction. That layering is what the next section unpacks.

## The option matrix

Once the requirement and the primitives are settled, the design surface is "how do we get SR-IOV VFs into the scheduler's allocatable resource list?" There are five plausible answers; we worked through all five before settling on one. The full verdict table lives in [PRD 07 § Options considered](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md#options-considered); the short version follows.

**EKS managed node groups + Multus + SR-IOV CNI.** This is the obvious first attempt: use AWS's recommended node-group surface, layer Multus on top, advertise VFs. It fails on the seam between AWS's AMI lifecycle and the SR-IOV stack's kernel-driver expectations. Managed node groups don't expose the boot-time kernel parameters the SR-IOV device plugin needs (`intel_iommu=on iommu=pt`), don't let you pin to a specific AMI version, and don't surface the launch-template knobs (`enable_ena_support`, instance-type-specific tuning) without effectively building a custom AMI — at which point most of the "managed" value evaporates. We lose more than we keep.

**Fargate.** Fargate is a managed-*pod* offering, not a managed-*node* one. There is no node to attach an SR-IOV VF to; pods run on AWS-managed compute that the cluster operator never sees. Fargate is the right answer for many EKS workloads; it's not the right answer for any workload that needs SR-IOV.

**EC2 + kubeadm.** Spin up plain EC2 instances and bring up a vanilla Kubernetes cluster on top with `kubeadm`. This gives total control of the data plane, at the cost of losing every reason for picking EKS in the first place: no managed control plane, no IRSA, no EKS Console surface, no upgrade story. The complexity of running a production control plane outweighs the SR-IOV freedom. `roksbnkctl` made the same call for IBM Cloud (managed ROKS, not self-managed OpenShift) and the reasoning carries over.

**EKS Auto Mode.** Auto Mode is AWS's Karpenter-driven node provisioner — you describe pod requirements, AWS allocates instances. As of writing, Karpenter has no clean integration with SR-IOV device plugins (the VF discovery + scheduling happens after node boot, but Karpenter assumes a more conventional scheduling lifecycle). Revisit in v1.x.

**EKS self-managed node groups + Multus + SR-IOV CNI.** **The selected option.** We own the launch template, the AMI, and the user-data script that applies kernel parameters at first boot. AWS still manages the EKS control plane, IRSA still works, the upgrade story is still EKS's. What we give up — and pay for — is the AMI lifecycle, which becomes our problem rather than AWS's.

The selected option is the only one that lets us pin the kernel-level SR-IOV plumbing the way the SR-IOV stack expects without throwing away the EKS surface entirely. It is more work than managed node groups; it is much less work than EC2 + kubeadm.

## The selected design

In one sentence: awsbnkctl provisions an EKS cluster with a self-managed node group on ENA-enabled instance types (`c5n.4xlarge` by default), enables ENA support in the launch template, applies IOMMU kernel parameters at boot via user-data, and layers Multus + SR-IOV CNI + SR-IOV device plugin DaemonSets on top of the AWS VPC CNI.

Each piece pulls its weight:

- The **launch template** carries `enable_ena_support = true`, a pinned EKS-optimised AL2023 AMI for the cluster's Kubernetes version, and a user-data script that enables `intel_iommu=on iommu=pt` on the kernel command line. The kernel parameters are what let the in-kernel VFIO machinery surface VFs to userspace; without them, the host sees the physical function only.
- The **AWS VPC CNI** stays the primary CNI. It owns pod-to-pod networking via ENIs in the conventional way; pods that don't need SR-IOV (most pods, including everything in the control-plane add-on path) never see the SR-IOV stack.
- **Multus** chains in as the meta-CNI. When a pod's spec carries the `k8s.v1.cni.cncf.io/networks` annotation, Multus calls into the next CNI in the chain after VPC CNI completes — adding a second network interface to the pod's namespace. Without the annotation, Multus is a no-op.
- The **SR-IOV CNI** is what Multus calls into for VF-carrying pods. It allocates a free VF, moves it into the pod's network namespace, and configures the in-pod interface.
- The **SR-IOV device plugin** is the discovery side. It runs as a DaemonSet on every node, reads `/sys/class/net/*/device/sriov_*` to enumerate VFs, and advertises them to the kubelet as a schedulable resource (`intel.com/sriov` by default, configurable via the `sriov_resource_name` module variable). The scheduler then refuses to land VF-requesting pods on nodes without free VFs, which is exactly the property BNK needs.

The wiring is conventional: this is the upstream `k8snetworkplumbingwg` stack, deployed unmodified. What's specific to awsbnkctl is the *node shape underneath* — ENA SR-IOV on a self-managed AL2023 node group — and the device-plugin config that names the ENA vendor/device IDs as the discoverable VF pool.

## Trade-offs accepted

Choosing self-managed node groups means owning some things AWS would otherwise own. Three of those things matter enough to call out.

**The AMI lifecycle is the operator's problem.** AWS publishes EKS-optimised AMIs and rolls them with Kubernetes minor versions; on managed node groups, AWS bumps your nodes when the new AMI lands. On self-managed nodes, the operator (and awsbnkctl by extension) pins an AMI version in the launch template and rolls it explicitly. awsbnkctl bumps the AMI per release; a manual rollover path is documented for users who want to update faster than awsbnkctl's release cadence. The trade-off is real but well-understood — every Kubernetes-on-EC2 deployment from the last decade made this same call.

**Karpenter / elastic scaling is deferred.** Karpenter is AWS's most-loved node autoscaler; SR-IOV device plugins are not on its happy path as of v1.0. awsbnkctl v1.0 ships fixed-size node groups (configurable min/max/desired, but no in-cluster autoscaling). v1.x revisits — possibly with EKS Auto Mode in the loop — once the SR-IOV semantics are validated against production traffic.

**One instance family per cluster, effectively.** The SR-IOV device plugin advertises VFs that match a PCIe vendor/device ID pair declared in its ConfigMap. Mixing `c5n` and `m5n` in the same node group is *possible* — you write a config that matches both vendor/device pairs — but it doubles the surface area for failure and gives the scheduler no useful information about which family a given VF lives on. awsbnkctl v1.0's docs recommend single-family clusters; mixed-family is a v1.x exploration.

These three trade-offs are not surprising: every "we want SR-IOV on a managed Kubernetes service" project hits them. The point is to be explicit about them up front so users with strong elastic-scaling or mixed-family needs know to wait for v1.x.

## The validation gap, and the spike

Everything above is a *design hypothesis*. The hypothesis says: "self-managed node group with ENA-enabled instances + Multus + SR-IOV CNI + SR-IOV device plugin gets BNK a data plane that its CNEInstance reconciler will accept." That hypothesis has not been validated against live AWS at the time this chapter was written.

The validation lives in PRD 07's **spike protocol** — a three-day operator-run procedure that provisions a cluster, layers the SR-IOV stack, and schedules a pod with a VF allocated to it. The third day's check is the load-bearing one: from inside the pod, `ip link show` surfaces a second interface that BNK would accept as the data-plane attachment. If that check fails, the project re-scopes (the PRD's "Spike fail modes" subsection enumerates the mitigations, in escalating order from "tune the device-plugin config" to "fall back to multi-ENI without SR-IOV, accept the perf hit").

The spike is operator-run because it requires real AWS resources (cost is small — a handful of dollars for a few hours of `c5n.4xlarge` time — but the credentials and accounts are real). The Sprint 1 commit that lands the Terraform module and the `awsbnkctl up cluster` verb pre-stages everything the spike needs; the spike then runs and folds its findings into the PRD's "Resolved-in-spike" section. The `v0.2` release tag is gated on the spike completing successfully; this chapter is published at the same time as the v0.2 tag, with the spike findings already in the PRD.

If you're reading this chapter in a v0.2-or-later release, the PRD's "Resolved-in-spike" section is the place to look for *what actually worked*: which instance types, which device-plugin config, which kernel-parameter posture, which BNK release was tested against. The narrative in this chapter is the design rationale; the PRD section is the production validation.

## Cross-references

- [Chapter 2 — Why EKS + self-managed SR-IOV node groups](./02-why-roks.md) — the motivation chapter that frames why managed Kubernetes on AWS is the substrate at all. Read first if you arrived here without that context.
- [Chapter 8 — The cluster phase (cluster up/down)](./08-cluster-phase.md) — how `awsbnkctl up cluster` drives the EKS module that this chapter describes.
- [Chapter 26 — Troubleshooting](./26-troubleshooting.md) — symptoms and mitigations when the SR-IOV stack misbehaves on a running cluster.
- [PRD 07 — EKS cluster + self-managed SR-IOV node group](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md) — the design document this chapter narrates. See in particular its [Resolved-in-spike](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/07-EKS-CLUSTER-SRIOV.md#resolved-in-spike-placeholder) section for the operator-validated findings.
- [Multus CNI upstream](https://github.com/k8snetworkplumbingwg/multus-cni) and [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) — the manifests the module deploys, unmodified from upstream.
- [AWS EKS networking docs](https://docs.aws.amazon.com/eks/latest/userguide/pod-networking.html) — AWS's canonical reference for VPC CNI on EKS, useful when reasoning about the Multus chaining behaviour.
