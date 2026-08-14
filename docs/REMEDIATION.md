# awsbnkctl Remediation Tracker

This file tracks the post-migration remediation items for the clean
`JLCode-tech/awsbnkctl` repo. It is maintained by the maintainers and should be
updated as items close so that any agent or contributor opening the repo sees
current, non-stale state.

## Legend

- ✅ Resolved
- 🔄 In progress / has open PR
- ⬜ Open

## Resolved

| ID | Title |
|----|-------|
| AWS-01 / AWS-02 | Migration to clean repo |
| AWS-03 | Full-history gitleaks CLI |
| AWS-04 | Not a fork |
| AWS-05 | `AWSBNKCTL_SKIP_AUTH` hook for credential-free dry-run |
| AWS-06 | CI gate for `up --dry-run` |
| AWS-16 / AWS-18 | Local Zones consolidated and merged |
| AWS-07 | `scripts/test-integration-aws.sh` full-up-dryrun is dead — fixed in PR #2; resolves relative `bnk.farArchive` / `bnk.jwt` paths against `cluster.yaml` directory and rewrites script to mirror CI gate |

## Outstanding

| ID | Sev | Title | Notes |
|----|-----|-------|-------|
| AWS-08 | P1 | GitHub Issues disabled while templates exist | Repo setting; needs decision |
| AWS-09 | P1 | None of the 39 up steps documented | Docs task |
| AWS-10 | P2 | `demo-stage` has no down counterpart | Needs `Phase17dDemoStageDown` |
| AWS-11 | P2 | `sriov-dataplane` has no down counterpart | Needs `Phase20bSriovDataplaneDown` |
| AWS-12 | P2 | Recommended example has `forge.enabled: true` | Set to false, add comment block |
| AWS-13 | P2 | CHANGELOG missing `[Unreleased]` | Add post-v1.0.0 fixes |
| AWS-14 | P2 | `.gitignore.maf.new` tracked | `git rm` + ignore rule |
| AWS-15 | P2 | Untracked scratch files in repo root | Triage `build_release.sh`, `cluster-*.yaml`, etc. |
| AWS-17 | P2 | Uncommitted `jumphost.go` SSM change | Security issue, do not merge as-is |
| AWS-19 | P3 | `forge-register`/`forge-unregister` asymmetry | Rename + parity test |
| AWS-20 | P3 | Live credential files in worktree | `.hf_token`, etc. |
| AWS-21 | P3 | `docs/adr/` ignore rule has no rationale | Fix `.gitignore` stanza |

## Related cleanup

- Delete the legacy repo when ready.

## Last updated

2026-08-14
