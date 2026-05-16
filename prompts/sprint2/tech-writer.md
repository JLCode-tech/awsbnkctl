You are the tech writer agent for Sprint 2 of the `awsbnkctl` project. Sprint 2's theme is "S3 supply chain + IRSA (PRD 08)". You run after the three sibling agents — read-only review + dogfooding gate.

**Do NOT edit project files.** Only output is `issues/issue_sprint2_tech-writer.md`.

Project location: `/Users/j.lucia/Code/github/awsbnkctl/`.

**Read first** (in order):
1. `/Users/j.lucia/Code/github/awsbnkctl/agents/tech-writer.md`
2. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/tech-writer.md` (this file)
3. `/Users/j.lucia/Code/github/awsbnkctl/prompts/sprint2/README.md`
4. `/Users/j.lucia/Code/github/awsbnkctl/docs/prd/08-S3-SUPPLY-CHAIN-IRSA.md`
5. Sibling agents' Sprint 2 issue files: `issues/issue_sprint2_{architect,staff,validator}.md`.
6. The new prose (chapter 25) + new code surface (`internal/aws/{s3,iam}.go`, `terraform/modules/{s3_supply_chain,iam_irsa,ecr_mirror}/`).

## Your scope (read-only audits)

| Check | What to look for |
|---|---|
| **PRD 08 ↔ implementation alignment** | Module variables.tf + outputs.tf match PRD 08's tables exactly |
| **Chapter 25 narrative coherence** | Post-rewrite chapter reads as awsbnkctl's S3 supply chain story (not COS), cross-links resolve |
| **Workspace schema retarget** | `grep -r 'IBMCloud' internal/` should return ≤1 hit (deprecated alias if back-compat chosen) or zero (clean break) — confirm staff's choice matches what they documented |
| **Build dogfood** | `make build`; `./bin/awsbnkctl init --help` documents the AWS path; `./bin/awsbnkctl init --dry-run` runs the wizard offline without touching AWS; `./bin/awsbnkctl doctor` shows S3 + IRSA + vCPU rows when a workspace is configured |
| **Terraform dogfood** | `terraform -chdir=terraform/modules/s3_supply_chain init && validate`; same for `iam_irsa` and `ecr_mirror` if shipped |
| **Cross-link integrity** | PRD 08 + chapter 25 + README + CHANGELOG links all resolve |
| **Dockerfile multi-arch** | `docker buildx build --platform linux/amd64,linux/arm64 tools/docker/aws/` succeeds if buildx is available; else check the Dockerfile reads sanely |
| **cspell** | New IRSA / OIDC / KMS / skopeo / CMK / aarch64 terms land in cspell.json |
| **Sprint 1 carry-over closure** | Tech-writer Issue 1 (doctor visibility) — does Sprint 2's workspace retarget unblock the unconditional AWS-checks visibility? If yes, did staff also relax the gate? |

## Tasks

1. **PRD 08 ↔ Terraform reconciliation.** Open the PRD's Inputs/Outputs tables alongside the staff agent's `variables.tf` + `outputs.tf` for each new module. File one issue per material mismatch.

2. **Chapter 25 read-through.** As a first-time reader. Note sentences that confuse, references that don't resolve, claims unsupported by PRD 08.

3. **Build + init + dry-run dogfood.** Walk the verification checklist. Record actual command output (or one-line summary) per step.

4. **Workspace retarget audit.** `grep -r 'IBMCloud' internal/` — note count + locations. Match against staff's reported choice (back-compat alias vs clean break).

5. **Tools image multi-arch smoke.** If buildx available; else read the Dockerfile.

6. **Cross-link audit.** PRD 08, chapter 25, README, CHANGELOG.

7. **Sprint 1 carry-over verification.** Tech-writer Sprint 1 Issue 1 (doctor visibility): is it closed by Sprint 2's workspace retarget? Staff Sprint 1 Issue 2 (vCPU quota): is the new check wired and visible?

## Issue tracking

File to `/Users/j.lucia/Code/github/awsbnkctl/issues/issue_sprint2_tech-writer.md`. Schema same as Sprint 1.

## Verification before reporting done

- Actually ran `make build` + `init --dry-run` + `doctor` + `terraform validate`.
- Cross-referenced sibling issue files.
- Did NOT edit any project file.

## Final report

Under 200 words — files reviewed, build/dogfood results, issues filed (severity breakdown), PRD ↔ implementation verdict, workspace retarget verdict, Sprint 1 carry-overs closure verdict, ready-for-integrator verdict. Do NOT commit.
