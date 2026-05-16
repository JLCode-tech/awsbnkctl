# Sprint 3

**Theme:** Port reusable modules + first end-to-end `awsbnkctl up`

_Drafted from `docs/PLAN.md` Sprint 3 section._

Sprint 3 ports the four "reusable" Terraform modules inherited from roksbnkctl — `cert_manager`, `flo`, `cne_instance`, `license`, `testing` — to consume AWS-shaped inputs from PRDs 07 + 08. Wires the top-level `terraform/main.tf` so `awsbnkctl up` (no subcommand) drives the full lifecycle end-to-end: EKS cluster → cert-manager → FLO → CNEInstance → license → testing. Plus retires the IBM-named workspace alias (`Workspace.IBMCloud`) now that PRD 04's cred/exec retarget can land alongside.

End-of-sprint gate: `terraform validate` succeeds on root + all ported modules; `awsbnkctl up --dry-run` plans the full end-to-end resource graph without panicking; offline build green (`go build / test / vet`).

**SPIKE DEFERRAL** carries — no `terraform apply` against live AWS. `v0.2` still gates on operator-run spike per PRD 07.

Carry-overs from Sprint 2:
1. **tech-writer Issue 4 (HIGH)** — Sprint 1 doctor visibility (AWS rows invisible on stock dev box, gated behind workspace nil-check). Sprint 3 architect resolves by updating the inherited `TestRunWithWhy_StockDevBox_NoWorkspace` contract to accept the AWS credentials warning row; staff applies the gate relaxation.
2. **staff Issue 1** — `legacy_helpers.go` + `doctor_backend.go` deep retirement. Sprint 3 staff retargets cred/exec per PRD 04 (the deferred piece from Sprint 2's back-compat alias).
3. **tech-writer Issue 2 (medium)** — bucket versioning claim drift (PRD 08 says "off by default", module forces enabled). Sprint 3 architect updates PRD 08.
4. **tech-writer Issue 8** — chapter 25 cross-references chapter 26 troubleshooting (still stub). Sprint 3 architect drafts chapter 26 first-pass.

Four-agent dispatch:

1. **architect** — updates the inherited `TestRunWithWhy_StockDevBox_NoWorkspace` contract (or files implementation issue for staff); PRD 08 corrections; chapter 26 first-pass (troubleshooting); cross-link Sprint 2 chapter 25 → chapter 26; updates PLAN.md Sprint 3 close.
2. **staff** — ports `cert_manager`, `flo`, `cne_instance`, `license`, `testing` modules (variable rename: `ibmcloud_*` → `aws_*`, `roks_cluster_*` → `eks_cluster_*`, `cos_*` → `s3_*`, `trusted_profile_*` → `irsa_role_*`); top-level `terraform/main.tf` rewire; full `awsbnkctl up --dry-run` lifecycle; PRD 04 cred/exec retarget (IBMCLOUD_API_KEY → AWS_PROFILE/AWS_ACCESS_KEY_ID); retires `legacy_helpers.go` + `doctor_backend.go`; doctor visibility gate relaxation.
3. **validator** — extends CI matrix for the full-up dry-run integration; updates `scripts/e2e-test.sh` per-phase markers (cluster phases now Sprint 3 enabled at dry-run level); cspell additions for any new module-internal terminology.
4. **tech-writer** — read-only at sprint close.

The integrator commits. Sprint 3 produces the first end-to-end `up --dry-run` plan; live `apply` still gates on operator-run spike.
