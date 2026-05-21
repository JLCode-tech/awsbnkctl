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

const clusterWithEKSYAML = `
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
cluster:
  kubernetesVersion: "1.30"
  nodeGroups:
    - name: default
      instanceType: t3.medium
      desiredSize: 1
      minSize: 1
      maxSize: 2
      diskSize: 50
`

func TestLoad_ClusterSpecParsed(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", clusterWithEKSYAML)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ClusterSpec == nil {
		t.Fatal("ClusterSpec: nil, want populated struct")
	}
	if c.ClusterSpec.KubernetesVersion != "1.30" {
		t.Errorf("KubernetesVersion: got %q, want 1.30", c.ClusterSpec.KubernetesVersion)
	}
	if len(c.ClusterSpec.NodeGroups) != 1 {
		t.Fatalf("NodeGroups len: got %d, want 1", len(c.ClusterSpec.NodeGroups))
	}
	ng := c.ClusterSpec.NodeGroups[0]
	if ng.Name != "default" {
		t.Errorf("NodeGroup.Name: got %q, want default", ng.Name)
	}
	if ng.InstanceType != "t3.medium" {
		t.Errorf("NodeGroup.InstanceType: got %q, want t3.medium", ng.InstanceType)
	}
	if ng.DesiredSize != 1 {
		t.Errorf("NodeGroup.DesiredSize: got %d, want 1", ng.DesiredSize)
	}
	if ng.DiskSize != 50 {
		t.Errorf("NodeGroup.DiskSize: got %d, want 50", ng.DiskSize)
	}
}

func TestLoad_ClusterSpecDefaults(t *testing.T) {
	yaml := minimalYAML + `
cluster:
  nodeGroups:
    - name: ng
`
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", yaml)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ClusterSpec.KubernetesVersion != "1.30" {
		t.Errorf("default KubernetesVersion: got %q, want 1.30", c.ClusterSpec.KubernetesVersion)
	}
	ng := c.ClusterSpec.NodeGroups[0]
	if ng.InstanceType != "t3.medium" {
		t.Errorf("default InstanceType: got %q, want t3.medium", ng.InstanceType)
	}
	if ng.DesiredSize != 1 {
		t.Errorf("default DesiredSize: got %d, want 1", ng.DesiredSize)
	}
	if ng.MinSize != 1 {
		t.Errorf("default MinSize: got %d, want 1", ng.MinSize)
	}
	if ng.MaxSize != 2 {
		t.Errorf("default MaxSize: got %d, want 2", ng.MaxSize)
	}
	if ng.DiskSize != 50 {
		t.Errorf("default DiskSize: got %d, want 50", ng.DiskSize)
	}
}

func TestLoad_ClusterSpecRejectsEmptyNodeGroups(t *testing.T) {
	yaml := minimalYAML + `
cluster:
  kubernetesVersion: "1.30"
  nodeGroups: []
`
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", yaml)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty nodeGroups with cluster block, got nil")
	}
}

func TestLoad_ClusterSpecRejectsInvalidNodeGroupName(t *testing.T) {
	cases := []struct {
		name string
		desc string
	}{
		{"UPPER", "uppercase not allowed"},
		{"-starts-dash", "starts with dash"},
		{"ends-dash-", "ends with dash"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			yaml := minimalYAML + `
cluster:
  nodeGroups:
    - name: ` + tc.name + `
`
			dir := t.TempDir()
			p := writeFile(t, dir, "cluster.yaml", yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("expected error for node group name %q (%s), got nil", tc.name, tc.desc)
			}
		})
	}
}

func TestLoad_ClusterSpecOmittedWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", minimalYAML)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ClusterSpec != nil {
		t.Errorf("ClusterSpec: got %+v, want nil when cluster block absent", c.ClusterSpec)
	}
}

// ─── BnkSpec tests (slice 5) ──────────────────────────────────────────────────

func TestLoad_BnkBlockOmittedWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "cluster.yaml", minimalYAML)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Bnk != nil {
		t.Errorf("Bnk: got %+v, want nil when bnk block absent", c.Bnk)
	}
}

func TestLoad_BnkBlockParsed(t *testing.T) {
	dir := t.TempDir()
	// Write placeholder files so path-existence validation passes.
	farPath := writeFile(t, dir, "far.json", `{"auths":{}}`)
	jwtPath := writeFile(t, dir, "license.jwt", "jwt-token")

	yaml := minimalYAML + `
bnk:
  farArchive: ` + farPath + `
  jwt: ` + jwtPath + `
  certManagerVersion: "1.16.1"
`
	p := writeFile(t, dir, "cluster.yaml", yaml)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load with bnk block: %v", err)
	}
	if c.Bnk == nil {
		t.Fatal("Bnk: nil, want populated struct")
	}
	if c.Bnk.FARArchive != farPath {
		t.Errorf("Bnk.FARArchive: got %q, want %q", c.Bnk.FARArchive, farPath)
	}
	if c.Bnk.JWT != jwtPath {
		t.Errorf("Bnk.JWT: got %q, want %q", c.Bnk.JWT, jwtPath)
	}
	if c.Bnk.CertManagerVersion != "1.16.1" {
		t.Errorf("Bnk.CertManagerVersion: got %q, want 1.16.1", c.Bnk.CertManagerVersion)
	}
}

func TestLoad_BnkBlockDefaultCertManagerVersion(t *testing.T) {
	dir := t.TempDir()
	farPath := writeFile(t, dir, "far.json", `{"auths":{}}`)
	jwtPath := writeFile(t, dir, "license.jwt", "jwt-token")

	// certManagerVersion omitted — should default to 1.16.1.
	yaml := minimalYAML + `
bnk:
  farArchive: ` + farPath + `
  jwt: ` + jwtPath + `
`
	p := writeFile(t, dir, "cluster.yaml", yaml)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load with bnk (no version): %v", err)
	}
	if c.Bnk.CertManagerVersion != EmbeddedCertManagerVersion {
		t.Errorf("default CertManagerVersion: got %q, want %q", c.Bnk.CertManagerVersion, EmbeddedCertManagerVersion)
	}
}

func TestLoad_BnkBlockRejectsMismatchedVersion(t *testing.T) {
	dir := t.TempDir()
	farPath := writeFile(t, dir, "far.json", `{"auths":{}}`)
	jwtPath := writeFile(t, dir, "license.jwt", "jwt-token")

	yaml := minimalYAML + `
bnk:
  farArchive: ` + farPath + `
  jwt: ` + jwtPath + `
  certManagerVersion: "1.15.0"
`
	p := writeFile(t, dir, "cluster.yaml", yaml)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for mismatched certManagerVersion, got nil")
	}
	if !containsStr(err.Error(), "certManagerVersion") {
		t.Errorf("error should mention 'certManagerVersion': %v", err)
	}
}

func TestLoad_BnkBlockRejectsMissingFARArchive(t *testing.T) {
	dir := t.TempDir()
	jwtPath := writeFile(t, dir, "license.jwt", "jwt-token")

	yaml := minimalYAML + `
bnk:
  farArchive: ""
  jwt: ` + jwtPath + `
`
	p := writeFile(t, dir, "cluster.yaml", yaml)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty farArchive, got nil")
	}
}

func TestLoad_BnkBlockRejectsMissingJWT(t *testing.T) {
	dir := t.TempDir()
	farPath := writeFile(t, dir, "far.json", `{"auths":{}}`)

	yaml := minimalYAML + `
bnk:
  farArchive: ` + farPath + `
  jwt: ""
`
	p := writeFile(t, dir, "cluster.yaml", yaml)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty jwt, got nil")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
