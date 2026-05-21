package phases

import (
	"bytes"
	"fmt"
	"text/template"
)

// renderKubeconfig renders the standard EKS exec-auth kubeconfig as raw bytes.
// Both Phase09 (in-memory, for forge) and Phase11 (write-to-file) use this.
// The template is defined in phase11_kubeconfig.go alongside the file-write path.
func renderKubeconfig(clusterARN, endpoint, ca, clusterName, region string) ([]byte, error) {
	data := kubeconfigData{
		ClusterARN:  clusterARN,
		Endpoint:    endpoint,
		CA:          ca,
		Region:      region,
		ClusterName: clusterName,
	}
	tmpl, err := template.New("kubeconfig").Parse(kubeconfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("renderKubeconfig: parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("renderKubeconfig: executing template: %w", err)
	}
	return buf.Bytes(), nil
}
