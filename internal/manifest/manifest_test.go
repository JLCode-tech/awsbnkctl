package manifest

import (
	"bytes"
	"os"
	"path/filepath"
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

// TestKnownReleases_DefaultIsNewestAndPaired pins the release table: the
// default manifest is the last (newest) entry and every entry names its FLO
// chart, so Phase 14 never installs an operator older than the manifest.
func TestKnownReleases_DefaultIsNewestAndPaired(t *testing.T) {
	if len(KnownReleases) == 0 {
		t.Fatal("KnownReleases is empty")
	}
	last := KnownReleases[len(KnownReleases)-1]
	if last.Version != DefaultManifestVersion {
		t.Errorf("DefaultManifestVersion %q is not the newest KnownReleases entry %q", DefaultManifestVersion, last.Version)
	}
	for _, r := range KnownReleases {
		if r.FLOChart == "" {
			t.Errorf("release %s has no FLO chart", r.Version)
		}
	}
	if got, ok := FLOChartFor("2.3.0-3.2598.3-0.0.170"); !ok || got != "v2.21.13-0.0.28" {
		t.Errorf("FLOChartFor(2.3.0) = %q,%v want v2.21.13-0.0.28,true", got, ok)
	}
	if _, ok := FLOChartFor("2.9.9-0.0.0-0.0.1"); ok {
		t.Error("unknown manifest reported as known")
	}
	if DefaultFLOChart() != last.FLOChart {
		t.Errorf("DefaultFLOChart() = %q, want %q", DefaultFLOChart(), last.FLOChart)
	}
}

// TestKnownReleases_24Default pins the BNK 2.4 pairing: the default is the
// "2.4.0" manifest (the string FLO matches, not the docs' "+0.0.380" form)
// and it installs the FLO chart F5 shipped with it.
func TestKnownReleases_24Default(t *testing.T) {
	if DefaultManifestVersion != "2.4.0" {
		t.Errorf("DefaultManifestVersion = %q, want 2.4.0", DefaultManifestVersion)
	}
	if got, ok := FLOChartFor("2.4.0"); !ok || got != "v2.30.0-0.5.2" {
		t.Errorf("FLOChartFor(2.4.0) = %q,%v want v2.30.0-0.5.2,true", got, ok)
	}
	if got, ok := FLOChartFor("2.3.3-3.2598.3-0.0.509"); !ok || got != "v2.21.13-0.0.64" {
		t.Errorf("FLOChartFor(2.3.3) = %q,%v want v2.21.13-0.0.64,true (2.3.x must stay deployable)", got, ok)
	}
	if _, ok := FLOChartFor("2.4.0-3.3175.0+0.0.380"); ok {
		t.Error("the docs' +0.0.380 spelling must not be treated as a known manifest; FLO cannot pull it")
	}
}

// TestFindManifestFile covers the 2.4.0 chart layout, where the OCI tag
// (2.4.0-3.3175.0-0.0.380) no longer matches the manifest file name
// (bigip-k8s-manifest-2.4.0.yaml).
func TestFindManifestFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("releases: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := findManifestFile(dir, "2.4.0"); err == nil {
		t.Error("empty dir: want error, got nil")
	}

	write("bigip-k8s-manifest-2.4.0.yaml")
	got, err := findManifestFile(dir, "2.4.0")
	if err != nil || filepath.Base(got) != "bigip-k8s-manifest-2.4.0.yaml" {
		t.Errorf("exact name: got %q, %v", got, err)
	}
	got, err = findManifestFile(dir, "2.4.0-3.3175.0-0.0.380")
	if err != nil || filepath.Base(got) != "bigip-k8s-manifest-2.4.0.yaml" {
		t.Errorf("tag differs from file name: got %q, %v; want the single manifest in the chart", got, err)
	}

	write("bigip-k8s-manifest-2.3.3-3.2598.3-0.0.509.yaml")
	got, err = findManifestFile(dir, "2.3.3-3.2598.3-0.0.509")
	if err != nil || filepath.Base(got) != "bigip-k8s-manifest-2.3.3-3.2598.3-0.0.509.yaml" {
		t.Errorf("exact name with two files: got %q, %v", got, err)
	}
	if _, err := findManifestFile(dir, "9.9.9"); err == nil {
		t.Error("two files, none matching: want error, got nil")
	}
}
