#!/usr/bin/env bash
# scripts/e2e-test-backends.sh — backend-matrix end-to-end driver.
#
# ▶ Sprint 4 status: the AWS-flavoured test verbs (connectivity, dns,
#   throughput) and the K8s + SSH execution backends have landed in
#   dry-run / mocked form — the offline regression surface is exercised
#   by the `test-dryrun` job in `.github/workflows/ci.yml`. The
#   live-AWS exercise of these phases still gates on the operator-run
#   PRD 07 spike (SPIKE DEFERRAL carries — no live AWS resources in
#   the agent dispatch lane).
#
#     Phase I     (SSH backend)              → Sprint 4 dry-run + spike for apply
#     Phase K     (Docker backend)           → Sprint 4 dry-run + spike for apply
#     Phase L     (K8s backend / iperf3)     → Sprint 4 dry-run + spike for apply
#     Phase L-DNS (DNS probe + GSLB compare) → Sprint 4 dry-run + spike for apply
#     Phase M     (cred-leak audit)          → Sprint 4 implements; spike validates
#     Phase N     (mixed-mode lifecycle)     → Sprint 4 implements; spike validates
#
#   Invocation surface preserved (PHASE_FROM, DRY_RUN, env vars) so
#   downstream consumers (scripts/e2e-test-full.sh, the babysit loop)
#   keep parsing the script. Phase bodies still emit skip banners —
#   the markers below point operators at the right Sprint 4 artefact
#   (the CI `test-dryrun` job or the PRD 07 spike) for each surface.

set -e
set -u
set -o pipefail

# ── config ──────────────────────────────────────────────────────────
WORKSPACE=${WORKSPACE:-e2e}
PHASE_FROM=${PHASE_FROM:-I}
DRY_RUN=${DRY_RUN:-0}
LOG_DIR=${LOG_DIR:-/tmp/awsbnkctl-e2e-backends}
AWSBNKCTL=${AWSBNKCTL:-awsbnkctl}

mkdir -p "$LOG_DIR"
RUN_TS=$(date +%Y%m%d-%H%M%S)
RUN_LOG="$LOG_DIR/run-$RUN_TS.log"

# ── helpers ─────────────────────────────────────────────────────────
yellow() { printf '\033[33m%s\033[0m\n' "$*" >&2; }
bold()   { printf '\033[1m%s\033[0m\n'  "$*" >&2; }

phase_header() {
    echo "" >&2
    bold "════════════════════════════════════════════════════════════"
    bold "Phase $1 — $2"
    bold "════════════════════════════════════════════════════════════"
}

skip_phase() {
    local letter="$1"
    local desc="$2"
    local marker="$3"
    phase_header "$letter" "$desc"
    # Auto-suffix " retarget" only on the bare "Sprint N" style markers
    # used by Sprint 0-3 callers; Sprint 4 callers pass a fully-formed
    # sentence so the skip banner reads naturally without the suffix.
    # Mirrors the helper in scripts/e2e-test.sh for consistency.
    if [[ "$marker" =~ ^Sprint\ [0-9]+$ ]]; then
        marker="pending $marker retarget"
    fi
    yellow "  ⊘ Phase $letter skipped — $marker (see docs/PLAN.md)"
}

should_run() {
    [[ "$1" > "$PHASE_FROM" || "$1" == "$PHASE_FROM" ]]
}

# ── phases (Sprint 4 marker refresh — dry-run tier covered by CI's
#   test-dryrun job, live tier gates on the operator-run PRD 07 spike) ─
phase_I()     { skip_phase I     "SSH backend (awsbnkctl --backend ssh)"            "Sprint 4 implements dry-run; live apply gates on PRD 07 spike"; }
phase_K()     { skip_phase K     "Docker backend (awsbnkctl --backend docker)"      "Sprint 4 implements dry-run; live apply gates on PRD 07 spike"; }
phase_L()     { skip_phase L     "K8s backend (iperf3 + ops pod via --backend k8s)" "Sprint 4 implements dry-run; live apply gates on PRD 07 spike"; }
phase_L_DNS() { skip_phase L-DNS "AWS Route 53 GSLB DNS probe + cross-vantage compare (miekg/dns)" "Sprint 4 implements dry-run; live apply gates on PRD 07 spike"; }
phase_M()     { skip_phase M     "cred-leak audit across all backends"              "Sprint 4 implements; live exercise in PRD 07 spike"; }
phase_N()     { skip_phase N     "mixed-mode lifecycle (backends share state)"      "Sprint 4 implements; live exercise in PRD 07 spike"; }

# ── main ────────────────────────────────────────────────────────────
main() {
    bold "awsbnkctl backends E2E — run-id $RUN_TS"
    echo "[$(date +%H:%M:%S)] log: $RUN_LOG" | tee -a "$RUN_LOG" >&2
    echo "[$(date +%H:%M:%S)] Sprint 4 status: backend + DNS phases at dry-run tier; live apply gates on PRD 07 spike." \
        | tee -a "$RUN_LOG" >&2

    should_run I     && phase_I
    should_run K     && phase_K
    should_run L     && phase_L
    should_run L     && phase_L_DNS
    should_run M     && phase_M
    should_run N     && phase_N

    echo "" >&2
    yellow "════════════════════════════════════════════════════════════"
    yellow "Sprint 4 status: backend matrix (I, K, L) + L-DNS + audit (M, N)"
    yellow "  at dry-run / mocked tier — see CI test-dryrun job."
    yellow "Live-apply tier still gates on the operator-run PRD 07 spike"
    yellow "  (docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\")."
    yellow "(see docs/PLAN.md § Sprint 4 for the retarget plan)."
    yellow "════════════════════════════════════════════════════════════"
}

main "$@"
