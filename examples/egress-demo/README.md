# BNK Transparent Egress + Firewall (ACL) Demo — AWS `external-only`

A live, operator-driven demo that shows F5 BIG-IP Next for Kubernetes (BNK)
**transparent egress**: an application pod's outbound traffic is captured and
**source-translated (SNAT)** to a single BNK-controlled identity, with an
**egress firewall policy (ACL)** enforced on that traffic — all toggled by
applying/deleting **one** `F5SPKEgress` CR, with **zero changes to the pod**.

This is the productized, repeatable form of a result first proven live on a
throwaway `bnk-egress` cluster (ap-southeast-2, 2026-07): the observed internet
source IP flips between the pod's node public IP (uncontrolled) and the BNK NAT
EIP (controlled) as the egress CR is toggled.

> **Why `external-only`?** Transparent egress via the pseudo-CNI VXLAN overlay
> works on the **external-only** and **sriov-external** patterns, where TMM does
> NOT consume the node's internal NIC (it reaches pods over the CNI). It does
> **NOT** work on `dual-interface`/host-device on AWS VPC CNI — there the
> node-side VXLAN VTEP never comes up and the capture route is un-scopable and
> hijacks ingress (see `docs/audits/2026-05-27-egress-reinvestigation.md`). Use
> `examples/external-only/cluster.yaml` (or `cluster.yaml` here) for this demo.

---

## What's in this directory

| File | Purpose |
|------|---------|
| `cluster.yaml` | The demo cluster: `external-only`, ap-southeast-2, auto-registers into a localhost BNK Forge on `up`. |
| `egress-toggle.yaml` | **The toggle** — the `F5SPKEgress` CR (`SRC_TRANS_AUTOMAP` + `firewallEnforcedPolicy` + pseudo-CNI VXLAN). Apply = BNK ON, delete = BNK OFF. |
| `firewall-policy.yaml` | **The ACL** — `F5BigCneAddresslist` (`1.1.1.1/32`) + `F5BigFwPolicy` (drop the target, allow everything else, log both). Applied once. |
| `probe.sh` / `watch.sh` | The **agent's-eye view**. Shipped into the app pod via a ConfigMap; report the egress source IP the internet sees + reachability to the policy target. |
| `workload.yaml` | The captured `agent` pod (mounts the probe ConfigMap) + an uncaptured `control` pod for a side-by-side "unaffected workload" beat. |


---

## Prerequisites

- An `external-only` BNK cluster (this dir's `cluster.yaml`, or
  `examples/external-only/cluster.yaml`). Bring it up with:
  ```bash
  awsbnkctl up --config examples/egress-demo/cluster.yaml
  ```
- Your kubeconfig + AWS profile exported (paths below assume the standard
  `awsbnkctl` layout):
  ```bash
  cd /path/to/awsbnkctl
  export KUBECONFIG=.awsbnkctl/bnk-egress/kubeconfig
  export AWS_PROFILE=<your-sso-profile>
  ```

---

## Setup (once)

```bash
# 1. Firewall policy + address list (applied once; stays on the cluster).
kubectl apply -f examples/egress-demo/firewall-policy.yaml

# 2. The captured app pod + probe ConfigMap + the uncaptured control pod.
kubectl apply -f examples/egress-demo/workload.yaml
kubectl -n bnk-egress-demo rollout status deploy/agent --timeout=120s

# 3. Turn BNK egress ON (apply the F5SPKEgress CR).
kubectl apply -f examples/egress-demo/egress-toggle.yaml
```

---

## Run the demo

Split the screen: **left** = the agent pod's live view; **right** = the operator
toggle (a second terminal, or the Forge UI).

**Terminal 1 — the agent's view (leave running on screen):**
```bash
kubectl exec -it -n bnk-egress-demo deploy/agent -c nginx -- sh /demo/watch.sh
# refreshes every 3s; or run `sh /demo/probe.sh` for a one-shot
```

**Terminal 2 — the toggle:**
```bash
# BNK OFF  (uncontrolled egress)
kubectl delete -f examples/egress-demo/egress-toggle.yaml
#   → within ~10s the watch flips:
#       egress source IP = the pod's NODE PUBLIC IP  (uncontrolled, per-node)
#       1.1.1.1 reachable                            (no policy control)

# BNK ON  (controlled egress)
kubectl apply -f examples/egress-demo/egress-toggle.yaml
#   → within ~10-15s (CR programs on TMM):
#       egress source IP = the BNK NAT EIP           (single controlled identity)
#       google / github still reachable (allowed)
#       1.1.1.1 BLOCKED                              (firewall drop rule)
```

**The story:** same pod, zero pod changes — one CR flips its egress from a
per-node/uncontrolled identity to a single BNK-controlled identity **with policy
enforcement**. The `control` pod in the `bnk-egress-control` namespace is never
captured, so it's the "unaffected workload" side-by-side beat.

### Optional Forge beats (best story — no terminal)
With this cluster registered in a localhost Forge:
- Toggle from the UI: **cluster `bnk-egress` → Networking → Egress →** delete /
  re-create `bnk-egress-demo` (Create Resource / Configuration Builder, paste
  `egress-toggle.yaml`). The pod's `watch.sh` flips ~10–15s later.
- Show **Insights** while ON: the **Traffic Flow** egress lane, the **Gateway
  Topology** egress node, and the **Policy Gateway Map** (`block-test-target →
  1.1.1.1/32`). (Egress-display support shipped in bnk-forge PR #473.)

---

## ⚠️ One gotcha to rehearse before a real demo

The **SNAT identity flip always works**. The **firewall-block beat** depends on
the ACL blob being loaded into TMM — and on BNK 2.3 there is a `blobd` TLS bug
where the blob only reaches TMM right **after a TMM pod restart**. If, while BNK
is ON, `1.1.1.1` comes back reachable (e.g. HTTP 301) instead of BLOCKED, bounce
TMM and wait ~2–3 minutes:

```bash
kubectl delete pod -n f5-cne-system -l app=f5-tmm
```

Then re-check the probe shows `1.1.1.1` BLOCKED. (The egress SNAT flip is
unaffected by this.)

---

## Teardown

```bash
kubectl delete -f examples/egress-demo/egress-toggle.yaml
kubectl delete -f examples/egress-demo/workload.yaml
kubectl delete -f examples/egress-demo/firewall-policy.yaml
# Then tear down the cluster (stops AWS spend):
awsbnkctl down --config examples/egress-demo/cluster.yaml --yes
```

---

## How it works (one paragraph)

The `F5SPKEgress` CR (`snatType: SRC_TRANS_AUTOMAP` + `pseudoCNIConfig.vxlan`)
tells the BNK controller to stand up a VXLAN tunnel endpoint on TMM and signal
the `f5-spk-csrc` DaemonSet to program the worker node, so outbound traffic from
pods in the captured namespace(s) is intercepted on `eth0` and routed through TMM
over the overlay. TMM then AUTOMAP-SNATs it to its external self-IP identity (the
BNK NAT EIP the internet sees) and applies the `firewallEnforcedPolicy` ACL. With
`external-only`, TMM's only data-plane interface is the external ENI and it
reaches pods over the CNI — so nothing consumes the node's internal NIC and the
overlay works. See `docs/ARCHITECTURE.md` and, for the data-plane rationale,
`docs/audits/2026-05-27-egress-reinvestigation.md`.
