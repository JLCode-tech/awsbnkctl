package phases

import "github.com/JLCode-tech/awsbnkctl/internal/intent"

// Exported entry points into the phase 14 Helm glue so other packages
// (awsbnkctl bnk upgrade) can drive the same OCI-authenticated Helm SDK
// against the FLO release without duplicating the registry login and action
// configuration.

const (
	// FLOReleaseName is the Helm release name of the F5 Lifecycle Operator.
	FLOReleaseName = floReleaseName
	// FLONamespace is the namespace the FLO release lives in.
	FLONamespace = floNamespace
	// FLOValuesTemplatePath is the embedded flo-values template
	// (render.RenderFLOValues turns it into the Helm values map).
	FLOValuesTemplatePath = floValuesYAMLPath
)

// FLOChartRefFor returns the OCI reference of the FLO chart for cl's
// configured registry host (cl.Bnk.RegistryHost, default "repo.f5.com" — see
// BnkSpec.RegistryHost).
func FLOChartRefFor(cl *intent.Cluster) string {
	return floChartRefFor(cl)
}

// HelmInstaller is the Helm SDK surface phase 14 and bnk upgrade share:
// list, install, upgrade, uninstall and OCI pull of the FLO chart.
type HelmInstaller = helmInstaller

// NewHelmInstaller logs in to registryHost with the FAR key and returns a
// HelmInstaller bound to the FLO namespace of the cluster kubeconfigPath
// points at (empty = kubectl's default lookup). registryHost is normally
// cl.Bnk.RegistryHost (default "repo.f5.com" — see BnkSpec.RegistryHost).
func NewHelmInstaller(kubeconfigPath, farKeyB64, registryHost string) (HelmInstaller, error) {
	return newHelmInstaller(kubeconfigPath, farKeyB64, registryHost)
}
