#!/usr/bin/env bash
# scripts/e2e-test-backends.sh — backend-matrix end-to-end driver.
#
# ▶ Sprint 0 status: skip-stub. The inherited driver exercised the local /
#   docker / k8s / ssh execution backends + the GSLB-aware DNS probe
#   against the IBM-Cloud cluster from `e2e-test.sh` Phase D. Per
#   docs/PLAN.md Sprint 0, every phase is a header-echo + early-return
#   citing the sprint that retargets it (Sprint 4 owns the backend
#   matrix + L-DNS retarget; Sprint 6 closes the full sign-off).
#
#   Invocation surface preserved (PHASE_FROM, DRY_RUN, env vars) so
#   downstream consumers (scripts/e2e-test-full.sh, the babysit loop)
#   keep parsing the script.

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
    local sprint="$3"
    phase_header "$letter" "$desc"
    yellow "  ⊘ Phase $letter skipped — pending $sprint retarget (see docs/PLAN.md)"
}

should_run() {
    [[ "$1" > "$PHASE_FROM" || "$1" == "$PHASE_FROM" ]]
}

# ── phases (all stubbed) ────────────────────────────────────────────
phase_I()     { skip_phase I     "SSH backend (awsbnkctl --backend ssh)"          "Sprint 4"; }
phase_K()     { skip_phase K     "Docker backend (awsbnkctl --backend docker)"    "Sprint 4"; }
phase_L()     { skip_phase L     "K8s backend (iperf3 + ops pod via --backend k8s)" "Sprint 4"; }
phase_L_DNS() { skip_phase L-DNS "DNS probe + GSLB compare (miekg/dns)"           "Sprint 4"; }
phase_M()     { skip_phase M     "cred-leak audit across all backends"            "Sprint 4"; }
phase_N()     { skip_phase N     "mixed-mode lifecycle (backends share state)"    "Sprint 4"; }

# ── main ────────────────────────────────────────────────────────────
main() {
    bold "awsbnkctl backends E2E — run-id $RUN_TS"
    echo "[$(date +%H:%M:%S)] log: $RUN_LOG" | tee -a "$RUN_LOG" >&2
    echo "[$(date +%H:%M:%S)] Sprint 0 stub: every phase is a skip-marker pending Sprint 4 retarget." \
        | tee -a "$RUN_LOG" >&2

    should_run I     && phase_I
    should_run K     && phase_K
    should_run L     && phase_L
    should_run L     && phase_L_DNS
    should_run M     && phase_M
    should_run N     && phase_N

    echo "" >&2
    yellow "════════════════════════════════════════════════════════════"
    yellow "Sprint 0 stub: all backend phases skipped pending Sprint 4."
    yellow "(see docs/PLAN.md for the retarget plan)."
    yellow "════════════════════════════════════════════════════════════"
}

main "$@"
