# Troubleshooting

The failures you're statistically most likely to hit running `awsbnkctl` against a real AWS account, organised **symptom → root cause → fix**. The catalogue is mined from the issue logs accumulated across Sprints 0-3 plus the failure shapes that surface in PRDs 07 + 08. Each entry tries to be diagnostic-first — name the symptom in the exact wording the binary or AWS API returns, then walk to the fix.

When your symptom isn't on this page: re-run with `--verbose` (the verbose output usually surfaces the root cause directly), then check [Chapter 23 — The E2E test plan](./23-e2e-test-plan.md) for the phase-by-phase pass criteria. The per-phase log files under `/tmp/awsbnkctl-e2e-backends/` are the next stop. This chapter expands sprint-by-sprint; expect the catalogue to grow alongside Sprint 4's test-surface work and Sprint 6's hardening pass.

## Cluster + node group

### Symptom: `awsbnkctl up cluster` returns success but `kubectl describe node` shows no `intel.com/sriov` resource

**Root cause**: the SR-IOV device plugin DaemonSet isn't advertising VFs from the node's ENA interface. Either the plugin's `ConfigMap` is pointing at a vendor/device ID that doesn't match what the `c5n.4xlarge`'s ENI exposes, or `intel_iommu=on iommu=pt` didn't apply at first boot (kernel parameter missing → no VFs visible to the host kernel → device plugin sees nothing to advertise).

**Fix**: confirm the kernel parameters with `awsbnkctl k exec <any-pod> --hostnetwork -- cat /proc/cmdline | grep iommu` — if `intel_iommu=on iommu=pt` is missing, the launch template's user-data didn't apply (often: a custom AMI overrode the user-data hook). Then `lspci -nn | grep -i ethernet` from a node-debug pod to find the actual vendor/device IDs; cross-check against `kubectl get cm sriov-device-plugin-config -n kube-system`. If the IDs differ, edit the `sriov_device_plugin_config` Terraform variable on the `eks_cluster` module input and re-apply. [Chapter 33 — The data-plane decision](./33-data-plane-decision.md) walks the SR-IOV-on-EKS background in depth.

### Symptom: `CNEInstance` stuck in `Pending` indefinitely

**Root cause**: most commonly the same root cause as the previous symptom — no schedulable VFs means the CNEInstance reconciler's pod spec (which requests `intel.com/sriov: 1`) never schedules. Other causes: not enough VFs left in the pool (the device plugin advertised N, you've already consumed N); a NodeSelector mismatch (CNEInstance pinned to a node label your node group doesn't carry); the SR-IOV CNI's NetworkAttachmentDefinition referencing a `resourceName` that doesn't match what the device plugin advertises.

**Fix**: `awsbnkctl k describe pod -n flo-system <pending-pod>` — the `Events` block names the unschedulable reason. If it's "Insufficient intel.com/sriov", check `kubectl get node -o jsonpath='{range .items[*]}{.metadata.name}: {.status.allocatable.intel\.com/sriov}{"\n"}{end}'` to see VF availability. If it's "MatchNodeSelector", reconcile the CNEInstance CR's `spec.nodeSelector` against your node labels. If the NAD references a `resourceName` typo, edit and re-apply.

## AWS credentials + auth

### Symptom: `awsbnkctl up` errors with `operation error STS: GetCallerIdentity: failed to retrieve credentials from any of the providers in the chain`

**Root cause**: no link in the AWS standard chain resolved. The chain is documented in [PRD 04 § "Host-side: the AWS standard credential chain"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#host-side-the-aws-standard-credential-chain) — `AWS_ACCESS_KEY_ID` env, `~/.aws/credentials` profile, SSO cached token, EC2 instance-role IMDS, container task role, web-identity token. None resolved.

**Fix**: pick a chain link and populate it. The supported paths on a stock dev box are `aws configure` (writes `~/.aws/credentials` for a named profile; export `AWS_PROFILE=<name>` if not `default`), `aws sso login` (writes a cached token under `~/.aws/sso/cache/`), or directly setting `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` in env. Then `aws sts get-caller-identity` to confirm the chain resolves before re-running `awsbnkctl up`. `awsbnkctl doctor` reports the resolved provider name when the chain succeeds, which narrows "which link is winning" when multiple are configured.

### Symptom: `awsbnkctl up` errors with `EntityTooSmallException` or `AccessDenied` reading the supply-chain bucket during `terraform apply`

**Root cause**: not actually a creds issue most of the time — the host-side chain resolved, but the IRSA trust policy's `<issuer>:sub` condition key doesn't match the namespace + service account FLO actually runs under (typo in `flo_namespace` or `flo_service_account_name`, or the `flo` module created the SA under a different name). `AccessDenied` from S3 against an IRSA-pulled credential almost always traces back to STS having issued a token for a role whose policy doesn't cover the bucket.

**Fix**: cross-check the trust policy against the actual SA. `aws iam get-role --role-name awsbnkctl-<workspace>-flo-supply-reader --query 'AssumeRolePolicyDocument.Statement[0].Condition'` shows the condition keys. `awsbnkctl k get sa flo-controller -n flo-system -o yaml | grep role-arn` confirms the annotation. Both must match byte-for-byte — a stray hyphen breaks STS silently. [Chapter 25 § "IRSA trust chain"](./25-cos-supply-chain.md#irsa-trust-chain) walks the four-hop trust chain in detail.

## EKS cluster access

### Symptom: `kubectl get nodes` returns `error: You must be logged in to the server (Unauthorized)` against a freshly-created EKS cluster

**Root cause**: the IAM identity that ran `awsbnkctl up cluster` is not the same identity `kubectl` is now using. EKS embeds the cluster-creator IAM ARN as a `system:masters` mapping at create time; if your local `kubectl` is now resolving a different IAM identity (different `AWS_PROFILE`, different SSO role assumption, role-chained through `source_profile`), the cluster's access-entry table doesn't recognise it.

**Fix**: check `aws sts get-caller-identity` against the identity used at `awsbnkctl up cluster` time — if they differ, either `export AWS_PROFILE=<the-creator-profile>` and re-run, or add the new identity via `aws eks create-access-entry` + `aws eks associate-access-policy --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy`. The `awsbnkctl ops install` flow (Sprint 4) ships an automated path for the ops-pod identity; for ad-hoc operator access, the `aws eks` CLI is the canonical surface.

### Symptom: `awsbnkctl up cluster` returns success but `awsbnkctl k get pods --all-namespaces` returns `error: the server doesn't have a resource type "pods"`

**Root cause**: the wrong kubeconfig is active. The EKS endpoint is up, the cluster is healthy, but `kubectl`'s current context is pointing at a different cluster (or at a stale entry whose token expired). The `KUBECONFIG` env var or `~/.kube/config` is loading a context that resolves to a non-existent or wrong-cluster endpoint.

**Fix**: `aws eks update-kubeconfig --name <cluster> --region <region>` rewrites `~/.kube/config` with a fresh context for the cluster, then `kubectl config use-context <context-name>`. The post-apply hook in `awsbnkctl up cluster` does this automatically when the cluster module reports `Ready`; if it didn't, the hook hit a 4xx (transient EKS API delay — re-run the kubeconfig fetch alone, no need to re-apply).

## Terraform + AWS quotas

### Symptom: `terraform plan` succeeds but `terraform apply` errors with `VcpuLimitExceeded` or `InstanceLimitExceeded`

**Root cause**: the AWS account's vCPU quota for the chosen instance family in the chosen region is below what the `node_desired_size × instance_type_vcpus` math requires. Greenfield AWS accounts cap "Running On-Demand `c5n.*` instances" and "Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances" surprisingly low (often 5 vCPUs for `c5n.*` on a fresh account — not enough for two `c5n.4xlarge`).

**Fix**: `aws service-quotas get-service-quota --service-code ec2 --quota-code L-1216C47A` (the standard-instance vCPU quota) reports the current limit. Open a quota-increase request via `aws service-quotas request-service-quota-increase`; typical turnaround is 1-24 hours. `awsbnkctl doctor` performs a pre-flight check on the workspace's instance type + region in Sprint 4; until then, run the quota check manually before the first `up cluster`.

### Symptom: `terraform apply` errors `InvalidParameterException: Subnets specified must be in at least two different AZs` for the EKS cluster

**Root cause**: EKS requires the cluster subnets to span at least two AZs (and the `eks_cluster` module defaults to requiring three for HA). Either the `subnet_ids` input lists subnets in one AZ only, or the existing VPC's subnets happen to all live in the same AZ.

**Fix**: `aws ec2 describe-subnets --subnet-ids <id1> <id2> ... --query 'Subnets[*].AvailabilityZone'` shows the AZ for each. Add subnets from at least two more AZs (`aws ec2 create-subnet` per AZ) before re-applying. If using the `eks_cluster` module's create-VPC path (`var.create_vpc = true`), the module provisions across three AZs automatically — this symptom only surfaces against operator-supplied VPCs.

### Symptom: `terraform destroy` leaves orphan ENIs the destroy reports as "successfully removed"

**Root cause**: the LoadBalancer-service-backed ENIs and the AWS VPC CNI's pod-IP-allocation ENIs both attach to nodes outside Terraform's view. When a node group is destroyed, those ENIs are eligible for cleanup but AWS's cleanup is asynchronous; Terraform's destroy returns success against the resources *it* manages and leaves the side-effect ENIs for the AWS-side reaper to handle.

**Fix**: `aws ec2 describe-network-interfaces --filters 'Name=status,Values=available' --query 'NetworkInterfaces[*].[NetworkInterfaceId,Description]'` lists the orphans; `aws ec2 delete-network-interface --network-interface-id <id>` deletes each. The orphans don't accrue cost by themselves (idle ENIs are free) but they hold subnet IP-space allocations that block a subsequent `up cluster` against the same VPC.

## CI-specific

### Symptom: nightly e2e run fails on phase D with `Error: Provider configuration is missing`

**Root cause**: a `terraform init` cache invalidation under `~/.awsbnkctl/<ws>/state/.terraform/` left a partial provider download. Happens after a CI worker is recycled mid-init.

**Fix**: `rm -rf ~/.awsbnkctl/<ws>/state/.terraform/` then re-run `awsbnkctl up`. Terraform-init re-downloads the providers cleanly. For CI workers that get recycled often, add a pre-step that purges `.terraform/` before each run.

### Symptom: cred audit (phase M) reports `AWS_SECRET_ACCESS_KEY found in docker inspect output`

**Root cause**: real stop-ship — credentials leaked into a docker container's runtime env. Check `internal/exec/docker.go::buildEnvArgv` for any code path that passes the secret by value (`-e AWS_SECRET_ACCESS_KEY=<value>`) rather than by reference (`-e AWS_SECRET_ACCESS_KEY` — let docker pull from the caller's env).

**Fix**: file an issue immediately, do not tag a release until this is green. Phase M is the v1.0 release gate; a leak here means the redactor or the cred-passing logic regressed. See [PRD 04 § "Backend × credential matrix (AWS retarget)"](https://github.com/JLCode-tech/awsbnkctl/blob/main/docs/prd/04-CREDENTIALS.md#backend--credential-matrix-aws-retarget) for the threat model.

## Getting more help

When the symptom isn't on this page:

1. Re-run with `--verbose` (`-v`) — the verbose output usually surfaces the root cause directly.
2. Check `/tmp/awsbnkctl-e2e-backends/<phase>-<ts>.log` for the per-phase trail.
3. Cross-reference [Chapter 23 — The E2E test plan](./23-e2e-test-plan.md) — the phase-by-phase pass criteria usually narrow down where the breakage lives.
4. For supply-chain-specific failures (IRSA `AccessDenied`, bucket-policy propagation lag, FLO image-pull issues), [Chapter 25 — S3 (and optional ECR) supply chain](./25-cos-supply-chain.md) walks the trust chain end-to-end.
5. For SR-IOV / data-plane oddities, [Chapter 33 — The data-plane decision](./33-data-plane-decision.md) explains why awsbnkctl made the self-managed-node-group choice and what alternatives exist if your workload doesn't fit it.
6. File an issue on [github.com/JLCode-tech/awsbnkctl](https://github.com/JLCode-tech/awsbnkctl/issues) with the verbose output, the `awsbnkctl --version` stamp, and the per-phase log if there is one.
