#!/usr/bin/env bash
# scripts/test-integration-aws.sh — convenience wrapper for the
# `-tags integration` test pass over `internal/aws/...` plus the
# offline `awsbnkctl up/down --dry-run` regression gates.
#
# `internal/aws/...` ships with companion test files that exercise the
# helpers against mocked aws-sdk-go-v2 clients (no live AWS). The wildcard
# `./internal/aws/...` picks all of them up — no per-file invocation needed.
#
# The dry-run gate mirrors the `up-dryrun` job in `.github/workflows/ci.yml`:
# it builds the binary, validates the example configs, then runs `up` and
# `down --dry-run` against the three public example clusters. No live AWS
# credentials are used; AWSBNKCTL_SKIP_AUTH=1 bypasses SSO and the fake creds
# below let the SDK construct a signer without panicking.
#
# Toggle the end-to-end gate with FULL_UP_DRYRUN=1 (default); set
# FULL_UP_DRYRUN=0 to run only the per-package suite, useful when iterating
# on a single internal/aws helper.
#
# Extra args (after the script name) are forwarded to `go test`, not
# to the dry-run gate — keep the contract narrow.
#
# Live-AWS validation is a separate operator-run path (spike protocol);
# this script is mocked-only and never touches a real AWS endpoint.

set -euo pipefail

# Move to repo root so relative paths resolve regardless of where the caller
# invokes from.
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

# Fake creds — only ever consumed by SDK signer construction. If a code path
# leaks past the skip-auth gate and reaches AWS, the 403 from these fake creds
# is the failure signal.
export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-testing}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-testing}
export AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN:-testing}
export AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-us-east-1}
export AWS_REGION=${AWS_REGION:-us-east-1}

# Disable IMDS lookup — without this the SDK's default credential chain will
# block for ~5 s probing the IMDS endpoint on a developer laptop.
export AWS_EC2_METADATA_DISABLED=true

# Skip AWS authentication in dry-run mode.
export AWSBNKCTL_SKIP_AUTH=1

FULL_UP_DRYRUN=${FULL_UP_DRYRUN:-1}

# ── per-package integration suite ───────────────────────────────────
# -v keeps per-test output visible; -timeout 3m matches the CI budget.
# Extra args (`-run`, `-count`, `-race`) pass through.
echo "→ go test -tags integration ./internal/aws/..." >&2
go test -tags integration -timeout 3m -v ./internal/aws/... "$@"

# ── full-up dry-run gate ─────────────────────────────────────────────
# Mirrors the `up-dryrun` job in CI: validate example configs and run
# up/down --dry-run for each public example cluster.
if [[ "$FULL_UP_DRYRUN" != "1" ]]; then
    echo "→ full-up-dryrun: skipped (FULL_UP_DRYRUN=$FULL_UP_DRYRUN)" >&2
    exit 0
fi

echo "→ full-up-dryrun: go build -o bin/awsbnkctl ./cmd/awsbnkctl" >&2
mkdir -p bin
go build -o bin/awsbnkctl ./cmd/awsbnkctl

# Keep dry-run logs in a temp dir so the script doesn't leave untracked files in
# the repo root (the CI job uses artifacts/ for upload, but this local wrapper
# cleans up after itself).
artifacts_dir=$(mktemp -d "${TMPDIR:-/tmp}/awsbnkctl-test-integration-aws.XXXXXX")
trap 'rm -rf "$artifacts_dir"' EXIT

# Example configs that include a bnk: block reference F5 credential files
# relative to the cluster.yaml directory. Create inert placeholders so the
# dry-run path can stat them without leaking real secrets into git.
# These filenames are gitignored; the script writes them fresh every run.
for example_dir in \
    examples/full-cluster \
    examples/external-only \
    internal/intent/testdata/sriov-external
do
    echo '{"auths":{}}' > "$example_dir/cne_pull_64.json"
    echo 'dummy-license-jwt' > "$example_dir/license.jwt"
done

# Validate example configs.
echo "→ full-up-dryrun: validate example configs" >&2
./bin/awsbnkctl validate examples/full-cluster/cluster.yaml
./bin/awsbnkctl validate examples/external-only/cluster.yaml
./bin/awsbnkctl validate internal/intent/testdata/sriov-external/cluster.yaml

run_up_dryrun() {
    local config_path=$1
    local log_name=$2
    local log_path="$artifacts_dir/$log_name"
    echo "→ full-up-dryrun: ./bin/awsbnkctl up -f $config_path --dry-run" >&2
    if ! ./bin/awsbnkctl up -f "$config_path" --dry-run \
            2>&1 | tee "$log_path"; then
        echo "✗ full-up-dryrun: up -f $config_path --dry-run exited non-zero" >&2
        echo "  log: $log_path" >&2
        return 1
    fi
    if ! grep -q 'postflight' "$log_path"; then
        echo "✗ full-up-dryrun: 'postflight' not found in $log_name" >&2
        return 1
    fi
    if ! grep -q 'dry-run complete' "$log_path"; then
        echo "✗ full-up-dryrun: 'dry-run complete' not found in $log_name" >&2
        return 1
    fi
    echo "  ✓ $log_name" >&2
}

run_down_dryrun() {
    local config_path=$1
    local log_name=$2
    local log_path="$artifacts_dir/$log_name"
    echo "→ full-up-dryrun: ./bin/awsbnkctl down -f $config_path --dry-run --yes" >&2
    if ! ./bin/awsbnkctl down -f "$config_path" --dry-run --yes \
            2>&1 | tee "$log_path"; then
        echo "✗ full-up-dryrun: down -f $config_path --dry-run exited non-zero" >&2
        echo "  log: $log_path" >&2
        return 1
    fi
    if ! grep -q 'dry-run complete' "$log_path"; then
        echo "✗ full-up-dryrun: 'dry-run complete' not found in $log_name" >&2
        return 1
    fi
    echo "  ✓ $log_name" >&2
}

fail=0
run_up_dryrun   examples/full-cluster/cluster.yaml     up-dryrun-full-cluster.log     || fail=1
run_down_dryrun examples/full-cluster/cluster.yaml     down-dryrun-full-cluster.log   || fail=1
run_up_dryrun   examples/external-only/cluster.yaml    up-dryrun-external-only.log    || fail=1
run_down_dryrun examples/external-only/cluster.yaml    down-dryrun-external-only.log  || fail=1
run_up_dryrun   internal/intent/testdata/sriov-external/cluster.yaml   up-dryrun-sriov-external.log   || fail=1
run_down_dryrun internal/intent/testdata/sriov-external/cluster.yaml   down-dryrun-sriov-external.log || fail=1

if [[ "$fail" -ne 0 ]]; then
    echo "✗ full-up-dryrun: one or more dry-run gates failed" >&2
    exit 1
fi

echo "✓ full-up-dryrun: all dry-run gates passed" >&2
