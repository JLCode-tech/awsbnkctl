// Package render provides Go text/template rendering helpers for BNK manifest
// templates. Templates use {{ .Field }} syntax (not shell $VAR envsubst).
package render

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// CertChainVars holds the substitution variables for shared/bnk-cert-chain.yaml.
// All fields are derived from cluster.metadata.name at Phase 12 entry time.
type CertChainVars struct {
	SelfSignedIssuer string // <cluster>-selfsigned-cluster-issuer
	CACertName       string // <cluster>-ca
	CASecretName     string // <cluster>-ca-secret
	CAIssuer         string // <cluster>-ca-cluster-issuer
}

// CertChainVarsFromCluster derives the BNK cert chain template variables from
// the cluster intent. All variable names match aws-gpu-setup's convention so
// existing cert naming is consistent between bash and Go paths.
func CertChainVarsFromCluster(cl *intent.Cluster) CertChainVars {
	name := cl.Metadata.Name
	return CertChainVars{
		SelfSignedIssuer: name + "-selfsigned-cluster-issuer",
		CACertName:       name + "-ca",
		CASecretName:     name + "-ca-secret",
		CAIssuer:         name + "-ca-cluster-issuer",
	}
}

// Render executes a Go text/template given in tmpl with data as the dot-value
// and returns the rendered bytes. Returns a descriptive error on any parse or
// execution failure.
func Render(tmpl []byte, data interface{}) ([]byte, error) {
	t, err := template.New("manifest").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("render: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render: execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderCertChain renders the BNK cert chain template with vars derived from
// the cluster intent. Convenience wrapper over Render + CertChainVarsFromCluster.
func RenderCertChain(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	vars := CertChainVarsFromCluster(cl)
	return Render(tmpl, vars)
}
