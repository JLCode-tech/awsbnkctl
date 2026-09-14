package bnkscan

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Options tunes Scan.
type Options struct {
	// ControllerNamespace and ControllerName locate the f5-cne-controller
	// Deployment. Empty values use the awsbnkctl defaults.
	ControllerNamespace string
	ControllerName      string
}

func (o Options) withDefaults() Options {
	if o.ControllerNamespace == "" {
		o.ControllerNamespace = DefaultControllerNamespace
	}
	if o.ControllerName == "" {
		o.ControllerName = DefaultControllerName
	}
	return o
}

// raw holds the live objects Scan read, for DiscoverMCP.
type raw struct {
	gateways     []*unstructured.Unstructured
	classes      []*unstructured.Unstructured
	httpRoutes   []*unstructured.Unstructured
	netPolicies  []*unstructured.Unstructured
	secPolicies  []*unstructured.Unstructured
	persistence  []*unstructured.Unstructured
	gatewayReady map[string]bool // namespace/name → Ready
}

// Scan indexes the cluster. cs may be nil, in which case the controller
// rollout is reported as not found. A CRD the API server does not serve is
// recorded in Index.Missing; any other API error aborts the scan.
func Scan(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, opts Options) (*Index, error) {
	opts = opts.withDefaults()
	idx := &Index{Controller: Controller{Namespace: opts.ControllerNamespace, Name: opts.ControllerName}}
	r := &raw{gatewayReady: map[string]bool{}}

	groups, err := f5CRDGroups(ctx, dyn)
	if err != nil {
		return nil, err
	}
	idx.Groups = groups
	idx.Generation = DetectGeneration(groups)

	list := func(kind string, gvr schema.GroupVersionResource) ([]*unstructured.Unstructured, error) {
		items, err := listAll(ctx, dyn, gvr)
		if err != nil {
			if apierrors.IsNotFound(err) {
				idx.Missing = append(idx.Missing, kind)
				return nil, nil
			}
			return nil, fmt.Errorf("list %s (%s): %w", kind, gvr.GroupResource(), err)
		}
		return items, nil
	}

	// 2.4 kinds.
	type kindSpec struct {
		kind  string
		gvr   schema.GroupVersionResource
		ready func(*unstructured.Unstructured) (bool, string)
		dest  *[]Object
	}
	specs := []kindSpec{
		{"Infra", InfraGVR, requireTrue("Programmed"), &idx.Infras},
		{"GatewaySettings", GatewaySettingsGVR, trueIfPresent("Accepted", "Programmed"), &idx.GatewaySettings},
		{"EgressGateway", EgressGatewayGVR, trueIfPresent("Accepted", "Programmed"), &idx.EgressGateways},
		{"SecPolicy", SecPolicyGVR, trueIfPresent("Accepted", "Programmed"), &idx.SecPolicies},
		{"NetPolicy", NetPolicyGVR, trueIfPresent("Accepted", "Programmed"), &idx.NetPolicies},
		{"F5BigPersistenceProfile", PersistenceProfileGVR, requireTrue("Programmed"), &idx.PersistenceProfiles},
	}
	keep := map[string][]*unstructured.Unstructured{}
	for _, s := range specs {
		items, err := list(s.kind, s.gvr)
		if err != nil {
			return nil, err
		}
		keep[s.kind] = items
		for _, it := range items {
			*s.dest = append(*s.dest, toObject(it, s.ready))
		}
	}
	r.netPolicies = keep["NetPolicy"]
	r.secPolicies = keep["SecPolicy"]
	r.persistence = keep["F5BigPersistenceProfile"]

	// Gateway API: keep the Gateways whose class belongs to a BNK controller.
	classes, err := list("GatewayClass", GatewayClassGVR)
	if err != nil {
		return nil, err
	}
	r.classes = classes
	bnkClasses := bnkGatewayClasses(classes)
	gateways, err := list("Gateway", GatewayGVR)
	if err != nil {
		return nil, err
	}
	for _, gw := range gateways {
		cls, _, _ := unstructured.NestedString(gw.Object, "spec", "gatewayClassName")
		if len(bnkClasses) > 0 && !bnkClasses[cls] {
			continue
		}
		o := toObject(gw, requireTrue("Accepted", "Programmed"))
		idx.Gateways = append(idx.Gateways, o)
		r.gateways = append(r.gateways, gw)
		r.gatewayReady[o.ID()] = o.Ready
	}
	routes, err := list("HTTPRoute", HTTPRouteGVR)
	if err != nil {
		return nil, err
	}
	r.httpRoutes = routes
	for _, rt := range routes {
		idx.HTTPRoutes = append(idx.HTTPRoutes, toObject(rt, routeReady))
	}

	// Legacy kinds: counted, never required.
	idx.Legacy.BnkGateways = countAll(ctx, dyn, BnkGatewayGVR)
	idx.Legacy.BNKSecPolicy = countAll(ctx, dyn, BNKSecPolGVR)
	idx.Legacy.BNKNetPolicy = countAll(ctx, dyn, BNKNetPolGVR)
	idx.Legacy.F5SPKVlans = countAll(ctx, dyn, VlanGVR)
	idx.Legacy.F5SPKEgresses = countAll(ctx, dyn, EgressGVR)

	// Controller rollout.
	if cs != nil {
		dep, err := cs.AppsV1().Deployments(opts.ControllerNamespace).Get(ctx, opts.ControllerName, metav1.GetOptions{})
		switch {
		case err == nil:
			idx.Controller.Found = true
			idx.Controller.Desired = 1
			if dep.Spec.Replicas != nil {
				idx.Controller.Desired = *dep.Spec.Replicas
			}
			idx.Controller.Available = dep.Status.AvailableReplicas
			idx.Controller.Ready = idx.Controller.Desired > 0 && idx.Controller.Available >= idx.Controller.Desired
		case apierrors.IsNotFound(err):
		default:
			return nil, fmt.Errorf("get deployment %s/%s: %w", opts.ControllerNamespace, opts.ControllerName, err)
		}
	}

	idx.MCP = DiscoverMCP(r)
	sort.Strings(idx.Missing)
	return idx, nil
}

// listAll lists a GVR across every namespace.
func listAll(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource) ([]*unstructured.Unstructured, error) {
	l, err := dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]*unstructured.Unstructured, 0, len(l.Items))
	for i := range l.Items {
		items = append(items, &l.Items[i])
	}
	sort.Slice(items, func(a, b int) bool {
		if items[a].GetNamespace() != items[b].GetNamespace() {
			return items[a].GetNamespace() < items[b].GetNamespace()
		}
		return items[a].GetName() < items[b].GetName()
	})
	return items, nil
}

// countAll returns the number of objects of a GVR, 0 when the CRD is absent
// or the list fails: legacy kinds are informational.
func countAll(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource) int {
	l, err := dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0
	}
	return len(l.Items)
}

// f5CRDGroups returns the sorted F5 CRD groups the API server serves. When
// CRDs cannot be listed (RBAC), it probes the two policy groups directly.
func f5CRDGroups(ctx context.Context, dyn dynamic.Interface) ([]string, error) {
	seen := map[string]bool{}
	l, err := dyn.Resource(CRDGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for i := range l.Items {
			g, _, _ := unstructured.NestedString(l.Items[i].Object, "spec", "group")
			if strings.HasSuffix(g, "f5.com") || strings.HasSuffix(g, "f5net.com") {
				seen[g] = true
			}
		}
	} else if !apierrors.IsForbidden(err) && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("list CRDs: %w", err)
	} else {
		for g, gvr := range map[string]schema.GroupVersionResource{
			GatewayF5Group:    NetPolicyGVR,
			LegacyPolicyGroup: BNKNetPolGVR,
			DataPlaneGroup:    IRuleGVR,
			FLOGroup:          CNEInstanceGVR,
		} {
			if _, err := dyn.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
				seen[g] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Strings(out)
	return out, nil
}

// bnkGatewayClasses returns the names of the GatewayClasses whose controller
// is an F5 CNE controller. Empty when no class matches, in which case the
// caller keeps every Gateway.
func bnkGatewayClasses(classes []*unstructured.Unstructured) map[string]bool {
	out := map[string]bool{}
	for _, c := range classes {
		ctrl, _, _ := unstructured.NestedString(c.Object, "spec", "controllerName")
		if strings.HasPrefix(ctrl, "f5.com/") || strings.Contains(ctrl, "f5-cne-controller") {
			out[c.GetName()] = true
		}
	}
	return out
}

// toObject converts an unstructured object into an Object with the readiness
// verdict computed by ready.
func toObject(u *unstructured.Unstructured, ready func(*unstructured.Unstructured) (bool, string)) Object {
	o := Object{
		Kind:       u.GetKind(),
		APIVersion: u.GetAPIVersion(),
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
		Conditions: conditionsOf(u.Object, "status", "conditions"),
	}
	o.Ready, o.Detail = ready(u)
	return o
}

// conditionsOf reads a []condition at the given path.
func conditionsOf(obj map[string]any, path ...string) []Condition {
	items, found, _ := unstructured.NestedSlice(obj, path...)
	if !found {
		return nil
	}
	var out []Condition
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		c := Condition{}
		c.Type, _ = m["type"].(string)
		c.Status, _ = m["status"].(string)
		c.Reason, _ = m["reason"].(string)
		c.Message, _ = m["message"].(string)
		out = append(out, c)
	}
	return out
}

// requireTrue is ready only when every named condition exists and is True.
func requireTrue(types ...string) func(*unstructured.Unstructured) (bool, string) {
	return func(u *unstructured.Unstructured) (bool, string) {
		conds := conditionsOf(u.Object, "status", "conditions")
		var missing []string
		for _, t := range types {
			c, ok := findCondition(conds, t)
			switch {
			case !ok:
				missing = append(missing, t+" absent")
			case c.Status != "True":
				missing = append(missing, describe(c))
			}
		}
		if len(missing) == 0 {
			return true, ""
		}
		return false, strings.Join(missing, "; ")
	}
}

// trueIfPresent is ready unless a named condition exists and is not True.
func trueIfPresent(types ...string) func(*unstructured.Unstructured) (bool, string) {
	return func(u *unstructured.Unstructured) (bool, string) {
		conds := conditionsOf(u.Object, "status", "conditions")
		var bad []string
		for _, t := range types {
			if c, ok := findCondition(conds, t); ok && c.Status != "True" {
				bad = append(bad, describe(c))
			}
		}
		if len(bad) == 0 {
			return true, ""
		}
		return false, strings.Join(bad, "; ")
	}
}

// routeReady requires every parent in status.parents to report Accepted=True.
func routeReady(u *unstructured.Unstructured) (bool, string) {
	parents, found, _ := unstructured.NestedSlice(u.Object, "status", "parents")
	if !found || len(parents) == 0 {
		return false, "no parent status"
	}
	var bad []string
	for _, p := range parents {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		conds := conditionsOf(pm, "conditions")
		for _, t := range []string{"Accepted", "ResolvedRefs"} {
			if c, ok := findCondition(conds, t); ok && c.Status != "True" {
				bad = append(bad, describe(c))
			}
		}
	}
	if len(bad) == 0 {
		return true, ""
	}
	return false, strings.Join(bad, "; ")
}

func findCondition(conds []Condition, t string) (Condition, bool) {
	for _, c := range conds {
		if c.Type == t {
			return c, true
		}
	}
	return Condition{}, false
}

func describe(c Condition) string {
	s := c.Type + "=" + c.Status
	if c.Reason != "" {
		s += " (" + c.Reason + ")"
	}
	return s
}
