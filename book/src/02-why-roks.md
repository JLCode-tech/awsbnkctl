# Why EKS + self-managed SR-IOV node groups

This book and the `awsbnkctl` tool target **Amazon EKS** specifically. Other Kubernetes flavours can run BNK, and most of the patterns you'll learn here translate, but the bundled Terraform that `awsbnkctl` ships only knows how to provision an EKS cluster with the specific node-group shape BNK needs.

This chapter explains why. If you're already on EKS and you just want a BNK trial, you can skim it. If you're evaluating whether EKS is the right substrate, read in full. The chapter assumes a working familiarity with AWS — what an account is, what a region is, what IAM is — but does not assume any EKS-specific background.

## BNK is a data-plane workload

The starting point for every decision in this book is what [Chapter 1](./01-what-is-bnk.md) lays out: BNK is F5's TMM data plane in a Kubernetes pod. It moves packets at line rate; it needs network primitives below the conventional pod-networking layer; it expects SR-IOV virtual functions advertised as schedulable resources on the worker nodes it lands on. Everything else in awsbnkctl's design is downstream of that.

"It's a data-plane workload" is the framing that makes the rest of this chapter make sense. A control-plane operator with modest network needs would have very different requirements; for example, the choice of node group shape would mostly not matter, and the project would happily run on Fargate. BNK is not that workload. The cluster has to look the way the data plane needs.

## Why managed Kubernetes over rolling your own

Once you accept "BNK runs in Kubernetes", the next question is *which* Kubernetes — managed or self-managed. The case for managed is straightforward and well-rehearsed across the industry, but worth stating explicitly because it sets the baseline against which the EKS-specific choices below are measured.

Self-managed Kubernetes on EC2 — `kubeadm`, or one of the build-it-yourself distributions — is a multi-week lift before you have a cluster. You provision the underlying instances, bootstrap the control plane, configure etcd backups, stand up DNS and load balancers, integrate with whichever cloud's IAM and storage you're on, build your own upgrade story, chase CVE patches across the control-plane fleet. Every one of those is solvable; very few of them are the *interesting* problem when what you actually want to do is evaluate BNK.

Managed Kubernetes compresses that to one Terraform apply. You hand the provider a cluster spec, you get back a kubeconfig that authenticates against a control plane somebody else patches and backs up, and you proceed directly to deploying workloads. For an evaluation, a sales engineering demo, or a customer proof-of-concept, that compression is the whole point. The sibling fork `roksbnkctl` made the same call for IBM Cloud: managed ROKS over self-managed OpenShift, with the same rationale.

awsbnkctl's analogous call is **managed EKS over kubeadm-on-EC2**. The cost — you give up some control of the control plane to AWS — is precisely the cost we want to pay. The data plane is where awsbnkctl spends its complexity budget; the control plane is somebody else's problem.

## Why EKS specifically (vs. other AWS Kubernetes surfaces)

AWS offers more than one path to Kubernetes. The relevant ones, with the short version of why each was not picked:

- **EKS (managed control plane, customer-managed worker nodes)** — the selected option. The control plane is AWS's responsibility, the nodes are ours, and the boundary between the two is well-documented and stable. The two pieces that matter most for BNK's lifecycle on AWS — OIDC for IRSA, and a launch-template-controlled worker shape — are both first-class on EKS.
- **EKS Fargate (managed control plane, managed pod compute)** — rejected for reasons [Chapter 33](./33-data-plane-decision.md) unpacks. The short version: Fargate doesn't expose a node, and you can't attach SR-IOV VFs to compute you don't see.
- **ECS (Amazon's non-Kubernetes container service)** — not Kubernetes. BNK is shipped as Kubernetes operators and CRDs; the cost of porting it off Kubernetes is the whole project, not awsbnkctl.
- **kubeadm on EC2** — rejected per the previous section. We pay too much in operational surface to gain too little.
- **Karpenter-driven EKS Auto Mode** — deferred to v1.x. Karpenter doesn't currently integrate with the SR-IOV device plugin in the way BNK needs.

Three EKS-specific properties are doing real work in the decision:

**The control plane is managed.** AWS runs the API server, the scheduler, etcd. They roll Kubernetes minor versions on a published cadence; CVE patches happen without operator action. For a project whose value is "BNK on AWS, easily" rather than "we run a hardened Kubernetes control plane", this is exactly the right division of labour.

**OIDC and IRSA are first-class.** Every EKS cluster gets an OIDC issuer URL on creation; that issuer URL is what lets a Kubernetes ServiceAccount assume an IAM role via the standard IRSA pattern. PRD 08 (S3 supply chain + IRSA, landing in Sprint 2) builds on this: the FLO operator authenticates to AWS without any static credentials in the cluster, just by virtue of running under a ServiceAccount that's bound to an IAM role via the cluster's OIDC provider. This is the modern, secret-free way to do cross-cloud-and-cluster identity, and EKS has it built in.

**Self-managed node groups are a supported surface.** This is the most important property for BNK specifically: AWS officially supports clusters where the customer owns the worker-node launch template, the AMI, and the boot-time configuration. The customer gets back the kubeconfig and an IAM trust relationship that lets nodes join; everything below the kubelet-to-API-server seam is the customer's. That's exactly the surface area awsbnkctl needs to layer in the SR-IOV stack [Chapter 33](./33-data-plane-decision.md) describes.

You could imagine an alternate universe where AWS exposed SR-IOV through managed node groups directly. In that universe, awsbnkctl would be a thinner project. We don't live in that universe today, and self-managed node groups are the bridge.

## What this book covers, and what it doesn't

awsbnkctl is opinionated about the cluster shape it builds. It is *not* a general-purpose EKS Terraform module. The cluster it stands up:

- Runs on a fixed-size self-managed node group (configurable min/max, but no Karpenter autoscaling in v1.0).
- Uses ENA-enabled instance types (`c5n.4xlarge` by default; `c5n.9xlarge`, `m5n.4xlarge`, and `m5dn.4xlarge` as alternates), single instance family per cluster.
- Pins the EKS-optimised AL2023 AMI per awsbnkctl release.
- Layers Multus + SR-IOV CNI + SR-IOV device plugin on top of the AWS VPC CNI.

If your workload doesn't need any of that — if you'd be happy with managed node groups on plain `t3.medium`s — then awsbnkctl is the wrong tool for your cluster. F5 publishes Helm charts for BNK that work on any conformant Kubernetes cluster, and "stand up an EKS cluster with `eksctl`, install BNK with Helm" is a perfectly good path. awsbnkctl exists for the case where you specifically want BNK's data-plane characteristics on AWS without doing the SR-IOV plumbing by hand.

The Kubernetes flavours this book does **not** cover:

- **AKS / GKE** — BNK runs on these, but `awsbnkctl up` won't provision them. Forkable from this codebase the same way `awsbnkctl` was forked from `roksbnkctl`; not in scope for v1.0.
- **ROKS / OpenShift on IBM Cloud** — see the upstream [`roksbnkctl`](https://github.com/jgruberf5/roksbnkctl) project, which is what this fork inherits from.
- **Self-managed Kubernetes or OpenShift on bare metal / VMs** — F5's general BNK install path covers these; awsbnkctl does not.

## What you need before continuing

To follow this book end-to-end, you need:

- An **AWS account** with billing enabled. The EKS control plane is metered at roughly $0.10/hour; a small `c5n.4xlarge` node group adds a few more dollars per hour. A short evaluation is inexpensive; a left-running cluster is not.
- **AWS credentials** available via the standard chain (environment variables, an AWS profile, an EC2 instance role, or AWS SSO). [Chapter 14](./14-credentials-resolver.md) walks through what awsbnkctl looks for and in what order.
- **IAM permissions** sufficient to create EKS clusters, EC2 launch templates and instances, VPCs and subnets, IAM roles and OIDC providers, and S3 buckets in the target account. A user with the `AdministratorAccess` policy is overkill but easy; production deployments narrow this to a least-privilege role per environment.

You do **not** need prior experience with:

- **EKS itself** — awsbnkctl drives Terraform on your behalf; you can ignore the EKS API until you want to customise.
- **SR-IOV or low-level networking** — `awsbnkctl up cluster` provisions the SR-IOV stack as part of the cluster bring-up. [Chapter 33](./33-data-plane-decision.md) is the design rationale if you want it; for normal use, it's a chapter you can skip.
- **F5 BIG-IP Next** — BNK is the thing the book deploys; you don't need to be a Big-IP engineer to evaluate it. [Chapter 1](./01-what-is-bnk.md) is the 5-minute primer.

The next chapters walk through installation and the quick-start path. By the end of [Chapter 7](./07-quick-start.md) you'll have a deployed BNK trial on a fresh EKS cluster with SR-IOV node groups, and the rest of the book will make more sense in retrospect.
