package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Well-known names used by the k8s backend.
const (
	// K8sTestNamespace holds the one-shot probe Jobs. EnsureTestNamespace
	// creates it on first use; phase 12 and bnk heal (test-namespace) do the
	// same so doctor can report it.
	K8sTestNamespace = "awsbnkctl-test"

	// k8sJobReadyTimeout is how long we wait for an ephemeral Job's pod
	// to reach Running before streaming logs. Image pulls on a cold
	// node can chew up most of this budget; 3m matches the iperf3
	// fixture's defaultReadyTimeout in internal/k8s/iperf3.go.
	k8sJobReadyTimeout = 3 * time.Minute

	// k8sExitFailedToStart (127): backend couldn't reach the cluster or
	// could not create the Job.
	k8sExitFailedToStart = 127

	// k8sExitStartedThenFailed (126): Job created but its pod failed to
	// come up or the log stream errored.
	k8sExitStartedThenFailed = 126
)

// jobNameSanitizer maps docker-ref characters that are invalid in k8s
// label values (`:`, `/`, `@`) to `-`. Hit when argv[0] is a literal
// image ref via the toolImages-fallback test path; production callers
// pass tool names from toolImages and aren't affected.
var jobNameSanitizer = strings.NewReplacer(":", "-", "/", "-", "@", "-")

// K8sBackend executes argv as a one-shot Job in the awsbnkctl-test
// namespace (iperf3 client, the dns probe re-exec). argv[0] picks the
// tool image; argv[1:] is the in-container command.
//
// The namespace is created on first use (EnsureTestNamespace), so the
// backend needs nothing installed in the cluster beyond a reachable
// kubeconfig.
type K8sBackend struct {
	// once-init plumbing for client + config so a `--help` invocation
	// doesn't dial the apiserver. Mirror DockerBackend's lazy-init.
	mu     sync.Mutex
	client kubernetes.Interface
	config *rest.Config
	initFn func() (kubernetes.Interface, *rest.Config, error)
}

// Name implements Backend.
func (b *K8sBackend) Name() string { return "k8s" }

// Run implements Backend: ensures the test namespace, then runs argv as a Job.
func (b *K8sBackend) Run(ctx context.Context, argv []string, opts RunOpts) (int, error) {
	if len(argv) == 0 {
		return 0, errors.New("argv is empty")
	}

	cs, _, err := b.ensureClient()
	if err != nil {
		return k8sExitFailedToStart, fmt.Errorf("k8s backend: %w", err)
	}
	if _, err := EnsureTestNamespace(ctx, cs); err != nil {
		return k8sExitFailedToStart, fmt.Errorf("k8s backend: %w", err)
	}
	return b.runAsJob(ctx, cs, argv, opts)
}

// testNamespaceLabels mark the namespace as awsbnkctl-managed.
var testNamespaceLabels = map[string]string{"awsbnkctl.io/managed": "true"}

// TestNamespaceExists reports whether awsbnkctl-test is present.
func TestNamespaceExists(ctx context.Context, cs kubernetes.Interface) (bool, error) {
	_, err := cs.CoreV1().Namespaces().Get(ctx, K8sTestNamespace, metav1.GetOptions{})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	}
	return false, fmt.Errorf("namespace %s: %w", K8sTestNamespace, err)
}

// EnsureTestNamespace creates awsbnkctl-test when it is missing and reports
// whether it did. Every probe Job runs there and nothing else provisions it,
// so the backend, phase 12 and the bnk heal test-namespace repair share this.
func EnsureTestNamespace(ctx context.Context, cs kubernetes.Interface) (bool, error) {
	exists, err := TestNamespaceExists(ctx, cs)
	if err != nil || exists {
		return false, err
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: K8sTestNamespace, Labels: testNamespaceLabels}}
	if _, err := cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, fmt.Errorf("creating namespace %s: %w", K8sTestNamespace, err)
	}
	return true, nil
}

// ensureClient lazy-builds the client + REST config. Reuses the
// integrator's k8s package conventions (DefaultKubeconfigPath +
// rest.InClusterConfig fallback). The initFn hook lets tests substitute
// a fake clientset without touching the real loader.
func (b *K8sBackend) ensureClient() (kubernetes.Interface, *rest.Config, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client != nil && b.config != nil {
		return b.client, b.config, nil
	}
	if b.initFn != nil {
		cs, cfg, err := b.initFn()
		if err != nil {
			return nil, nil, err
		}
		b.client, b.config = cs, cfg
		return cs, cfg, nil
	}
	cs, cfg, err := defaultK8sInit()
	if err != nil {
		return nil, nil, err
	}
	b.client, b.config = cs, cfg
	return cs, cfg, nil
}

// defaultK8sInit is the package-level seam mirroring internal/k8s.
// Exposed as a var so tests can stub the kubeconfig discovery without
// importing internal/k8s (which would create a backend → k8s package
// cycle if we ever added the reverse import).
var defaultK8sInit = func() (kubernetes.Interface, *rest.Config, error) {
	return nil, nil, errors.New("k8s backend: no client initialiser registered; call SetK8sInit from internal/cli before dispatch")
}

// SetK8sInit lets the CLI layer wire its kubeconfig-loading logic into
// the backend without forcing a circular import. internal/cli/k_root.go
// (or wherever feels natural) calls this in init().
func SetK8sInit(fn func() (kubernetes.Interface, *rest.Config, error)) {
	defaultK8sInit = fn
}

// jobToolCmdOverride declares the in-container binary `runAsJob` should
// exec when argv[0] is a known tool name. Entries here mirror the
// docker backend's `dockerImageBinary` map (see `internal/exec/docker.go`)
// so the k8s Job path and the docker container path resolve the same
// tool→binary mapping.
//
// The entry is necessary when the tool's image has an ENTRYPOINT the
// caller needs to bypass (e.g. the awsbnkctl tools image's ENTRYPOINT
// doesn't match the dns-probe re-exec path that needs the awsbnkctl
// binary directly).
//
// Tools NOT in this map keep the legacy shape
// (`Container.Command = argv[1:]`, image's ENTRYPOINT picks the
// binary) — `iperf3` (image's ENTRYPOINT="iperf3") continues to work
// without an entry.
var jobToolCmdOverride = map[string][]string{
	"awsbnkctl": {"/usr/local/bin/awsbnkctl"},
}

// runAsJob spawns a one-shot Job in awsbnkctl-test, materialises Files
// + creds via projected Secret(s), waits for Running, streams logs,
// then waits for completion + cleanup.
//
// argv[0] picks the per-tool image (mirrors DockerBackend.toolImages);
// argv[1:] is the in-container command, EXCEPT for tools listed in
// jobToolCmdOverride which get a full `Command + Args` shape that
// bypasses the image's ENTRYPOINT.
func (b *K8sBackend) runAsJob(ctx context.Context, cs kubernetes.Interface, argv []string, opts RunOpts) (int, error) {
	tool := argv[0]
	image, ok := toolImages[tool]
	if !ok {
		// Test path: argv[0] is a literal image ref + argv[1:] is
		// the in-container command. Mirrors docker.go's fallback.
		image = tool
	}

	// Job + per-Job files Secret share a randomised suffix for trivial
	// teardown via owner refs. Sanitise tool name into k8s-label-safe
	// shape: docker-style refs ("busybox:latest", "myrepo/img@sha256:…")
	// surface in the test fallback above and would otherwise trip
	// label-validation regex on Job creation.
	suffix := rand.String(6)
	safeTool := jobNameSanitizer.Replace(tool)
	jobName := "awsbnkctl-" + safeTool + "-" + suffix
	if len(jobName) > 60 {
		jobName = jobName[:60]
	}
	filesSecretName := jobName + "-files"

	// Files Secret (per-Job, owned by the Job for auto-delete).
	var filesSecretCreated bool
	if len(opts.Files) > 0 {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      filesSecretName,
				Namespace: K8sTestNamespace,
				Labels:    map[string]string{"awsbnkctl.io/job": jobName},
			},
			Type: corev1.SecretTypeOpaque,
			Data: opts.Files,
		}
		if _, err := cs.CoreV1().Secrets(K8sTestNamespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return k8sExitFailedToStart, fmt.Errorf("creating files secret: %w", err)
		}
		filesSecretCreated = true
	}

	// Cmd + Args translation:
	//   - For entrypoint-bypass tools, prepend the override's argv as
	//     Command (overrides image's ENTRYPOINT) and pass argv[1:] as
	//     Args (replaces image's CMD).
	//   - Otherwise pass argv[1:] as Args so the image's ENTRYPOINT
	//     stays in place and the supplied args flow to it. Setting
	//     Command in the no-override path would OVERRIDE the image's
	//     ENTRYPOINT, causing the kubelet to try exec'ing argv[1]
	//     directly as a binary (e.g., "-c" for iperf3 → exec /-c →
	//     CreateContainerError). This was the v1.0.2 fix for the
	//     L2 throughput Job's CreateContainerError; pre-fix the
	//     comment claimed "image's ENTRYPOINT picks the binary"
	//     which contradicts actual k8s Container.Command semantics.
	var cmdArgv []string
	var argsArgv []string
	if override, hasOverride := jobToolCmdOverride[tool]; hasOverride {
		cmdArgv = append([]string(nil), override...)
		argsArgv = argv[1:]
	} else {
		argsArgv = argv[1:]
	}

	job := buildJobSpecWithArgs(jobName, image, cmdArgv, argsArgv, opts, filesSecretCreated, filesSecretName)

	created, err := cs.BatchV1().Jobs(K8sTestNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		if filesSecretCreated {
			_ = cs.CoreV1().Secrets(K8sTestNamespace).Delete(context.Background(), filesSecretName, metav1.DeleteOptions{})
		}
		return k8sExitFailedToStart, fmt.Errorf("creating job: %w", err)
	}

	// Owner-ref the files Secret to the Job so it auto-deletes on Job
	// cleanup. Done after Create so we have the Job's UID.
	if filesSecretCreated {
		_ = setSecretOwnerRef(ctx, cs, filesSecretName, created)
	}

	// Cleanup goroutine: ctx cancel → delete Job + Secret. Job's
	// ttlSecondsAfterFinished handles the happy-path cleanup.
	cancelDone := make(chan struct{})
	// #nosec G118 -- this goroutine intentionally uses a fresh context.Background(); the parent ctx is what just got cancelled and we still need to delete the Job
	go func() {
		select {
		case <-ctx.Done():
			cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // #nosec G118
			defer cancel()
			pp := metav1.DeletePropagationForeground
			_ = cs.BatchV1().Jobs(K8sTestNamespace).Delete(cleanCtx, jobName, metav1.DeleteOptions{PropagationPolicy: &pp})
			if filesSecretCreated {
				_ = cs.CoreV1().Secrets(K8sTestNamespace).Delete(cleanCtx, filesSecretName, metav1.DeleteOptions{})
			}
		case <-cancelDone:
		}
	}()
	defer close(cancelDone)

	// Wait for the Job's pod to be Running.
	pod, err := waitForJobPodRunning(ctx, cs, jobName, k8sJobReadyTimeout)
	if err != nil {
		// A pod that never starts never finishes, so ttlSecondsAfterFinished
		// would leave the Job behind; delete it here.
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // #nosec G118 -- ctx may already be cancelled
		defer cancel()
		pp := metav1.DeletePropagationForeground
		_ = cs.BatchV1().Jobs(K8sTestNamespace).Delete(cleanCtx, jobName, metav1.DeleteOptions{PropagationPolicy: &pp})
		return k8sExitStartedThenFailed, fmt.Errorf("waiting for job pod: %w", err)
	}

	// Stream logs.
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		stdout, stdoutClose := wrapForRedaction(opts.Stdout, opts.Credentials)
		defer func() {
			if stdoutClose != nil {
				_ = stdoutClose()
			}
		}()
		stream, lerr := cs.CoreV1().Pods(K8sTestNamespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow: true,
		}).Stream(ctx)
		if lerr != nil {
			return
		}
		defer stream.Close()
		_, _ = io.Copy(stdout, stream)
	}()

	// Wait for Job completion.
	rc, werr := waitForJobCompletion(ctx, cs, jobName)
	<-streamDone
	if werr != nil {
		return k8sExitStartedThenFailed, werr
	}
	return rc, nil
}

// buildJobSpec renders the per-Job spec. SCC-clean (matches the iperf3
// SCC fix). Mounts the per-Job files Secret at /work read-only when
// present. No credentials are injected: the Job inherits only the env
// vars the caller passes through RunOpts.Env. The Credentials struct keeps only the
// kubeconfig surface (see internal/exec/creds.go).
//
// buildJobSpecWithArgs is buildJobSpec extended for the entrypoint-
// bypass shape. When `args` is non-nil, the rendered container has
// `Command=cmd, Args=args`; this overrides the image's Docker
// ENTRYPOINT and runs `cmd[0] cmd[1:] ...args` instead. For tools that
// keep the legacy "image ENTRYPOINT picks the binary" shape, pass
// args=nil.
//
// Example: the dns-probe Job sets
// `cmd=["/usr/local/bin/awsbnkctl"]` + `args=["test","dns",...]` so
// the tools image's ENTRYPOINT doesn't override the binary the dns
// probe wants to run.
func buildJobSpecWithArgs(jobName, image string, cmd, args []string, opts RunOpts, hasFilesSecret bool, filesSecretName string) *batchv1.Job {
	envVars := buildJobEnv(opts)
	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount
	if hasFilesSecret {
		volumes = append(volumes, corev1.Volume{
			Name: "files",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: filesSecretName,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "files",
			MountPath: "/work",
			ReadOnly:  true,
		})
	}

	workDir := opts.WorkDir
	if workDir == "" && hasFilesSecret {
		workDir = "/work"
	}

	ttl := int32(60)
	backoffLimit := int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: K8sTestNamespace,
			Labels:    map[string]string{"awsbnkctl.io/managed": "true", "app": jobName},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": jobName},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptrBool(true),
						// Pin the UID: the public tool images (for
						// example networkstatic/iperf3) declare no USER,
						// and RunAsNonRoot alone then fails the pod with
						// CreateContainerConfigError. EKS has no SCC
						// range to collide with.
						RunAsUser:  ptrInt64(jobRunAsUID),
						RunAsGroup: ptrInt64(jobRunAsUID),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes: volumes,
					Containers: []corev1.Container{{
						Name:       "tool",
						Image:      image,
						Command:    cmd,
						Args:       args,
						Env:        envVars,
						WorkingDir: workDir,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptrBool(false),
							RunAsNonRoot:             ptrBool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
						VolumeMounts: mounts,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					}},
				},
			},
		},
	}
}

// buildJobEnv merges opts.Env (caller-supplied KEY=VALUE) with
// opts.Credentials.EnvVars() (resolver-derived). Late entries override
// earlier ones, mirroring the local backend's semantics.
func buildJobEnv(opts RunOpts) []corev1.EnvVar {
	merged := make(map[string]string)
	for _, kv := range opts.Env {
		k, v, ok := splitKV(kv)
		if !ok {
			continue
		}
		merged[k] = v
	}
	if opts.Credentials != nil {
		for _, kv := range opts.Credentials.EnvVars() {
			k, v, ok := splitKV(kv)
			if !ok {
				continue
			}
			merged[k] = v
		}
	}
	out := make([]corev1.EnvVar, 0, len(merged))
	for k, v := range merged {
		out = append(out, corev1.EnvVar{Name: k, Value: v})
	}
	return out
}

func splitKV(kv string) (string, string, bool) {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], i > 0
		}
	}
	return "", "", false
}

// waitForJobPodRunning polls until the Job has a pod in Running phase.
// Returns the pod (so the caller can stream logs).
func waitForJobPodRunning(ctx context.Context, cs kubernetes.Interface, jobName string, timeout time.Duration) (*corev1.Pod, error) {
	deadline := time.Now().Add(timeout)
	pollInt := 1 * time.Second
	for {
		pods, err := cs.CoreV1().Pods(K8sTestNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=" + jobName,
		})
		if err == nil {
			for i := range pods.Items {
				p := &pods.Items[i]
				if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
					return p, nil
				}
				// Surface terminal-ish waiting reasons early so we don't
				// burn the whole timeout on configs the kubelet will
				// never start (PSS rejects, runAsNonRoot mismatches,
				// missing images, crash loops).
				for _, st := range p.Status.ContainerStatuses {
					if st.State.Waiting != nil {
						switch st.State.Waiting.Reason {
						case "ImagePullBackOff", "ErrImagePull",
							"CrashLoopBackOff",
							"CreateContainerConfigError",
							"CreateContainerError",
							"RunContainerError",
							"InvalidImageName":
							return nil, fmt.Errorf("pod %s: %s (%s)", p.Name, st.State.Waiting.Reason, st.State.Waiting.Message)
						}
					}
				}
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for job %s pod to be Running", jobName)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInt):
		}
	}
}

// waitForJobCompletion polls the Job until Complete or Failed. Returns
// the wrapped container's exit code (0 on success; the actual code on
// failure).
func waitForJobCompletion(ctx context.Context, cs kubernetes.Interface, jobName string) (int, error) {
	pollInt := 1 * time.Second
	for {
		j, err := cs.BatchV1().Jobs(K8sTestNamespace).Get(ctx, jobName, metav1.GetOptions{})
		if err == nil {
			for _, cond := range j.Status.Conditions {
				if cond.Status != corev1.ConditionTrue {
					continue
				}
				switch cond.Type {
				case batchv1.JobComplete:
					return 0, nil
				case batchv1.JobFailed:
					// Inspect the latest pod's container terminated state for
					// the wrapped tool's exit code.
					rc := jobFailureExitCode(ctx, cs, jobName)
					return rc, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return 137, ctx.Err()
		case <-time.After(pollInt):
		}
	}
}

// jobFailureExitCode pulls the most recent pod's tool-container
// terminated.exitCode. Returns 1 when the data isn't available — the
// caller treats any non-zero as failure.
func jobFailureExitCode(ctx context.Context, cs kubernetes.Interface, jobName string) int {
	pods, err := cs.CoreV1().Pods(K8sTestNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=" + jobName,
	})
	if err != nil || len(pods.Items) == 0 {
		return 1
	}
	for _, p := range pods.Items {
		for _, st := range p.Status.ContainerStatuses {
			if st.State.Terminated != nil {
				return int(st.State.Terminated.ExitCode)
			}
		}
	}
	return 1
}

// setSecretOwnerRef stamps the Job as the owner of the per-Job files
// Secret so kube garbage-collection cleans it up when the Job is
// deleted (TTL or explicit). Best-effort — failures don't break the run.
func setSecretOwnerRef(ctx context.Context, cs kubernetes.Interface, name string, owner *batchv1.Job) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"ownerReferences":[{"apiVersion":"batch/v1","kind":"Job","name":%q,"uid":%q,"controller":true,"blockOwnerDeletion":true}]}}`, owner.Name, owner.UID))
	_, err := cs.CoreV1().Secrets(K8sTestNamespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

func ptrBool(b bool) *bool { return &b }

func ptrInt64(v int64) *int64 { return &v }

// jobRunAsUID is the non-root UID every probe Job runs as; it matches the
// USER in the bundled tools images.
const jobRunAsUID int64 = 1000

func init() {
	Register("k8s", &K8sBackend{})
}
