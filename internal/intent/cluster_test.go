package intent

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return p
}

const minimalYAML = `
apiVersion: awsbnkctl/v1
kind: Cluster
metadata:
  name: my-cluster
  region: ap-southeast-2
network:
  vpcCidr: 10.0.0.0/16
  azs:
    - ap-southeast-2a
    - ap-southeast-2b
  subnets:
    public:
      - cidr: 10.0.1.0/24
        az: ap-southeast-2a
    private:
      - cidr: 10.0.11.0/24
        az: ap-southeast-2a
  natGateways: 1
`

func TestLoad_HappyPath(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", minimalYAML)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Metadata.Name != "my-cluster" {
		t.Errorf("name: got %q, want %q", c.Metadata.Name, "my-cluster")
	}
	if c.Metadata.Region != "ap-southeast-2" {
		t.Errorf("region: got %q", c.Metadata.Region)
	}
	if len(c.Network.AZs) != 2 {
		t.Errorf("azs len: got %d, want 2", len(c.Network.AZs))
	}
	if c.Network.VPCCidr != "10.0.0.0/16" {
		t.Errorf("vpcCidr: got %q", c.Network.VPCCidr)
	}
}

func TestLoad_OmitsForgeWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", minimalYAML)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Forge != nil {
		t.Errorf("Forge: got %+v, want nil for cluster.yaml without forge block", c.Forge)
	}
}

func TestLoad_ForgeBlockEnabled(t *testing.T) {
	dir := t.TempDir()
	withForge := minimalYAML + `
forge:
  enabled: true
  url: http://localhost:8000
  mcpUrl: http://localhost:8081/mcp/
`
	p := writeFile(t, dir, "cluster.yaml", withForge)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load with forge: %v", err)
	}
	if c.Forge == nil {
		t.Fatal("Forge: nil, want populated struct")
	}
	if !c.Forge.Enabled {
		t.Errorf("Forge.Enabled: got false, want true")
	}
	if c.Forge.URL != "http://localhost:8000" {
		t.Errorf("Forge.URL: got %q", c.Forge.URL)
	}
	if c.Forge.MCPURL != "http://localhost:8081/mcp/" {
		t.Errorf("Forge.MCPURL: got %q", c.Forge.MCPURL)
	}
}

func TestLoad_RejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	bad := minimalYAML + "\nunknownField: boom\n"
	p := writeFile(t, dir, "cluster.yaml", bad)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestLoad_RejectsInvalidName(t *testing.T) {
	cases := []struct {
		name string
		desc string
	}{
		{"UPPER", "uppercase not allowed"},
		{"a", "too short (single char)"},
		{"-starts-with-dash", "starts with dash"},
		{"ends-with-dash-", "ends with dash"},
		{"this-name-is-way-too-long-to-be-valid-for-eks-cluster-rules-x", "too long"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			yaml := `
apiVersion: awsbnkctl/v1
kind: Cluster
metadata:
  name: ` + tc.name + `
  region: ap-southeast-2
network:
  vpcCidr: 10.0.0.0/16
  azs:
    - ap-southeast-2a
  subnets:
    public:
      - cidr: 10.0.1.0/24
        az: ap-southeast-2a
    private:
      - cidr: 10.0.11.0/24
        az: ap-southeast-2a
  natGateways: 1
`
			dir := t.TempDir()
			p := writeFile(t, dir, "cluster.yaml", yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("expected error for name %q (%s), got nil", tc.name, tc.desc)
			}
		})
	}
}

func TestLoad_ValidatesAZsNonEmpty(t *testing.T) {
	yaml := `
apiVersion: awsbnkctl/v1
kind: Cluster
metadata:
  name: my-cluster
  region: ap-southeast-2
network:
  vpcCidr: 10.0.0.0/16
  azs: []
  subnets:
    public:
      - cidr: 10.0.1.0/24
        az: ap-southeast-2a
    private:
      - cidr: 10.0.11.0/24
        az: ap-southeast-2a
  natGateways: 1
`
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", yaml)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty azs, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/cluster.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestStateDir(t *testing.T) {
	c := &Cluster{Metadata: Metadata{Name: "tracer"}}
	want := ".awsbnkctl/tracer"
	if got := c.StateDir(); got != want {
		t.Errorf("StateDir: got %q, want %q", got, want)
	}
}
