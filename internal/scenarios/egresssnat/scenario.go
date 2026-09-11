// Package egresssnat implements scenario "egress-snat" — transparent pod
// egress through TMM with AUTOMAP source translation using the pseudo-CNI
// VXLAN overlay (how-to #10, Green).
//
// The worker node has no interface on TMM's VLANs (host-device moves those
// NICs into the TMM pod), so the tunnel runs over the pod network: F5SPKEgress
// with vxlan.create=true and nodeInterfaceName = the node's primary NIC
// (NODE_PRIMARY_IFNAME from Phase 17c). TMM also needs two static routes —
// the VPC via the tunnel VLAN's gateway and a default via ext-vlan's — which
// 04-staticroutes.yaml provides.
//
// Verify:
//
//   - egress-client pod becomes Ready.
//   - F5SPKEgress awsbnkctl-egress is present (status checked when available).
//   - Data path (gating): curls from the pod to an out-of-VPC target must move
//     TMM's own counters — see verifyTMMEgress. Without an exec client the step
//     is recorded as skipped so the result stays honest.
package egresssnat

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/JLCode-tech/awsbnkctl/internal/scenarios"
)

//go:embed manifests/*.yaml
var manifestFS embed.FS

const (
	scnName      = "egress-snat"
	scnTitle     = "Pod egress via TMM with AUTOMAP SNAT (pseudo-CNI VXLAN)"
	scnNamespace = "awsbnkctl-scn-egress"

	// defaultEgressNamespace is where the F5SPKEgress CR is applied.
	// On the live cluster the f5-spk-csrc DaemonSet and F5SPKVlan objects
	// live in f5-cne-system; the egress CR must be co-located.
	// LIVE-CONFIRM: verify the exact namespace on first apply.
	defaultEgressNamespace = "f5-cne-system"

	// defaultTmmIntVlan is the TMM internal-VLAN interface name.
	// Confirmed live: f5-cne-system/int-vlan (selfip 10.0.20.240).
	// LIVE-CONFIRM: verify tmmInterfaceName matches the live F5SPKVlan name.
	defaultTmmIntVlan = "int-vlan"

	// defaultTmmExtVlan is the client-side F5SPKVlan every pattern has. On
	// external-only / sriov-external clusters it is the ONLY VLAN, so the
	// egress tunnel must terminate here (examples/egress-demo/egress-toggle.yaml).
	defaultTmmExtVlan = "ext-vlan"
)

// f5SPKEgressGVR is the GroupVersionResource for F5SPKEgress objects.
// Plural confirmed live from the 2026-05-25 spike: f5-spk-egresses.k8s.f5net.com.
// LIVE-CONFIRM: re-check on the live cluster if the CR apply fails.
var f5SPKEgressGVR = schema.GroupVersionResource{
	Group:    "k8s.f5net.com",
	Version:  "v3",
	Resource: "f5-spk-egresses",
}

// f5SPKStaticRouteGVR addresses the two TMM routes 04-staticroutes.yaml creates.
var f5SPKStaticRouteGVR = schema.GroupVersionResource{
	Group:    "k8s.f5net.com",
	Version:  "v1",
	Resource: "f5-spk-staticroutes",
}

func init() { scenarios.Register(&scenario{}) }

// VerifyDeps holds the function pointers used by Verify. The zero value routes
// every call to the real package-level implementations. Tests swap individual
// fields to a recording stub to assert call order without touching the cluster.
type VerifyDeps struct {
	WaitPodReadyFn   func(ctx context.Context, sctx *scenarios.Context, ns, name string, timeout time.Duration) error
	GetF5SPKEgressFn func(ctx context.Context, sctx *scenarios.Context, ns, name string) (bool, string, error)
	// The two below implement the data-path proof (see verifyTMMEgress). A nil
	// ExecInPodFn skips it and Verify stays control-plane only, which is what
	// the unit tests exercise.
	TMMPodFn    func(sctx *scenarios.Context) (string, error)
	ExecInPodFn func(sctx *scenarios.Context, ns, pod, container string, command ...string) (string, error)
}

const (
	tmmNamespace      = "f5-cne-system"
	tmmPodSelector    = "app=f5-tmm"
	tmmDebugContainer = "debug" // the TMM pod sidecar that ships tmctl
	egressCurlCount   = 3
	// defaultEgressTarget must be OUTSIDE the VPC: the CSRC DaemonSet keeps
	// pod→VPC traffic on the node ("addVpcCniEastWestRule"), so only external
	// destinations traverse the tunnel. Override with --opt egress-target=URL.
	defaultEgressTarget = "http://checkip.amazonaws.com/"
)

// Poll budget for the data-path proof (package vars so tests can shrink them).
var (
	sourceIPPollTimeout  = 2 * time.Minute
	sourceIPPollInterval = 10 * time.Second
)

func realVerifyDeps() VerifyDeps {
	return VerifyDeps{
		WaitPodReadyFn:   waitPodReady,
		GetF5SPKEgressFn: getF5SPKEgress,
		TMMPodFn:         tmmPodName,
		ExecInPodFn:      scenarios.ExecInPod,
	}
}

// tmmPodName returns the first Running TMM pod (the scenario runs single-TMM shapes).
func tmmPodName(sctx *scenarios.Context) (string, error) {
	pods, err := sctx.Clientset.CoreV1().Pods(tmmNamespace).List(sctx.Ctx, metav1.ListOptions{LabelSelector: tmmPodSelector})
	if err != nil {
		return "", fmt.Errorf("listing TMM pods: %w", err)
	}
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			return p.Name, nil
		}
	}
	return "", fmt.Errorf("no Running pod with %s in %s", tmmPodSelector, tmmNamespace)
}

// expectedSNATSource is the address egress must be SNATed to when captured:
// TMM's external SelfIP (AUTOMAP picks the egress interface's SelfIP, and the
// default static route pins that interface to ext-vlan). state.env carries it
// from Phase 17; the intent's derived value is the fallback.
func expectedSNATSource(sctx *scenarios.Context) string {
	return scenarios.NetVarsFor(sctx.Cluster, sctx.State).TMMExtSelfIP
}

// tmmCounters is what TMM has seen for this scenario's egress: connections on
// the egress virtual server and connections SNATed to the external SelfIP.
type tmmCounters struct{ virtual, snat int }

// readTMMCounters pulls both counters with tmctl from the TMM debug sidecar.
func readTMMCounters(d *VerifyDeps, sctx *scenarios.Context, tmmPod, vsName, selfIPHex string) (tmmCounters, error) {
	vs, err := d.ExecInPodFn(sctx, tmmNamespace, tmmPod, tmmDebugContainer,
		"tmctl", "-d", "blade", "-w", "400", "virtual_server_stat", "-s", "name,clientside.tot_conns")
	if err != nil {
		return tmmCounters{}, fmt.Errorf("tmctl virtual_server_stat: %w", err)
	}
	sn, err := d.ExecInPodFn(sctx, tmmNamespace, tmmPod, tmmDebugContainer,
		"tmctl", "-d", "blade", "-w", "400", "pool_member_stat", "-s", "pool_name,addr,serverside.tot_conns")
	if err != nil {
		return tmmCounters{}, fmt.Errorf("tmctl pool_member_stat: %w", err)
	}
	return tmmCounters{virtual: lastIntOnLine(vs, vsName), snat: lastIntOnLine(sn, selfIPHex)}, nil
}

// lastIntOnLine returns the last whitespace-separated integer on the first
// line of out containing needle, or 0 when there is no such line.
func lastIntOnLine(out, needle string) int {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			return 0
		}
		n, _ := strconv.Atoi(f[len(f)-1])
		return n
	}
	return 0
}

// ipv4Hex renders an IPv4 address the way tmctl prints pool-member addresses
// (IPv4-mapped, 20 bytes), e.g. 10.50.10.240 → "FF:FF:0A:32:0A:F0:00:00:00:00".
func ipv4Hex(ip string) string {
	p := net.ParseIP(ip).To4()
	if p == nil {
		return ""
	}
	return fmt.Sprintf("FF:FF:%02X:%02X:%02X:%02X:00:00:00:00", p[0], p[1], p[2], p[3])
}

// egressVirtualName is the listener TMM creates for an F5SPKEgress: <ns>-<cr>-egress-ipv4.
func egressVirtualName(egressNS string) string { return egressNS + "-awsbnkctl-egress-egress-ipv4" }

// verifyTMMEgress is the data-path proof: curl an out-of-VPC target FROM the
// captured pod and require TMM's own counters to move by the same amount —
// connections on the egress virtual server (traffic traversed the tunnel) and
// connections SNATed to the external SelfIP (AUTOMAP rewrote the source). It
// reads TMM, not a reflector, because the pseudo-CNI keeps pod→VPC traffic on
// the node, so nothing inside the VPC can observe the SelfIP. It returns
// (skipped, assertion); skipped is true when there is no exec client, so the
// scenario stays honest rather than red on control-plane-only runs.
func verifyTMMEgress(d *VerifyDeps, sctx *scenarios.Context, podNS, egressNS string) (bool, scenarios.Assertion) {
	want := expectedSNATSource(sctx)
	if d.ExecInPodFn == nil || d.TMMPodFn == nil || want == "" {
		return true, scenarios.Assertion{
			Description: "data-plane proof (TMM egress virtual + SNAT counters)",
			OK:          true,
			Got:         "skipped — needs a Kubernetes exec client and TMM_EXT_SELFIP in state.env",
		}
	}
	tmmPod, err := d.TMMPodFn(sctx)
	if err != nil {
		return false, scenarios.Assertion{Description: "locate the TMM pod", OK: false, Got: err.Error()}
	}
	target := sctx.Options["egress-target"]
	if target == "" {
		target = defaultEgressTarget
	}
	vsName, selfHex := egressVirtualName(egressNS), ipv4Hex(want)
	// The CSRC DaemonSet programs the worker-node VXLAN end asynchronously
	// after the CR is accepted; give it a couple of minutes.
	ok, detail := scenarios.PollMarkers(sctx.Ctx, sourceIPPollTimeout, sourceIPPollInterval, func() (bool, string) {
		before, err := readTMMCounters(d, sctx, tmmPod, vsName, selfHex)
		if err != nil {
			return false, err.Error()
		}
		okCurls := 0
		for i := 0; i < egressCurlCount; i++ {
			if _, err := d.ExecInPodFn(sctx, podNS, "egress-client", "curl", "curl", "-s", "-o", "/dev/null", "--max-time", "8", target); err == nil {
				okCurls++
			}
		}
		after, err := readTMMCounters(d, sctx, tmmPod, vsName, selfHex)
		if err != nil {
			return false, err.Error()
		}
		dv, ds := after.virtual-before.virtual, after.snat-before.snat
		pass := okCurls == egressCurlCount && dv >= egressCurlCount && ds >= egressCurlCount
		return pass, fmt.Sprintf("%d/%d curls from egress-client to %s succeeded; TMM egress virtual %s +%d conns; SNAT to %s +%d conns",
			okCurls, egressCurlCount, target, vsName, dv, want, ds)
	})
	return false, scenarios.Assertion{
		Description: "pod egress traverses TMM and is SNATed to the external SelfIP (egress virtual + AUTOMAP counters)",
		OK:          ok,
		Got:         detail,
	}
}

type scenario struct {
	// vDeps is nil for the registered singleton; tests inject a non-nil value.
	vDeps *VerifyDeps
}

func (s *scenario) Name() string             { return scnName }
func (s *scenario) Title() string            { return scnTitle }
func (s *scenario) Rating() scenarios.Rating { return scenarios.Green }
func (s *scenario) Dependencies() []string   { return []string{} }
func (s *scenario) Description() string {
	return strings.TrimSpace(`
Egress SNAT scenario — transparent pod outbound traffic through TMM via
pseudo-CNI VXLAN overlay with AUTOMAP source translation (how-to #10).

The F5SPKEgress CR (snatType: SRC_TRANS_AUTOMAP, vxlan.create=true) makes the
BNK controller build a VXLAN tunnel end on TMM and the CSRC DaemonSet build
the other end on each worker's primary NIC (nodeInterfaceName). Traffic from
the captured namespace to destinations outside the VPC is carried to TMM and
source-translated to the external SelfIP; pod-to-VPC traffic stays on the node.

Applies 4 templated manifests:
  01-namespace.yaml     — scenario namespace (awsbnkctl-scn-egress)
  02-curlpod.yaml       — egress-client pod (curlimages/curl, normal pod network)
  03-f5spkegress.yaml   — F5SPKEgress CR in EgressNamespace (f5-cne-system)
  04-staticroutes.yaml  — TMM routes: VPC via the tunnel VLAN gateway, default
                          via the external VLAN gateway (needs cluster.yaml)

Verify:
  1. egress-client pod Ready.
  2. F5SPKEgress awsbnkctl-egress present (status conditions when available).
  3. Data path: 3 curls from the pod to an out-of-VPC URL (default
     http://checkip.amazonaws.com/) must raise TMM's egress virtual-server
     connections and its SNAT-to-external-SelfIP connections by 3 (tmctl in
     the TMM debug sidecar).

Cleanup: delete the scenario namespace, the F5SPKEgress CR and the two
F5SPKStaticRoute CRs (idempotent; not-found ignored).
`)
}

// manifestVars holds the template variables for the manifests.
type manifestVars struct {
	scenarios.NetVars // NodeIfname (VXLAN endpoint NIC), VpcNet/VpcPrefixLen, ExtGateway/IntGateway
	Namespace         string
	EgressNamespace   string
	TmmIntVlan        string
	// TunnelGateway is the router of the VLAN the tunnel ends on: VXLAN replies
	// to workers must leave on the VLAN they arrived on (04-staticroutes.yaml,
	// skipped when there is no cluster.yaml to derive gateways from).
	TunnelGateway string
}

const staticRoutesManifest = "04-staticroutes.yaml"

func (s *scenario) Manifests(ctx *scenarios.Context) ([]string, error) {
	v := buildManifestVars(ctx)

	var paths []string
	err := fs.WalkDir(manifestFS, "manifests", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return werr
		}
		if strings.HasSuffix(p, staticRoutesManifest) && v.ExtGateway == "" {
			return nil
		}
		tmplBytes, e := manifestFS.ReadFile(p)
		if e != nil {
			return e
		}
		rendered, e := scenarios.RenderTemplate(string(tmplBytes), v)
		if e != nil {
			return fmt.Errorf("rendering %s: %w", p, e)
		}
		base := p[len("manifests/"):]
		out, e := scenarios.WriteManifest(ctx.WorkspaceDir, scnName, base, rendered)
		if e != nil {
			return e
		}
		paths = append(paths, out)
		return nil
	})
	return paths, err
}

func (s *scenario) Apply(ctx *scenarios.Context) error {
	return scenarios.ApplyManifests(ctx, scnName)
}

func (s *scenario) Verify(ctx *scenarios.Context) scenarios.Result {
	d := s.vDeps
	if d == nil {
		real := realVerifyDeps()
		d = &real
	}
	v := buildManifestVars(ctx)
	res := scenarios.Result{}

	// 1. egress-client pod Ready.
	err := d.WaitPodReadyFn(ctx.Ctx, ctx, v.Namespace, "egress-client", 3*time.Minute)
	res.Assertions = append(res.Assertions, scenarios.Assertion{
		Description: "egress-client pod Ready",
		OK:          err == nil,
		Got:         scenarios.ErrString(err),
	})

	// 2. F5SPKEgress awsbnkctl-egress present (+ status if available).
	present, got, err := d.GetF5SPKEgressFn(ctx.Ctx, ctx, v.EgressNamespace, "awsbnkctl-egress")
	if err != nil {
		got = scenarios.ErrString(err)
	}
	res.Assertions = append(res.Assertions, scenarios.Assertion{
		Description: "F5SPKEgress awsbnkctl-egress present in " + v.EgressNamespace,
		OK:          present,
		Got:         got,
	})

	// 3. Data path, read from TMM itself (gating); without an exec client it
	// is recorded as skipped so the result stays honest.
	skipped, a := verifyTMMEgress(d, ctx, v.Namespace, v.EgressNamespace)
	res.Assertions = append(res.Assertions, a)
	if skipped {
		res.Details = "Control plane only on this run — the TMM-counter data-path proof needs a Kubernetes exec client."
	} else {
		res.DataPath = true
		res.Details = "Control plane + data path: the pod's out-of-VPC curls moved TMM's egress-virtual and SNAT-to-external-SelfIP counters."
	}

	return scenarios.FinalizeResult(res)
}

func (s *scenario) Cleanup(ctx *scenarios.Context) error {
	v := buildManifestVars(ctx)

	// Delete the scenario namespace (idempotent).
	err := ctx.Clientset.CoreV1().Namespaces().Delete(ctx.Ctx, v.Namespace, metav1.DeleteOptions{})
	if err != nil && !scenarios.IsNotFound(err) {
		return fmt.Errorf("deleting namespace %s: %w", v.Namespace, err)
	}

	// Delete the F5SPKEgress CR and the two static routes from EgressNamespace (idempotent).
	for _, r := range []struct {
		gvr  schema.GroupVersionResource
		name string
	}{
		{f5SPKEgressGVR, "awsbnkctl-egress"},
		{f5SPKStaticRouteGVR, "awsbnkctl-egress-vpc"},
		{f5SPKStaticRouteGVR, "awsbnkctl-egress-default"},
	} {
		err := ctx.Dynamic.Resource(r.gvr).Namespace(v.EgressNamespace).Delete(ctx.Ctx, r.name, metav1.DeleteOptions{})
		if err != nil && !scenarios.IsNotFound(err) {
			return fmt.Errorf("deleting %s %s from %s: %w", r.gvr.Resource, r.name, v.EgressNamespace, err)
		}
	}
	return nil
}

func (s *scenario) Namespace(ctx *scenarios.Context) string {
	return namespace(ctx)
}

// --- internal helpers ---

func namespace(ctx *scenarios.Context) string {
	if v := ctx.Options["namespace"]; v != "" {
		return v
	}
	return scnNamespace
}

func egressNamespace(ctx *scenarios.Context) string {
	if v := ctx.Options["egress-namespace"]; v != "" {
		return v
	}
	return defaultEgressNamespace
}

// tmmIntVlan picks the F5SPKVlan the VXLAN tunnel terminates on. Phase 23b
// names them ext-vlan (every pattern) and int-vlan (dual-interface only), so
// the default follows the cluster's pattern: single-interface clusters have no
// int-vlan and must use ext-vlan — the shape examples/egress-demo validated
// end to end. Dual-interface keeps int-vlan. A nil cluster (unit tests, no
// --config) falls back to int-vlan for backwards compatibility.
func tmmIntVlan(ctx *scenarios.Context) string {
	if v := ctx.Options["tmm-int-vlan"]; v != "" {
		return v
	}
	if ctx.Cluster != nil && ctx.Cluster.IsBNKPattern() && !ctx.Cluster.HasInternalInterface() {
		return defaultTmmExtVlan
	}
	return defaultTmmIntVlan
}

func buildManifestVars(ctx *scenarios.Context) manifestVars {
	v := manifestVars{
		NetVars:         scenarios.NetVarsFor(ctx.Cluster, ctx.State),
		Namespace:       namespace(ctx),
		EgressNamespace: egressNamespace(ctx),
		TmmIntVlan:      tmmIntVlan(ctx),
	}
	if o := ctx.Options["node-interface"]; o != "" {
		v.NodeIfname = o
	}
	v.TunnelGateway = v.ExtGateway
	if v.TmmIntVlan == defaultTmmIntVlan && v.IntGateway != "" {
		v.TunnelGateway = v.IntGateway
	}
	return v
}

// waitPodReady polls until the named Pod has a Ready condition True, or timeout.
func waitPodReady(ctx context.Context, sctx *scenarios.Context, ns, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := sctx.Clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("pod %s/%s not Ready after %s", ns, name, timeout)
}

// getF5SPKEgress fetches the F5SPKEgress CR and checks status conditions.
// Returns (present, got, err). If present, got describes the status.
func getF5SPKEgress(ctx context.Context, sctx *scenarios.Context, ns, name string) (bool, string, error) {
	obj, err := sctx.Dynamic.Resource(f5SPKEgressGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if scenarios.IsNotFound(err) {
			return false, "not found", nil
		}
		return false, "", err
	}

	// Check .status.conditions for Ready or Programmed.
	conditions, found, _ := nestedSlice(obj.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		return true, "present (no status conditions yet)", nil
	}
	for _, cRaw := range conditions {
		c, ok := cRaw.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := c["type"].(string)
		st, _ := c["status"].(string)
		if (t == "Ready" || t == "Programmed") && st == "True" {
			return true, fmt.Sprintf("present; %s=True", t), nil
		}
	}
	return true, "present; no Ready/Programmed=True condition yet", nil
}

// nestedSlice extracts a []interface{} from an unstructured object following
// the given field path. Mirrors k8s.io/apimachinery/pkg/apis/meta/v1/unstructured.NestedSlice
// without the import cycle.
func nestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	m := obj
	for i, f := range fields {
		if i == len(fields)-1 {
			v, ok := m[f]
			if !ok {
				return nil, false, nil
			}
			s, ok := v.([]interface{})
			if !ok {
				return nil, false, fmt.Errorf("field %q is not a slice", f)
			}
			return s, true, nil
		}
		next, ok := m[f]
		if !ok {
			return nil, false, nil
		}
		m, ok = next.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("field %q is not a map", f)
		}
	}
	return nil, false, nil
}
