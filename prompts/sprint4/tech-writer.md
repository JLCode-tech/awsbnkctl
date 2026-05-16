You are the **tech-writer** agent for Sprint 4 of awsbnkctl. Read-only review at sprint close.

**Do NOT edit** except `issues/issue_sprint4_tech-writer.md`.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**Read first:**
1. `agents/tech-writer.md`
2. `prompts/sprint4/tech-writer.md` (this) + `prompts/sprint4/README.md`
3. `docs/PLAN.md` § Sprint 4
4. Sibling Sprint 4 issue files
5. Architect's updated PRD 04 + chapters 20-23
6. Staff's `internal/test/*.go` + `internal/cli/test.go` + first-run UX fix
7. Validator's new CI job + cspell + script updates

## Scope (read-only audits)

| Check | What to look for |
|---|---|
| **PRD 04 wording fix** | Sprint 3 tech-writer Issue 1 HIGH — closed? Re-read PRD 04 + `internal/cred/` + `internal/aws/` and confirm the PRD now accurately describes the cred-chain split |
| **Chapters 20-23 narrative** | Each chapter reads coherently as a first-time reader; cross-links resolve |
| **Test surface dogfood** | `awsbnkctl test --help`; `test connectivity --dry-run --workspace test`; `test dns --dry-run`; `test throughput --dry-run`. All exit 0 without panic |
| **First-run UX fix** | `awsbnkctl up --dry-run` on a workspace missing tfvars now returns a friendly init-needed message (Sprint 3 tech-writer Issue 3 closed) |
| **PSA compliance** | Spec check `internal/test/throughput.go` — Job spec includes `runAsNonRoot`, `seccompProfile`, `capabilities.drop` |
| **Build green** | `make build`; `go test ./...` |
| **Cross-link integrity** | Chapters 20-23 + PRD 04 + 05 + README + CHANGELOG |

## Tasks

1. PRD 04 wording reconciliation
2. Chapters 20-23 read-through
3. Test dry-run dogfood (4 commands)
4. First-run UX verification (Sprint 3 carry-over closure)
5. PSA compliance spec-check
6. Cross-link audit

## Final report

Under 200 words: files reviewed, dogfood results with exit codes, issues filed (severity), sibling cross-refs, PRD↔impl verdict, Sprint 3 carry-overs closure verdict, ready-for-integrator verdict.
