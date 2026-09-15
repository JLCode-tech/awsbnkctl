package phases

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func healDeps(objs ...runtime.Object) *HealDeps {
	return &HealDeps{Clients: &Clients{K8s: k8sfake.NewClientset(objs...)}}
}

func controllerDeploy(sa string, mounts ...corev1.VolumeMount) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: h4DeploymentName, Namespace: InstanceNamespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "f5-cne-controller"}},
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				ServiceAccountName: sa,
				Containers:         []corev1.Container{{Name: "f5-cne-controller"}, {Name: h4ContainerName, VolumeMounts: mounts}},
			}},
		},
	}
}

var grpcMount = corev1.VolumeMount{Name: "tls-f5ingress-grpc-clt-volume", MountPath: "/tls/f5ingress/grpc/clt"}

func controllerPod(name string, pmReady bool, restarts int32, waiting string) *corev1.Pod {
	cs := corev1.ContainerStatus{Name: h4ContainerName, Ready: pmReady, RestartCount: restarts}
	if waiting != "" {
		cs.State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: waiting}}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: InstanceNamespace, Labels: map[string]string{"app": "f5-cne-controller"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{cs}},
	}
}

// An empty cluster is healthy everywhere except the metrics API, which the
// fake discovery cannot serve; nothing is fixed and nothing errors.
func TestRunHeal_EmptyClusterDetectsOnly(t *testing.T) {
	d := healDeps()
	d.Clients.Dynamic = nil
	out := RunHeal(context.Background(), d, nil)
	if len(out) != len(Repairs) {
		t.Fatalf("got %d statuses, want %d", len(out), len(Repairs))
	}
	for _, s := range out {
		switch s.Name {
		case "metrics-server":
			if s.Healthy || !s.Skipped || !strings.Contains(s.Detail, "bnk heal -f") {
				t.Errorf("metrics-server: %+v", s)
			}
		case "tmm-k8s-routes":
			if s.Error == "" { // no dynamic client → detect error, never a fix
				t.Errorf("tmm-k8s-routes: expected an error without a dynamic client: %+v", s)
			}
		default:
			if !s.Healthy || s.Changed || s.Error != "" {
				t.Errorf("%s: %+v", s.Name, s)
			}
		}
	}
}

func TestRunHeal_OnlyAndDryRun(t *testing.T) {
	d := healDeps(multusDaemonSetForHeal())
	d.DryRun = true
	out := RunHeal(context.Background(), d, []string{"multus-token-watch"})
	if len(out) != 1 || out[0].Name != "multus-token-watch" {
		t.Fatalf("out=%+v", out)
	}
	if out[0].Healthy || out[0].Changed || !strings.HasPrefix(out[0].Detail, "would repair:") {
		t.Errorf("dry-run status %+v", out[0])
	}
	d.DryRun = false
	out = RunHeal(context.Background(), d, []string{"multus-token-watch"})
	if !out[0].Changed || !out[0].Healthy || !strings.Contains(out[0].Detail, "DaemonSet rolled") {
		t.Errorf("fix status %+v", out[0])
	}
}

func multusDaemonSetForHeal() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-multus-ds", Namespace: "kube-system", Generation: 1},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "kube-multus", Args: []string{"--multus-conf-file=auto"},
		}}}}},
		Status: appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 1, UpdatedNumberScheduled: 1, NumberReady: 1, NumberAvailable: 1},
	}
}

func TestDetectPodManager(t *testing.T) {
	ctx := context.Background()
	t.Run("missing mount", func(t *testing.T) {
		d := healDeps(controllerDeploy("f5-cne-controller"))
		ok, detail, err := detectPodManager(ctx, d.Clients)
		if err != nil || ok || !strings.Contains(detail, "/tls/f5ingress/grpc/clt") {
			t.Errorf("ok=%v detail=%q err=%v", ok, detail, err)
		}
	})
	t.Run("crash loop", func(t *testing.T) {
		d := healDeps(controllerDeploy("f5-cne-controller", grpcMount), controllerPod("c-1", false, 3, "CrashLoopBackOff"))
		ok, detail, err := detectPodManager(ctx, d.Clients)
		if err != nil || ok || !strings.Contains(detail, "CrashLoopBackOff") {
			t.Errorf("ok=%v detail=%q err=%v", ok, detail, err)
		}
	})
	t.Run("healthy", func(t *testing.T) {
		d := healDeps(controllerDeploy("f5-cne-controller", grpcMount), controllerPod("c-1", true, 1, ""))
		ok, _, err := detectPodManager(ctx, d.Clients)
		if err != nil || !ok {
			t.Errorf("ok=%v err=%v", ok, err)
		}
	})
}

func TestDetectControllerRBAC(t *testing.T) {
	ctx := context.Background()
	if ok, detail, _ := detectControllerRBAC(ctx, healDeps(controllerDeploy("f5-cne-controller-x-serviceaccount")).Clients); !ok || !strings.Contains(detail, "2.3") {
		t.Errorf("2.3 SA must not need the role: ok=%v %q", ok, detail)
	}
	if ok, detail, _ := detectControllerRBAC(ctx, healDeps(controllerDeploy("f5-cne-controller")).Clients); ok || !strings.Contains(detail, "EndpointSlice") {
		t.Errorf("no binding: ok=%v %q", ok, detail)
	}
	role := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "x-cne-controller-endpointslices"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"discovery.k8s.io"}, Resources: []string{"endpointslices"}, Verbs: []string{"get", "list"}}}}
	crb := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "x-cne-controller-endpointslices"}, RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: role.Name}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: InstanceNamespace, Name: "f5-cne-controller"}}}
	if ok, _, err := detectControllerRBAC(ctx, healDeps(controllerDeploy("f5-cne-controller"), role, crb).Clients); !ok || err != nil {
		t.Errorf("bound role: ok=%v err=%v", ok, err)
	}
}

func TestDSSMOverlayKeys(t *testing.T) {
	cm := &corev1.ConfigMap{Data: map[string]string{
		"readiness_probe.sh": "redis-cli --tls -p 6379 ping",
		"liveness_probe.sh":  "redis-cli --tls --insecure -p 6379 ping",
		"init.sh":            "echo hello",
	}}
	toPatch, done := dssmOverlayKeys(cm)
	if len(toPatch) != 1 || toPatch[0] != "readiness_probe.sh" || done != 1 {
		t.Errorf("toPatch=%v done=%d", toPatch, done)
	}
}

func TestDetectControllerIRSA(t *testing.T) {
	ctx := context.Background()
	dep := controllerDeploy("f5-cne-controller", grpcMount)
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "f5-cne-controller", Namespace: InstanceNamespace}}
	if ok, detail, err := detectControllerIRSA(ctx, healDeps(dep, sa).Clients); ok || err != nil || !strings.Contains(detail, irsaRoleARNAnnotation) {
		t.Errorf("unannotated: ok=%v %q err=%v", ok, detail, err)
	}
}
