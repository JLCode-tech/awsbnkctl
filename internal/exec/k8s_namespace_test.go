package exec

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestEnsureTestNamespace(t *testing.T) {
	cs := k8sfake.NewClientset()
	ctx := context.Background()

	exists, err := TestNamespaceExists(ctx, cs)
	if err != nil || exists {
		t.Fatalf("empty cluster: exists=%v err=%v", exists, err)
	}
	created, err := EnsureTestNamespace(ctx, cs)
	if err != nil || !created {
		t.Fatalf("first ensure: created=%v err=%v", created, err)
	}
	ns, err := cs.CoreV1().Namespaces().Get(ctx, K8sTestNamespace, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("namespace missing after ensure: %v", err)
	}
	if ns.Labels["awsbnkctl.io/managed"] != "true" {
		t.Errorf("labels = %v, want awsbnkctl.io/managed=true", ns.Labels)
	}
	created, err = EnsureTestNamespace(ctx, cs)
	if err != nil || created {
		t.Fatalf("second ensure should be a no-op: created=%v err=%v", created, err)
	}
}
