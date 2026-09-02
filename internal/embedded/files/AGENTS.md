# AGENTS.md — awsbnkctl deployment reference for agentic operators

> This file is scaffolded into a **workspace** by `awsbnkctl agent init`. It
> is the shared knowledge base every persona reads first when driving an AWS EKS +
> BIG-IP Next for Kubernetes (BNK) deployment with an agentic CLI. `awsbnkctl`
> itself embeds no LLM — you bring your own coding agent (`awsbnkctl agent
> <cli>` prints the invocation) and it acts under one of the personas in
> `personas/`.

---

## Quick orientation

```
cluster.yaml           single source of truth for THIS deployment (the contract)
decisions.md           why we chose what we chose — alternatives rejected + rationale
journal/<date>-*.md    append-only timeline (one entry per significant action)
report.md              the customer-facing deliverable (doc-specialist writes it)
personas/              role definitions — act as exactly ONE at a time
```

State `awsbnkctl` manages for you (read, don't hand-edit):

```
.awsbnkctl/<cluster>/state.env          cached phase state (rebuildable from AWS tags)
.awsbnkctl/<cluster>/kubeconfig         cluster admin kubeconfig (auto-fetched)
.awsbnkctl/<cluster>/ids.json           persisted AWS resource IDs
```

---

## The Phased Lifecycle (39 Go SDK phases)

```
init  →  validate  →  up  →  scenarios run  →  down
```

- `awsbnkctl validate <file>` validates your intent schema and prerequisite parameters without calling AWS APIs.
- `awsbnkctl up -f <file>` provisions all 39 phases end-to-end (VPC, subnets, IGW, NAT, EKS, NodeGroups, IRSA, Multus, host-device secondary ENIs, BNK activation, jumphost, forge registration).
- `awsbnkctl scenarios run <name>` validates data plane connectivity, L4/L7 routing, or AI Gateway features from inside the VPC.
- `awsbnkctl down -f <file> --yes` safely tears down all AWS resources in reverse dependency order.

---

## Required gates on every destructive command

- `up` provisions real AWS cloud resources (EKS cluster, NAT Gateways, EC2 nodes).
- `down` tears down AWS resources in reverse order. Always requires `--yes` in non-interactive/scripted contexts.
- Pass `--dry-run` to preview operations before execution.

---

## Credentials — never print or commit secrets

- AWS credentials are read from standard environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`), SSO profiles (`AWS_PROFILE`), or IAM instance roles.
- `cluster.yaml` holds paths/references to credentials (`bnk.farArchive`, `bnk.jwt`), never inline secret keys.
- Never write secrets, JWT tokens, or kubeconfig data into `decisions.md`, `journal/`, or `report.md`.

