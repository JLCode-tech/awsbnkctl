# Tracer — Minimal VPC Topology

> [!NOTE]
> The `tracer` topology is the first slice of the awsbnkctl post-Terraform direction. It provisions the minimum viable network for BNK: a VPC, public/private subnets across two AZs, one NAT gateway, and necessary route tables.

This topology explicitly excludes EKS, IAM, and the BNK install to validate fundamental plumbing before proceeding to the next slice.

## Prerequisites

- **AWS Account:** Credentials configured via SSO or static keys.
- **Local Binary:** `awsbnkctl` built locally (`go build -o awsbnkctl ./cmd/...`).
- **AWS Quotas:** 1 VPC, 1 Internet Gateway, 1 NAT Gateway, 1 Elastic IP, 4 subnets, 2 route tables.

## Quick Start

### 1. Authenticate
```bash
aws sso login --profile <your-profile>
```

### 2. Configure (Optional)
Edit `examples/tracer/cluster.yaml` to adjust `metadata.region`, subnet CIDRs, or AZ lists. 
> [!TIP]
> Keep `metadata.name` (`tracer`) lowercase alphanumeric, as it is used for AWS tags and local state directory naming.

### 3. Provision
```bash
# Dry-run first to validate without AWS mutations:
awsbnkctl up --config examples/tracer/cluster.yaml --dry-run

# Live provision:
awsbnkctl up --config examples/tracer/cluster.yaml
```
> The command prints each phase as it runs. State is safely cached to `.awsbnkctl/tracer/state.env`, making mid-run failures safe to resume.

### 4. Verify & Teardown
```bash
# Verify outputs
cat .awsbnkctl/tracer/state.env

# Tear down (tolerates already-deleted resources)
awsbnkctl down --config examples/tracer/cluster.yaml --yes
```
