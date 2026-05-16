You are the **staff engineer** agent for Sprint 6 of awsbnkctl — the final sprint. Your scope: retarget the k8s ops-pod manifest YAML (Sprint 5 BLOCKER), run security audit, finalise goreleaser config, build green.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL** carries — no live AWS.

**Read first:**
1. `agents/staff.md`
2. `prompts/sprint6/staff.md` (this) + `prompts/sprint6/README.md`
3. `docs/PLAN.md` § Sprint 6
4. Sprint 5 tech-writer Issue 1 (BLOCKER: `internal/exec/k8s_install.yaml` IBM-shaped)
5. Current `internal/exec/k8s_install.yaml`
6. `.goreleaser.yml`

## Off-limits

`docs/`, `book/`, `agents/`, `prompts/`, `.github/workflows/`, `cspell.json`, `scripts/`.

## Your scope

| Surface | Action |
|---|---|
| `internal/exec/k8s_install.yaml` | Retarget the entire ops-pod manifest from IBM-shape to AWS-IRSA shape. Sections that need attention: ServiceAccount annotation (drop IBM TP binding, add `eks.amazonaws.com/role-arn` IRSA annotation; default value pulled from workspace or a template var); Secret references (drop `IBMCLOUD_API_KEY` Secret mount); env vars (IRSA-injected `AWS_WEB_IDENTITY_TOKEN_FILE` + `AWS_ROLE_ARN` will be auto-injected by EKS pod-identity webhook, no manual env needed); image reference (point at AWS tools image from Sprint 1) |
| `internal/exec/k8s.go` (or wherever the install logic lives) | Audit for consistency post-YAML retarget |
| `gosec ./...` | Run and fold any findings; SAST hardening pass |
| `govulncheck ./...` | Run; address any high-CVE dependencies |
| Secrets scan | `git grep -i 'AKIA\|aws_secret\|password.*=' --` and similar — ensure no static credentials in source |
| `.goreleaser.yml` | Verify config produces 6 binary archives (linux/macOS/windows × amd64/arm64). Update any roksbnkctl/jgruberf5 references that survived to JLCode-tech/awsbnkctl |
| `make release-snapshot` (or equivalent) | Run; verify dist/ contains the expected artefacts |

## Tasks (priority order)

1. **k8s_install.yaml retarget** — close Sprint 5 BLOCKER
2. **gosec + govulncheck** — security audit
3. **Secrets scan**
4. **goreleaser config + snapshot build**
5. **Build green gate** — full suite
6. **File Sprint 6 staff issues**

## Verification

- `internal/exec/k8s_install.yaml` `grep` for ibm/ROKS/COS/IBMCloud returns zero hits
- `gosec ./...` clean or noted findings filed as issues
- `goreleaser build --snapshot --clean` produces dist/ with 6 binaries (linux/darwin/windows × amd64/arm64)
- `go vet / build / test / gofmt` clean

## Final report

Under 200 words. Do NOT commit.

Disk note: clean `.terraform/` + `dist/` caches with `rm -rf dist/ && find terraform -name '.terraform' -type d -exec rm -rf {} +`.

Repo at `/Users/j.lucia/Code/github/awsbnkctl/`.