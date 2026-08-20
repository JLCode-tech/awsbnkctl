# External-Only (Single-Interface) BNK Topology

This directory contains the `cluster.yaml` intent file for provisioning an **external-only** BNK-on-EKS deployment. 

## Architecture

This configuration uses the `external-only` pattern. In this topology:
- **TMM** (Traffic Management Microkernel) is provisioned with exactly **one** data-plane interface (the external/ingress ENI).
- **Backend Routing:** TMM reaches the in-cluster backend pods over the standard CNI (e.g., Calico/VPC-CNI) rather than through a dedicated internal VLAN.
- **Resource Footprint:** Because there is only one secondary ENI required for the data path, the preflight ENI floor is reduced (2 total ENIs per node: primary + external) compared to the standard dual-interface host-device pattern.

## Usage

To provision this topology:

1. Copy the example configuration and replace the credential paths:
   ```bash
   cp cluster.yaml my-cluster.yaml
   # Edit my-cluster.yaml to point farArchive and jwt to your actual credentials
   ```

2. Validate the configuration (dry-run):
   ```bash
   awsbnkctl validate my-cluster.yaml
   AWSBNKCTL_SKIP_AUTH=1 awsbnkctl up --config my-cluster.yaml --dry-run
   ```

3. Provision the environment:
   ```bash
   awsbnkctl up --config my-cluster.yaml
   ```

4. Tear down the environment when finished:
   ```bash
   awsbnkctl down --config my-cluster.yaml --yes
   ```

## Cost & teardown

Billable while up: 3x `m6i.4xlarge` workers, the EKS control plane, one NAT
gateway, and a `t3.small` jumphost — roughly **$3/hour** at `ap-southeast-2`
on-demand rates, excluding data transfer and EBS. Dropping the internal
interface saves an ENI, not money: the node group is still sized for the dSSM
quorum, so this costs the same as `examples/full-cluster`.

Destroy everything when you are done:

```bash
awsbnkctl down --config my-cluster.yaml --yes
```
