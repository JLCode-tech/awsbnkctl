package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func multusObjects(podAge time.Duration, now time.Time) []runtime.Object {
	return []runtime.Object{
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: MultusDaemonSet, Namespace: MultusNamespace},
			Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1, NumberReady: 1},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-multus-ds-abc", Namespace: MultusNamespace, Labels: map[string]string{"app": "multus"}},
			Status:     corev1.PodStatus{StartTime: &metav1.Time{Time: now.Add(-podAge)}},
		},
	}
}

func TestMultusTokenStale(t *testing.T) {
	now := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("not installed", func(t *testing.T) {
		installed, stale, _, err := MultusTokenStale(ctx, k8sfake.NewClientset(), 0, now)
		if err != nil || installed || stale {
			t.Errorf("installed=%v stale=%v err=%v", installed, stale, err)
		}
	})
	t.Run("fresh pods are fine", func(t *testing.T) {
		installed, stale, _, err := MultusTokenStale(ctx, k8sfake.NewClientset(multusObjects(time.Hour, now)...), 0, now)
		if err != nil || !installed || stale {
			t.Errorf("installed=%v stale=%v err=%v", installed, stale, err)
		}
	})
	t.Run("old pod is stale", func(t *testing.T) {
		_, stale, reason, err := MultusTokenStale(ctx, k8sfake.NewClientset(multusObjects(13*24*time.Hour, now)...), 0, now)
		if err != nil || !stale || !strings.Contains(reason, "312h0m0s ago") {
			t.Errorf("stale=%v reason=%q err=%v", stale, reason, err)
		}
	})
	t.Run("unauthorized sandbox event is stale even with a fresh pod", func(t *testing.T) {
		objs := append(multusObjects(time.Hour, now), &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: "flo.1", Namespace: "f5-cne-core"},
			Reason:         "FailedCreatePodSandBox",
			Message:        `Failed to create pod sandbox: plugin type="multus" name="multus-cni-network" failed (add): Multus: [f5-cne-core/f5-lifecycle-operator-1/uid]: error waiting for pod: Unauthorized`,
			InvolvedObject: corev1.ObjectReference{Namespace: "f5-cne-core", Name: "f5-lifecycle-operator-1"},
			LastTimestamp:  metav1.Time{Time: now.Add(-time.Minute)},
		})
		_, stale, reason, err := MultusTokenStale(ctx, k8sfake.NewClientset(objs...), 0, now)
		if err != nil || !stale || !strings.Contains(reason, "f5-cne-core/f5-lifecycle-operator-1") {
			t.Errorf("stale=%v reason=%q err=%v", stale, reason, err)
		}
	})
	t.Run("event older than the newest multus pod is healed already", func(t *testing.T) {
		objs := append(multusObjects(10*time.Minute, now), &corev1.Event{
			ObjectMeta:    metav1.ObjectMeta{Name: "healed.1", Namespace: "f5-cne-core"},
			Reason:        "FailedCreatePodSandBox",
			Message:       `plugin type="multus" ... Multus: error waiting for pod: Unauthorized`,
			LastTimestamp: metav1.Time{Time: now.Add(-20 * time.Minute)},
		})
		_, stale, _, err := MultusTokenStale(ctx, k8sfake.NewClientset(objs...), 0, now)
		if err != nil || stale {
			t.Errorf("stale=%v err=%v", stale, err)
		}
	})
	t.Run("old unauthorized event is ignored", func(t *testing.T) {
		objs := append(multusObjects(time.Hour, now), &corev1.Event{
			ObjectMeta:    metav1.ObjectMeta{Name: "old.1", Namespace: "default"},
			Reason:        "FailedCreatePodSandBox",
			Message:       `plugin type="multus" ... Unauthorized`,
			LastTimestamp: metav1.Time{Time: now.Add(-3 * time.Hour)},
		})
		_, stale, _, err := MultusTokenStale(ctx, k8sfake.NewClientset(objs...), 0, now)
		if err != nil || stale {
			t.Errorf("stale=%v err=%v", stale, err)
		}
	})
}

func TestHealMultusToken(t *testing.T) {
	now := time.Now()
	cs := k8sfake.NewClientset(multusObjects(48*time.Hour, now)...)
	var log strings.Builder
	res, err := HealMultusToken(context.Background(), cs, 0, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Installed || !res.Stale || !res.Restarted {
		t.Errorf("result = %+v", res)
	}
	ds, _ := cs.AppsV1().DaemonSets(MultusNamespace).Get(context.Background(), MultusDaemonSet, metav1.GetOptions{})
	if ds.Spec.Template.Annotations[multusRestartAnnotation] == "" {
		t.Errorf("DaemonSet template not annotated: %+v", ds.Spec.Template.Annotations)
	}
	if !strings.Contains(log.String(), "restarting DaemonSet kube-system/kube-multus-ds") || !strings.Contains(log.String(), "rolled and ready") {
		t.Errorf("log = %q", log.String())
	}

	res, err = HealMultusToken(context.Background(), k8sfake.NewClientset(multusObjects(time.Hour, now)...), 0, nil)
	if err != nil || res.Restarted || res.Stale {
		t.Errorf("fresh: res=%+v err=%v", res, err)
	}
}
