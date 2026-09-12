package manifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

const (
	FARRegistryHost      = "repo.f5.com"
	ReleaseManifestRepo  = "oci://repo.f5.com/release"
	ReleaseManifestChart = "f5-bigip-k8s-manifest"
	// DefaultManifestVersion is the BNK 2.4 release manifest on repo.f5.com
	// (last checked 2026-09-11; see KnownReleases). Examples and scenarios
	// always target this; the 2.3.x builds stay deployable through
	// bnk.manifestVersion. F5 documents 2.4.0 as "2.4.0-3.3175.0+0.0.380", but
	// the published chart, its manifest file and releases[].version all say
	// "2.4.0", and that is the string FLO matches against.
	DefaultManifestVersion = "2.4.0"
)

// ReleaseManifest represents the parsed contents of the f5-bigip-k8s-manifest chart.
type ReleaseManifest struct {
	HelmRepo    string            `json:"helmRepo"`
	DockerRepo  string            `json:"dockerRepo"`
	Version     string            `json:"version"`
	HelmCharts  map[string]string `json:"helmCharts"`
	DockerImgs  map[string]string `json:"dockerImgs"`
	rawManifest []byte
}

type rawReleaseManifest struct {
	F5HelmRepo   string `yaml:"f5_helm_repo"`
	F5DockerRepo string `yaml:"f5_docker_repo"`
	Releases     []struct {
		Version    string `yaml:"version"`
		HelmCharts []struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"helm_charts"`
		DockerImages []struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"docker_images"`
	} `yaml:"releases"`
}

// ParseReleaseManifest parses raw YAML bytes of bigip-k8s-manifest-<version>.yaml.
func ParseReleaseManifest(body []byte) (*ReleaseManifest, error) {
	var raw rawReleaseManifest
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse release-manifest yaml: %w", err)
	}
	if len(raw.Releases) == 0 {
		return nil, fmt.Errorf("release-manifest has no `releases[]` entries")
	}
	rel := raw.Releases[0]
	if rel.Version == "" {
		return nil, fmt.Errorf("release-manifest releases[0].version is empty")
	}
	m := &ReleaseManifest{
		HelmRepo:    raw.F5HelmRepo,
		DockerRepo:  raw.F5DockerRepo,
		Version:     rel.Version,
		HelmCharts:  make(map[string]string, len(rel.HelmCharts)),
		DockerImgs:  make(map[string]string, len(rel.DockerImages)),
		rawManifest: body,
	}
	for _, c := range rel.HelmCharts {
		if c.Name != "" {
			m.HelmCharts[c.Name] = c.Version
		}
	}
	for _, i := range rel.DockerImages {
		if i.Name != "" {
			m.DockerImgs[i.Name] = i.Version
		}
	}
	return m, nil
}

func (m *ReleaseManifest) Chart(name string) string { return m.HelmCharts[name] }
func (m *ReleaseManifest) Image(name string) string { return m.DockerImgs[name] }
func (m *ReleaseManifest) RawYAML() []byte          { return m.rawManifest }

// SinkSummary prints a concise summary of the manifest contents.
func (m *ReleaseManifest) SinkSummary(w io.Writer) {
	fmt.Fprintf(w, "Release-manifest:  %s\n", m.Version)
	fmt.Fprintf(w, "  helm repo:       %s\n", m.HelmRepo)
	fmt.Fprintf(w, "  docker repo:     %s\n", m.DockerRepo)
	fmt.Fprintf(w, "  helm charts:     %d\n", len(m.HelmCharts))
	fmt.Fprintf(w, "  docker images:   %d\n", len(m.DockerImgs))
	for _, name := range []string{
		"charts/f5-lifecycle-operator",
		"utils/f5-cert-gen",
		"charts/cwc",
		"charts/f5-cert-manager",
	} {
		if v := m.Chart(name); v != "" {
			fmt.Fprintf(w, "    %-35s %s\n", name, v)
		}
	}
}

// PrintFullBOM dumps every chart and image in the manifest in sorted order.
func PrintFullBOM(w io.Writer, m *ReleaseManifest) {
	for _, sec := range []struct {
		title string
		items map[string]string
	}{
		{"helm charts", m.HelmCharts},
		{"docker images", m.DockerImgs},
	} {
		names := make([]string, 0, len(sec.items))
		for n := range sec.items {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "\n  %s (%d):\n", sec.title, len(names))
		for _, n := range names {
			fmt.Fprintf(w, "    %-45s %s\n", n, sec.items[n])
		}
	}
}

// ExtractFARAuth extracts the password and username for repo.f5.com from a FAR archive.
// Supports both .tar.gz / .tgz archives containing cne_pull_64.json or raw base64 JSON.
func ExtractFARAuth(farPath string) (username, password string, err error) {
	data, err := os.ReadFile(farPath) // #nosec G304 -- operator-supplied path via cluster.yaml or CLI
	if err != nil {
		return "", "", fmt.Errorf("read FAR file %s: %w", farPath, err)
	}

	// Try reading as gzip tar archive
	if isGzipTar(data) {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			defer gz.Close()
			tr := tar.NewReader(gz)
			for {
				hdr, err := tr.Next()
				if err != nil {
					break
				}
				if hdr.Typeflag == tar.TypeReg {
					content, err := io.ReadAll(tr)
					if err == nil && len(content) > 0 {
						data = content
						break
					}
				}
			}
		}
	}

	contentStr := strings.TrimSpace(string(data))
	// If it's base64-encoded GCP service account JSON
	decoded, err := base64.StdEncoding.DecodeString(contentStr)
	if err == nil {
		var sa map[string]any
		if json.Unmarshal(decoded, &sa) == nil && sa["type"] == "service_account" {
			return "_json_key", string(decoded), nil
		}
	}

	// If it's raw service account JSON
	var sa map[string]any
	if json.Unmarshal(data, &sa) == nil && sa["type"] == "service_account" {
		return "_json_key", string(data), nil
	}

	// Fallback to _json_key_base64
	return "_json_key_base64", contentStr, nil
}

func isGzipTar(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// PullReleaseManifest logs into repo.f5.com and pulls the release manifest chart into cacheDir.
func PullReleaseManifest(ctx context.Context, username, password, manifestVersion, cacheDir string) (*ReleaseManifest, error) {
	if manifestVersion == "" {
		manifestVersion = DefaultManifestVersion
	}
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir cache %s: %w", cacheDir, err)
	}
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, err
	}

	// Pull with the Helm SDK's OCI client — the same library Phase 14 uses for
	// the FLO chart — so the probe needs no host helm binary (the binary
	// promises "no host kubectl, no host helm"). Credentials are passed to the
	// client in memory rather than via Login, so nothing is written under
	// ~/.config/helm (that directory is read-only in some containers).
	regClient, err := registry.NewClient(registry.ClientOptBasicAuth(username, password))
	if err != nil {
		return nil, fmt.Errorf("create helm registry client: %w", err)
	}

	extractedDir := filepath.Join(absCache, ReleaseManifestChart+"-"+manifestVersion)
	_ = os.RemoveAll(extractedDir)
	_ = os.RemoveAll(filepath.Join(absCache, ReleaseManifestChart)) // Untar target name

	cfg := &action.Configuration{RegistryClient: regClient}
	pull := action.NewPullWithOpts(action.WithConfig(cfg))
	pull.Settings = cli.New()
	pull.Version = manifestVersion
	pull.DestDir = absCache
	pull.Untar = true
	pull.UntarDir = absCache
	if _, err := pull.Run(ReleaseManifestRepo + "/" + ReleaseManifestChart); err != nil {
		return nil, fmt.Errorf("pull release-manifest %s from %s: %w", manifestVersion, ReleaseManifestRepo, err)
	}
	// Helm untars into <UntarDir>/<chart name>; keep the version in the
	// directory name so probes of two builds can share one cache dir.
	if err := os.Rename(filepath.Join(absCache, ReleaseManifestChart), extractedDir); err != nil {
		return nil, fmt.Errorf("rename extracted chart: %w", err)
	}

	manifestPath, err := findManifestFile(extractedDir, manifestVersion)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(manifestPath) // #nosec G304 -- reading extracted manifest yaml
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	m, err := ParseReleaseManifest(body)
	if err != nil {
		return nil, err
	}

	_ = os.WriteFile(filepath.Join(absCache, "manifest.yaml"), body, 0o600) // #nosec G306,G703 -- cache write
	return m, nil
}

// findManifestFile returns the release manifest inside an extracted chart.
// Up to 2.3.x the file was named after the OCI tag
// (bigip-k8s-manifest-2.3.3-3.2598.3-0.0.509.yaml). The 2.4.0 chart is
// published under two tags (2.4.0 and 2.4.0-3.3175.0-0.0.380) but ships
// bigip-k8s-manifest-2.4.0.yaml, so prefer the exact name and otherwise take
// the single bigip-k8s-manifest-*.yaml the chart contains.
func findManifestFile(dir, manifestVersion string) (string, error) {
	exact := filepath.Join(dir, fmt.Sprintf("bigip-k8s-manifest-%s.yaml", manifestVersion))
	if _, err := os.Stat(exact); err == nil {
		return exact, nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, "bigip-k8s-manifest-*.yaml"))
	if err != nil {
		return "", fmt.Errorf("list manifests in %s: %w", dir, err)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no bigip-k8s-manifest-*.yaml in %s (requested %s)", dir, manifestVersion)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("%d release manifests in %s, none named for %s: %v", len(matches), dir, manifestVersion, matches)
	}
}

// Release records what awsbnkctl needs to know about one BNK release manifest
// without pulling it: the F5 Lifecycle Operator chart that ships with it. FLO
// is installed by Phase 14 BEFORE the release manifest is available in-cluster,
// so the pairing has to be known up front. Values are read from
// oci://repo.f5.com/release/f5-bigip-k8s-manifest:<version>
// (helm_charts → charts/f5-lifecycle-operator).
type Release struct {
	// Version is the release-manifest tag and the CNEInstance manifestVersion,
	// e.g. "2.4.0" or "2.3.3-3.2598.3-0.0.509".
	Version string
	// FLOChart is the f5-lifecycle-operator chart version paired with it.
	FLOChart string
}

// KnownReleases lists every BNK release manifest awsbnkctl has been exercised
// against, as published on repo.f5.com,
// oldest first. Extend it when F5 publishes a new build (the tags list is
// `awsbnkctl manifest probe` territory); DefaultManifestVersion must be the
// last entry.
var KnownReleases = []Release{
	{Version: "2.3.0-3.2598.3-0.0.170", FLOChart: "v2.21.13-0.0.28"},
	{Version: "2.3.1-3.2598.3-0.0.304", FLOChart: "v2.21.13-0.0.53"},
	{Version: "2.3.2-3.2598.3-0.0.392", FLOChart: "v2.21.13-0.0.58"},
	{Version: "2.3.3-3.2598.3-0.0.509", FLOChart: "v2.21.13-0.0.64"},
	{Version: "2.4.0", FLOChart: "v2.30.0-0.5.2"},
}

// FLOChartFor returns the FLO chart version paired with manifestVersion.
// ok is false when the manifest is not in KnownReleases; callers then fall
// back to the default release's FLO chart and should warn, because a newer
// manifest may need a newer operator.
func FLOChartFor(manifestVersion string) (chart string, ok bool) {
	for _, r := range KnownReleases {
		if r.Version == manifestVersion {
			return r.FLOChart, true
		}
	}
	return "", false
}

// DefaultFLOChart is the FLO chart paired with DefaultManifestVersion.
func DefaultFLOChart() string {
	chart, _ := FLOChartFor(DefaultManifestVersion)
	return chart
}

// IsKnownRelease reports whether manifestVersion is a build awsbnkctl has been
// exercised against (KnownReleases).
func IsKnownRelease(manifestVersion string) bool {
	_, ok := FLOChartFor(manifestVersion)
	return ok
}
