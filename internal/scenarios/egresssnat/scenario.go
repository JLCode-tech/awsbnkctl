// Package egresssnat implements scenario "egress-snat" — transparent pod
// egress through TMM with AUTOMAP source translation using the pseudo-CNI
// VXLAN overlay (how-to #10, Amber).
//
// On the awsbnkctl host-device topology the internal NIC (ens7) is moved into
// the TMM pod, so the worker node has no internal-VLAN interface to use for
// the appNodeInterface mode. The VXLAN overlay is required: F5SPKEgress with
// vxlan.create=true creates the TMM tunnel end inline; the CSRC DaemonSet
// (f5-spk-csrc, already running) programs the worker-node end.
//
// Verify:
//
//   - egress-client pod becomes Ready (proves namespace + image pull OK).
//   - F5SPKEgress awsbnkctl-egress is present in EgressNamespace (proves CR
//     accepted by the API server; status conditions checked when available).
//   - Data path (gating when the cluster has a jumphost): a source-IP reflector
//     is started on the jumphost's data-path ENI, the egress-client pod curls
//     it, and the source the reflector saw must be TMM's external SelfIP —
//     see verifySourceIP. Without jumphost state the step is recorded as
//     skipped, so the result stays honest on clusters built without one.
//
// Rating stays Amber until the data-path step has passed on a live cluster.
package egresssnat

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/JLCode-tech/awsbnkctl/internal/jumphost"
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

func init() { scenarios.Register(&scenario{}) }

// VerifyDeps holds the function pointers used by Verify. The zero value routes
// every call to the real package-level implementations. Tests swap individual
// fields to a recording stub to assert call order without touching the cluster.
type VerifyDeps struct {
	WaitPodReadyFn   func(ctx context.Context, sctx *scenarios.Context, ns, name string, timeout time.Duration) error
	GetF5SPKEgressFn func(ctx context.Context, sctx *scenarios.Context, ns, name string) (bool, string, error)
	// The three below implement the data-path proof (see verifySourceIP). A nil
	// StartResponderFn skips it and Verify stays control-plane only, which is
	// what the unit tests exercise.
	StartResponderFn func(ctx context.Context, sctx *scenarios.Context, port int) error
	StopResponderFn  func(ctx context.Context, sctx *scenarios.Context, port int)
	ExecInPodFn      func(sctx *scenarios.Context, ns, pod, container string, command ...string) (string, error)
}

// reflectorPort is where the jumphost source-IP reflector listens. 8080 is
// external-resource-pool's marker responder; keep clear of it.
const reflectorPort = 8081

// Poll budget for the data-path proof (package vars so tests can shrink them).
var (
	sourceIPPollTimeout  = 2 * time.Minute
	sourceIPPollInterval = 10 * time.Second
)

func realVerifyDeps() VerifyDeps {
	return VerifyDeps{
		WaitPodReadyFn:   waitPodReady,
		GetF5SPKEgressFn: getF5SPKEgress,
		StartResponderFn: func(ctx context.Context, sctx *scenarios.Context, port int) error {
			return jumphost.StartSourceIPResponder(ctx, probeOpts(sctx), port)
		},
		StopResponderFn: func(ctx context.Context, sctx *scenarios.Context, port int) {
			_ = jumphost.StopHTTPResponder(ctx, probeOpts(sctx), port)
		},
		ExecInPodFn: scenarios.ExecInPod,
	}
}

func probeOpts(sctx *scenarios.Context) jumphost.ProbeOptions {
	return jumphost.ProbeOptions{
		Region:     sctx.Cluster.Metadata.Region,
		InstanceID: sctx.State.Get("JUMPHOST_INSTANCE_ID"),
		SourceIP:   sctx.State.Get("JUMPHOST_BNK_EXT_ENI_IP"),
	}
}

// expectedSNATSource is the address the reflector must see when egress is
// captured: TMM's external SelfIP (AUTOMAP picks the egress interface's
// SelfIP). state.env carries it from Phase 17; the intent's derived value is
// the fallback.
func expectedSNATSource(sctx *scenarios.Context) string {
	if sctx.State != nil {
		if v := sctx.State.Get("TMM_EXT_SELFIP"); v != "" {
			return v
		}
	}
	if sctx.Cluster != nil && sctx.Cluster.Network.DataPath != nil {
		return sctx.Cluster.Network.DataPath.SelfIPs.External
	}
	return ""
}

// verifySourceIP is the Amber→Green data-path proof: start a reflector on the
// jumphost's data-path ENI, curl it FROM the captured pod, and check the
// source the reflector saw is TMM's external SelfIP — i.e. the pod's egress
// really went pod → VXLAN → TMM → AUTOMAP SNAT. It returns (skipped, assertion).
// skipped is true when the cluster shape cannot run it (no jumphost state);
// the caller records that as informational so the scenario is honest rather
// than red on a cluster built without a jumphost.
func verifySourceIP(d *VerifyDeps, sctx *scenarios.Context, podNS string) (bool, scenarios.Assertion) {
	if d.StartResponderFn == nil || d.ExecInPodFn == nil || sctx.Cluster == nil || sctx.State == nil ||
		sctx.State.Get("JUMPHOST_INSTANCE_ID") == "" || sctx.State.Get("JUMPHOST_BNK_EXT_ENI_IP") == "" {
		return true, scenarios.Assertion{
			Description: "data-plane SNAT source-IP proof",
			OK:          true,
			Got:         "skipped — needs testing.jumphost.enabled=true (JUMPHOST_INSTANCE_ID / JUMPHOST_BNK_EXT_ENI_IP in state.env); AUTOMAP should present source IP = TMM external SelfIP",
		}
	}
	want := expectedSNATSource(sctx)
	reflector := sctx.State.Get("JUMPHOST_BNK_EXT_ENI_IP")
	if err := d.StartResponderFn(sctx.Ctx, sctx, reflectorPort); err != nil {
		return false, scenarios.Assertion{Description: "source-IP reflector on the jumphost data-path ENI", OK: false, Got: err.Error()}
	}
	if d.StopResponderFn != nil {
		defer d.StopResponderFn(sctx.Ctx, sctx, reflectorPort)
	}
	url := fmt.Sprintf("http://%s:%d/", reflector, reflectorPort)
	var seen, lastErr string
	// The CSRC DaemonSet programs the worker-node VXLAN end asynchronously
	// after the CR is accepted; give it a couple of minutes.
	ok, detail := scenarios.PollMarkers(sctx.Ctx, sourceIPPollTimeout, sourceIPPollInterval, func() (bool, string) {
		out, err := d.ExecInPodFn(sctx, podNS, "egress-client", "curl", "curl", "-s", "--max-time", "8", url)
		if err != nil {
			lastErr = err.Error()
			return false, "curl from egress-client failed: " + lastErr
		}
		seen = strings.TrimSpace(out)
		return seen == want, fmt.Sprintf("reflector %s saw source %q, want TMM external SelfIP %q", url, seen, want)
	})
	return false, scenarios.Assertion{
		Description: "pod egress leaves TMM with source IP = TMM external SelfIP (AUTOMAP SNAT proven end to end)",
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
func (s *scenario) Rating() scenarios.Rating { return scenarios.Amber }
func (s *scenario) Dependencies() []string   { return []string{} }
func (s *scenario) Description() string {
	return strings.TrimSpace(`
Egress SNAT scenario — transparent pod outbound traffic through TMM via
pseudo-CNI VXLAN overlay with AUTOMAP source translation (how-to #10).

The F5SPKEgress CR (snatType: SRC_TRANS_AUTOMAP) with pseudoCNIConfig
vxlan.create=true causes the BNK controller to:
  1. Create a VXLAN tunnel on the TMM side (tmmInterfaceName=int-vlan).
  2. Signal the CSRC DaemonSet to program the worker-node VXLAN end.
  3. Intercept traffic from designated namespaces on eth0 and route it
     through TMM, source-translating to the external self-IP (10.0.10.240).

Applies 3 templated manifests:
  01-namespace.yaml    — scenario namespace (awsbnkctl-scn-egress)
  02-curlpod.yaml      — egress-client pod (curlimages/curl:8.10.1, eth0 only,
                         no NAD annotation — normal pod network)
  03-f5spkegress.yaml  — F5SPKEgress CR in EgressNamespace (f5-cne-system)

Verify (control-plane only, Amber):
  1. egress-client pod Ready (namespace + image pull OK).
  2. F5SPKEgress awsbnkctl-egress present in EgressNamespace; status
     conditions checked when available.
  3. Informational: data-plane SNAT source-IP proof deferred to live
     validation — the Amber→Green promotion gate.

Cleanup: delete the scenario namespace (idempotent) AND delete the
F5SPKEgress CR from EgressNamespace (idempotent; not-found ignored).

LIVE-CONFIRM items:
  - Exact namespace the F5SPKEgress CR must live in (default: f5-cne-system).
  - tmmInterfaceName value matching the live F5SPKVlan (default: int-vlan).
  - F5SPKEgress status condition names (Ready / Programmed) if any.
`)
}

// manifestVars holds the template variables for the 3 manifests.
type manifestVars struct {
	Namespace       string
	EgressNamespace string
	TmmIntVlan      string
}

func (s *scenario) Manifests(ctx *scenarios.Context) ([]string, error) {
	v := buildManifestVars(ctx)

	var paths []string
	err := fs.WalkDir(manifestFS, "manifests", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return werr
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
	res := scenarios.Result{
		Details: "Amber: control-plane only — data-plane SNAT source-IP proof (pod egress observed source == TMM external self-IP 10.0.10.240) is deferred to live validation.",
	}

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

	// 3. Data path. With a jumphost this is the real proof (gating); without
	// one it is recorded as skipped so the result stays honest.
	skipped, a := verifySourceIP(d, ctx, v.Namespace)
	res.Assertions = append(res.Assertions, a)
	if skipped {
		res.Details = "Amber: control-plane only on this run — data-plane SNAT source-IP proof needs the jumphost (testing.jumphost.enabled=true)."
	} else {
		res.DataPath = true
		res.Details = "Control plane + data path: the reflector on the jumphost data-path ENI reports the source the pod's egress arrived from."
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

	// Delete the F5SPKEgress CR from EgressNamespace (idempotent).
	err = ctx.Dynamic.Resource(f5SPKEgressGVR).Namespace(v.EgressNamespace).Delete(
		ctx.Ctx, "awsbnkctl-egress", metav1.DeleteOptions{},
	)
	if err != nil && !scenarios.IsNotFound(err) {
		return fmt.Errorf("deleting F5SPKEgress awsbnkctl-egress from %s: %w", v.EgressNamespace, err)
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
	return manifestVars{
		Namespace:       namespace(ctx),
		EgressNamespace: egressNamespace(ctx),
		TmmIntVlan:      tmmIntVlan(ctx),
	}
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
