# Changelog

All notable changes to `awsbnkctl` are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project uses [semantic versioning](https://semver.org/spec/v2.0.0.html). Pre-`v1.0.0` minor versions may include breaking changes — see the per-version notes.

## v1.0.0 — 2026-08-06

### Changed
- **Documentation rewrite** — modernized and rewrote all READMEs across the project.
- **Repository cleanup** — removed unused/internal references from public tracked state.

### Fixed
- **Local Zone Cluster Creation** — Fixed an issue where EKS `CreateCluster` failed when AWS Local Zone subnets were provided for the control plane. Local Zone subnets are now correctly filtered out from the `CreateCluster` API request while remaining available for worker nodes in the data plane.

## v0.9.0 — 2026-07-19

Finalizes `v0.9.0-rc1`. Everything in the rc plus the pool-member auto-heal daemon, resync safety hardening, doctor green-by-default fixes, and a reproducible security gate.

### Added

- **`bnk resync --watch`** — daemon mode that watches EndpointSlices for the Services referenced by the targeted HTTPRoutes' backendRefs and auto-fires the weight-toggle resync (debounced, default 2s) when one changes. The operator-side mitigation for the upstream cne-controller gap becomes unattended: the VIP self-heals instead of serving HTTP 500 until someone notices. Composable with all target selectors and `--dry-run`.
- **Dependabot** — weekly grouped go.mod updates + GitHub Actions pin updates, keeping the security gate green as upstream fixes are released.
- **Demo experience subsystem** — `awsbnkctl up --demo` provisions the identical cluster as a normal `up`, plus pre-stages a demo client on the jumphost, tags resources with an absolute expiry, and renders a rocket-themed staged launch UI on interactive terminals. Non-TTY and `--no-color` runs fall back to the plain per-phase log byte-for-byte unchanged.
- **`demo {list,run,clean}` command group** — curated audience catalogue alongside the `scenarios` validation suite. The two registries stay disjoint; `demo list` shows the union (demos + Green scenarios) with a `KIND` column.
- **`http2` demo use-case** — proves end-to-end HTTP/2 (h2c) through TMM, asserting both legs (client→TMM wire HTTP/2 + TMM→backend body `HTTP/2.0`) via SSH+EICE curl from the pre-staged jumphost.
- **`diameter` demo use-case** — proves Diameter (RFC 6733) CER→CEA Result-Code 2001 transit across an L4 BNK Gateway, pushing the embedded Python client via `CopyFileViaEICE` and running it via `RunStagingCommands`.
- **`ingress-migration` demo use-case** — runs ingress-nginx, HAProxy, and a BNK Gateway API route side-by-side over one shared backend, so the legacy-ingress → BNK migration path can be compared live before cutover.
- **`bigip-cis` demo use-case** — stands up an external F5 BIG-IP VE fronted by in-cluster CIS (`k8s-bigip-ctlr`) programming a `VirtualServer` — the traditional appliance model BNK replaces. Opt-in via the `bigipVE:` block; admin password supplied out-of-band via `AWSBNKCTL_BIGIP_PASSWORD`.
- **Jumphost staging primitives** — exported `jumphost.RunStagingCommands` + `jumphost.CopyFileViaEICE` that mint+push ephemeral EICE keys internally (no operator key dance), shared by demo use-cases and the demo-client pre-staging phase.
- **VPC CNI prefix delegation (Phase 08b)** — moved before the node group so nodes boot in prefix mode. Eliminates the cold-start hang caused by secondary-ENI asymmetric drop on the EKS CNI.
- **Phase 11b** — EBS CSI managed addon + `gp3` StorageClass + hugepages-2Mi DaemonSet, in front of the BNK install.
- **Phases 17b/c/d** — multi-ENI jumphost provisioning + interface discovery + (under `--demo`) jumphost client pre-staging.
- **Phase 23b** — `F5SPKVlan` + `GatewayClass` for the host-device pattern, completing the TMM data-plane plumbing.
- **Selectable interface patterns** — `pattern: external-only | dual-interface | sriov-external` (`host-device` is the legacy alias for `dual-interface`). `sriov-external` runs TMM's DPDK dataplane over a `vfio-pci`-bound ENA and is experimental.

### Changed

- **Terraform removed entirely.** The production path is now AWS-SDK-only across all phases. The repository no longer carries Terraform sources, lock files, or vendored modules. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the rationale.
- **`-f/--config` flag unification** — `bnk resync` now accepts `-f/--config`; `status` reads the targeted cluster's `state.env` instead of the host default kubeconfig.
- **CNE Instance auto-resync** — `awsbnkctl bnk resync` ships as a first-class subcommand to work around the upstream HTTPRoute pool-member stale bug (see [`docs/upstream-issues/`](docs/upstream-issues/)).
- **`cluster.yaml` validation** — strict YAML parsing (`KnownFields(true)`); unknown top-level fields fail loud rather than silently being ignored.
- **Security gate policy** — the CI govulncheck step now fails only on reachable vulnerabilities with a released fix; reachable findings with no fix published anywhere surface as warnings instead of permanently blocking every PR. govulncheck is pinned (v1.6.0) for reproducibility, and workflow actions moved to Node24-native majors (checkout/setup-go/upload-artifact v7).
- **Dependency bumps** — `containerd` v1.7.30 → v1.7.33 and Go toolchain go1.26.4 → go1.26.5, clearing the four govulncheck findings that had released fixes.
- **Upstream issue report tightened** — `docs/upstream-issues/cne-controller-endpointslice-not-watched.md` gained an expected-vs-actual section, a kubectl-only recovery, a hardened reference fix, and now documents the `--watch` mitigation.

### Fixed

- **`bnk resync` restore is spec-identical and race-safe** — backendRefs that had no explicit weight are restored with a JSON-Patch `remove` (no more permanent `weight: 1` residue), and both toggle patches carry RFC 6902 `test` guards so a concurrent spec edit fails loudly instead of being clobbered.
- **Doctor green-by-default on region-less hosts** — credentials-resolve-but-no-region now degrades to a warning on the `aws credentials` row (downstream `aws *` rows skipped) instead of a `StatusError` exit; `internal/aws` exposes the `ErrRegionEmpty` sentinel.
- **Phase 14 + Phase 24b idempotency** — both phases now skip cleanly on healthy re-runs (FLO Helm upgrade was unconditional; DSSM overlay reverted its own marker). Healthy `up -f <existing>` is now a true no-op for the BNK install path.
- **Phase 17c on TMM-owns-ENI re-runs** — guard added so an `up` re-run against a healthy cluster no longer fails with `MAC not found` when TMM has already claimed the secondary ENIs into its netns.
- **Pool-member stale workaround in scenarios** — `pkg/bnk.ResyncHTTPRoutes` is now wired into every scenario's Verify step (before the data-plane probe) so probes observe a healed pool.

## v0.x

The pre-`v1.0` series is captured in git history. Each `feat()` / `fix()` commit on `main` / `staging` includes a self-contained design note.
