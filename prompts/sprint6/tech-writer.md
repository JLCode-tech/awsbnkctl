You are the **tech-writer** agent for Sprint 6 of awsbnkctl — the **final sprint**. Read-only review at sprint close. This is the **v1.0-readiness preview**.

**Do NOT edit project files.** Only output: `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint6_tech-writer.md`.

**SPIKE DEFERRAL** carries — v0.2 first-tag still gates on operator-run PRD 07 spike. v1.0 candidate ships structurally complete; "officially supported" tag waits on spike validation.

**Read first:**
1. `agents/tech-writer.md`
2. `prompts/sprint6/tech-writer.md` (this) + `prompts/sprint6/README.md`
3. `docs/PLAN.md` § Sprint 6 + the new "What's deferred to post-v1.0" appendix
4. Sibling Sprint 6 issue files
5. **Every chapter under `book/src/`** — final dogfood pass
6. `README.md` (refreshed sprint-count framing)
7. `CHANGELOG.md` (every sprint entry)
8. `MIGRATING.md`
9. `.github/workflows/*.yml` (final CI matrix end-state)

## Scope (read-only audits)

| Check | What to look for |
|---|---|
| **v1.0-readiness verdict** | If a customer downloaded the v1.0-candidate binary today, would `awsbnkctl init → up → test → down` succeed against an AWS account? (Modulo the operator-run spike validation — that's expected open intent.) |
| **Full book read-through (final)** | Walk Parts I-X as a first-time reader. Note any remaining IBM-residue, broken cross-link, missing v1.x annotation |
| **Sprint 5 BLOCKER closure** | `internal/exec/k8s_install.yaml` no longer references IBM/ROKS/COS; ServiceAccount has IRSA annotation; chapter 19 prose matches reality |
| **Chapter 8/9/11 v1.x annotations** | Sections referencing absent subverbs explicitly noted as "Available in v1.x" |
| **Glossary correctness** | Sprint 5 Issue 3 closure — entries match current implementation |
| **Chapters 17/18/19/32 IBM-residue closure** | Sprint 5 Issue 4 closure — secondary residue swept |
| **README sprint-count** | Reflects "Sprint 6 complete; v0.9-rc ready; v1.0 awaits spike" |
| **CHANGELOG comprehensive** | Every Sprint (0-6) has an entry |
| **MIGRATING.md** | Final word for both migration paths (roksbnkctl → awsbnkctl; manual EKS+BNK → awsbnkctl) |
| **goreleaser snapshot** | `dist/` produces 6 binary archives (if staff ran snapshot) |
| **Security audit** | gosec / govulncheck clean or findings filed |
| **Release-readiness build** | `make build`, `go test ./...`, `terraform validate` on root + all 8 modules |

## Tasks

1. Full book read-through (Parts I-X)
2. Sprint 5 blocker closure verification (Issue 1 + 2)
3. Glossary correctness audit
4. Sprint 5 IBM-residue closure (Issue 4)
5. README/CHANGELOG/MIGRATING comprehensive check
6. CI matrix end-state verification
7. Release-readiness build dogfood
8. v1.0-readiness verdict

## Final report

Under 200 words: chapters reviewed (count), v1.0-readiness verdict (yes / yes-with-spike-pending / no with reason), Sprint 5 blocker closure (closed / not closed), CI matrix verdict, release artefact verdict, top-N residual findings.

This is the final sprint's final report. Goes into the v0.9-rc1 / v1.0-candidate release notes.

Repo at `/Users/j.lucia/Code/github/awsbnkctl/`.
