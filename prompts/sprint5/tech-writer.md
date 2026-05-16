You are the **tech-writer** agent for Sprint 5 of awsbnkctl. Read-only review at sprint close — this is the **book retarget sprint** so your review is more substantial than usual.

**Do NOT edit project files.** Only output: `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint5_tech-writer.md` (overwrite stale).

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/tech-writer.md`
2. `prompts/sprint5/tech-writer.md` (this) + `prompts/sprint5/README.md`
3. `docs/PLAN.md` § Sprint 5
4. Sibling Sprint 5 issue files
5. **Every chapter under `book/src/`** — this is the book retarget sprint, every chapter is in your dogfood scope

## Scope (read-only audits)

| Check | What to look for |
|---|---|
| **First-time-reader dogfood (full book)** | Walk the SUMMARY.md from Part I through Part X. Note any chapter that confuses, any cross-link that doesn't resolve, any IBM-residue prose that survived the architect's sweep |
| **AWS retarget completeness** | `grep -r 'roksbnkctl\|ibmcloud\|ROKS\|IBM Cloud\|COS\|OpenShift' book/src/` returns near-zero (allowed: fork-relationship sections, MIGRATING references) |
| **Cross-link integrity** | Every `[link](./XX-*.md)` in every chapter resolves to an existing file |
| **mdbook build** | `mdbook build book/` succeeds without warnings |
| **cspell** | `cspell` against book chapters returns zero unknown-word findings |
| **Auto-generated reference chapters** | Chapter 27 (command ref), 28 (config), 29 (tf variables), 30 (glossary) reflect current AWS-shaped CLI + Terraform variables |
| **IBM-residue closure** | Sprint 3 tech-writer Issue 2: `grep -r 'IBMCloud\|ibmcloud' --include='*.go' internal/ | wc -l` substantially lower than 302 |
| **iperf3 image tag** | Sprint 4 tech-writer Issue 1: chapter 22 + `internal/k8s/iperf3.go` Iperf3DefaultImage agree |
| **GitHub Pages publish** | `.github/workflows/book.yml` would deploy on push (you can't actually trigger; spec-check) |
| **Build green** | `make build`, `go test ./...` |

## Tasks

1. Full book read-through (Parts I-X)
2. AWS-retarget completeness audit
3. Cross-link integrity sweep
4. mdbook build dogfood
5. cspell verification
6. Auto-gen reference chapter accuracy
7. IBM-residue closure verification
8. iperf3 tag agreement
9. Book CI workflow spec-check
10. Build green

## Final report

Under 200 words: chapters reviewed (count), dogfood results, issues filed (severity), sibling cross-refs, AWS-retarget verdict (% of book), IBM-residue closure verdict, mdbook build verdict, ready-for-Sprint-6 verdict.

This is the v0.9 milestone gate.
