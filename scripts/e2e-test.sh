#!/usr/bin/env bash
# scripts/e2e-test.sh — end-to-end shake-out driver for awsbnkctl.
#
# ▶ Sprint 1 status: still a skip-stub for every phase. The Sprint 0
#   IBM-strip emptied every phase body; Sprint 1 has landed the EKS
#   cluster Terraform module + internal/aws/ helpers + cobra verbs,
#   but the end-to-end wire-up (a single script driving `awsbnkctl up
#   cluster` through to BNK deploy) lands later — see retarget calendar
#   below. The cluster-phase Terraform module is exercised against
#   mocked aws-sdk-go-v2 clients in the unit-test suite this sprint
#   instead. Live-AWS validation is operator-run via the Sprint 1 spike
#   (PRD 07 §"Spike protocol", gating v0.2).
#
#     Phases A-H (cluster bring-up)  →  Sprint 1 implements module;
#                                       Sprint 3 wires end-to-end
#     Phases I-N (backend matrix)    →  Sprint 4
#     Phase L-DNS                    →  Sprint 4
#     v1.0 sign-off run              →  Sprint 6
#
#   The script preserves the inherited public surface (env vars,
#   --dry-run flag via DRY_RUN=1, exit code) so downstream consumers
#   (scripts/e2e-test-full.sh, .github/workflows/e2e-full.yml, the
#   integrator's babysit loop) keep working — they just see "all phases
#   skipped" instead of a real run.
#
#   Sprint 1 adds a `--spike-mode` flag (stub-only this sprint) so the
#   operator running PRD 07's day-1 / day-2 / day-3 spike against live
#   AWS has a single entry-point to opt into the spike protocol once
#   Sprint 3 wires it. With `--spike-mode` set, the script will emit
#   the PRD 07 §4 protocol as inline guidance (this sprint) or dispatch
#   to it (Sprint 3+).
#
# Usage (when Sprint 3+ rehydrates this driver):
#   AWS_PROFILE=... ./scripts/e2e-test.sh
#   AWS_PROFILE=... PHASE_FROM=D ./scripts/e2e-test.sh
#   AWS_PROFILE=... DRY_RUN=1 ./scripts/e2e-test.sh
#   AWS_PROFILE=... ./scripts/e2e-test.sh --spike-mode   # PRD 07 spike

set -e
set -u
set -o pipefail

# ── config ──────────────────────────────────────────────────────────
WORKSPACE=${WORKSPACE:-e2e}
PHASE_FROM=${PHASE_FROM:-A}
DRY_RUN=${DRY_RUN:-0}
SPIKE_MODE=0
LOG_DIR=${LOG_DIR:-/tmp/awsbnkctl-e2e}
AWSBNKCTL=${AWSBNKCTL:-awsbnkctl}

# ── flag parsing ────────────────────────────────────────────────────
# --spike-mode opts the operator into PRD 07's spike protocol. Stub
# this sprint: the body just emits the protocol text. Sprint 3 wires
# the actual cluster-only bring-down to the matching phases.
for arg in "$@"; do
    case "$arg" in
        --spike-mode) SPIKE_MODE=1 ;;
        *)            ;;
    esac
done

mkdir -p "$LOG_DIR"
RUN_TS=$(date +%Y%m%d-%H%M%S)
RUN_LOG="$LOG_DIR/run-$RUN_TS.log"

# ── helpers ─────────────────────────────────────────────────────────
red()    { printf '\033[31m%s\033[0m\n' "$*" >&2; }
green()  { printf '\033[32m%s\033[0m\n' "$*" >&2; }
yellow() { printf '\033[33m%s\033[0m\n' "$*" >&2; }
bold()   { printf '\033[1m%s\033[0m\n'  "$*" >&2; }

log()    { echo "[$(date +%H:%M:%S)] $*" | tee -a "$RUN_LOG" >&2; }

phase_header() {
    echo "" >&2
    bold "════════════════════════════════════════════════════════════"
    bold "Phase $1 — $2"
    bold "════════════════════════════════════════════════════════════"
}

# skip_phase emits a uniform "skipped pending Sprint N retarget" banner
# and returns 0 so the driver keeps walking forward through remaining
# phase stubs. Pass: phase letter, original description, retarget sprint.
skip_phase() {
    local letter="$1"
    local desc="$2"
    local sprint="$3"
    phase_header "$letter" "$desc"
    yellow "  ⊘ Phase $letter skipped — pending $sprint retarget (see docs/PLAN.md)"
}

# ── phases ──────────────────────────────────────────────────────────
#
# Each phase below is a skip-stub. The descriptions preserve the
# original IBM-shaped intent so the Sprint-3/4 author has a brief
# pointer to the canonical phase contract — see git history (and the
# upstream `jgruberf5/roksbnkctl` repo's `scripts/e2e-test.sh`) for the
# full inherited shape.
#
# Sprint 1 skip-marker refinement: phases A-H (cluster bring-up) now
# cite "Sprint 1 implements module; Sprint 3 wires end-to-end" so the
# operator running the spike has the right pointer; backend / DNS
# phases remain Sprint 4. The phase bodies themselves are unchanged —
# all still return 0 after emitting the skip banner.

phase_A() { skip_phase A "sanity (version + doctor + init + tfvars)"     "Sprint 1 module + Sprint 3 wire-up"; }
phase_B() { skip_phase B "cluster up + show + kubectl get nodes"         "Sprint 1 module + Sprint 3 wire-up"; }
phase_C() { skip_phase C "register an existing cluster + down"           "Sprint 1 module + Sprint 3 wire-up"; }
phase_D() { skip_phase D "full lifecycle: cluster + BNK + test verbs"    "Sprint 1 module + Sprint 3 wire-up"; }
phase_E() { skip_phase E "workspace ops (during D's idle window)"        "Sprint 1 module + Sprint 3 wire-up"; }
phase_F() { skip_phase F "S3 object CRUD (replaces COS in Sprint 2)"     "Sprint 1 module + Sprint 3 wire-up"; }
phase_G() { skip_phase G "passthrough commands (aws / kubectl / exec)"   "Sprint 1 module + Sprint 3 wire-up"; }
phase_H() { skip_phase H "final cleanup (workspace teardown)"            "Sprint 1 module + Sprint 3 wire-up"; }
phase_I() { skip_phase I "backend matrix — local execution backend"     "Sprint 4"; }
phase_J() { skip_phase J "backend matrix — docker execution backend"    "Sprint 4"; }
phase_K() { skip_phase K "backend matrix — multi-tool docker phase"     "Sprint 4"; }
phase_L() { skip_phase L "backend matrix — k8s execution backend"       "Sprint 4"; }
phase_M() { skip_phase M "backend matrix — ssh execution backend"       "Sprint 4"; }
phase_N() { skip_phase N "backend matrix — mixed-mode integration"      "Sprint 4"; }
phase_L_DNS() { skip_phase L-DNS "AWS-hosted GSLB DNS probe"            "Sprint 4"; }

# spike_mode_banner emits the PRD 07 §4 spike protocol as inline text
# when --spike-mode is set. Sprint 1: stub-only — the operator follows
# the protocol by hand against live AWS. Sprint 3: wires the same flag
# to actual cluster-only verbs (`awsbnkctl up cluster --spike` etc.).
spike_mode_banner() {
    echo "" >&2
    bold "════════════════════════════════════════════════════════════"
    bold "Spike mode — PRD 07 §4 protocol"
    bold "════════════════════════════════════════════════════════════"
    yellow "  Spike protocol per PRD 07 §4 (Sprint 1, days 1-3):"
    yellow "    • Day 1: provision EKS 1.30 + self-managed c5n.4xlarge node group"
    yellow "    • Day 2: install Multus + SR-IOV CNI + device plugin DaemonSets"
    yellow "    • Day 3: schedule a pod requesting intel.com/sriov:1 — verify"
    yellow "             VF surfaces in the pod and BNK CNEInstance accepts it"
    yellow ""
    yellow "  Sprint 1 stub: this flag emits the protocol pointer only."
    yellow "  Sprint 3 wires --spike-mode to actual cluster-only verbs."
    yellow "  See docs/prd/07-EKS-CLUSTER-SRIOV.md §'Spike protocol'."
    echo "" >&2
}

# should_run compares the current phase letter against PHASE_FROM so
# resume-at-phase semantics (PHASE_FROM=D) keep working even while every
# phase is a skip-stub.
should_run() {
    [[ "$1" > "$PHASE_FROM" || "$1" == "$PHASE_FROM" ]]
}

# ── main ────────────────────────────────────────────────────────────
main() {
    bold "awsbnkctl E2E test — run-id $RUN_TS"
    log "log: $RUN_LOG"
    log "Sprint 1 stub: every phase is a skip-marker pending Sprint 3 wire-up."

    if [[ "$SPIKE_MODE" == "1" ]]; then
        spike_mode_banner
    fi

    should_run A     && phase_A
    should_run B     && phase_B
    should_run C     && phase_C
    should_run D     && phase_D
    should_run E     && phase_E
    should_run F     && phase_F
    should_run G     && phase_G
    should_run H     && phase_H
    should_run I     && phase_I
    should_run J     && phase_J
    should_run K     && phase_K
    should_run L     && phase_L
    should_run M     && phase_M
    should_run N     && phase_N
    # L-DNS is a sub-phase of L in the inherited driver; keep it
    # adjacent so the skip banner mirrors the canonical sequence.
    should_run L     && phase_L_DNS

    echo "" >&2
    yellow "════════════════════════════════════════════════════════════"
    yellow "Sprint 1 stub: cluster phases A-H pending Sprint 3 wire-up."
    yellow "Backend phases I-N + L-DNS pending Sprint 4."
    yellow "(see docs/PLAN.md for the retarget plan; PRD 07 §4 for the spike.)"
    yellow "════════════════════════════════════════════════════════════"
}

main "$@"
