// Package intent holds the cluster.yaml schema (v1) and loader.
//
// The canonical format is described in docs/POST_TERRAFORM_DIRECTION.md §5.
// Every field maps directly to an AWS resource or provisioning decision —
// there is no intermediate Terraform variable layer.
package intent

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// clusterNameRE enforces EKS cluster name rules: lowercase alphanumeric +
// hyphens, 2–40 chars, must start with a letter and end with a letter or digit.
var clusterNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`)

// Cluster is the Go representation of cluster.yaml (apiVersion: awsbnkctl/v1,
// kind: Cluster). Unknown fields are rejected at load time.
type Cluster struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Network    Network  `yaml:"network"`
	// Pattern selects internal/k8s/manifests/<pattern>/ (not used in slice 1).
	// Loaded here for forward-compat so later slices don't change the struct.
	Pattern string `yaml:"pattern,omitempty"`
	// Forge declares the bnk-forge integration shape (slice 4+). Optional;
	// when omitted the new Go-SDK phased path skips the forge handoff
	// silently. Shape inspired by mwiget/kindbnkctl examples/two-node.yaml.
	Forge *ForgeSpec `yaml:"forge,omitempty"`
	// Tags are merged into every AWS resource created by awsbnkctl alongside
	// the required awsbnkctl:* keys.
	Tags map[string]string `yaml:"tags,omitempty"`
}

// Metadata carries the cluster identity fields.
type Metadata struct {
	// Name is load-bearing: it becomes the awsbnkctl:cluster tag value and the
	// directory name under .awsbnkctl/. Must match clusterNameRE.
	Name   string            `yaml:"name"`
	Region string            `yaml:"region"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// Network describes the VPC topology the provisioner creates.
type Network struct {
	VPCCidr string   `yaml:"vpcCidr"`
	AZs     []string `yaml:"azs"`
	Subnets Subnets  `yaml:"subnets"`
	// NatGateways is 1 (cost-optimised) or the number of AZs (HA).
	NatGateways int `yaml:"natGateways"`
}

// Subnets groups the public and private subnet definitions.
type Subnets struct {
	Public  []SubnetSpec `yaml:"public"`
	Private []SubnetSpec `yaml:"private"`
}

// SubnetSpec is one CIDR + AZ pair.
type SubnetSpec struct {
	CIDR string `yaml:"cidr"`
	AZ   string `yaml:"az"`
}

// ForgeSpec captures the operator-declared forge integration for slice 4+.
// When Enabled is false (or the whole block is omitted), the phased path
// skips the forge handoff entirely. When Enabled is true, slice 4 registers
// the cluster with a running bnk-forge instance via MCP (preferred) or
// REST (fallback). awsbnkctl NEVER auto-installs forge — if Enabled is
// true and the URL is unreachable, the soft-fail-with-retry path writes
// a `pending` link file and exits 0.
//
// See docs/FORGE_MCP_INTEGRATION.md for the handoff details. Shape borrowed
// from mwiget/kindbnkctl's bnk_forge: block (camelCase here to match the
// rest of our schema).
type ForgeSpec struct {
	// Enabled is the master switch. Default false (omitted block = disabled).
	Enabled bool `yaml:"enabled"`
	// URL is the forge REST base. Default http://localhost:8000.
	URL string `yaml:"url,omitempty"`
	// MCPURL is the forge MCP endpoint. Default http://localhost:8081/mcp/.
	// Slice 4 prefers MCP and falls back to REST at URL on capability gaps.
	MCPURL string `yaml:"mcpUrl,omitempty"`
}

// StateDir returns the path to the IDs-cache directory for this cluster
// relative to the caller's working directory. Callers that need an absolute
// path should use filepath.Abs on the result.
func (c *Cluster) StateDir() string {
	return ".awsbnkctl/" + c.Metadata.Name
}

// Load reads and validates a cluster.yaml file at path.
//
// Validation rules:
//   - Unknown fields are errors (strict decoding).
//   - metadata.name must match clusterNameRE.
//   - network.azs must be non-empty.
//   - network.subnets.public and network.subnets.private must be non-empty.
func Load(path string) (*Cluster, error) {
	// #nosec G304 -- path is operator-supplied via --config flag; awsbnkctl is
	// a CLI tool so reading a user-named config file is intentional behaviour,
	// not a directory-traversal risk.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cluster.yaml %s: %w", path, err)
	}

	var c Cluster
	if err := decodeStrict(data, &c); err != nil {
		return nil, fmt.Errorf("parsing cluster.yaml %s: %w", path, err)
	}

	if err := validate(&c); err != nil {
		return nil, fmt.Errorf("validating cluster.yaml %s: %w", path, err)
	}
	return &c, nil
}

// decodeStrict decodes YAML rejecting unknown fields.
func decodeStrict(data []byte, out interface{}) error {
	dec := yaml.NewDecoder(bytesReader(data))
	dec.KnownFields(true)
	return dec.Decode(out)
}

// bytesReader wraps a byte slice in an io.Reader for yaml.NewDecoder.
type byteReader struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// validate checks semantic constraints on the loaded cluster.
func validate(c *Cluster) error {
	if !clusterNameRE.MatchString(c.Metadata.Name) {
		return fmt.Errorf("metadata.name %q does not match required pattern %s", c.Metadata.Name, clusterNameRE.String())
	}
	if c.Metadata.Region == "" {
		return fmt.Errorf("metadata.region is required")
	}
	if len(c.Network.AZs) == 0 {
		return fmt.Errorf("network.azs must contain at least one availability zone")
	}
	if len(c.Network.Subnets.Public) == 0 {
		return fmt.Errorf("network.subnets.public must contain at least one subnet")
	}
	if len(c.Network.Subnets.Private) == 0 {
		return fmt.Errorf("network.subnets.private must contain at least one subnet")
	}
	if c.Network.VPCCidr == "" {
		return fmt.Errorf("network.vpcCidr is required")
	}
	return nil
}
