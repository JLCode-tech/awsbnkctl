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

// Cross-verb helpers shared by test.go + tfvars.go + doctor_backend.go.
//
// Origin: Sprint 0 split-out from the deleted IBM lifecycle verbs.
// Sprint 2 (PRD 04 fold) retargets the workspace schema onto AWS;
// these helpers stay as the smallest cross-package surface that
// test.go / tfvars.go / doctor_backend.go can share without an import
// cycle. The IBMCLOUD_API_KEY env propagation that the original
// helpers carried lives in internal/cred + internal/exec now;
// retiring those packages' IBM lineage is tracked as a Sprint 3 task
// per PRD 04. See `issues/issue_sprint2_staff.md` Issue 1 for the
// retirement plan.

// workspaceEnv composes a child-process env for inherited tool
// passthroughs. Returns the host env plus KUBECONFIG if a kubeconfig
// is on disk. AWS credentials are resolved by the SDK chain (env /
// profile / instance role) so no cred-shaped env vars are injected
// here; the IBM API key path moved to internal/cred + internal/exec
// in the v0.x lineage and stays there until Sprint 3 retargets the
// docker / k8s execution backends per PRD 04.
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
// §"Tool migration plan"). The IBM `ibmcloud` row retired with the
// IBM cleanup; AWS doesn't ship a CLI passthrough (the binary uses
// internal/aws SDK directly per PRD 00 § "Inheritance map").
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
// the test surface is the only caller in this file. The cred package
// still owns the IBM API key resolution (used by docker/k8s exec
// backends today); Sprint 3 retargets that whole flow per PRD 04 and
// this stub retires alongside it.
var _ = func() any {
	var _ = (*cred.Resolver)(nil)
	var _ = context.Background
	return nil
}()
