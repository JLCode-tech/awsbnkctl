package k8s

// multusheal.go — the Multus thin plugin writes its kubeconfig once at pod
// start with the service-account token it holds at that moment. When that
// token expires the CNI call for every new pod fails with
// "Multus: ... error waiting for pod: Unauthorized" and nothing schedules on
// the node until the Multus pod restarts and writes a fresh kubeconfig.
// Seen live 2026-09-14 on a 13-day-old cluster during bnk upgrade. Every
// lifecycle command that creates pods on an existing cluster runs
// HealMultusToken first.

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	// MultusNamespace and MultusDaemonSet locate the Multus install.
	MultusNamespace = "kube-system"
	MultusDaemonSet = "kube-multus-ds"
	// MultusPodSelector selects the Multus pods.
	MultusPodSelector = "app=multus"
	// MultusMaxPodAge is how old a Multus pod may be before its kubeconfig
	// token is assumed stale. Bound service-account tokens outlive their
	// nominal hour only through kubelet refresh, which the thin plugin never
	// re-reads; a day is well inside the window seen live (13 days) and cheap
	// to restart.
	MultusMaxPodAge = 24 * time.Hour
	// multusRestartAnnotation marks the DaemonSet template to roll its pods.
	multusRestartAnnotation = "awsbnkctl.f5.com/restartedAt"
	multusRestartWait       = 3 * time.Minute
)

// MultusHealResult reports what HealMultusToken found and did.
type MultusHealResult struct {
	// Installed is false when the Multus DaemonSet does not exist.
	Installed bool `json:"installed"`
	// Stale is true when the pods needed a restart.
	Stale bool `json:"stale"`
	// Reason explains Stale.
	Reason string `json:"reason,omitempty"`
	// Restarted is true when the DaemonSet was rolled.
	Restarted bool `json:"restarted"`
}

// MultusTokenStale reports whether the Multus pods should be restarted: a
// recent FailedCreatePodSandBox event blaming Multus with Unauthorized, or a
// Multus pod older than maxAge (zero = MultusMaxPodAge). Installed is false
// when the DaemonSet is absent.
func MultusTokenStale(ctx context.Context, cs kubernetes.Interface, maxAge time.Duration, now time.Time) (installed, stale bool, reason string, err error) {
	if maxAge <= 0 {
		maxAge = MultusMaxPodAge
	}
	if _, err := cs.AppsV1().DaemonSets(MultusNamespace).Get(ctx, MultusDaemonSet, metav1.GetOptions{}); err != nil {
		return false, false, "", nil //nolint:nilerr — no Multus, nothing to heal
	}
	pods, err := cs.CoreV1().Pods(MultusNamespace).List(ctx, metav1.ListOptions{LabelSelector: MultusPodSelector})
	if err != nil {
		return true, false, "", fmt.Errorf("list multus pods: %w", err)
	}
	// Events older than the newest Multus pod predate a restart that already
	// healed them.
	var newest time.Time
	for i := range pods.Items {
		if st := pods.Items[i].Status.StartTime; st != nil && st.Time.After(newest) {
			newest = st.Time
		}
	}
	events, err := cs.CoreV1().Events("").List(ctx, metav1.ListOptions{FieldSelector: "reason=FailedCreatePodSandBox"})
	if err == nil {
		for i := range events.Items {
			ev := &events.Items[i]
			if !strings.Contains(ev.Message, "multus") || !strings.Contains(ev.Message, "Unauthorized") {
				continue
			}
			ts := ev.LastTimestamp.Time
			if ts.IsZero() {
				ts = ev.EventTime.Time
			}
			if !ts.IsZero() && ts.Before(newest) {
				continue
			}
			if ts.IsZero() || now.Sub(ts) <= time.Hour {
				return true, true, fmt.Sprintf("pod %s/%s failed to get a network sandbox: Multus Unauthorized (expired kubeconfig token)", ev.InvolvedObject.Namespace, ev.InvolvedObject.Name), nil
			}
		}
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.StartTime == nil {
			continue
		}
		if age := now.Sub(p.Status.StartTime.Time); age > maxAge {
			return true, true, fmt.Sprintf("multus pod %s started %s ago; its kubeconfig carries the service-account token from then", p.Name, age.Round(time.Hour)), nil
		}
	}
	return true, false, "", nil
}

// RestartMultus rolls the Multus DaemonSet and waits for it to be ready.
// Running pods keep their networking; only new pod creation needs Multus.
func RestartMultus(ctx context.Context, cs kubernetes.Interface, now time.Time) error {
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{%q:%q}}}}}`, multusRestartAnnotation, now.UTC().Format(time.RFC3339))
	if _, err := cs.AppsV1().DaemonSets(MultusNamespace).Patch(ctx, MultusDaemonSet, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("restart multus DaemonSet: %w", err)
	}
	return WaitForDaemonSetReady(ctx, cs, MultusNamespace, MultusDaemonSet, multusRestartWait)
}

// HealMultusToken restarts Multus when MultusTokenStale says so and logs
// what it did. Errors are returned, never fatal for callers that treat the
// heal as best-effort.
func HealMultusToken(ctx context.Context, cs kubernetes.Interface, maxAge time.Duration, log io.Writer) (MultusHealResult, error) {
	if log == nil {
		log = io.Discard
	}
	now := time.Now()
	installed, stale, reason, err := MultusTokenStale(ctx, cs, maxAge, now)
	res := MultusHealResult{Installed: installed, Stale: stale, Reason: reason}
	if err != nil || !installed || !stale {
		return res, err
	}
	fmt.Fprintf(log, "[multus] %s; restarting DaemonSet %s/%s so it writes a fresh kubeconfig\n", reason, MultusNamespace, MultusDaemonSet)
	if err := RestartMultus(ctx, cs, now); err != nil {
		return res, err
	}
	res.Restarted = true
	fmt.Fprintln(log, "[multus] DaemonSet rolled and ready")
	return res, nil
}
