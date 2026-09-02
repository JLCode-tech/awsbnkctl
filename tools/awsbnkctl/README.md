# awsbnkctl — BNK Forge Module

This directory packages **`awsbnkctl`** as a [BNK Forge](https://github.com/f5devcentral/bnk-forge) container-runner module.

## Module Details

- **Module Name:** `awsbnkctl`
- **Runner Image:** `ghcr.io/jlcode-tech/awsbnkctl-tools-runner:1.1.0`
- **Target Cloud:** AWS (EKS + EC2)
- **Persistent Workspace:** `/state`
- **Authentication:** Standard AWS credentials or IAM role propagation

## Usage in BNK Forge

1. Register this repository or `bnkctl-index` in the BNK Forge Catalog.
2. Select **`awsbnkctl`** from the discovered modules list.
3. Configure AWS region, cluster name, and instance type.
4. Deploy and observe real-time cluster provisioning and live traffic flow.
