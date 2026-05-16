# Sprint 6

**Theme:** Hardening + Sprint 5 blocker folds + v1.0 cut prep

_Drafted from `docs/PLAN.md` Sprint 6 section._

Sprint 6 is the final sprint. Closes the Sprint 5 tech-writer blockers (k8s ops-pod YAML retarget; cluster-subverb chapter prose), drafts the AWS-shaped E2E phases, runs the security audit, finalises goreleaser config, and prepares the v0.9 + v1.0-candidate release artefacts.

End-of-sprint gate: `awsbnkctl doctor` green-by-default on stock dev box; `gosec ./...` clean; secrets scan clean; goreleaser build succeeds across linux/macOS/windows × amd64/arm64; book PDF generates; `MIGRATING.md` is the final word for users coming from roksbnkctl or manual EKS+BNK.

**SPIKE DEFERRAL** carries — v0.2 still gates on operator-run PRD 07 spike. v1.0 candidate ships with binaries but the "officially supported on this account" tag waits on the spike. The Sprint 6 release is **structurally complete** — anyone with operator-run spike validation can cut v1.0 immediately.

Carry-overs from Sprint 5 (the blockers + highs):
1. **tech-writer Issue 1 (BLOCKER)** — `internal/exec/k8s_install.yaml` still entirely roksbnkctl-shaped ops-pod manifest. Sprint 6 staff retargets at IRSA-injected AWS-creds shape.
2. **tech-writer Issue 2 (BLOCKER)** — chapters 8/9/11 reference `cluster` / `bnk` subverbs that don't exist on the binary surface. Sprint 6 architect either rewrites prose OR annotates as v1.x roadmap (path b — explicit "Available in v1.x" notes on relevant sections).
3. **tech-writer Issue 3 (HIGH)** — glossary (chapter 30) factual errors. Sprint 6 architect cleanup.
4. **tech-writer Issue 4 (HIGH)** — chapters 17, 18, 19, 32 carry secondary IBM-residue prose. Sprint 6 architect sweep.
5. **tech-writer Issue 6 (medium)** — README sprint-count framing stale. Sprint 6 architect fixes.
6. **tech-writer Issue 7 (low)** — CHANGELOG missing Sprint 5 entry. (Now folded; the integrator added it pre-commit.)

Four-agent dispatch:

1. **architect** — closes Sprint 5 chapter blockers (path b for chapters 8/9/11 — "Available in v1.x" annotation); glossary cleanup; secondary IBM-residue sweep in chapters 17/18/19/32; README sprint-count refresh; final PLAN.md "What's deferred to post-v1.0" appendix; PLAN.md Sprint 6 close.
2. **staff** — retargets `internal/exec/k8s_install.yaml` (ops-pod manifest IBM → IRSA-injected AWS-creds shape); runs `gosec ./...` and folds findings; secrets scan; verifies goreleaser config produces 6 binary archives (linux/macOS/windows × amd64/arm64).
3. **validator** — full security audit workflow (`gosec`, `govulncheck`, secret-scan); release CI workflow validated; book PDF build verified; final cspell pass; CI matrix end-state documented.
4. **tech-writer** — read-only at sprint close: v1.0-readiness preview, full book read-through, release artefact spec-check, MIGRATING.md sanity check.

The integrator commits + cuts `v0.9-rc1` candidate tag (since v0.2 first-tag still gates on spike, the next available tag is v0.9 candidate; v1.0 candidate follows after spike validation).
