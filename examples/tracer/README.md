# tracer — minimal VPC topology

The `tracer` topology is the first slice of the awsbnkctl post-Terraform
direction. It provisions the minimum viable network for BNK: a VPC, two
public subnets, two private subnets across two AZs, one NAT gateway, and
the necessary route tables.

No EKS, no IAM, no BNK install — this validates the plumbing before slice 2.

## Prerequisites

- An AWS account with credentials configured (SSO or static keys).
- `awsbnkctl` built locally (`go build -o awsbnkctl ./cmd/...`).
- Sufficient AWS quotas: 1 VPC, 1 Internet Gateway, 1 NAT Gateway, 1 Elastic IP,
  4 subnets, 2 route tables.

## Steps

**1. Authenticate**

```bash
aws sso login --profile <your-profile>
# or: export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... AWS_REGION=ap-southeast-2
```

**2. Edit the intent (optional)**

Open `examples/tracer/cluster.yaml` and adjust `metadata.region`, the subnet
CIDRs, or the AZ list to match your target account. The `metadata.name` field
(`tracer`) is used as the AWS tag value and local state directory name — keep
it lowercase alphanumeric.

**3. Provision**

```bash
# Dry-run first (no AWS mutations):
awsbnkctl up --config examples/tracer/cluster.yaml --dry-run

# Live provision:
awsbnkctl up --config examples/tracer/cluster.yaml
```

The command prints each phase as it runs. The IDs cache is written to
`.awsbnkctl/tracer/state.env` after every successful phase, so a mid-run
failure is safe to resume.

**4. Verify (optional)**

```bash
cat .awsbnkctl/tracer/state.env
# Should list VPC_ID, IGW_ID, PUBLIC_SUBNETS, PRIVATE_SUBNETS, NAT_GW_ID, etc.

aws ec2 describe-vpcs --filters "Name=tag:awsbnkctl:cluster,Values=tracer" \
  --query 'Vpcs[*].{ID:VpcId,CIDR:CidrBlock}'
```

**5. Tear down**

```bash
awsbnkctl down --config examples/tracer/cluster.yaml --yes
```

Reverse-order destroy. Tolerates resources that are already gone (safe to
re-run). If `.awsbnkctl/tracer/state.env` is missing, down falls back to
tag-discovery (`awsbnkctl:cluster=tracer`) to find and delete resources.
