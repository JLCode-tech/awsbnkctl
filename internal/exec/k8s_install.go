package exec

import (
	_ "embed"
)

// k8sInstallYAML is the multi-document YAML template applied by
// `awsbnkctl ops install`. Embedded at build time so the binary is
// self-contained — no need to ship a separate manifests directory.
//
// Template placeholders substituted at apply-time. The manifest still
// carries inherited IBM-shape placeholders; the AWS retarget of
// `ops install` (Sprint 6 hardening) replaces these with IRSA-based
// surface so the ops Pod consumes AWS via projected SA token rather
// than a long-lived Secret.
//
//	${ROTATED_AT}  — RFC3339 timestamp of the apply, stamped on the
//	                 Secret as an annotation so `ops show` can render
//	                 rotation.
//	${OPS_IMAGE}   — the awsbnkctl tools image ref; version-pinned to
//	                 internal/cli.Version.
//
//go:embed k8s_install.yaml
var k8sInstallYAML string

// K8sInstallYAML returns the embedded install manifest template. The
// CLI layer (internal/cli/ops.go) substitutes ${ROTATED_AT} and
// ${OPS_IMAGE} before applying.
//
// Exported as a function (not a var) so callers can't accidentally
// mutate the embedded copy at runtime — the substitution happens on
// a strings.NewReplacer.Replace return value, which is a fresh string.
func K8sInstallYAML() string { return k8sInstallYAML }
