# Full Cluster BNK-on-EKS Topology

This directory contains the complete `cluster.yaml` reference intent file for a **full BNK-on-EKS deployment** utilizing the standard `host-device` pattern.

## Architecture

This topology provisions the entire infrastructure stack from the ground up:
- **VPC & Subnets:** Creates a full VPC with public subnets (routing to an IGW) and private subnets (routing to a NAT Gateway).
- **Data-Path Subnets:** Provisions dedicated `external` (ingress) and `internal` (backend) VLANs for the TMM data plane.
- **EKS Control Plane & Nodes:** Provisions an EKS cluster with a 3-node managed node group (`m6i.4xlarge`), appropriately sized for the BNK control plane and dSSM quorum.
- **F5 BNK Integration:** Wires up the secondary ENIs, node labels, IRSA roles, and the `host-device` dual-interface data path.
- **Test Jumphost:** (Optional) Deploys a multi-ENI jumphost instance within the VPC to generate test traffic directly into the BNK external data path.

## Usage

This configuration is intended to be copied and customized for your specific environments.

1. Copy the reference config:
   ```bash
   cp cluster.yaml my-cluster.yaml
   # Update bnk.farArchive and bnk.jwt to point to your real credentials.
   ```

2. Dry-Run / Validate:
   ```bash
   awsbnkctl validate my-cluster.yaml
   AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up --config my-cluster.yaml --dry-run
   ```

3. Provision the full cluster:
   ```bash
   awsbnkctl up --config my-cluster.yaml
   ```

4. Teardown:
   ```bash
   awsbnkctl down --config my-cluster.yaml --yes
   ```

## Cost & teardown

Billable while up: 3x `m6i.4xlarge` workers, the EKS control plane, one NAT
gateway, and (if `testing.jumphost.enabled`) a `t3.small` jumphost — roughly
**$3/hour** at `ap-southeast-2` on-demand rates, excluding data transfer and EBS.
Nothing in this topology scales to zero, so an idle cluster costs the same as a
busy one.

Destroy everything when you are done:

```bash
awsbnkctl down --config my-cluster.yaml --yes
```

`down` works in reverse phase order and is safe to re-run. It discovers
resources by the `awsbnkctl:cluster=<name>` tag, so it still cleans up if the
local state directory is lost.
