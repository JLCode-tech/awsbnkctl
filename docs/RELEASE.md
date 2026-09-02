# Release Guide for awsbnkctl

This document outlines the release process, versioning conventions, build artifacts, container runner image builds, and catalog synchronization for `awsbnkctl`.

---

## 1. Versioning Model

`awsbnkctl` adheres to [Semantic Versioning (SemVer 2.0.0)](https://semver.org/):
- **Major (`vX.0.0`)**: Breaking CLI flag changes, schema breaks in `cluster.yaml`, or major architectural migrations.
- **Minor (`v1.X.0`)**: New provisioning phases, new scenarios, newly supported BNK or Kubernetes minor versions.
- **Patch (`v1.0.X`)**: Bug fixes, security remediation, dependency updates, and documentation enhancements.

---

## 2. Automated Release Pipeline

The repository uses Google's `release-please` GitHub Action in combination with `goreleaser` (`.github/workflows/release.yml`):

1. **Conventional Commits**: Every PR merged to `main` must follow conventional commit prefixes (`feat:`, `fix:`, `chore:`, `docs:`, `test:`).
2. **Release PR**: `release-please` automatically maintains an open Release PR aggregating merged changes and bumping `CHANGELOG.md`.
3. **Tag & Build**: Merging the Release PR automatically cuts the `vX.Y.Z` git tag and triggers GoReleaser.
4. **Binary Artifacts**: GoReleaser builds statically linked binaries for:
   - Linux (`amd64`, `arm64`) — `.tar.gz`
   - macOS (`arm64` Apple Silicon, `amd64` Intel) — `.tar.gz`
   - Windows (`amd64`, `arm64`) — `.zip`
   - `checksums.txt` containing SHA-256 digests.

---

## 3. Container Runner Images for BNK Forge

`awsbnkctl` can be executed as a containerized runner module within BNK Forge without requiring host binary installations:

```bash
# Build the multi-arch runner image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg AWSBNKCTL_VERSION=1.1.0 \
  -t ghcr.io/jlcode-tech/awsbnkctl-tools-runner:1.1.0 \
  -f runner.Dockerfile --push .
```

---

## 4. bnkctl-index Catalog Registration

The runner image and pack metadata are registered in `mwiget/bnkctl-index` under `tools/awsbnkctl/`:
1. Update `tools/awsbnkctl/bnkforge.pack.json` with the new version.
2. Update `tools/awsbnkctl/bnkforge.artifact.json` with the image digest (`sha256:...`).
3. Run `python3 <bnk-forge>/scripts/validate_catalog_content.py <bnkctl-index>` before publishing.

---

## 5. In-Place Self Upgrades

Users can update their installed binary directly via:
```bash
awsbnkctl self update
```
The command fetches the latest GitHub release metadata, matches the host's operating system and architecture, verifies the SHA-256 checksum against `checksums.txt`, and safely replaces the executing binary.
