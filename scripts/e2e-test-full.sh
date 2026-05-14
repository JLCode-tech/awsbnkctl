#!/usr/bin/env bash
# scripts/e2e-test-full.sh — combined A-H + I-N + L-DNS e2e runner.
#
# ▶ Sprint 0 status: skip-stub. The inherited wrapper chained the IBM-
#   Cloud-shaped baseline driver (`e2e-test.sh`) and backends driver
#   (`e2e-test-backends.sh`). Both are skip-stubs in Sprint 0; this
#   wrapper preserves its invocation surface (--teardown / --no-teardown
#   flags, env vars, PHASE_FROM semantics) so .github/workflows/e2e-full.yml
#   and the integrator's babysit loop keep parsing it. It just echoes the
#   skip-banner and exits 0 instead of dispatching to the IBM-shaped
#   drivers.
#
#   Sprint 6 owns the full-e2e rehydration; the cluster-bring-up retarget
#   lands in Sprint 3 and the backend-matrix retarget lands in Sprint 4.

set -e
set -u
set -o pipefail

# ── config ──────────────────────────────────────────────────────────
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
WORKSPACE=${WORKSPACE:-e2e}
PHASE_FROM=${PHASE_FROM:-A}
DRY_RUN=${DRY_RUN:-0}
LOG_DIR=${LOG_DIR:-/tmp/awsbnkctl-e2e-full}
AWSBNKCTL=${AWSBNKCTL:-awsbnkctl}

mkdir -p "$LOG_DIR"
RUN_TS=$(date +%Y%m%d-%H%M%S)
RUN_LOG="$LOG_DIR/run-$RUN_TS.log"

# ── helpers ─────────────────────────────────────────────────────────
yellow() { printf '\033[33m%s\033[0m\n' "$*" >&2; }
bold()   { printf '\033[1m%s\033[0m\n'  "$*" >&2; }

# ── flag parsing (preserved for compatibility) ──────────────────────
TEARDOWN_ON_SUCCESS=1
for arg in "$@"; do
    case "$arg" in
        --teardown)    TEARDOWN_ON_SUCCESS=1 ;;
        --no-teardown) TEARDOWN_ON_SUCCESS=0 ;;
        *)             ;;
    esac
done

# ── main ────────────────────────────────────────────────────────────
bold "awsbnkctl full E2E — run-id $RUN_TS"
echo "[$(date +%H:%M:%S)] log: $RUN_LOG" | tee -a "$RUN_LOG" >&2
echo "" >&2
yellow "════════════════════════════════════════════════════════════"
yellow "Sprint 0 stub: full e2e is skipped pending sprint retargets."
yellow "  Baseline driver (A-H):     Sprint 3 retargets cluster-bring-up"
yellow "  Backends driver (I-N):     Sprint 4 retargets backend matrix"
yellow "  L-DNS phase:               Sprint 4"
yellow "  Full v1.0 sign-off:        Sprint 6"
yellow ""
yellow "Invocation surface preserved: --teardown / --no-teardown flags"
yellow "still parse, PHASE_FROM=<letter> still parses, DRY_RUN=1 still"
yellow "parses — they just don't drive anything yet."
yellow ""
yellow "See docs/PLAN.md for the retarget plan."
yellow "════════════════════════════════════════════════════════════"

exit 0
