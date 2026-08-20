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
| AWS-13 | CHANGELOG `[Unreleased]` section present, carrying the `AWSBNKCTL_SKIP_AUTH` entry |
| AWS-15 | Repo root is clean — no untracked scratch files remain |
| AWS-21 | `docs/adr/` ignore rule now has its own stanza and a stated rationale |

## Superseded

| ID | Title | Outcome |
|----|-------|---------|
| AWS-12 | Recommended example has `forge.enabled: true` | **Closed as won't-fix, by decision.** `forge.enabled: true` is now deliberate and applied to *every* example, not just the recommended one. Phase 09 soft-fails when no Forge is reachable — it writes a pending link, warns, and `up` still exits 0 — so the setting costs a reader nothing. Each `forge:` block states how to switch it off and links `f5devcentral/bnk-forge` for anyone who wants the UI. |

## Outstanding

| ID | Sev | Title | Notes |
|----|-----|-------|-------|
| AWS-08 | P1 | GitHub Issues disabled while templates exist | Repo setting; needs decision |
| AWS-09 | P1 | None of the up phases documented individually | Docs task. 39 phase files exist; a `full-cluster` dry-run reports 35 phases (the count varies with `pattern:` and the opt-in blocks). `docs/ARCHITECTURE.md` covers the model but not phase-by-phase. |
| AWS-10 | P2 | `demo-stage` has no down counterpart | Needs `Phase17dDemoStageDown` |
| AWS-11 | P2 | `sriov-dataplane` has no down counterpart | Needs `Phase20bSriovDataplaneDown` |
| AWS-14 | P2 | `.gitignore.maf.new` tracked | Still tracked (57 lines, a MAF-installer artifact). Needs `git rm --cached` + an ignore rule — deletion not yet authorised. |
| AWS-17 | P2 | Uncommitted `jumphost.go` SSM change | Security issue, do not merge as-is |
| AWS-19 | P3 | `forge-register`/`forge-unregister` asymmetry | Rename + parity test |
| AWS-20 | P3 | Live credential files in worktree | Reduced: `.hf_token` is absent, and the dummy `cne_pull_64.json` / `license.jwt` placeholders were removed from `examples/{full-cluster,external-only,sriov-external}`. The only remaining pair is `examples/agentcore-demo/{cne_pull_64.json,license.jwt}` — real, but gitignored and untracked. |

## Related cleanup

- Delete the legacy repo when ready.

## Last updated

2026-08-21 — statuses above re-verified against the working tree, not carried
forward from the previous pass.
