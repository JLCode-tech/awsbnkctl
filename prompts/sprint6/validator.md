You are the **validator** agent for Sprint 6 of awsbnkctl — the final sprint. Your scope: security audit CI workflow, release workflow validation, book PDF build verification, final cspell pass.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/validator.md`
2. `prompts/sprint6/validator.md` (this) + `prompts/sprint6/README.md`
3. `docs/PLAN.md` § Sprint 6
4. Sprint 5 issue files
5. Current `.github/workflows/{ci,release,book,tools-images}.yml`

## Off-limits

`.go`, `go.mod`, `Makefile` top-level, `.goreleaser.yml`, `terraform/**`, `docs/`, `book/`, `agents/`, `prompts/`, `internal/exec/k8s_install.yaml` (staff scope).

## Your scope

| Surface | Action |
|---|---|
| `.github/workflows/ci.yml` | Add `security-audit` job: gosec + govulncheck + secret-scan (gitleaks or trufflehog). Runs on every PR; reports findings |
| `.github/workflows/release.yml` | Verify on tag push: goreleaser builds 6 archives, generates checksums, attaches book PDF, publishes to GitHub Releases. Validate the workflow logic spec-only (don't trigger) |
| Book PDF | Verify `make book-pdf` (or equivalent) produces a PDF with Mermaid diagrams pre-baked; ship via release.yml |
| `cspell` final sweep | Zero findings on all `book/src/**/*.md` + `docs/**/*.md` + new chapters from Sprint 5 |
| `.github/workflows/e2e-full.yml` | Final stub gate update — references PRD 07 spike + the AWS phases that are now implemented |
| CI matrix end-state doc | If a README section or similar documents the CI matrix, refresh it |

## Tasks (priority order)

1. `security-audit` CI job
2. release.yml validation (spec-only)
3. Book PDF build verification
4. Final cspell sweep
5. e2e-full.yml final refresh

## Issue tracking

`issues/issue_sprint6_validator.md`.

## Verification

- Every `.github/workflows/*.yml` parses as valid YAML
- `actionlint` passes (if installed)
- cspell zero findings
- Book PDF: if generator exists, runs to completion (else note + file issue)

## Final report

Under 200 words. Do NOT commit. Repo at `/Users/j.lucia/Code/github/awsbnkctl/`.
