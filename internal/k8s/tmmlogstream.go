package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// BNK 2.4 TMM log stream.
//
// The TMM pod's f5-fluentbit sidecar forwards every TMM log line (including
// the iRule `log local0.` records the governance pipeline consumes) to
// f5-toda-fluentd in the core namespace and nowhere else: FLO 2.30 renders no
// stdout output for it, so `kubectl logs f5-tmm -c f5-fluentbit` shows only
// Fluent Bit's own engine messages. fluentd writes the lines to its volume
// and, through F5's documented extension point, to stdout when the
// f5-toda-fluentd-custom ConfigMap's stdout.conf carries a `@type stdout`
// store (it ships commented out). EnableTMMLogStream turns that store on and
// bounces the fluentd pod (the config is read at start and the log volume is
// ReadWriteOnce, so a rolling update deadlocks on the volume; the pod has to
// go first). awsbnkctl up, bnk upgrade, doctor and `logs tmm` share it.
const (
	// TMMLogNamespace is the CNE core namespace f5-toda-fluentd runs in.
	TMMLogNamespace = "f5-cne-core"
	// TMMLogDeployment is the fluentd Deployment.
	TMMLogDeployment = "f5-toda-fluentd"
	// TMMLogPodSelector selects the fluentd pod.
	TMMLogPodSelector = "app=toda-fluentd"
	// TMMLogContainer is the fluentd container whose stdout carries the stream.
	TMMLogContainer = "f5-fluentd"
	// TMMLogCustomConfigMap is F5's user-owned pipeline extension ConfigMap.
	TMMLogCustomConfigMap = "f5-toda-fluentd-custom"
	// TMMLogStdoutKey is the file match.conf includes; F5 ships it with the
	// store commented out.
	TMMLogStdoutKey = "stdout.conf"
	// TMMLogStdoutStore is the enabled store.
	TMMLogStdoutStore = "<store>\n  @type stdout\n</store>\n"
	// TMMLogPodMarker identifies TMM pod lines on the fluentd stdout stream.
	TMMLogPodMarker = `"pod_name":"f5-tmm"`
)

// TMMLogStreamEnabled reports whether the fluentd stdout store is on: the
// stdout.conf key holds an uncommented `@type stdout`.
func TMMLogStreamEnabled(ctx context.Context, cs kubernetes.Interface) (bool, error) {
	cm, err := cs.CoreV1().ConfigMaps(TMMLogNamespace).Get(ctx, TMMLogCustomConfigMap, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get configmap %s/%s: %w", TMMLogNamespace, TMMLogCustomConfigMap, err)
	}
	return stdoutStoreOn(cm.Data[TMMLogStdoutKey]), nil
}

func stdoutStoreOn(conf string) bool {
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "@type stdout") {
			return true
		}
	}
	return false
}

// EnableTMMLogStream turns the stdout store on and bounces the fluentd pod so
// the new config is read. Returns changed=false when the store was already
// on (nothing is touched). timeout bounds the wait for the new pod.
func EnableTMMLogStream(ctx context.Context, cs kubernetes.Interface, timeout time.Duration) (changed bool, err error) {
	cm, err := cs.CoreV1().ConfigMaps(TMMLogNamespace).Get(ctx, TMMLogCustomConfigMap, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get configmap %s/%s: %w", TMMLogNamespace, TMMLogCustomConfigMap, err)
	}
	if stdoutStoreOn(cm.Data[TMMLogStdoutKey]) {
		return false, nil
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[TMMLogStdoutKey] = TMMLogStdoutStore
	if _, err := cs.CoreV1().ConfigMaps(TMMLogNamespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("update configmap %s/%s: %w", TMMLogNamespace, TMMLogCustomConfigMap, err)
	}
	// Delete the pod rather than rolling the Deployment: the log volume is
	// ReadWriteOnce, so a replacement pod cannot start while the old one holds
	// it and `rollout restart` waits forever.
	pods, err := cs.CoreV1().Pods(TMMLogNamespace).List(ctx, metav1.ListOptions{LabelSelector: TMMLogPodSelector})
	if err != nil {
		return true, fmt.Errorf("list %s pods: %w", TMMLogPodSelector, err)
	}
	for i := range pods.Items {
		if err := cs.CoreV1().Pods(TMMLogNamespace).Delete(ctx, pods.Items[i].Name, metav1.DeleteOptions{}); err != nil {
			return true, fmt.Errorf("delete pod %s/%s: %w", TMMLogNamespace, pods.Items[i].Name, err)
		}
	}
	if timeout <= 0 {
		return true, nil
	}
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		list, err := cs.CoreV1().Pods(TMMLogNamespace).List(ctx, metav1.ListOptions{LabelSelector: TMMLogPodSelector})
		if err != nil {
			return false, nil
		}
		for i := range list.Items {
			if list.Items[i].DeletionTimestamp == nil && podReady(&list.Items[i]) {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return true, fmt.Errorf("waiting for a Ready %s pod in %s: %w", TMMLogPodSelector, TMMLogNamespace, err)
	}
	return true, nil
}

func podReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
