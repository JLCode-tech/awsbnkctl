# Throughput testing

`awsbnkctl test throughput` measures TCP bandwidth between an iperf3 client and an iperf3 server, with at least one side running adjacent to (or inside) the EKS cluster so the number reflects something useful — cluster fabric (**east-west**), the inbound path through an AWS NLB (the default **north-south** mode), or the outbound path from an EC2 jumphost.

The probe implementation lives at `internal/test/throughput.go::RunThroughput` (~110 LOC), which shells out to a local `iperf3` binary against an endpoint the caller resolved. The cluster-side server pod + Service shape (Pod with iperf3 listening on 5201, ClusterIP or LoadBalancer Service) is owned by the CLI layer in `internal/cli/test.go`, which feeds the resolved endpoint into `RunThroughput`.

## What the suite measures

Plain TCP throughput, plus retransmits, between two endpoints both running iperf3:

- The **server** runs in the cluster — a single Pod plus a Service in the `awsbnkctl-test` namespace.
- The **client** runs wherever you point the backend — by default in the cluster as a one-shot Job, alternatively on the host or on a registered SSH target.
- Output is iperf3's native `-J` JSON; `parseIperf3JSON` pulls `end.sum_received.bits_per_second` (receiver-side, post-retransmit) as the headline number and `end.sum_received.retransmits` as the loss/congestion indicator.

The suite is appropriate for "is the cluster fabric healthy", "is the BNK data path delivering the bandwidth I expect from outside", and "is this jumphost the bottleneck between my office and the cluster". It is not a precision benchmark — TCP throughput is sensitive to MTU, NIC offloads, kernel tunables, ENA queue sizing, and the iperf3 server's own resource limits, none of which the suite tries to control.

## The two modes

Mode is selected by `--mode`. The default is `north-south`.

### `--mode north-south`

Measures the **inbound path** from outside the cluster to a Pod inside it. The server's Service is `LoadBalancer`, so EKS provisions an AWS Network Load Balancer (the AWS Load Balancer Controller's default for L4 Services on EKS). The client connects to the NLB's hostname on 5201.

Use cases: "is the BNK ingress path delivering the bandwidth I expect", "is my office uplink the bottleneck", "is the cluster's egress capacity what AWS promised for this instance family".

Combine with `--backend local` when you specifically want to measure the laptop-to-cluster path. Combine with `--backend ssh:<bastion>` when you want a known-stable measurement vantage from an EC2 jumphost in a known VPC — useful when laptop Wi-Fi is suspect or when you want an intra-AWS-region number that doesn't include the public-internet hop.

### `--mode east-west`

Measures the **intra-cluster fabric** — Pod-to-Pod. The server's Service is `ClusterIP`, reachable only from inside the cluster. The default `--backend k8s` runs the client adjacent to the server (a one-shot Job in the same namespace), so the number reflects the CNI's pod-to-pod throughput.

On the SR-IOV-enabled `c5n.4xlarge` node group [Chapter 33](./33-data-plane-decision.md) walks, two interesting east-west measurements:

- **Standard CNI (kube-proxy + VPC CNI).** The default pod network — eth0 in each pod, NAT through the node's primary ENI. Expect roughly the node's link rate minus CNI overhead. On `c5n.4xlarge` (25 Gbps line rate), expect ~10-20 Gbps single-flow with default kernel tunables.
- **Multus + SR-IOV secondary interface.** Pods that BNK schedules with a `intel.com/sriov: 1` request get a dedicated VF; iperf3 over that interface (specify the secondary interface in the iperf3 invocation) skips the kernel's TCP/IP stack overhead and gets closer to line rate.

True pod-to-pod east-west with both ends pinned to specific nodes (`--cross-node`) is the v1.x refinement; today the in-cluster Job client gets you the headline measurement.

## Per-tool default backend

The default backend for `iperf3` is **`k8s`**. The wiring is `internal/cli/test.go::resolveBackendSpecWith` consulting the `perToolDefaultBackend` map in `internal/cli/legacy_helpers.go`. The default holds whether or not you have set `exec.iperf3.backend` in workspace config. To override per-invocation:

```bash
awsbnkctl test throughput --backend local                  # client on laptop
awsbnkctl test throughput --backend ssh:bastion            # client on EC2 jumphost
awsbnkctl test throughput --backend k8s                    # default; explicit
```

`--backend docker` is rejected at parse time — a Docker container running on the user's laptop has the same network identity as the host (default bridge networking), so the client's view is identical to `--backend local`.

## EKS 1.25+ Pod Security Admission compliance

EKS 1.25+ enforces Pod Security Admission (PSA) at the namespace level — the upstream replacement for OpenShift's SCC framework that the inherited `roksbnkctl` codebase was tuned against. The iperf3 server pod's `securityContext` is set to satisfy the `restricted` PSA profile (the strictest of the three built-in profiles):

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  seccompProfile:
    type: RuntimeDefault
containers:
- name: iperf3
  securityContext:
    allowPrivilegeEscalation: false
    runAsNonRoot: true
    capabilities:
      drop: ["ALL"]
```

iperf3 listens on 5201 (unprivileged) so root is not needed. Two things follow:

1. **The bundled image must run as non-root.** The `awsbnkctl-tools-iperf3` image declares `USER 1000` in its Dockerfile so the pod's `runAsUser: 1000` is satisfied without further config. Stock images that run as root (the unbundled `networkstatic/iperf3:latest`) will fail PSA admission with `pods "..." is forbidden: violates PodSecurity "restricted:v1.x"`.
2. **A custom workspace image must respect `runAsNonRoot`.** If you point `test.throughput.image` at your own iperf3 image, drop the `USER root` line and rebuild. iperf3 does not need root.

If you see one of these failure shapes:

- `violates PodSecurity "restricted:v1.x": allowPrivilegeEscalation != false`
- `unable to validate against any pod security policy: ... must run as non-root`
- `runAsNonRoot is required`

…then either the configured image runs as root (use the bundled image) or the cluster's PSA enforcement is stricter than the `restricted` baseline (a cluster-policy question outside the suite's control). The `awsbnkctl-test` namespace itself ships with the `pod-security.kubernetes.io/enforce: restricted` label so the admission decision is the namespace's, not the cluster's.

## What "normal" looks like on `c5n.4xlarge`

The PRD 07 SR-IOV node group lands `c5n.4xlarge` instances by default (25 Gbps ENA-advertised). Baseline iperf3 results to expect, with the defaults `--duration 30 --streams 8`:

| Measurement | Expected range | Caveats |
|---|---|---|
| North-south via NLB, single-AZ, intra-VPC client | 9-15 Gbps | NLB target group health-check should be green; AWS doesn't cap per-flow but does cap per-NLB-listener at ~5 Gbps single-flow — `--streams 8` is what unlocks the headline number. |
| North-south via NLB, cross-AZ | 6-10 Gbps | AZ-to-AZ inter-az charges apply; the number is the link, not the cost. |
| East-west, pod-to-pod, same node, standard CNI | 12-18 Gbps | Loopback-ish; the kernel's TCP stack is the cap, not the wire. |
| East-west, pod-to-pod, cross-node, standard CNI | 8-15 Gbps | Two ENA round-trips; expect ~half line rate after CNI overhead. |
| East-west via SR-IOV VF (pods scheduled with `intel.com/sriov: 1`) | 20-24 Gbps | Closer to line rate; kernel bypass through the VF skips most of the standard stack. |

Numbers below the low end of the range usually point at one of: CPU on the client (check `end.cpu_utilization_percent.host_total` in the iperf3 JSON; >80% means the client maxed out, bump `--streams` and rerun on a beefier instance), the wrong instance type (`c5.4xlarge` instead of `c5n.4xlarge` — the `n` family is where ENA's 25 Gbps lives), or a CNI mismatch (the pod landed on the standard interface instead of an SR-IOV VF; check `kubectl exec` into the pod, `ip addr` for `eth0` vs `net1`).

## Reading the output

Default output is human-readable on stderr; `-o json` switches to JSON on stdout. The JSON envelope is the umbrella `awsbnkctl.v1` shape; the headline number is in `extra.throughput_gbps`, and the iperf3 retransmit count is `extra.retransmits`. Example:

```bash
$ awsbnkctl test throughput
running throughput ...
  PASS  iperf3 north-south → awsbnkctl-iperf3-...elb.us-east-1.amazonaws.com  3.41 Gbit/s (127 retransmits)
throughput PASS (1/1 passed)
```

The `Extra` block captures `endpoint`, `mode`, `duration_s`, `streams` alongside the throughput + retransmit counters so a CI consumer can pin to any of them.

## Tuning knobs in workspace config

```yaml
# ~/.awsbnkctl/<workspace>/config.yaml
test:
  throughput:
    image: ghcr.io/JLCode-tech/awsbnkctl-tools-iperf3:v0.7.0
    duration: 30        # iperf3 -t flag, seconds
    streams: 8          # iperf3 -P flag, parallel streams
    default_mode: north-south
```

For deeper diagnosis: bump `duration` to 60-90 for a stable average on variable paths; bump `streams` to 16-32 for high-bandwidth-delay paths; drop `streams` to 1 to reproduce a customer's "single-stream upload feels slow" complaint.

## Cleanup and `--keep`

By default the suite tears down the iperf3 server pod and Service after the client run. Pass `--keep` to leave the fixture up for follow-on debugging (kubectl exec into the server, hand-run `iperf3 -c` from a third vantage). The fixture is in the `awsbnkctl-test` namespace — `kubectl delete -n awsbnkctl-test pod/awsbnkctl-iperf3 svc/awsbnkctl-iperf3` removes it.

## Cross-references

- [Chapter 17 — Execution backends](./17-execution-backends.md) — k8s one-shot-Job mechanics, SSH binary-materialisation.
- [Chapter 12 — Workspace config](./12-workspace-config.md) — workspace-config schema for `test.throughput.*`.
- [Chapter 20 — Connectivity testing](./20-connectivity-testing.md) — the "does HTTP work" companion suite.
- [Chapter 21 — DNS testing for GSLB](./21-dns-testing-gslb.md) — DNS validation companion.
- [Chapter 23 — The E2E test plan](./23-e2e-test-plan.md) — phase G runs `test throughput` against a Sprint-3 deployment.
- [Chapter 33 — The data-plane decision](./33-data-plane-decision.md) — SR-IOV-on-EKS background; explains the c5n.4xlarge / VF interpretation in this chapter.
