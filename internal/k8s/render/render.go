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

// FLOValuesVars holds the substitution variables for shared/flo-values.yaml.tmpl.
// All fields are derived from the cluster intent + slice-5 state at Phase 14 entry.
type FLOValuesVars struct {
	CAIssuer      string // <cluster>-ca-cluster-issuer
	FARSecretName string // far-secret
	JWT           string // raw JWT token contents (NOT base64)
	ClusterName   string // cl.Metadata.Name
}

// FLOValuesVarsFromCluster derives the FLO values template variables from the
// cluster intent. JWT is passed explicitly because it is file-read data, not
// derivable from the intent struct alone.
func FLOValuesVarsFromCluster(cl *intent.Cluster, jwt string) FLOValuesVars {
	cvars := CertChainVarsFromCluster(cl)
	return FLOValuesVars{
		CAIssuer:      cvars.CAIssuer,
		FARSecretName: "far-secret",
		JWT:           jwt,
		ClusterName:   cl.Metadata.Name,
	}
}

// RenderFLOValues renders the FLO values template with vars derived from
// the cluster intent and the raw JWT string.
func RenderFLOValues(tmpl []byte, cl *intent.Cluster, jwt string) ([]byte, error) {
	vars := FLOValuesVarsFromCluster(cl, jwt)
	return Render(tmpl, vars)
}

// OTELCertsVars holds the substitution variables for shared/otel-certs.yaml.
// All fields are derived from the cluster intent at Phase 15 entry.
type OTELCertsVars struct {
	OTELSvrCert     string // external-otelsvr
	OTELSvrSecret   string // external-otelsvr-secret
	OTELF5IngCert   string // external-f5ingotelsvr
	OTELF5IngSecret string // external-f5ingotelsvr-secret
	OperatorNS      string // f5-cne-core
	CAIssuer        string // <cluster>-ca-cluster-issuer
}

// OTELCertsVarsFromCluster derives the OTEL certs template variables from the
// cluster intent. Names match aws-gpu-setup's vars.env OTEL_* constants.
func OTELCertsVarsFromCluster(cl *intent.Cluster) OTELCertsVars {
	cvars := CertChainVarsFromCluster(cl)
	return OTELCertsVars{
		OTELSvrCert:     "external-otelsvr",
		OTELSvrSecret:   "external-otelsvr-secret",
		OTELF5IngCert:   "external-f5ingotelsvr",
		OTELF5IngSecret: "external-f5ingotelsvr-secret",
		OperatorNS:      "f5-cne-core",
		CAIssuer:        cvars.CAIssuer,
	}
}

// RenderOTELCerts renders the OTEL certs template with vars derived from
// the cluster intent.
func RenderOTELCerts(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	vars := OTELCertsVarsFromCluster(cl)
	return Render(tmpl, vars)
}
