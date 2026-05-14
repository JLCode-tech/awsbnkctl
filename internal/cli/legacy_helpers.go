package cli

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/cred"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

// Sprint 0 carry-overs. The IBM-coupled CLI verbs (cluster.go,
// cluster_phase.go, init.go's wizard, ops.go) shipped a handful of
// shared helpers (workspaceEnv, resolveBackendSpecWith, podReady,
// refDescription) used by surface that survives Sprint 0 (test.go,
// tfvars.go, doctor_backend.go). The verb files were deleted alongside
// internal/ibm; these helpers live here in their AWS-tracker-stripped
// form until Sprint 1+ retargets the AWS credential propagation.
//
// Each helper is intentionally minimal: existing call sites compile,
// runtime behaviour is "best-effort + Sprint 1 will replace this".

// workspaceEnv composes a child-process env for inherited tool
// passthroughs. The Sprint 0 stub returns the host env plus KUBECONFIG
// if a kubeconfig is on disk — the IBM API key / region env vars that
// the legacy implementation injected are dropped pending the Sprint 2
// AWS credential adapter (PRD 04 + PRD 08).
func workspaceEnv() (*config.Context, []string, error) {
	cctx, err := config.New(flagWorkspace)
	if err != nil {
		return nil, nil, err
	}
	if cctx.Workspace == nil {
		return nil, nil, fmt.Errorf("workspace %q is not initialised; run `awsbnkctl init` first", cctx.WorkspaceName)
	}
	env := os.Environ()
	if path := k8s.DefaultKubeconfigPath(); path != "" {
		env = append(env, "KUBECONFIG="+path)
	}
	return cctx, env, nil
}

// resolveBackendSpecWith picks the execution backend for tool. Order:
//
//  1. flagOverride (the explicit per-invocation flag)
//  2. workspace's exec.<tool>.backend
//  3. perToolDefaultBackend[tool]
//  4. "local" default
//
// Mirrors the v1.0.x shape from the deleted cluster.go so inherited
// callers (test.go) keep compiling unchanged.
func resolveBackendSpecWith(cctx *config.Context, tool, flagOverride string) string {
	if flagOverride != "" {
		return flagOverride
	}
	if cctx != nil && cctx.Workspace != nil {
		if entry, ok := cctx.Workspace.Exec[tool]; ok && entry.Backend != "" {
			return entry.Backend
		}
	}
	if def, ok := perToolDefaultBackend[tool]; ok {
		return def
	}
	return "local"
}

// perToolDefaultBackend is the per-tool default backend table (PRD 03
// §"Tool migration plan"). Sprint 0 keeps the inherited shape; the
// `ibmcloud` row will be replaced with AWS-shaped equivalents in
// Sprint 1+.
var perToolDefaultBackend = map[string]string{
	"iperf3":    "k8s",
	"terraform": "local",
}

// podReady reports whether a pod's ContainerStatuses agree that it is
// Ready. Lifted verbatim from the deleted ops.go so doctor_backend.go's
// ops-pod check keeps compiling.
func podReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// refDescription renders a TFSourceCfg as a short human-readable string
// for log output. Lifted from the deleted init.go.
func refDescription(c config.TFSourceCfg) string {
	switch c.Type {
	case "", "embedded":
		return "embedded"
	case "github":
		return fmt.Sprintf("%s@%s", c.Repo, c.Ref)
	case "local":
		return "local:" + c.Path
	default:
		return "<unknown>"
	}
}

// silenceUnused keeps the cred + context imports referenced even when
// the test surface is the only caller. Sprint 1+ retires this once the
// AWS credential adapter calls cred.Resolver directly here.
var _ = func() any {
	var _ = (*cred.Resolver)(nil)
	var _ = context.Background
	return nil
}()
