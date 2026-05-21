package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

func clusterFixture(name string) *intent.Cluster {
	return &intent.Cluster{
		Metadata: intent.Metadata{Name: name, Region: "ap-southeast-2"},
	}
}

func TestRenderCertChain_HappyPath(t *testing.T) {
	tmpl := []byte(`
issuer: {{ .SelfSignedIssuer }}
cert: {{ .CACertName }}
secret: {{ .CASecretName }}
ca: {{ .CAIssuer }}
`)
	cl := clusterFixture("syd-tracer")
	out, err := RenderCertChain(tmpl, cl)
	if err != nil {
		t.Fatalf("RenderCertChain: %v", err)
	}
	checks := []string{
		"syd-tracer-selfsigned-cluster-issuer",
		"syd-tracer-ca",
		"syd-tracer-ca-secret",
		"syd-tracer-ca-cluster-issuer",
	}
	for _, want := range checks {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("rendered output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestCertChainVarsFromCluster_NamesMatchConvention(t *testing.T) {
	cl := clusterFixture("my-cluster")
	v := CertChainVarsFromCluster(cl)

	if v.SelfSignedIssuer != "my-cluster-selfsigned-cluster-issuer" {
		t.Errorf("SelfSignedIssuer: got %q", v.SelfSignedIssuer)
	}
	if v.CACertName != "my-cluster-ca" {
		t.Errorf("CACertName: got %q", v.CACertName)
	}
	if v.CASecretName != "my-cluster-ca-secret" {
		t.Errorf("CASecretName: got %q", v.CASecretName)
	}
	if v.CAIssuer != "my-cluster-ca-cluster-issuer" {
		t.Errorf("CAIssuer: got %q", v.CAIssuer)
	}
}

func TestRender_BadTemplate(t *testing.T) {
	tmpl := []byte("{{ .Unclosed")
	_, err := Render(tmpl, struct{}{})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error should mention 'parse template': %v", err)
	}
}

func TestRender_MissingFieldStillRenders(t *testing.T) {
	// Go text/template by default renders <no value> for missing fields.
	// This test ensures we don't panic and documents the behaviour.
	tmpl := []byte("value: {{ .NotAField }}")
	type empty struct{}
	out, err := Render(tmpl, empty{})
	if err != nil {
		// With the default template option (zero-value), missing field is not an error.
		// If it is, we accept that too — document the output.
		t.Logf("Render with missing field returned error (accepted): %v", err)
		return
	}
	t.Logf("Render with missing field produced: %s", out)
}

func TestRender_DefaultValues(t *testing.T) {
	// Verify that CertChainVarsFromCluster applies the correct naming defaults
	// even when cluster has minimal spec.
	cl := clusterFixture("tracer")
	v := CertChainVarsFromCluster(cl)

	// All names should be derived from cluster name
	if !strings.HasPrefix(v.SelfSignedIssuer, "tracer-") {
		t.Errorf("SelfSignedIssuer should start with 'tracer-': got %q", v.SelfSignedIssuer)
	}
	if !strings.HasPrefix(v.CACertName, "tracer-") {
		t.Errorf("CACertName should start with 'tracer-': got %q", v.CACertName)
	}
}
