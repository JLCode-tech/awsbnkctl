package manifest

import (
	"bytes"
	"testing"
)

const sampleManifestYAML = `
f5_helm_repo: oci://repo.f5.com/charts
f5_docker_repo: repo.f5.com/images
releases:
  - version: 2.3.0-3.2598.3-0.0.170
    helm_charts:
      - name: charts/f5-lifecycle-operator
        version: v2.21.13-0.0.28
      - name: utils/f5-cert-gen
        version: 0.0.1
      - name: charts/cwc
        version: v1.0.0
    docker_images:
      - name: f5-tmm
        version: 2.3.0
      - name: f5-cwc
        version: 1.0.0
`

func TestParseReleaseManifest(t *testing.T) {
	m, err := ParseReleaseManifest([]byte(sampleManifestYAML))
	if err != nil {
		t.Fatalf("unexpected error parsing manifest: %v", err)
	}

	if m.Version != "2.3.0-3.2598.3-0.0.170" {
		t.Errorf("got version %q, want 2.3.0-3.2598.3-0.0.170", m.Version)
	}
	if m.Chart("charts/f5-lifecycle-operator") != "v2.21.13-0.0.28" {
		t.Errorf("got FLO chart version %q, want v2.21.13-0.0.28", m.Chart("charts/f5-lifecycle-operator"))
	}
	if m.Image("f5-tmm") != "2.3.0" {
		t.Errorf("got TMM image %q, want 2.3.0", m.Image("f5-tmm"))
	}

	var buf bytes.Buffer
	m.SinkSummary(&buf)
	if buf.Len() == 0 {
		t.Errorf("expected non-empty summary output")
	}

	buf.Reset()
	PrintFullBOM(&buf, m)
	if buf.Len() == 0 {
		t.Errorf("expected non-empty full BOM output")
	}
}
