package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validClusterYAML = `apiVersion: awsbnkctl/v1
kind: Cluster
metadata:
  name: validate-test
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
      - cidr: 10.0.2.0/24
        az: ap-southeast-2b
    private:
      - cidr: 10.0.11.0/24
        az: ap-southeast-2a
      - cidr: 10.0.12.0/24
        az: ap-southeast-2b
  natGateways: 1
`

func TestValidateCmd_Happy(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(p, []byte(validClusterYAML), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// runValidate writes to stderr; capture it.
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	err := runValidate(nil, []string{p})
	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	if err != nil {
		t.Fatalf("runValidate: %v\nstderr: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "parses cleanly") {
		t.Errorf("expected 'parses cleanly' in stderr; got: %s", out.String())
	}
	if !strings.Contains(out.String(), "validate-test") {
		t.Errorf("expected cluster name in stderr; got: %s", out.String())
	}
}

func TestValidateCmd_RejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cluster.yaml")
	bad := validClusterYAML + "\nbogusField: 1\n"
	if err := os.WriteFile(p, []byte(bad), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	err := runValidate(nil, []string{p})
	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("err: %v", err)
	}
}

func TestValidateCmd_MissingFile(t *testing.T) {
	err := runValidate(nil, []string{"/nonexistent/path/cluster.yaml"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
