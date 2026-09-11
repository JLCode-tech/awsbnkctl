package phases

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// deployPollInterval paces waitForDeployment and waitForServiceAccount
// (a var so tests can shorten it).
var deployPollInterval = 5 * time.Second

// waitForDeployment returns the Deployment once it exists, polling until timeout.
// Used for FLO/CNEController-owned Deployments that appear some seconds after
// the CNEInstance CR is applied.
func waitForDeployment(ctx context.Context, clients *Clients, ns, name string, timeout time.Duration) (*appsv1.Deployment, error) {
	deadline := time.Now().Add(timeout)
	for {
		d, err := clients.K8s.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return d, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get deploy %s/%s: %w", ns, name, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("deploy %s/%s not created within %s", ns, name, timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(deployPollInterval):
		}
	}
}

// waitForServiceAccount returns the ServiceAccount once it exists, polling
// until timeout. FLO creates the controller Deployment a few seconds before
// the SA it references (2 s apart on bnk-staging-test, 2026-09-11), so a
// caller that just saw the Deployment must not assume the SA is there yet.
func waitForServiceAccount(ctx context.Context, clients *Clients, ns, name string, timeout time.Duration) (*corev1.ServiceAccount, error) {
	deadline := time.Now().Add(timeout)
	for {
		sa, err := clients.K8s.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return sa, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get sa %s/%s: %w", ns, name, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("sa %s/%s not created within %s", ns, name, timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(deployPollInterval):
		}
	}
}

// deploymentPodsHaveEnv reports whether every pod selected by deploy has a
// container exposing envName. With no pods there is nothing to fix, so it
// returns true.
func deploymentPodsHaveEnv(ctx context.Context, clients *Clients, deploy *appsv1.Deployment, envName string) (bool, error) {
	sel := labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String()
	pods, err := clients.K8s.CoreV1().Pods(deploy.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return false, fmt.Errorf("list pods of deploy %s/%s: %w", deploy.Namespace, deploy.Name, err)
	}
	for _, p := range pods.Items {
		found := false
		for _, c := range p.Spec.Containers {
			for _, e := range c.Env {
				if e.Name == envName {
					found = true
				}
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// restartDeployment triggers a rollout-restart by stamping the pod template,
// the same mechanism as `kubectl rollout restart`.
func restartDeployment(ctx context.Context, clients *Clients, ns, name string) error {
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"awsbnkctl.io/restartedAt":%q}}}}}`,
		time.Now().UTC().Format(time.RFC3339))
	if _, err := clients.K8s.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType,
		[]byte(patch), metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("restart deploy %s/%s: %w", ns, name, err)
	}
	return nil
}

// waitForDeploymentAvailable returns once the Deployment exists and reports at
// least one available replica, polling until timeout. Phase 23b uses it so the
// F5 validating webhook (served by the cne-controller pod) has an endpoint
// before the first F5SPKVlan apply.
func waitForDeploymentAvailable(ctx context.Context, clients *Clients, ns, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		d, err := clients.K8s.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("get deploy %s/%s: %w", ns, name, err)
		}
		if err == nil && d.Status.AvailableReplicas > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("deploy %s/%s not available within %s", ns, name, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deployPollInterval):
		}
	}
}

// isWebhookUnavailable reports whether err is the apiserver telling us an
// admission webhook could not be reached — typically "no endpoints available
// for service" while the serving pod is still starting.
func isWebhookUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed calling webhook") ||
		strings.Contains(msg, "no endpoints available for service")
}

// retryWhileWebhookUnavailable runs fn, retrying on webhook-unavailable errors
// until timeout. Any other error is returned immediately.
func retryWhileWebhookUnavailable(ctx context.Context, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := fn()
		if err == nil || !isWebhookUnavailable(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("after %s the admission webhook is still unavailable: %w", timeout, err)
		}
		fmt.Fprintf(os.Stderr, "[phase] admission webhook not ready yet, retrying in %s: %v\n", deployPollInterval, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deployPollInterval):
		}
	}
}

// waitForConditionTrue polls a namespaced CR until status.conditions carries
// condType with status "True", or timeout. The error names the last observed
// status/reason/message so a stuck Infra or Gateway is diagnosable from the log.
func waitForConditionTrue(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, ns, name, condType string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := "not found"
	for {
		obj, err := dyn.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			conds, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
			last = "no " + condType + " condition yet"
			for _, c := range conds {
				m, ok := c.(map[string]interface{})
				if !ok || m["type"] != condType {
					continue
				}
				if m["status"] == "True" {
					return nil
				}
				last = fmt.Sprintf("%s=%v reason=%v message=%v", condType, m["status"], m["reason"], m["message"])
			}
		} else if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get %s %s/%s: %w", gvr.Resource, ns, name, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s %s/%s: %s after %s", gvr.Resource, ns, name, last, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deployPollInterval):
		}
	}
}
