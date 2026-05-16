# Connectivity testing

`awsbnkctl test connectivity` answers one question: *can my workspace reach the HTTP/HTTPS endpoints I care about right now?*

It is the simplest of the three test suites — no cluster fixtures, no remote vantage, no JSON parsing harness. Each configured URL gets one HTTP `GET`, the suite reports pass/fail, and the runner exits `0` if every probe passed. Use it as the first sanity check after `awsbnkctl up`, as a CI smoke step against a known-good fixture set, or as the "is it me or is it the network" baseline before reaching for `curl -v` or `openssl s_client`.

The implementation lives at `internal/test/connectivity.go::RunConnectivity` (~100 LOC); the umbrella runner is `internal/test/runner.go`. The shape carried forward from the upstream `roksbnkctl` fork unchanged — the probe is HTTP, not AWS-specific, and the existing logic works against an EKS LoadBalancer Service the same way it worked against ROKS. What changed in the AWS retarget is the *recognition* of LoadBalancer shapes (NLB vs ALB) in the worked examples, the doctor cross-link, and the `awsbnkctl` binary name.

## What the suite does

For each configured URL the runner:

1. Adds an `https://` scheme if the entry has none.
2. Issues a single `GET` with a 10-second timeout and the user-agent `awsbnkctl/test`.
3. Records the HTTP status code, the wall-clock duration, and (for HTTPS) the negotiated TLS version.
4. Marks the probe **pass** if the status code is in `[200, 400)` (any 2xx or 3xx); **fail** for anything else, any TLS error, any DNS error, any timeout.
5. Aggregates the per-URL results into a suite result; the suite passes only when every URL passed.

No retries, no expected-body matching, no configurable status assertions, no L4-only reachability — those are deliberate non-goals (see [§ When connectivity is the wrong tool](#when-connectivity-is-the-wrong-tool) below).

## Configuring `extra_hosts`

The list of URLs to probe lives in your workspace config under `test.connectivity.extra_hosts`:

```yaml
# ~/.awsbnkctl/<workspace>/config.yaml
test:
  connectivity:
    extra_hosts:
      - https://my-bnk-cis-controller.example.com
      - https://bigip-next-admin.example.com:8443
      - https://gslb.example.com
      - my-bare-host.example.com    # scheme defaults to https://
```

The schema is intentionally minimal — `extra_hosts` is a `[]string` of URLs (or bare hostnames; `https://` is added when no scheme is present). One entry per line. The order in the file is the order the runner probes. There is no per-host method, no per-host expected-status, and no per-host TLS-trust override today; a richer per-host schema is queued for v1.x.

[Chapter 12 — Workspace config](./12-workspace-config.md) covers the full `test:` block; this chapter expands the `connectivity` slice.

## AWS LoadBalancer shapes the suite recognises

On an EKS deployment `awsbnkctl up` provisions, the URLs in `extra_hosts` typically resolve to one of two AWS LoadBalancer shapes — and the connectivity probe handles both transparently:

- **NLB (Network Load Balancer, `service.beta.kubernetes.io/aws-load-balancer-type: "external"` with `nlb-ip` target type).** Returns a stable `*.elb.<region>.amazonaws.com` hostname; the AWS Load Balancer Controller programs the target group. Probe sees a regular HTTPS endpoint; TLS termination is at the pod (re-encrypt) or at the NLB if you front it with ACM.
- **ALB (Application Load Balancer, ingress-class `alb`).** Returns a `*.<region>.elb.amazonaws.com` hostname; certificate is typically ACM-issued. Probe records `TLS 1.3` in the `tls_version` field if the listener is configured for it (ACM defaults).

A typical post-`up` `extra_hosts` covers three things: the BNK CIS controller endpoint (data-plane front), the F5 BIG-IP Next admin endpoint (management, often `:8443`), and the GSLB VIP fronting the application. The probe doesn't care whether the answer is an NLB or an ALB — both are reachable HTTPS endpoints, and the success criterion is "did I get a 2xx or 3xx back". For diagnosing the failure mode when one *doesn't* answer, see [Chapter 26 § AWS LoadBalancer](./26-troubleshooting.md) and the related-DNS-vantage walk in [Chapter 21](./21-dns-testing-gslb.md).

## The `--insecure` flag

Self-signed certs are common in pre-production BNK deployments — the BIG-IP Next admin endpoint, an internal GSLB VIP not yet fronted by ACM. By default Go's TLS stack rejects them and the probe fails with `x509: certificate signed by unknown authority`. Pass `--insecure` to skip verification for the run:

```bash
awsbnkctl test connectivity --insecure
```

This sets `tls.Config.InsecureSkipVerify = true` on the HTTP client for one invocation only, applies to every URL in `extra_hosts`, and is not persisted. There is no per-host insecure flag in v1.0; if you need split TLS posture, run two invocations in two workspaces.

## Reading the output

Default output is human-readable on stderr; pass `-o json` for machine-readable on stdout.

```bash
$ awsbnkctl test connectivity
running connectivity ...
  PASS  https://bnk-cis.dev-tor.example.com                   200 OK in 142ms
  PASS  https://bigip-next-admin.dev-tor.example.com:8443     302 Found in 88ms
  FAIL  https://gslb.dev-tor.example.com                      Get "...": dial tcp: i/o timeout
connectivity FAIL (2/3 passed)
$ echo $?
1
```

A 3xx redirect counts as pass — the runner doesn't follow redirects, but the redirect itself is a successful HTTP response, which is what the suite measures. If you specifically need the final 200 after a redirect chain, `curl -L` is the tool.

The JSON shape is the umbrella `awsbnkctl.v1` envelope (suite + results array). Per-probe entries carry `status_code` and `tls_version` in `extra`:

```bash
$ awsbnkctl test connectivity -o json | jq '.results[0]'
{
  "suite": "connectivity",
  "name": "https://bnk-cis.dev-tor.example.com",
  "status": "pass",
  "detail": "200 OK in 142ms",
  "duration_ms": 142,
  "extra": { "status_code": 200, "tls_version": "TLS 1.3" }
}
```

Exit code: `0` on `overall: pass`, `1` on `overall: fail`. CI can branch on the exit code or consume the JSON for partial-tolerance assertions.

## Failure-mode reading guide

The probe's `detail` field is the Go HTTP-client error string verbatim. The most common shapes against an EKS deployment:

| Detail substring | What it means | First check |
|---|---|---|
| `dial tcp: i/o timeout` | Connection didn't establish in 10s. Either the LB hasn't programmed (post-apply window) or a security group is dropping the SYN. | `aws elbv2 describe-load-balancers` to confirm the LB is `active`; check the LB's security group ingress on 443. |
| `x509: certificate signed by unknown authority` | The cert isn't trusted by the system CA bundle. Common when the endpoint is fronted by a self-signed cert or an internal CA. | Pass `--insecure` to confirm reachability; long-term, front with ACM. |
| `no such host` | DNS resolution failed. Either the name doesn't exist or the resolver can't reach the authoritative server. | Run `awsbnkctl test dns --target <host>` to see what the resolver returned. |
| `dial tcp: lookup <name>: server misbehaving` | DNS resolver returned SERVFAIL. Often a Route 53 private-hosted-zone misconfig. | Cross-vantage probe with `awsbnkctl test dns --gslb-compare` — divergence often pinpoints the misconfigured resolver. |
| `HTTP 502/503/504` | The LB reached, but the upstream pod didn't answer (or answered slowly). | `awsbnkctl k get pods -n <ns>` for crash/restart counts; check the Service's endpoint slice for backend pod readiness. |

When the probe fails with one of these, the next step is one of the more-specific tools in the table at the bottom of this chapter — not another connectivity run.

## Running connectivity inside `awsbnkctl test all`

Connectivity is one of the suites the bare `awsbnkctl test` (or `awsbnkctl test all`) command dispatches:

```bash
$ awsbnkctl test
running connectivity ...
  PASS  https://bnk-cis.dev-tor.example.com  200 OK in 174ms
running dns ...
  PASS  bnk-cis.dev-tor.example.com  resolved 1 address(es)
connectivity PASS (1/1 passed)
dns          PASS (1/1 passed)

PASS overall (2/2 suites passed)
```

In `-o json` mode, `awsbnkctl test all` emits an `all`-shape envelope with one `suites[]` entry per suite. CI can pin to either the suite-level overall or a specific probe's status:

```bash
awsbnkctl test all -o json | jq -e '.suites[] | select(.suite=="connectivity") | .overall == "pass"'
```

## When connectivity is the wrong tool

`awsbnkctl test connectivity` is "does HTTP work". For anything finer-grained, reach for the right tool:

| Scenario | Use this instead |
|---|---|
| Full TLS handshake, cert chain, SNI, negotiated cipher | `openssl s_client -connect host:port -servername host` |
| Headers, redirect-following, body matching, specific-status assertion | `curl -v -L --fail-with-body <url>` |
| L4 reachability on a specific port, no HTTP layer | `nc -vz host port` |
| DNS resolution from a specific resolver, especially cross-vantage for GSLB | [`awsbnkctl test dns`](./21-dns-testing-gslb.md) |
| What answer a name returns from inside the cluster vs. the laptop | [`awsbnkctl test dns --gslb-compare`](./21-dns-testing-gslb.md#the---gslb-compare-workflow) |
| Bandwidth between two endpoints | [`awsbnkctl test throughput`](./22-throughput-testing.md) |

The connectivity suite is intentionally a thin probe. When the answer to "is it broken" is "yes" and you need to know why, the suite has done its job — it has flagged the URL — and the next step is one of the tools above.

## Cross-references

- [Chapter 12 — Workspace config](./12-workspace-config.md) — full `test:` block schema.
- [Chapter 21 — DNS testing for GSLB](./21-dns-testing-gslb.md) — when "the URL fails" actually means "the name doesn't resolve from this vantage".
- [Chapter 22 — Throughput testing](./22-throughput-testing.md) — bandwidth-measurement companion suite.
- [Chapter 23 — The E2E test plan](./23-e2e-test-plan.md) — the phase that runs connectivity as a smoke step against a fresh `up`.
- [Chapter 26 — Troubleshooting](./26-troubleshooting.md) — symptom-shaped catalogue, including LoadBalancer programming windows and security-group ingress mismatches.
