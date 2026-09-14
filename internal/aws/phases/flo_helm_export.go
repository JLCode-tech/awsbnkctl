package phases

// Exported entry points into the phase 14 Helm glue so other packages
// (awsbnkctl bnk upgrade) can drive the same OCI-authenticated Helm SDK
// against the FLO release without duplicating the registry login and action
// configuration.

const (
	// FLOReleaseName is the Helm release name of the F5 Lifecycle Operator.
	FLOReleaseName = floReleaseName
	// FLONamespace is the namespace the FLO release lives in.
	FLONamespace = floNamespace
	// FLOChartRef is the OCI reference of the FLO chart on repo.f5.com.
	FLOChartRef = floChartRef
	// FLOValuesTemplatePath is the embedded flo-values template
	// (render.RenderFLOValues turns it into the Helm values map).
	FLOValuesTemplatePath = floValuesYAMLPath
)

// HelmInstaller is the Helm SDK surface phase 14 and bnk upgrade share:
// list, install, upgrade, uninstall and OCI pull of the FLO chart.
type HelmInstaller = helmInstaller

// NewHelmInstaller logs in to repo.f5.com with the FAR key and returns a
// HelmInstaller bound to the FLO namespace of the cluster kubeconfigPath
// points at (empty = kubectl's default lookup).
func NewHelmInstaller(kubeconfigPath, farKeyB64 string) (HelmInstaller, error) {
	return newHelmInstaller(kubeconfigPath, farKeyB64)
}
