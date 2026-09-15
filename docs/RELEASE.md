# Release Guide

## Versioning

SemVer. Major (`vX.0.0`) for breaking CLI or `cluster.yaml` changes; minor (`vX.Y.0`) for new phases, scenarios, or supported BNK and Kubernetes versions; patch (`vX.Y.Z`) for fixes, security and dependency updates.

## Branches and commits

PRs target `staging`; `staging` is promoted to `main` by a PR. Feature PRs are squash-merged with a conventional title (`feat:`, `fix:`, `chore:`, `docs:`, `test:`): release-please reads the commit subjects on `main`, so a plain `Merge pull request #N` subject is invisible to it. A major bump needs `feat!:` or a `Release-As:` footer.

## Pipeline (`.github/workflows/release.yml`)

1. release-please keeps a release PR open against `main` with the version bump and `CHANGELOG.md`.
2. Merging it creates the `vX.Y.Z` tag and the GitHub release.
3. goreleaser builds the assets: Linux and macOS `amd64`/`arm64` as `.tar.gz`, Windows `amd64`/`arm64` as `.zip`, plus `checksums.txt` (seven assets). It runs with `--parallelism 2` and a 45-minute budget. No signing, no Homebrew tap.

Rebuild the assets for an existing tag:

```
gh workflow run release.yml -f tag=vX.Y.Z
```

## Before the tag

```
make release              # CHANGELOG stamp, staticcheck, integration-tag build, goreleaser check, snapshot
make goreleaser-check     # config only
make goreleaser-snapshot  # local unpublished build
```

## Other workflows

`ci.yml` (vet, fmt, staticcheck, tests, integration tiers, dry-run smoke, security audit), `spellcheck.yml` (cspell over Markdown), `e2e-full.yml` (manual live run), `tools-images.yml` (below).

## Tools images

`tools-images.yml` publishes `ghcr.io/jlcode-tech/awsbnkctl-tools-aws` and `ghcr.io/jlcode-tech/awsbnkctl-tools-iperf3` (multi-arch): `:<tag>` and `:latest` on a tag push, `:dev` on a push to `main`. The docker execution backend pulls them (`internal/exec/docker.go`). `runner.Dockerfile` builds a container with a released binary; no workflow publishes it.

## Self upgrade

```
awsbnkctl self update
```

Fetches the latest release, matches the host OS and architecture, verifies the SHA-256 against `checksums.txt`, and replaces the running binary in place.
