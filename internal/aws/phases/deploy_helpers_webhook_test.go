package phases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// TestRetryWhileWebhookUnavailable_RetriesThenSucceeds reproduces the BNK
// 2.4.0 phase 23b failure: the first F5SPKVlan apply hits the F5 validating
// webhook before the cne-controller pod serves it.
func TestRetryWhileWebhookUnavailable_RetriesThenSucceeds(t *testing.T) {
	old := deployPollInterval
	deployPollInterval = time.Millisecond
	t.Cleanup(func() { deployPollInterval = old })

	calls := 0
	err := retryWhileWebhookUnavailable(context.Background(), time.Second, func() error {
		calls++
		if calls < 3 {
			return errors.New(`SSA F5SPKVlan f5-cne-system/ext-vlan: Internal error occurred: failed calling webhook "f5validate.f5net.com": failed to call webhook: Post "https://f5-validation-svc.f5-cne-system.svc:3340/f5-validator?timeout=10s": no endpoints available for service "f5-validation-svc"`)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("want success after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryWhileWebhookUnavailable_OtherErrorsReturnImmediately(t *testing.T) {
	calls := 0
	err := retryWhileWebhookUnavailable(context.Background(), time.Second, func() error {
		calls++
		return errors.New("admission webhook denied: selfip_v4s must be set")
	})
	if err == nil || calls != 1 {
		t.Fatalf("want immediate non-webhook error after 1 call, got calls=%d err=%v", calls, err)
	}
}

func TestRetryWhileWebhookUnavailable_TimesOut(t *testing.T) {
	old := deployPollInterval
	deployPollInterval = time.Millisecond
	t.Cleanup(func() { deployPollInterval = old })

	err := retryWhileWebhookUnavailable(context.Background(), 20*time.Millisecond, func() error {
		return errors.New(`failed calling webhook "f5validate.f5net.com": no endpoints available for service "f5-validation-svc"`)
	})
	if err == nil {
		t.Fatal("want timeout error, got nil")
	}
}

// TestWaitForDeploymentAvailable_WaitsForReplica covers the phase 23b gate:
// the Deployment exists first with no available replica, then becomes ready.
func TestWaitForDeploymentAvailable_WaitsForReplica(t *testing.T) {
	old := deployPollInterval
	deployPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { deployPollInterval = old })

	ctx := context.Background()
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: h4DeploymentName, Namespace: InstanceNamespace}}
	k8s := kubefake.NewClientset(deploy)
	clients := &Clients{K8s: k8s}

	go func() {
		time.Sleep(30 * time.Millisecond)
		d, _ := k8s.AppsV1().Deployments(InstanceNamespace).Get(ctx, h4DeploymentName, metav1.GetOptions{})
		d.Status.AvailableReplicas = 1
		_, _ = k8s.AppsV1().Deployments(InstanceNamespace).UpdateStatus(ctx, d, metav1.UpdateOptions{})
	}()

	if err := waitForDeploymentAvailable(ctx, clients, InstanceNamespace, h4DeploymentName, 2*time.Second); err != nil {
		t.Fatalf("want the wait to clear once a replica is available, got %v", err)
	}
	if err := waitForDeploymentAvailable(ctx, clients, InstanceNamespace, "missing", 20*time.Millisecond); err == nil {
		t.Fatal("want timeout for a Deployment that never appears, got nil")
	}
}

// TestWaitForConditionTrue covers the Infra Programmed gate added for BNK 2.4:
// the CR appears without the condition, then flips to Programmed=True.
func TestWaitForConditionTrue(t *testing.T) {
	old := deployPollInterval
	deployPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { deployPollInterval = old })

	ctx := context.Background()
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.k8s.f5.com/v1alpha1", "kind": "Infra",
		"metadata": map[string]interface{}{"name": "infra", "namespace": InstanceNamespace},
		"status": map[string]interface{}{"conditions": []interface{}{
			map[string]interface{}{"type": "Programmed", "status": "False", "reason": "Programming", "message": "sending to TMM"},
		}},
	}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(buildScheme(),
		map[schema.GroupVersionResource]string{infraGVR: "InfraList"}, obj)

	if err := waitForConditionTrue(ctx, dyn, infraGVR, InstanceNamespace, "infra", "Programmed", 30*time.Millisecond); err == nil {
		t.Fatal("want timeout while Programmed=False, got nil")
	} else if !strings.Contains(err.Error(), "Programming") {
		t.Errorf("timeout error should carry the last reason, got %v", err)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cur, _ := dyn.Resource(infraGVR).Namespace(InstanceNamespace).Get(ctx, "infra", metav1.GetOptions{})
		_ = unstructured.SetNestedSlice(cur.Object, []interface{}{
			map[string]interface{}{"type": "Programmed", "status": "True", "reason": "Programmed"},
		}, "status", "conditions")
		_, _ = dyn.Resource(infraGVR).Namespace(InstanceNamespace).Update(ctx, cur, metav1.UpdateOptions{})
	}()
	if err := waitForConditionTrue(ctx, dyn, infraGVR, InstanceNamespace, "infra", "Programmed", 2*time.Second); err != nil {
		t.Fatalf("want success once Programmed=True, got %v", err)
	}
}
