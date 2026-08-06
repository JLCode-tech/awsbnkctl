# Demo — Full BNK-on-EKS Demo Deployment

> [!NOTE]
> Provisions a complete BNK 2.3 cluster (host-device data path) alongside a multi-ENI jumphost. This is marked as a **demo** topology, pre-staging test clients to drive traffic through curated protocol use-cases.

It behaves exactly like `examples/full-cluster` but with `demo.enabled: true` (equivalent to running `awsbnkctl up --demo`).

## Prerequisites

- **AWS Account & CLI:** Configured AWS credentials and locally built `awsbnkctl`.
- **F5 Supply-Chain Files:** 
  - FAR pull credentials JSON (`./cne_pull_64.json`)
  - Subscription JWT (`./license.jwt`)
  *(Update paths in `cluster.yaml` if these live elsewhere. They are gitignored).*
- **AWS Quotas:** 1 VPC, 1 IGW, 1 NAT GW, 1 EIP, 6 subnets, EKS cluster, 3-node `m6i.4xlarge` managed node group, and a `t3.small` jumphost.

## Quick Start

### 1. Validate & Dry-Run
```bash
# Validate intent (no AWS calls)
awsbnkctl validate examples/demo/cluster.yaml

# Dry-run plan (requires AWS creds, no mutations)
awsbnkctl up --config examples/demo/cluster.yaml --dry-run
```

### 2. Provision & Run Demos
```bash
# Provision infrastructure
awsbnkctl up --config examples/demo/cluster.yaml

# Run the demos
awsbnkctl demo list
awsbnkctl demo run http2 --config examples/demo/cluster.yaml
awsbnkctl demo run --all --config examples/demo/cluster.yaml
```

### 3. Teardown
```bash
awsbnkctl down --config examples/demo/cluster.yaml --yes
```
> [!TIP]
> Reverse-order destroy ensures demo use-cases are cleaned before the underlying infrastructure.

## Migration Scenarios

This topology includes two core scenarios that highlight the **"migrate to BNK"** story:

- **`ingress-migration`**: Installs `ingress-nginx`, `HAProxy`, and a BNK Gateway API route fronting a shared backend to compare traffic paths live before cutover.
  ```bash
  awsbnkctl demo run ingress-migration --config examples/demo/cluster.yaml
  ```
- **`bigip-cis`**: Demonstrates the traditional external F5 BIG-IP VE model that BNK replaces. 
  > [!WARNING]
  > Enabling this provisions a chargeable `c5n.2xlarge` BIG-IP VE appliance. You must provide the password via `export AWSBNKCTL_BIGIP_PASSWORD='<pass>'` before running.
