#!/usr/bin/env bash
# hack/ci-local.sh — Mirror all GitHub Actions CI gates locally before pushing.
#
# Runs the exact checks from .github/workflows/ci.yml in fail-fast order:
#   1. gofmt (clean check)
#   2. go vet ./...
#   3. staticcheck ./...
#   4. go test -race ./...
#   5. scripts/lint-shell.sh
#   6. make build
#   7. up --dry-run (example configs)
#   8. test --dry-run
#   9. cspell (docs + README + CONTRIBUTING)
#  10. gosec ./...
#  11. govulncheck + gitleaks
#
# Usage:
#   ./hack/ci-local.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
RESET='\033[0m'

PASS_COUNT=0
SKIP_COUNT=0

CREATED_PLACEHOLDERS=()
cleanup() {
  for f in "${CREATED_PLACEHOLDERS[@]:-}"; do
    [[ -n "$f" && -e "$f" ]] && rm -f "$f"
  done
  rm -rf "$REPO_ROOT/.awsbnkctl" "$REPO_ROOT/artifacts"
}
trap cleanup EXIT

step() {
  echo -e "\n${BOLD}==> [$1] $2${RESET}"
}

pass() {
  echo -e "${GREEN}✔ PASS:${RESET} $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

skip() {
  echo -e "${YELLOW}↷ SKIP:${RESET} $1"
  SKIP_COUNT=$((SKIP_COUNT + 1))
}

fail() {
  echo -e "${RED}✘ FAIL:${RESET} $1" >&2
  exit 1
}

ensure_example_placeholders() {
  local dir
  dir="$(dirname "$1")"
  if [[ ! -f "$dir/cne_pull_64.json" ]]; then
    printf '{"auths":{}}\n' > "$dir/cne_pull_64.json"
    CREATED_PLACEHOLDERS+=("$dir/cne_pull_64.json")
  fi
  if [[ ! -f "$dir/license.jwt" ]]; then
    printf 'eyJhbGciOiJub25lIn0.eyJzdWIiOiJjaSJ9.\n' > "$dir/license.jwt"
    CREATED_PLACEHOLDERS+=("$dir/license.jwt")
  fi
}

# 1. gofmt
step "1/11" "gofmt clean check"
UNFORMATTED="$(gofmt -l .)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "$UNFORMATTED"
  fail "gofmt found unformatted Go files (run 'gofmt -w .')"
fi
pass "gofmt"

# 2. go vet
step "2/11" "go vet ./..."
go vet ./...
pass "go vet"

# 3. staticcheck
step "3/11" "staticcheck ./..."
if command -v staticcheck >/dev/null 2>&1; then
  staticcheck ./...
  pass "staticcheck"
elif go tool staticcheck -version >/dev/null 2>&1; then
  go tool staticcheck ./...
  pass "staticcheck (via go tool)"
else
  skip "staticcheck not installed (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"
fi

# 4. go test -race
step "4/11" "go test -race ./..."
go test -race ./...
pass "go test -race"

# 5. lint-shell
step "5/11" "scripts/lint-shell.sh"
if [[ -x ./scripts/lint-shell.sh ]]; then
  ./scripts/lint-shell.sh
  pass "scripts/lint-shell.sh"
else
  fail "scripts/lint-shell.sh is missing or not executable"
fi

# 6. Build binary
step "6/11" "make build"
make build
[[ -x ./bin/awsbnkctl ]] || fail "./bin/awsbnkctl not produced by make build"
pass "make build (./bin/awsbnkctl)"

# 7. up --dry-run across example configs
step "7/11" "awsbnkctl up --dry-run (example configs)"
CONFIGS=(
  "cluster.yaml"
  "examples/full-cluster/cluster.yaml"
  "examples/dual-interface/cluster.yaml"
  "examples/host-device/cluster.yaml"
  "internal/intent/testdata/external-only/cluster.yaml"
  "internal/intent/testdata/sriov-external/cluster.yaml"
)
RAN_CONFIGS=0
for cfg in "${CONFIGS[@]}"; do
  if [[ -f "$cfg" ]]; then
    ensure_example_placeholders "$cfg"
    echo "  -> AWSBNKCTL_SKIP_AUTH=1 ./bin/awsbnkctl up -f $cfg --dry-run"
    AWSBNKCTL_SKIP_AUTH=1 ./bin/awsbnkctl up -f "$cfg" --dry-run >/dev/null
    RAN_CONFIGS=$((RAN_CONFIGS + 1))
  fi
done
[[ "$RAN_CONFIGS" -gt 0 ]] || fail "no example cluster.yaml configs found for up --dry-run"
pass "up --dry-run ($RAN_CONFIGS configs)"

# 8. test --dry-run
step "8/11" "awsbnkctl test --dry-run"
./bin/awsbnkctl test --dry-run >/dev/null
pass "test --dry-run"

# 9. cspell
step "9/11" "cspell (docs/**/*.md README.md CONTRIBUTING.md)"
if command -v cspell >/dev/null 2>&1; then
  cspell --config cspell.json "docs/**/*.md" README.md CONTRIBUTING.md
  pass "cspell"
elif command -v npx >/dev/null 2>&1; then
  npx --yes cspell@8 --config cspell.json "docs/**/*.md" README.md CONTRIBUTING.md
  pass "cspell (via npx)"
else
  skip "cspell / npx not installed"
fi

# 10. gosec
step "10/11" "gosec ./..."
if command -v gosec >/dev/null 2>&1; then
  gosec -quiet ./...
  pass "gosec"
else
  skip "gosec not installed (install: go install github.com/securego/gosec/v2/cmd/gosec@latest)"
fi

# 11. govulncheck + gitleaks
step "11/11" "govulncheck + gitleaks"
if command -v govulncheck >/dev/null 2>&1; then
  if [[ -x ./scripts/govulncheck-gate.sh ]]; then
    ./scripts/govulncheck-gate.sh ./...
  else
    govulncheck ./...
  fi
  pass "govulncheck"
else
  skip "govulncheck not installed (install: go install golang.org/x/vuln/cmd/govulncheck@latest)"
fi

if command -v gitleaks >/dev/null 2>&1; then
  gitleaks detect --source . --no-banner --redact
  pass "gitleaks"
else
  skip "gitleaks not installed"
fi

echo -e "\n${GREEN}${BOLD}CI local gate complete:${RESET} ${PASS_COUNT} passed, ${SKIP_COUNT} skipped."
