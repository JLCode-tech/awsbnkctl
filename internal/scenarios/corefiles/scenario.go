package corefiles

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/JLCode-tech/awsbnkctl/internal/scenarios"
)

//go:embed manifests/*.yaml
var manifestFS embed.FS

const (
	scnName      = "core-file-collection"
	scnTitle     = "Core file collection (how-to #4) — CNEInstance.spec.coreCollection.enabled + CoreMond"
	scnNamespace = "default"
)

var (
	cneInstanceGVR = schema.GroupVersionResource{
		Group:    "k8s.f5net.com",
		Version:  "v1",
		Resource: "cneinstances",
	}
	coreMondGVR = schema.GroupVersionResource{
		Group:    "k8s.f5.com",
		Version:  "v1",
		Resource: "coremonds",
	}
)

func init() { scenarios.Register(&scenario{}) }

type VerifyDeps struct {
	CheckCoreMondFn   func(ctx context.Context, sctx *scenarios.Context) (crOK bool, dsOK bool, condOK bool, details string)
	CheckTMMVolumesFn func(ctx context.Context, sctx *scenarios.Context) (bool, string)
}

func realVerifyDeps() VerifyDeps {
	return VerifyDeps{
		CheckCoreMondFn: func(ctx context.Context, sctx *scenarios.Context) (bool, bool, bool, string) {
			if sctx.Dynamic == nil || sctx.Clientset == nil {
				return true, true, true, "dry-run verification pass"
			}
			// Search for CoreMond CR in f5-cne-core and default
			crFound := false
			crNS := ""
			for _, ns := range []string{"f5-cne-core", "default"} {
				list, err := sctx.Dynamic.Resource(coreMondGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
				if err == nil && len(list.Items) > 0 {
					crFound = true
					crNS = ns
					break
				}
			}

			// Check DaemonSet
			dsFound := false
			if crNS != "" {
				dsList, err := sctx.Clientset.AppsV1().DaemonSets(crNS).List(ctx, metav1.ListOptions{
					LabelSelector: "app=f5-coremond",
				})
				if err == nil && len(dsList.Items) > 0 {
					dsFound = true
				}
			}

			// Check CNEInstance condition
			condOK := false
			cne, err := sctx.Dynamic.Resource(cneInstanceGVR).Namespace("default").Get(ctx, "bnk-instance", metav1.GetOptions{})
			if err == nil {
				conditions, _, _ := scenarios.NestedSlice(cne.Object, "status", "conditions")
				for _, cRaw := range conditions {
					if c, ok := cRaw.(map[string]interface{}); ok {
						t, _ := c["type"].(string)
						st, _ := c["status"].(string)
						if (t == "CoremondAvailable" || t == "CoreMonAvailable") && st == "True" {
							condOK = true
							break
						}
					}
				}
			}

			details := fmt.Sprintf("crFound=%v (ns=%s), dsFound=%v, condOK=%v", crFound, crNS, dsFound, condOK)
			return crFound, dsFound, condOK, details
		},
		CheckTMMVolumesFn: func(ctx context.Context, sctx *scenarios.Context) (bool, string) {
			if sctx.Clientset == nil {
				return true, "dry-run pass"
			}
			ds, err := sctx.Clientset.AppsV1().DaemonSets("default").Get(ctx, "f5-tmm", metav1.GetOptions{})
			if err != nil {
				return false, "f5-tmm daemonset not found: " + err.Error()
			}
			for _, v := range ds.Spec.Template.Spec.Volumes {
				if strings.Contains(strings.ToLower(v.Name), "core") || strings.Contains(strings.ToLower(v.Name), "crash") {
					return true, "found volume: " + v.Name
				}
			}
			return false, "no core/crash volume found in f5-tmm spec"
		},
	}
}

type scenario struct {
	vDeps *VerifyDeps
}

func (s *scenario) Name() string             { return scnName }
func (s *scenario) Title() string            { return scnTitle }
func (s *scenario) Rating() scenarios.Rating { return scenarios.Green }
func (s *scenario) Dependencies() []string   { return []string{} }
func (s *scenario) Description() string {
	return strings.TrimSpace(`
Demonstrates BNK's core-dump collection infrastructure: enables spec.coreCollection.enabled
on CNEInstance, reconciling CoreMond DaemonSet and crash host mounts on TMM pods.
`)
}

func (s *scenario) Namespace(ctx *scenarios.Context) string {
	return scnNamespace
}

func (s *scenario) Manifests(ctx *scenarios.Context) ([]string, error) {
	var paths []string
	err := fs.WalkDir(manifestFS, "manifests", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		body, rerr := manifestFS.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		base := p[len("manifests/"):]
		outPath, werr := scenarios.WriteManifest(ctx.WorkspaceDir, scnName, base, string(body))
		if werr != nil {
			return werr
		}
		paths = append(paths, outPath)
		return nil
	})
	return paths, err
}

func (s *scenario) Apply(ctx *scenarios.Context) error {
	if ctx.Dynamic == nil {
		return nil
	}
	patch := []byte(`{"spec":{"coreCollection":{"enabled":true},"advanced":{"coremon":{"hostPath":true}}}}`)
	_, err := ctx.Dynamic.Resource(cneInstanceGVR).Namespace("default").Patch(
		ctx.Ctx, "bnk-instance", types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil && !scenarios.IsNotFound(err) {
		return fmt.Errorf("patch CNEInstance for coreCollection: %w", err)
	}
	return nil
}

func (s *scenario) Verify(ctx *scenarios.Context) scenarios.Result {
	deps := realVerifyDeps()
	if s.vDeps != nil {
		deps = *s.vDeps
	}

	var assertions []scenarios.Assertion

	crOK, dsOK, condOK, details := deps.CheckCoreMondFn(ctx.Ctx, ctx)
	assertions = append(assertions, scenarios.Assertion{
		Description: "CoreMond CR auto-created by FLO",
		OK:          crOK,
		Got:         details,
	})
	assertions = append(assertions, scenarios.Assertion{
		Description: "CoreMond DaemonSet scheduled",
		OK:          dsOK,
		Got:         details,
	})
	assertions = append(assertions, scenarios.Assertion{
		Description: "CNEInstance CoreMonAvailable=True status condition",
		OK:          condOK,
		Got:         details,
	})

	volOK, volDetails := deps.CheckTMMVolumesFn(ctx.Ctx, ctx)
	assertions = append(assertions, scenarios.Assertion{
		Description: "TMM DaemonSet template includes crash / core dump volume mounts",
		OK:          volOK,
		Got:         volDetails,
	})

	res := scenarios.Result{
		Assertions: assertions,
	}
	return scenarios.FinalizeResult(res)
}

func (s *scenario) Cleanup(ctx *scenarios.Context) error {
	// Revert patch is preserved as no-op to protect subsequent test runs from controller churn
	return nil
}
