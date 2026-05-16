#!/usr/bin/env bash
# scripts/test-integration-aws.sh — convenience wrapper for the
# `-tags integration` test pass over `internal/aws/...`.
#
# Sprint 1 (PRD 07) introduces `internal/aws/{client,sts,ec2,eks,vpc}.go`
# with companion test files that exercise the helpers against mocked
# aws-sdk-go-v2 clients (no live AWS — see the staff agent's
# middleware-test wiring). This script sets the env vars the suite
# expects so a contributor can run the same matrix CI runs without
# remembering the incantation:
#
#   $ ./scripts/test-integration-aws.sh
#   $ ./scripts/test-integration-aws.sh -run TestIntegration_STS
#
# Live-AWS validation is a separate operator-run path (PRD 07 §4
# "Spike protocol"); this script is mocked-only and never touches a
# real AWS endpoint. The fake creds + `AWS_EC2_METADATA_DISABLED=true`
# below match `.github/workflows/ci.yml` job `aws-mocked`.

set -euo pipefail

# Move to repo root so the relative `./internal/aws/...` path resolves
# regardless of which subdirectory the caller invokes from.
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

# Fake creds — only ever consumed by SDK signer construction. If a
# test path leaks past its mock and hits AWS, the 403 from these fake
# creds is the test author's signal to add a mock.
export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-testing}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-testing}
export AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN:-testing}
export AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-us-east-1}
export AWS_REGION=${AWS_REGION:-us-east-1}

# Disable IMDS lookup — without this the SDK's default credential
# chain will block for ~5s probing the IMDS endpoint (169.254.169.254)
# on a developer laptop, slowing every test run.
export AWS_EC2_METADATA_DISABLED=true

# -v keeps the per-test output visible; -timeout 3m matches the CI
# job's budget. Extra args (`-run`, `-count`, `-race`) pass through.
exec go test -tags integration -timeout 3m -v ./internal/aws/... "$@"
