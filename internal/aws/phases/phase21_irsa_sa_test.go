//lint:file-ignore SA1019 k8sfake.NewSimpleClientset is still functional — NewClientset requires --with-applyconfig which adds significant test-codegen complexity

package phases

import (
	"context"
	"time"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

const (
	p21RoleName = "tracer-cne-controller-irsa"
	p21RoleARN  = "arn:aws:iam::111122223333:role/" + p21RoleName
	p21SA       = "f5-cne-controller" // what FLO 2.3.3 really uses — not the old convention
)

// p21State returns phase-18 state as Phase 21 expects it.
func p21State(t *testing.T) *state.State {
	t.Helper()
	st, _ := state.Load(t.TempDir())
	st.Set("CNE_IRSA_ROLE_ARN", p21RoleARN)
	st.Set("CNE_IRSA_ROLE_NAME", p21RoleName)
	st.Set("EKS_OIDC_URL", "https://oidc.eks.ap-southeast-2.amazonaws.com/id/TESTOIDC")
	return st
}

// p21Objects returns the FLO-created Deployment + SA and one controller pod,
// with or without the webhook-injected AWS_ROLE_ARN env.
func p21Objects(injected bool) []runtime.Object {
	sel := map[string]string{"app": h4DeploymentName}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: h4DeploymentName + "-abc", Namespace: InstanceNamespace, Labels: sel},
		Spec:       corev1.PodSpec{ServiceAccountName: p21SA, Containers: []corev1.Container{{Name: h4DeploymentName}}},
	}
	if injected {
		pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: irsaInjectedEnv, Value: p21RoleARN}}
	}
	return []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: h4DeploymentName, Namespace: InstanceNamespace},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: sel},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: sel},
					Spec:       corev1.PodSpec{ServiceAccountName: p21SA},
				},
			},
		},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: p21SA, Namespace: InstanceNamespace}},
		pod,
	}
}

func p21Clients(t *testing.T, injected bool) (*Clients, *mockIAM) {
	t.Helper()
	iamMock := newMockIAM()
	if _, err := iamMock.CreateRole(context.Background(), &iam.CreateRoleInput{RoleName: ptr(p21RoleName), AssumeRolePolicyDocument: ptr("{}")}); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return &Clients{K8s: k8sfake.NewSimpleClientset(p21Objects(injected)...), IAM: iamMock, Profile: "test"}, iamMock
}

func TestPhase21IRSASA_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	st := p21State(t)
	if err := Phase21IRSASA(context.Background(), hostDeviceCluster(), st, &Clients{Profile: "test"}, true); err != nil {
		t.Fatalf("Phase21 dry-run: %v", err)
	}
	if st.Get("IRSA_SA_APPLIED_AT") != "dry-run" {
		t.Errorf("IRSA_SA_APPLIED_AT = %q, want dry-run", st.Get("IRSA_SA_APPLIED_AT"))
	}
}

func TestPhase21IRSASA_MissingRoleARN_Errors(t *testing.T) {
	awsmw.ResetForTest()
	st, _ := state.Load(t.TempDir()) // CNE_IRSA_ROLE_ARN deliberately missing
	if err := Phase21IRSASA(context.Background(), hostDeviceCluster(), st, &Clients{Profile: "test"}, true); err == nil {
		t.Fatal("expected error when CNE_IRSA_ROLE_ARN missing, got nil")
	}
}

// TestPhase21IRSASA_DiscoversSAAndRestarts is the bnk-staging-test 2026-09-11
// case: FLO runs the controller as f5-cne-controller, the pod predates any
// annotation, so Phase 21 must trust + annotate that SA and bounce the deploy.
func TestPhase21IRSASA_DiscoversSAAndRestarts(t *testing.T) {
	awsmw.ResetForTest()
	st := p21State(t)
	clients, iamMock := p21Clients(t, false)
	ctx := context.Background()

	if err := Phase21IRSASA(ctx, hostDeviceCluster(), st, clients, false); err != nil {
		t.Fatalf("Phase21: %v", err)
	}

	if got := st.Get("CNE_SA_NAME"); got != p21SA {
		t.Errorf("CNE_SA_NAME = %q, want %q (discovered from the Deployment, not derived)", got, p21SA)
	}
	sa, _ := clients.K8s.CoreV1().ServiceAccounts(InstanceNamespace).Get(ctx, p21SA, metav1.GetOptions{})
	if sa.Annotations[irsaRoleARNAnnotation] != p21RoleARN {
		t.Errorf("SA annotation %s = %q, want %q", irsaRoleARNAnnotation, sa.Annotations[irsaRoleARNAnnotation], p21RoleARN)
	}
	wantSub := irsaSubject(InstanceNamespace, p21SA)
	if trust := iamMock.trustPolicies[p21RoleName]; !strings.Contains(trust, `"`+wantSub+`"`) {
		t.Errorf("trust policy not scoped to %s: %s", wantSub, trust)
	}
	d, _ := clients.K8s.AppsV1().Deployments(InstanceNamespace).Get(ctx, h4DeploymentName, metav1.GetOptions{})
	if d.Spec.Template.Annotations["awsbnkctl.io/restartedAt"] == "" {
		t.Error("deployment was not rollout-restarted although its pod lacked AWS_ROLE_ARN")
	}
}

func TestPhase21IRSASA_NoRestartWhenAlreadyInjected(t *testing.T) {
	awsmw.ResetForTest()
	st := p21State(t)
	clients, _ := p21Clients(t, true)
	ctx := context.Background()

	if err := Phase21IRSASA(ctx, hostDeviceCluster(), st, clients, false); err != nil {
		t.Fatalf("Phase21: %v", err)
	}
	d, _ := clients.K8s.AppsV1().Deployments(InstanceNamespace).Get(ctx, h4DeploymentName, metav1.GetOptions{})
	if d.Spec.Template.Annotations["awsbnkctl.io/restartedAt"] != "" {
		t.Error("deployment restarted although its pod already carried AWS_ROLE_ARN")
	}
}

func TestPhase21IRSASADown_ClearsState(t *testing.T) {
	awsmw.ResetForTest()
	st := p21State(t)
	st.Set("IRSA_SA_APPLIED_AT", "2026-09-11T00:00:00Z")
	st.Set("CNE_SA_NAME", p21SA)

	if err := Phase21IRSASADown(context.Background(), hostDeviceCluster(), st, &Clients{Profile: "test"}); err != nil {
		t.Fatalf("Phase21Down: %v", err)
	}
	for _, k := range []string{"IRSA_SA_APPLIED_AT", "CNE_SA_NAME"} {
		if st.Get(k) != "" {
			t.Errorf("%s should be cleared after down, got %q", k, st.Get(k))
		}
	}
}

// TestPhase21IRSASA_WaitsForLateSA reproduces bnk-staging-test 2026-09-11:
// FLO created deploy/f5-cne-controller two seconds before the SA it runs as,
// and Phase 21 aborted `up` with `serviceaccounts "f5-cne-controller" not found`.
func TestPhase21IRSASA_WaitsForLateSA(t *testing.T) {
	awsmw.ResetForTest()
	old := deployPollInterval
	deployPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { deployPollInterval = old })

	st := p21State(t)
	clients, _ := p21Clients(t, false)
	ctx := context.Background()
	if err := clients.K8s.CoreV1().ServiceAccounts(InstanceNamespace).Delete(ctx, p21SA, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("remove SA to simulate FLO lag: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = clients.K8s.CoreV1().ServiceAccounts(InstanceNamespace).Create(ctx,
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: p21SA, Namespace: InstanceNamespace}}, metav1.CreateOptions{})
	}()

	if err := Phase21IRSASA(ctx, hostDeviceCluster(), st, clients, false); err != nil {
		t.Fatalf("Phase21 must wait for the late SA, got: %v", err)
	}
	sa, _ := clients.K8s.CoreV1().ServiceAccounts(InstanceNamespace).Get(ctx, p21SA, metav1.GetOptions{})
	if sa.Annotations[irsaRoleARNAnnotation] != p21RoleARN {
		t.Errorf("late SA not annotated: %v", sa.Annotations)
	}
}
