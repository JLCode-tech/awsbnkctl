package cli

import (
	"fmt"
	"os"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

// Cross-verb helpers shared by test.go + doctor_backend.go.
//
// These are legacy back-compat shims split out from the deleted IBM
// lifecycle verbs. The file is trimmed to the helpers that still have
// live callers in test.go / doctor_backend.go (workspaceEnv,
// resolveBackendSpecWith). IBM Cloud API key injection was
// removed when AWS credentials moved to the SDK chain in internal/aws
// and IRSA in-cluster; there is nothing left for these helpers to inject.

// workspaceEnv composes a child-process env for inherited tool
// passthroughs. Returns the host env plus KUBECONFIG if a kubeconfig
// is on disk. AWS credentials are resolved by the SDK chain (env /
// profile / instance role / SSO) — no cred-shaped env vars are
// injected here.
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

// perToolDefaultBackend is the per-tool default backend table. AWS
// doesn't ship a CLI passthrough — the binary uses internal/aws SDK
// directly.
var perToolDefaultBackend = map[string]string{
	"iperf3": "k8s",
}
