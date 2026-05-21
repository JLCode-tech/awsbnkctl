package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// WaitForDeploymentReady polls apps/v1 Deployment in ns/name until
// Status.AvailableReplicas == Status.Replicas (both > 0). Returns an error if
// the deadline passes before the Deployment is ready.
//
// Interval is 5 s. timeout is the caller-supplied deadline.
func WaitForDeploymentReady(ctx context.Context, clientset kubernetes.Interface, ns, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		d, err := clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// Deployment may not exist yet — keep polling.
			return false, nil //nolint:nilerr
		}
		desired := d.Status.Replicas
		available := d.Status.AvailableReplicas
		if desired > 0 && available == desired {
			return true, nil
		}
		return false, nil
	})
}

// CertificateGVR is the GroupVersionResource for cert-manager Certificate CRs.
// Exported so phases and tests can reference it without duplicating the constant.
var CertificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

// WaitForCertificateReady polls cert-manager.io/v1 Certificate in ns/name
// via the dynamic client until the Ready condition is True. Returns an error
// if the deadline passes.
//
// Interval is 5 s. timeout is the caller-supplied deadline.
func WaitForCertificateReady(ctx context.Context, dyn dynamic.Interface, ns, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		obj, err := dyn.Resource(CertificateGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil //nolint:nilerr
		}
		return IsCertificateReady(obj.Object), nil
	})
}

// IsCertificateReady walks the unstructured status.conditions slice to find
// a condition with type=Ready and status=True. Exported for use by phases and tests.
func IsCertificateReady(obj map[string]interface{}) bool {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return false
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := cond["type"].(string); t != "Ready" {
			continue
		}
		if s, _ := cond["status"].(string); s == "True" {
			return true
		}
	}
	return false
}

// WaitForNamespaceGone polls until namespace ns no longer exists (for cleanup).
// Tolerates NotFound immediately. Returns nil if gone within timeout.
func WaitForNamespaceGone(ctx context.Context, clientset kubernetes.Interface, ns string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			// Any error (including NotFound) means the namespace is gone or unreachable.
			return true, nil
		}
		return false, nil
	})
}

// WaitForCRDExists polls apiextensions.k8s.io/v1 CustomResourceDefinition until
// the named CRD is visible in the API server. Used to confirm cert-manager CRDs
// are established before applying the cert chain.
func WaitForCRDExists(ctx context.Context, dyn dynamic.Interface, crdName string, timeout time.Duration) error {
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := dyn.Resource(crdGVR).Get(ctx, crdName, metav1.GetOptions{})
		if err != nil {
			return false, nil //nolint:nilerr
		}
		return true, nil
	})
}

// certReplicaStatus is used by Phase 13 postflight check (pure read).
// It returns (available, desired, error).
func DeploymentReplicaStatus(ctx context.Context, clientset kubernetes.Interface, ns, name string) (available, desired int32, err error) {
	d, err := clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("get deployment %s/%s: %w", ns, name, err)
	}
	return d.Status.AvailableReplicas, d.Status.Replicas, nil
}
