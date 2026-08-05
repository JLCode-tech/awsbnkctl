# Upstream issue draft — f5-cne-controller: pool members not refreshed on EndpointSlice change

> Draft upstream issue for filing against F5's BIG-IP Next for Kubernetes (BNK) project. Live-reproduced and worked around (BNK 2.3.0 / manifest `2.3.0-3.2598.3-0.0.170`). User-facing severity: **major** — VIP returns HTTP 500 silently until an operator notices and manually patches the HTTPRoute.

## Summary

`f5-cne-controller` resolves HTTPRoute `backendRefs` → Service → EndpointSlice → TMM pool members **only at HTTPRoute spec reconcile time**. It does NOT subscribe to EndpointSlice change events for backend services. When a backend pod is rescheduled (new pod IP), the EndpointSlice updates correctly but TMM's `pool_member` table retains the stale (deleted) pod IP. Traffic landing on the VIP returns HTTP 500 ("no available pool member") indefinitely.

The only effective recovery we have found is to force a **spec** change on the HTTPRoute (patch a field, or delete + re-apply) so the controller re-reads the current EndpointSlice. We have reproduced this live on multiple occasions; every non-spec recovery we tried (controller restart, annotation/finalizer patches) was confirmed ineffective — see below.

Verified with `HTTPRoute`. Other route types (e.g. `L4Route`, `TLSRoute`) were not tested, but likely share the reconcile-on-spec-change-only pattern.

## Expected vs actual behaviour

- **Expected:** when the EndpointSlice backing an HTTPRoute `backendRef` changes, the controller re-resolves pool members and pushes the new endpoint IPs to TMM — the same way it already reacts to EndpointSlice changes for its own internal services (see log evidence below).
- **Actual:** the EndpointSlice change is never propagated for user-defined backends. TMM keeps the deleted pod IP and the VIP returns HTTP 500 until an operator forces an HTTPRoute spec change.

## Versions

- BNK chart: `2.3.0`
- CNE manifest: `2.3.0-3.2598.3-0.0.170`
- EKS: `v1.30.14-eks-3385e9b`
- Containerd: `2.2.3`
- Cluster shape: 3 nodes, 1 TMM pod (host-device data plane), 1 nginx test backend

## Reproduction (any BNK 2.3.0 cluster)

1. Deploy a `Gateway` with `gatewayClassName: <bnk>` and an `HTTPRoute` whose `backendRef` points at a Service backed by a regular pod (e.g. nginx in the same namespace).
2. Curl the Gateway address → HTTP 200, traffic flows.
3. Delete the backend pod (`kubectl delete pod <nginx-pod>`). A new pod is scheduled with a **different IP**.
4. Verify the EndpointSlice updates immediately: `kubectl get endpointslices -n <ns>` shows the new pod IP.
5. Curl the Gateway address again → **HTTP 500** indefinitely. `tmctl pool_member_stat` (in the f5-tmm pod's `debug` sidecar) shows the OLD (now-deleted) pod IP, not the new one.

## Diagnostic evidence

### TMM perspective (stale pool member)

```
$ kubectl exec -n f5-cne-system f5-tmm-<id> -c debug -- \
    tmctl -d blade -c pool_member_stat | grep nginx
f5-cne-system-nginx-gw-http-80-nginx-route-rule-0-pool,\
00:00:00:00:00:00:00:00:00:00:FF:FF:0A:00:01:46:00:00:00:00,80,...
                                          ~~~~~~~~~~~
# 0A:00:01:46 = 10.0.1.70 — old, deleted pod IP
```

### Kubernetes perspective (current pod IP)

```
$ kubectl get endpointslices -n f5-cne-system | grep nginx
nginx-cvvms   IPv4   80   10.0.1.76   3h30m   # new pod IP
```

### cne-controller behaviour

After the backend pod is rescheduled, `kubectl logs f5-cne-controller-<id> -c f5-cne-controller` shows:

- No `"GatewayReconciler: handling http route update"` line for the affected HTTPRoute.
- Periodic `"Endpointslice nginx-<...> changed, syncing"` lines DO appear (the watch is firing) but only for **internal F5 services** (grpc-pccd-svc, grpc-pod-mgr-svc, f5-analyzer-grpc-svc, etc.) — not for user-defined backends referenced by HTTPRoute.

Annotation-only or finalizer-only patches to the HTTPRoute are explicitly ignored:

```
"Only CR status/finalizer is updated, ignoring the update" CrName="nginx-route"
```

A **spec** change DOES trigger a reconcile:

```
"Updating HTTPRoute" CrName="nginx-route" Operation="Update"
"GatewayReconciler: handling http route update" RouteName="nginx-route"
"Monitors derived from backendRefs" RouteName="nginx-route" rule index=0
```

After the spec-triggered reconcile, `tmctl pool_member_stat` shows the new pod IP (`0A:00:01:4C` = 10.0.1.76) and curl returns HTTP 200.

### Confirmed-ineffective recoveries

We tried, with full logs captured each time:

- **Restarting f5-cne-controller** — no effect. On restart, the controller observes the HTTPRoute (status shows `ResolvedRefs=True` at the post-restart timestamp) but never pushes the new pool member to TMM. Hypothesis: the restart-time reconcile dedups against the controller's in-memory cache, which was populated from the pre-restart state in TMM; the new EndpointSlice doesn't trigger a reconcile because it didn't *change* during the restart window.
- **Restarting f5-tmm** — would lose connection state and isn't a viable recovery for a production workload.
- **Patching HTTPRoute annotations** — explicitly filtered out by the controller's update predicate (log line above).
- **Patching HTTPRoute finalizers** — same.

## Suggested fix

Add an EndpointSlice informer to `GatewayReconciler` (or the equivalent component responsible for translating `HTTPRoute.backendRefs` → pool members). On EndpointSlice add/update/delete events, enqueue reconciles for every HTTPRoute whose `backendRefs` reference the parent Service. The Kubernetes controller-runtime `Watches()` builder supports this pattern out-of-the-box via `handler.EnqueueRequestsFromMapFunc`.

Note that the controller already runs an EndpointSlice watch for its internal services (the `"Endpointslice ... changed, syncing"` log lines above), so this extends existing machinery rather than adding new infrastructure — the watch just needs to cover Services referenced by HTTPRoute `backendRefs` as well.

Reference implementation pattern (controller-runtime):

```go
func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&gatewayv1.HTTPRoute{}).
        Watches(
            &discoveryv1.EndpointSlice{},
            handler.EnqueueRequestsFromMapFunc(r.findRoutesForSlice),
        ).
        Complete(r)
}

func (r *HTTPRouteReconciler) findRoutesForSlice(ctx context.Context, obj client.Object) []reconcile.Request {
    slice := obj.(*discoveryv1.EndpointSlice)
    serviceName := slice.Labels[discoveryv1.LabelServiceName] // "kubernetes.io/service-name"
    if serviceName == "" {
        return nil
    }
    var routes gatewayv1.HTTPRouteList
    if err := r.Client.List(ctx, &routes, client.InNamespace(slice.Namespace)); err != nil {
        return nil
    }
    var requests []reconcile.Request
    for _, route := range routes.Items {
        if routeReferencesService(route, serviceName) {
            requests = append(requests, reconcile.Request{
                NamespacedName: types.NamespacedName{
                    Name:      route.Name,
                    Namespace: route.Namespace,
                },
            })
        }
    }
    return requests
}

func routeReferencesService(route gatewayv1.HTTPRoute, serviceName string) bool {
    for _, rule := range route.Spec.Rules {
        for _, ref := range rule.BackendRefs {
            // Kind defaults to Service when unset.
            if ref.Kind != nil && *ref.Kind != "Service" {
                continue
            }
            if string(ref.Name) == serviceName {
                return true
            }
        }
    }
    return false
}
```

Two refinements a production implementation would also want: (1) cross-namespace `backendRefs` (`ref.Namespace` set, authorised via ReferenceGrant) require matching routes outside the slice's namespace, not just `client.InNamespace(slice.Namespace)`; (2) a field index on backendRef service names (`mgr.GetFieldIndexer()`) avoids listing every HTTPRoute on each EndpointSlice event.

## Operator-side workaround (until the fix lands)

`awsbnkctl bnk resync <httproute-name> -n <namespace>` does the spec-toggle for you, on every backendRef in the route:

```
weight N → N+1  (forces spec generation bump → controller reconciles)
weight N+1 → N  (restores the original weight ~1s later)
```

(A backendRef with no explicit weight is restored to `weight: 1` — the Gateway API default, so semantically identical.)

The same recovery with `kubectl` alone, for operators without `awsbnkctl`:

```
kubectl patch httproute <name> -n <ns> --type=json \
  -p='[{"op":"add","path":"/spec/rules/0/backendRefs/0/weight","value":2}]'
kubectl patch httproute <name> -n <ns> --type=json \
  -p='[{"op":"replace","path":"/spec/rules/0/backendRefs/0/weight","value":1}]'
```

The controller picks up the spec change, re-resolves the EndpointSlice, and pushes fresh pool members. Idempotent. Behaviour-preserving. No pod restarts. Verified live: curl response transitions from HTTP 500 to HTTP 200 within ~1 second of running `awsbnkctl bnk resync`.

For unattended mitigation, `awsbnkctl bnk resync --watch` runs the same logic as a daemon: it watches EndpointSlices for the Services referenced by the targeted HTTPRoutes and auto-triggers the spec-toggle when one changes, so the VIP self-heals without operator intervention until the upstream fix lands.

## Impact

This affects any production deployment where backend pods can be replaced — which includes routine rolling updates (`kubectl rollout restart`), node drains and upgrades, evictions, OOM kills, and spot reclaims. So: every production deployment. The current behaviour silently breaks the VIP and provides no signal to the operator that the pool is stale beyond observing HTTP 500 traffic. With multi-replica backends, a single replaced pod leaves one stale member in an otherwise-healthy pool, which would manifest as intermittent errors rather than a hard outage — harder still to diagnose. (Our reproduction used a single-replica backend, where the outage is total.) A naive operator who restarts the controller will see no improvement, escalating to "is BNK broken?" investigations that are expensive in operator time.

## Suggested labels

`area/gateway-api`, `area/data-plane-sync`, `kind/bug`, `priority/high`, `affects-version/2.3.0`
