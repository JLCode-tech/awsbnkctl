# SR-IOV External (DPDK) BNK Topology

This directory contains the `cluster.yaml` intent file for provisioning an **experimental SR-IOV/vfio DPDK** single-interface BNK-on-EKS deployment.

## Architecture

This configuration uses the `sriov-external` pattern. It differs from the standard kernel socket-based deployments:
- **DPDK over ENA:** TMM drives the data plane via DPDK over the Elastic Network Adapter (ENA) bound to `vfio-pci` (No-IOMMU), instead of relying on the standard kernel socket driver.
- **Node Preparation:** The external ENA is bound to `vfio-pci` automatically by the `vfio-node-prep` DaemonSet running on stock AL2023 nodes.
- **Resource Exposure:** The SR-IOV network device plugin exposes the interface as the `intel.com/ens8` resource. The Network Attachment Definition (NAD) used is `external-sriov` (of type `passthru`).
- **Single Interface:** Like the `external-only` pattern, this topology uses a single external (ingress) ENI, reaching backend pods via the standard CNI.

## Usage

To provision this experimental DPDK topology:

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
on-demand rates, excluding data transfer and EBS. DPDK/vfio changes how TMM
drives the NIC, not what the cluster costs, so this matches
`examples/external-only`.

Destroy everything when you are done:

```bash
awsbnkctl down --config my-cluster.yaml --yes
```

> [!NOTE]
> This pattern is **experimental**, and the `vfio-node-prep` DaemonSet rebinds the
> node's external ENA to `vfio-pci`. Prefer a dedicated cluster for it over
> converting an existing one.
