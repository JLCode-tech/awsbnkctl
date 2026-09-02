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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FARRegistryHost       = "repo.f5.com"
	ReleaseManifestRepo   = "oci://repo.f5.com/release"
	ReleaseManifestChart  = "f5-bigip-k8s-manifest"
	DefaultManifestVersion = "2.3.0-3.2598.3-0.0.170"
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
	data, err := os.ReadFile(farPath)
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
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache %s: %w", cacheDir, err)
	}
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, err
	}

	// Login to Helm registry
	loginCmd := exec.CommandContext(ctx, "helm", "registry", "login",
		FARRegistryHost, "--username", username, "--password-stdin")
	loginCmd.Stdin = strings.NewReader(password + "\n")
	var loginErr bytes.Buffer
	loginCmd.Stderr = &loginErr
	loginCmd.Stdout = io.Discard
	if err := loginCmd.Run(); err != nil {
		return nil, fmt.Errorf("helm registry login %s: %w\n%s",
			FARRegistryHost, err, strings.TrimSpace(loginErr.String()))
	}

	tgzPath := filepath.Join(absCache, fmt.Sprintf("f5-bigip-k8s-manifest-%s.tgz", manifestVersion))
	extractedDir := filepath.Join(absCache, fmt.Sprintf("f5-bigip-k8s-manifest-%s", manifestVersion))
	_ = os.Remove(tgzPath)
	_ = os.RemoveAll(extractedDir)

	pullCmd := exec.CommandContext(ctx, "helm", "pull",
		ReleaseManifestRepo+"/"+ReleaseManifestChart,
		"--version", manifestVersion,
		"-d", absCache)
	var pullErr bytes.Buffer
	pullCmd.Stderr = &pullErr
	pullCmd.Stdout = io.Discard
	if err := pullCmd.Run(); err != nil {
		return nil, fmt.Errorf("helm pull release-manifest %s: %w\n%s",
			manifestVersion, err, strings.TrimSpace(pullErr.String()))
	}

	tarCmd := exec.CommandContext(ctx, "tar", "-xzf", tgzPath, "-C", absCache)
	if err := tarCmd.Run(); err != nil {
		return nil, fmt.Errorf("tar -xzf %s: %w", tgzPath, err)
	}

	manifestPath := filepath.Join(extractedDir, fmt.Sprintf("bigip-k8s-manifest-%s.yaml", manifestVersion))
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	m, err := ParseReleaseManifest(body)
	if err != nil {
		return nil, err
	}

	_ = os.WriteFile(filepath.Join(absCache, "manifest.yaml"), body, 0o644)
	return m, nil
}
