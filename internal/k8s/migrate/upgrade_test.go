package migrate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// fakeHelm implements phases.HelmInstaller for the upgrade tests.
type fakeHelm struct {
	deployed  string // chart version of the existing release ("" = none)
	pulled    []string
	upgraded  []string
	upgradeTo string
	values    map[string]interface{}
	failPull  bool
}

func (f *fakeHelm) List(_, _ string) ([]*release.Release, error) {
	if f.deployed == "" {
		return nil, nil
	}
	return []*release.Release{{
		Name:   "f5-lifecycle-operator",
		Chart:  &chart.Chart{Metadata: &chart.Metadata{Version: f.deployed}},
		Config: map[string]interface{}{"license": map[string]interface{}{"jwt": "old"}},
		Info:   &release.Info{Status: release.StatusDeployed},
	}}, nil
}

func (f *fakeHelm) Install(string, string, *chart.Chart, map[string]interface{}) (*release.Release, error) {
	return nil, errors.New("install must not be called by upgrade")
}

func (f *fakeHelm) Upgrade(name, _ string, ch *chart.Chart, values map[string]interface{}) (*release.Release, error) {
	f.upgraded = append(f.upgraded, name)
	f.upgradeTo = ch.Metadata.Version
	f.values = values
	return &release.Release{Name: name}, nil
}

func (f *fakeHelm) Uninstall(string, string) error { return nil }

func (f *fakeHelm) PullAndLoad(ref, version string) (*chart.Chart, error) {
	if f.failPull {
		return nil, errors.New("registry unreachable")
	}
	f.pulled = append(f.pulled, ref+"@"+version)
	return &chart.Chart{Metadata: &chart.Metadata{Name: "f5-lifecycle-operator", Version: version}}, nil
}

// cneReady returns the fixture CNEInstance with both data-path conditions True.
func cneReady(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	for _, o := range parseFixture(t, fixture23) {
		if o.GetKind() == "CNEInstance" {
			_ = unstructured.SetNestedSlice(o.Object, []any{
				map[string]any{"type": "CNEControllerAvailable", "status": "True"},
				map[string]any{"type": "F5TmmAvailable", "status": "True"},
				map[string]any{"type": "Available", "status": "False"},
			}, "status", "conditions")
			return o
		}
	}
	t.Fatal("no CNEInstance in fixture")
	return nil
}

func infraCRD() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": InfraCRDName},
	}}
}

// controllerObjects returns a ready controller Deployment + pod (with or
// without the flag) and n Ready TMM pods.
func controllerObjects(withFlag bool, tmm int) []runtime.Object {
	env := []corev1.EnvVar{{Name: "CLOUD_PROVIDER", Value: "aws"}}
	if withFlag {
		env = append(env, corev1.EnvVar{Name: UseGatewaySettingsEnv, Value: "true"})
	}
	sel := map[string]string{"app": "f5-cne-controller"}
	objs := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: ControllerDeployment, Namespace: DefaultInstanceNamespace},
			Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: sel}},
			Status:     appsv1.DeploymentStatus{Replicas: 1, AvailableReplicas: 1},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "f5-cne-controller-abc", Namespace: DefaultInstanceNamespace, Labels: sel},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "f5-cne-controller", Env: env}}},
		},
	}
	for i := 0; i < tmm; i++ {
		objs = append(objs, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "f5-tmm-" + string(rune('a'+i)), Namespace: DefaultInstanceNamespace, Labels: map[string]string{"app": "f5-tmm"}},
			Status:     corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "f5-tmm", Ready: true}, {Name: "f5-tmm-routing", Ready: true}}},
		})
	}
	return objs
}

func TestUpgrade_Full(t *testing.T) {
	helm := &fakeHelm{deployed: "v2.21.13-0.0.64"}
	dyn := newFakeDynamic(t, cneReady(t), infraCRD())
	cs := k8sfake.NewClientset(controllerObjects(true, 2)...)
	var log bytes.Buffer
	res, err := Upgrade(context.Background(), UpgradeDeps{Helm: helm, Dyn: dyn, K8s: cs}, UpgradeOptions{
		ManifestVersion: "2.4.0-3.3175.0+0.0.380",
		Values:          map[string]any{"license": map[string]any{"jwt": "new"}},
		Timeout:         2 * time.Second,
		Poll:            5 * time.Millisecond,
		Log:             &log,
	})
	if err != nil {
		t.Fatalf("Upgrade: %v\n%s", err, log.String())
	}
	if res.ManifestTo != "2.4.0" || res.FLOTo != "v2.30.0-0.5.2" || res.FLOFrom != "v2.21.13-0.0.64" || res.ManifestFrom != "2.3.3-3.2598.3-0.0.509" {
		t.Errorf("result = %+v", res)
	}
	if !res.FLOUpgraded || !res.Patched || !res.ControllerReady || res.TMMReady != 2 || res.TMMTotal != 2 {
		t.Errorf("result flags = %+v", res)
	}
	if len(helm.pulled) != 1 || !strings.HasSuffix(helm.pulled[0], "f5-lifecycle-operator@v2.30.0-0.5.2") || helm.upgradeTo != "v2.30.0-0.5.2" {
		t.Errorf("helm pulled %v upgraded to %q", helm.pulled, helm.upgradeTo)
	}
	if helm.values["license"].(map[string]any)["jwt"] != "new" {
		t.Errorf("helm values = %v (rendered values must win over the release's)", helm.values)
	}
	if !strings.Contains(log.String(), `rewritten to "2.4.0"`) {
		t.Errorf("log lacks the version rewrite note:\n%s", log.String())
	}

	// The CNEInstance carries the new version and the env upserts, keeping
	// the existing entries.
	cne, err := dyn.Resource(CNEInstanceGVR).Namespace(DefaultInstanceNamespace).Get(context.Background(), "lab-bnk", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v := stringAt(cne.Object, "spec", "manifestVersion"); v != "2.4.0" {
		t.Errorf("manifestVersion = %q", v)
	}
	ctrlEnv, _, _ := unstructured.NestedSlice(cne.Object, "spec", "advanced", "cneController", "env")
	want := map[string]string{"TMM_DEFAULT_MTU": "9000", "CLOUD_PROVIDER": "aws", UseGatewaySettingsEnv: "true", MaxActiveTMMEnv: "32"}
	if got := envMap(ctrlEnv); len(got) != len(want) {
		t.Errorf("controller env = %v", got)
	} else {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("controller env %s = %q, want %q", k, got[k], v)
			}
		}
	}
	tmmEnv, _, _ := unstructured.NestedSlice(cne.Object, "spec", "advanced", "tmm", "env")
	if got := envMap(tmmEnv); got["TMM_CALICO_ROUTER"] != "default" || got["ZEBOS_STATE"] != "legacy" {
		t.Errorf("tmm env = %v", got)
	}
}

func envMap(env []any) map[string]string {
	out := map[string]string{}
	for _, e := range env {
		m := e.(map[string]any)
		out[m["name"].(string)] = m["value"].(string)
	}
	return out
}

func TestUpgrade_DryRun(t *testing.T) {
	helm := &fakeHelm{deployed: "v2.21.13-0.0.64"}
	dyn := newFakeDynamic(t, cneReady(t))
	var log bytes.Buffer
	res, err := Upgrade(context.Background(), UpgradeDeps{Helm: helm, Dyn: dyn}, UpgradeOptions{DryRun: true, Log: &log})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if len(helm.upgraded) != 0 || len(helm.pulled) != 0 || res.Patched || res.FLOUpgraded || !res.DryRun {
		t.Errorf("dry-run changed something: helm=%+v res=%+v", helm, res)
	}
	out := log.String()
	for _, want := range []string{
		"would helm upgrade f5-lifecycle-operator v2.21.13-0.0.64 -> v2.30.0-0.5.2",
		`would patch CNEInstance f5-cne-system/lab-bnk (manifestVersion "2.3.3-3.2598.3-0.0.509" -> "2.4.0")`,
		`"USE_GATEWAY_SETTINGS","value":"true"`,
		"step 3: dry-run: would wait",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	cne, _ := dyn.Resource(CNEInstanceGVR).Namespace(DefaultInstanceNamespace).Get(context.Background(), "lab-bnk", metav1.GetOptions{})
	if v := stringAt(cne.Object, "spec", "manifestVersion"); v != "2.3.3-3.2598.3-0.0.509" {
		t.Errorf("dry-run patched the CNEInstance: %q", v)
	}
	if res.ManifestTo != "2.4.0" {
		t.Errorf("default target = %q", res.ManifestTo)
	}
}

func TestUpgrade_AlreadyCurrent(t *testing.T) {
	helm := &fakeHelm{deployed: "v2.30.0-0.5.2"}
	cne := cneReady(t)
	_ = unstructured.SetNestedField(cne.Object, "2.4.0", "spec", "manifestVersion")
	env, _, _ := unstructured.NestedSlice(cne.Object, "spec", "advanced", "cneController", "env")
	env = append(env, map[string]any{"name": UseGatewaySettingsEnv, "value": "true"}, map[string]any{"name": MaxActiveTMMEnv, "value": "64"})
	_ = unstructured.SetNestedSlice(cne.Object, env, "spec", "advanced", "cneController", "env")
	tmmEnv, _, _ := unstructured.NestedSlice(cne.Object, "spec", "advanced", "tmm", "env")
	_ = unstructured.SetNestedSlice(cne.Object, append(tmmEnv, map[string]any{"name": "ZEBOS_STATE", "value": "legacy"}), "spec", "advanced", "tmm", "env")
	dyn := newFakeDynamic(t, cne, infraCRD())
	cs := k8sfake.NewClientset(controllerObjects(true, 1)...)
	var log bytes.Buffer
	res, err := Upgrade(context.Background(), UpgradeDeps{Helm: helm, Dyn: dyn, K8s: cs}, UpgradeOptions{Timeout: time.Second, Poll: time.Millisecond, Log: &log})
	if err != nil {
		t.Fatalf("Upgrade: %v\n%s", err, log.String())
	}
	if res.FLOUpgraded || res.Patched || len(helm.upgraded) != 0 {
		t.Errorf("idempotent run changed something: %+v", res)
	}
	if !strings.Contains(log.String(), "already at v2.30.0-0.5.2") || !strings.Contains(log.String(), "already at manifestVersion 2.4.0") {
		t.Errorf("log:\n%s", log.String())
	}
}

func TestUpgrade_Errors(t *testing.T) {
	ctx := context.Background()
	// No release.
	_, err := Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{}, Dyn: newFakeDynamic(t)}, UpgradeOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("no release: %v", err)
	}
	// Unknown manifest without --flo-version.
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "x"}, Dyn: newFakeDynamic(t)}, UpgradeOptions{ManifestVersion: "9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "not in the known releases") {
		t.Errorf("unknown manifest: %v", err)
	}
	// Pull failure surfaces.
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "x", failPull: true}, Dyn: newFakeDynamic(t, cneReady(t))}, UpgradeOptions{})
	if err == nil || !strings.Contains(err.Error(), "registry unreachable") {
		t.Errorf("pull failure: %v", err)
	}
	// No CNEInstance.
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "v2.30.0-0.5.2"}, Dyn: newFakeDynamic(t)}, UpgradeOptions{})
	if err == nil || !strings.Contains(err.Error(), "no CNEInstance") {
		t.Errorf("no instance: %v", err)
	}
	// Controller rolled out without the flag.
	dyn := newFakeDynamic(t, cneReady(t), infraCRD())
	cs := k8sfake.NewClientset(controllerObjects(false, 1)...)
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "v2.30.0-0.5.2"}, Dyn: dyn, K8s: cs}, UpgradeOptions{Timeout: time.Second, Poll: time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "do not carry USE_GATEWAY_SETTINGS=true") {
		t.Errorf("missing flag: %v", err)
	}
	// No TMM pods.
	dyn = newFakeDynamic(t, cneReady(t), infraCRD())
	cs = k8sfake.NewClientset(controllerObjects(true, 0)...)
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "v2.30.0-0.5.2"}, Dyn: dyn, K8s: cs}, UpgradeOptions{Timeout: time.Second, Poll: time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "TMM data plane not ready: 0/0") {
		t.Errorf("no TMM: %v", err)
	}
	// Condition never True -> the wait names it.
	cne := cneReady(t)
	_ = unstructured.SetNestedSlice(cne.Object, []any{map[string]any{"type": "F5TmmAvailable", "status": "False", "reason": "Pending", "message": "tmm-0 starting"}}, "status", "conditions")
	dyn = newFakeDynamic(t, cne, infraCRD())
	_, err = Upgrade(ctx, UpgradeDeps{Helm: &fakeHelm{deployed: "v2.30.0-0.5.2"}, Dyn: dyn}, UpgradeOptions{Timeout: 30 * time.Millisecond, Poll: 5 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "no CNEControllerAvailable condition yet") {
		t.Errorf("stuck condition: %v", err)
	}
}

func TestCanonicalManifestVersion(t *testing.T) {
	cases := map[string]string{
		"2.4.0-3.3175.0+0.0.380": "2.4.0",
		"2.4.0-3.3175.0-0.0.380": "2.4.0",
		"2.4.0":                  "2.4.0",
		"2.3.3-3.2598.3-0.0.509": "2.3.3-3.2598.3-0.0.509",
		"":                       "",
	}
	for in, want := range cases {
		got, note := CanonicalManifestVersion(in)
		if got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
		if (note != "") != (got != in) {
			t.Errorf("%q: note %q inconsistent with rewrite", in, note)
		}
	}
}

func TestUpsertEnv(t *testing.T) {
	env := []any{map[string]any{"name": "A", "value": "1"}, map[string]any{"name": MaxActiveTMMEnv, "value": "64"}}
	out, changed := upsertEnv(env, UseGatewaySettingsEnv, "true", true)
	if !changed || len(out) != 3 || envMap(out)[UseGatewaySettingsEnv] != "true" {
		t.Errorf("append: %v %v", out, changed)
	}
	out, changed = upsertEnv(out, MaxActiveTMMEnv, "32", false)
	if changed || envMap(out)[MaxActiveTMMEnv] != "64" {
		t.Errorf("no-overwrite must keep 64: %v %v", out, changed)
	}
	out, changed = upsertEnv(out, UseGatewaySettingsEnv, "true", true)
	if changed {
		t.Errorf("same value reported as change")
	}
	out, changed = upsertEnv(append(out, map[string]any{"name": UseGatewaySettingsEnv, "value": "false"}), UseGatewaySettingsEnv, "true", true)
	if !changed || envMap(out)[UseGatewaySettingsEnv] != "true" {
		t.Errorf("overwrite false->true: %v %v", out, changed)
	}
	if len(out) != 4 {
		t.Errorf("entries = %d (order and count preserved)", len(out))
	}
}
