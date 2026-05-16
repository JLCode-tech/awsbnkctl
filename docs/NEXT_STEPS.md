# NEXT_STEPS — session handoff

Snapshot of where the awsbnkctl AWS-retarget left off, and what to do when picking it back up.

## State as of last session

All six AWS-retarget sprints (Sprint 0 → Sprint 6) committed and pushed to `JLCode-tech/awsbnkctl@main`. **The repository is structurally complete for `v0.9-rc1`.**

Commit timeline:

| Commit | Sprint | Outcome |
|---|---|---|
| `9a59806` | 0 prep | Fork retarget docs (README/CHANGELOG/MIGRATING/PLAN/PRDs/prompts) |
| `7861e63` | 0 | IBM strip + AWS stub + identity rewrite (Go module path, binary rename) |
| `203f28a` | 1 | EKS cluster + SR-IOV node group module + `internal/aws/` (offline) |
| `dc8e5a6` | 2 | S3 supply chain + IRSA + workspace AWS retarget (offline) |
| `c40c238` | 3 | Five reusable modules ported + first end-to-end `up --dry-run` |
| `727b2bb` | 4 | Test surface + doctor Service Quotas + AWS E2E chapters |
| `018e2bf` | 5 | Book retarget + IBM-residue 297 → 1 + reference chapter regen |
| `e2debed` | 6 | Hardening + ops-pod IRSA retarget + v0.9-rc release-artefact prep |

Verification state (last green check on commit `e2debed`):
- `go build / vet / test / gofmt` clean across 13 packages
- `terraform validate` clean on root + all 8 modules
- `awsbnkctl init/up/down/test/doctor --dry-run` all work offline
- `awsbnkctl doctor` reports 6 AWS pre-flight rows when workspace configured
- `goreleaser build --snapshot --clean` produces 6 archives (linux/macOS/windows × amd64/arm64)
- `gosec` + `govulncheck` clean (findings documented + accepted)
- `mdbook build book/` + `cspell` zero findings (verified in CI)
- 10 CI jobs configured

## Immediate next actions

### 1. Cut `v0.9-rc1` (no AWS required)

```bash
cd /Users/j.lucia/Code/github/awsbnkctl
git tag v0.9-rc1
git push origin v0.9-rc1
```

This triggers `.github/workflows/release.yml` which runs goreleaser, attaches binaries + checksums + book PDF, and publishes to https://github.com/JLCode-tech/awsbnkctl/releases.

Verify post-tag:
- Release page shows 6 archive files + `checksums.txt` + `awsbnkctl-book-v0.9-rc1.pdf`
- `go install github.com/JLCode-tech/awsbnkctl/cmd/awsbnkctl@v0.9-rc1` succeeds
- `awsbnkctl --version` reports the tag

### 2. Run the PRD 07 spike (requires AWS account, ~$5–15 cost)

Operator-driven validation that gates `v1.0`. Full protocol at `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Spike protocol".

Summary (day-by-day):

**Day 1 — cluster + node group**
- `eksctl create cluster --without-nodegroup --version 1.30 --region us-east-1 ...`
- `eksctl create nodegroup --managed=false --node-type c5n.4xlarge --nodes 2 ...`
- Verify nodes report `Ready` and surface primary ENA interface

**Day 2 — SR-IOV stack**
- Install Multus thick-plugin DaemonSet (upstream `k8snetworkplumbingwg/multus-cni`)
- Install SR-IOV CNI binaries (upstream `k8snetworkplumbingwg/sriov-cni`)
- Install SR-IOV device plugin (upstream `k8snetworkplumbingwg/sriov-network-device-plugin`) configured to discover ENA VFs by vendor/device ID
- Verify `kubectl describe node <node>` shows `intel.com/sriov: <N>` in `Allocatable`

**Day 3 — pod schedules onto VF + BNK compatibility**
- Apply `NetworkAttachmentDefinition` referencing SR-IOV CNI
- Schedule pod requesting `intel.com/sriov: 1`
- `kubectl exec` in; verify second interface; verify `ethtool -i <iface>` reports ENA driver + SR-IOV active
- **BNK-specific check:** verify `ip link show` surfaces the VF in a state BNK's CNEInstance reconciler accepts — this is the load-bearing question

Spike fail modes + mitigations are catalogued in PRD 07 § "Spike fail modes". The hypothesis to validate: **does ENA SR-IOV match BNK's reference (Mellanox SR-IOV) closely enough that CNEInstance reconciles cleanly?**

After the spike:
1. Fill in `docs/prd/07-EKS-CLUSTER-SRIOV.md` § "Resolved in spike" with findings
2. If hypothesis holds → cut `v1.0` (`git tag v1.0.0 && git push origin v1.0.0`)
3. If hypothesis breaks → file Sprint 7 work to address (PRD 07 lists mitigations: ENA tuning, FLO patch, multi-ENI fallback, EC2-metal as last resort)

## How to resume work with Claude Code

The four-role parallel-agent dispatch pattern documented in `prompts/README.md` is what got us here. If you want to dispatch a Sprint 7 cleanup pass or a v1.x feature sprint:

1. Author `prompts/sprint7/{README,architect,staff,validator,tech-writer}.md` (use sprint6 as template)
2. Dispatch architect + staff + validator in parallel via Claude Code's Agent tool
3. Dispatch tech-writer after the three return
4. Integrator (you, or another agent) folds the four issue files and commits

Each sprint's task brief is checked in at `prompts/sprint<N>/` for auditability — that pattern carries forward.

### Practical operational notes (from this session)

- **Disk hygiene matters.** `terraform/modules/*/.terraform/` directories grow to 600-800MB each during validate; clean with `find terraform -name '.terraform' -type d -exec rm -rf {} +` after any module work. `.gitignore` excludes them but they fill the local disk fast.
- **Agent 529 errors.** Anthropic API can return `529 Overloaded` mid-dispatch; agents may complete substantial work before the error. Always check git state on disk before retrying — sometimes the work landed and you just need to take over the final tasks (file issues, commit) as integrator.
- **Test contract carry-over.** `internal/doctor/doctor_test.go::TestRunWithWhy_StockDevBox_NoWorkspace` enforces an inherited "stock dev box = no warnings" contract. Doctor changes that surface new warnings on a workspace-less box require updating this test.

## v1.x backlog (deferred from v1.0)

Consolidated in `docs/PLAN.md` § "What's deferred to post-v1.0". Highlights:

- **`--irsa-role=auto`** flow for `awsbnkctl ops install` (the AWS equivalent of the inherited IBM `--trusted-profile=auto`). v1.0 requires operator to pre-create the IRSA role and pass via `OPS_IRSA_ROLE_ARN`.
- **ECR mirror first-class story** for air-gapped customers. Sprint 2 shipped the `terraform/modules/ecr_mirror/` module as optional (gated on `var.enable_ecr_mirror`); v1.x makes it the default with automated FAR-image sync workflow.
- **Karpenter / EKS Auto Mode integration** — currently no clean integration with SR-IOV device plugins.
- **Calico-on-EKS as alternative CNI** to AWS VPC CNI.
- **AWS Secrets Manager** as the JWT home instead of S3 (current design uses S3 for both FAR archive + JWT).
- **Multi-region GSLB testing** — `awsbnkctl test dns` is single-region in v1.0.
- **Air-gapped install path** — currently assumes outbound HTTPS to AWS APIs + ECR + FAR registry.
- **Homebrew tap** — same as roksbnkctl's v1.x roadmap.
- **Doctor visibility on stock dev box** — Sprint 1 tech-writer Issue 1: AWS rows currently workspace-gated to preserve inherited test contract. v1.x relaxes the contract.

## Open issue files worth a read before resuming

The sprint-by-sprint audit trail lives in `issues/issue_sprint<N>_<role>.md`. Especially:

- `issues/issue_sprint6_tech-writer.md` — final v1.0-readiness preview with residual findings
- `issues/issue_sprint5_tech-writer.md` — book-retarget verification
- `issues/issue_sprint3_tech-writer.md` — first end-to-end `up --dry-run` review
- `issues/issue_sprint1_staff.md` — staff agent's design notes from the EKS module build

## Memory pointers

This session built up auto-memory at `~/.claude/projects/-Users-j-lucia-Code-github/memory/`. A future Claude Code session will load `MEMORY.md` automatically; specific entries referencing this project will surface relevant context.

## Quick orientation for a future agent

If you're a Claude Code agent picking this up cold:

1. `git log --oneline -10` to see the sprint-commit timeline
2. Read this file (`docs/NEXT_STEPS.md`) and `docs/PLAN.md` § Sprint 6 close + "What's deferred to post-v1.0"
3. Skim `docs/prd/07-EKS-CLUSTER-SRIOV.md` for the load-bearing design + spike protocol
4. Skim `agents/README.md` + `prompts/README.md` for the four-role dispatch pattern
5. `go build ./... && go test ./...` to verify the green-gate still holds
6. Confirm with the operator before:
   - cutting tags (visible action, can't easily un-tag in releases)
   - running `terraform apply` against live AWS (cost + irreversible state)
   - running the PRD 07 spike (~$5-15 in real AWS resources)
