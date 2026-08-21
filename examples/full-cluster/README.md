# Full Cluster BNK-on-EKS Topology

This directory contains the complete `cluster.yaml` reference intent file for a **full BNK-on-EKS deployment** utilizing the standard `host-device` pattern.

It is also the **demo cluster**: uncomment the `demo:` block in `cluster.yaml` (or
pass `--demo`) and the same infrastructure gains the curated protocol
walkthroughs. See [Demo mode](#demo-mode) below.

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

## Demo mode

Uncomment the `demo:` block in `cluster.yaml`, or pass `--demo` on the CLI. Demo
mode requires `testing.jumphost.enabled: true`, because every use-case drives
traffic from inside the BNK external subnet.

With it on, `up` writes `DEMO_MODE` / `DEMO_STAGED_AT` / `DEMO_EXPIRY` to
`state.env`, tags every resource `awsbnkctl:demo=true`, pre-stages the test
clients on the jumphost, and `down` cleans the use-cases before the
infrastructure underneath them.

```bash
awsbnkctl demo list
awsbnkctl demo run http2 --config my-cluster.yaml
awsbnkctl demo run --all --config my-cluster.yaml
```

> [!NOTE]
> `demo.ttl` (default `24h`) only records an expiry — `DEMO_EXPIRY` in `state.env`
> plus an `awsbnkctl:demo-expiry` tag, which `awsbnkctl status` shows as a
> countdown. No reaper acts on it. Nothing deletes the cluster when it expires.

### Migration scenarios

Two of the use-cases carry the "migrate to BNK" story:

- **`ingress-migration`** installs `ingress-nginx`, HAProxy, and a BNK Gateway API
  route in front of a shared backend, so you can compare the traffic paths live
  before cutover.
  ```bash
  awsbnkctl demo run ingress-migration --config my-cluster.yaml
  ```
- **`bigip-cis`** demonstrates the traditional external F5 BIG-IP VE model that
  BNK replaces. It needs the `bigipVE:` block in `cluster.yaml` uncommented.
  > [!WARNING]
  > Enabling `bigipVE` provisions a chargeable `c5n.2xlarge` BIG-IP VE appliance
  > with PAYG licensing. Supply its password via
  > `export AWSBNKCTL_BIGIP_PASSWORD='<pass>'` before running — it is never stored
  > in `cluster.yaml`.

## Cost & teardown

Billable while up: 3x `m6i.4xlarge` workers, the EKS control plane, one NAT
gateway, and (if `testing.jumphost.enabled`) a `t3.small` jumphost — roughly
**$3/hour** at `ap-southeast-2` on-demand rates, excluding data transfer and EBS.
Nothing in this topology scales to zero, so an idle cluster costs the same as a
busy one. Demo mode adds nothing; enabling `bigipVE` adds a `c5n.2xlarge` plus
PAYG BIG-IP licensing.

Destroy everything when you are done:

```bash
awsbnkctl down --config my-cluster.yaml --yes
```

`down` works in reverse phase order and is safe to re-run. It discovers
resources by the `awsbnkctl:cluster=<name>` tag, so it still cleans up if the
local state directory is lost.
