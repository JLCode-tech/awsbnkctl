package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
	execbackend "github.com/JLCode-tech/awsbnkctl/internal/exec"
	"github.com/JLCode-tech/awsbnkctl/internal/remote"
	"github.com/JLCode-tech/awsbnkctl/internal/tf"
)

// loadTFOutputsForTarget pulls a flat map[string]string of TF outputs
// when the target's KeySource is "tf-output:<name>". Returns nil for
// every other source (we don't need TF state, so don't open it).
//
// On open / read errors when we DO need outputs, returns the error so
// the caller fails fast — silently falling back to "key not found"
// produces a confusing downstream message.
func loadTFOutputsForTarget(ctx context.Context, cctx *config.Context, t *remote.Target) (map[string]string, error) {
	if t == nil || !needsTFOutputs(t) {
		return nil, nil
	}
	stateDir, err := config.WorkspaceStateDir(cctx.WorkspaceName)
	if err != nil {
		return nil, err
	}
	tfws, err := tf.Open(ctx, cctx.WorkspaceName, cctx.Workspace, stateDir, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("opening tf workspace: %w", err)
	}
	outs, err := tfws.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading tf outputs: %w", err)
	}
	flat := make(map[string]string, len(outs))
	for k, v := range outs {
		var s string
		if json.Unmarshal(v.Value, &s) == nil {
			flat[k] = s
		}
	}
	return flat, nil
}

func needsTFOutputs(t *remote.Target) bool {
	if t == nil {
		return false
	}
	return t.KeyPath == "" && t.KeySource != "" && t.KeySource != "agent"
}

func init() {
	// Wire the SSH backend's target resolver to the same tf-output-aware
	// signer the legacy --on path uses. The exec package can't import
	// internal/cli (cycle), so the cli layer pushes a fully-resolved
	// target back into the backend via SetSSHTargetResolver.
	//
	// PRD 03 §"SSH" — backend resolves its target identically to --on so
	// users don't have to maintain two key-resolution paths.
	execbackend.SetSSHTargetResolver(func(workspace, name string) (*remote.Target, map[string][]byte, error) {
		if workspace == "" {
			return nil, nil, fmt.Errorf("ssh backend: no workspace set")
		}
		t, err := remote.LoadTarget(workspace, name)
		if err != nil {
			return nil, nil, err
		}
		// Reload workspace cctx so loadTFOutputsForTarget can pull
		// outputs (only needed for the tf-output: key source).
		cctx, err := config.New(workspace)
		if err != nil {
			return nil, nil, err
		}
		tfOutputs, err := loadTFOutputsForTarget(context.Background(), cctx, t)
		if err != nil {
			return nil, nil, err
		}
		signer, err := remote.ResolveSigner(t, tfOutputs)
		if err != nil {
			return nil, nil, err
		}
		t.Signer = signer
		t.HostKeyCallback = remote.HostKeyCallback(remote.HostKeyOptions{Insecure: flagInsecureHostKey})
		return t, nil, nil
	})
}
