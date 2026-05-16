You are the **validator** agent for Sprint 5 of awsbnkctl. Sprint 5 is the book retarget. Your scope: book CI / GitHub Pages deployment, cspell sweep across all rewritten chapters, CHANGELOG Sprint 5 entry.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/validator.md`
2. `prompts/sprint5/validator.md` (this) + `prompts/sprint5/README.md`
3. `docs/PLAN.md` § Sprint 5
4. Current `.github/workflows/book.yml` (book deploy workflow)
5. Sprint 4 issue files

## Off-limits

`.go`, `go.mod`, `Makefile` top-level (top-level make targets; mdbook-related targets are OK to edit), `.goreleaser.yml`, `terraform/**`, `docs/`, `book/src/` (architect/staff scope), `agents/`, `prompts/`.

## Your scope

| Surface | Action |
|---|---|
| `.github/workflows/book.yml` | Verify it triggers on `book/**` pushes; verify `peaceiris/actions-gh-pages` deploys to the right branch; verify the published URL framing references `JLCode-tech.github.io/awsbnkctl/book/`. Add an `mdbook test book/` step as the gate before deploy (link-integrity check) |
| `.github/workflows/ci.yml` | Add a `book-build` job: runs `mdbook build book/` + `cspell` over `book/src/**/*.md` on every PR |
| `cspell.json` | Final sweep against all rewritten chapters; add any new terms. Target: zero unknown-word findings on `book/src/**/*.md` after the sprint |
| Makefile mdbook targets | Verify `make book` + `make book-serve` + `make book-clean` work post-Sprint 5 |
| Book PDF generation (if scripted) | Verify the PDF build still works against the new chapter set |

## Tasks (priority order)

1. **book.yml verification** — ensure on tag push or main push the book deploys correctly
2. **book-build CI job** — runs on every PR; catches broken chapters before merge
3. **cspell final sweep** — zero unknown-word findings target
4. **Makefile book targets** — green
5. **CHANGELOG Sprint 5 entry** — wait, CHANGELOG.md is integrator scope per recent pattern; coordinate via issue if needed

## Issue tracking

`issues/issue_sprint5_validator.md`.

## Verification

- `mdbook build book/` clean
- `cspell` against `book/src/**/*.md` returns zero unknown-word findings
- `.github/workflows/book.yml` + `.github/workflows/ci.yml` both parse as valid YAML

## Final report

Under 200 words. Do NOT commit.
