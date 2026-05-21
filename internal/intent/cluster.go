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

// BnkSpec holds the operator-supplied BNK supply-chain credentials required by
// Phase 12 (k8s install foundation). The bnk: block is optional at schema load
// time (slices 1–4 don't need it); Phase 12 returns a clear error if absent.
//
// certManagerVersion is validated at phase entry to match the pinned embedded
// YAML version (1.16.1). Mismatch → clear error.
type BnkSpec struct {
	// FARArchive is the path to F5's FAR pull credentials JSON file.
	// Type: kubernetes.io/dockerconfigjson. File must be readable + non-empty.
	FARArchive string `yaml:"farArchive"`
	// JWT is the path to F5's subscription JWT file.
	// Type: Opaque, key: license.jwt. File must be readable + non-empty.
	JWT string `yaml:"jwt"`
	// CertManagerVersion pins the embedded cert-manager YAML version.
	// Default "1.16.1". Must match the embedded YAML or phase 12 errors.
	CertManagerVersion string `yaml:"certManagerVersion,omitempty"`
}

// nodeGroupNameRE enforces Kubernetes label/name rules for node group names:
// lowercase alphanumeric + hyphens, must start/end with alphanumeric.
var nodeGroupNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

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
	// ClusterSpec holds the EKS control plane + node group configuration
	// (slice 3+). Optional for slices 1+2 (network + IAM only). Required
	// when running phases 08+.
	ClusterSpec *ClusterSpec `yaml:"cluster,omitempty"`
	// Pattern selects internal/k8s/manifests/<pattern>/ (not used in slice 1).
	// Loaded here for forward-compat so later slices don't change the struct.
	Pattern string `yaml:"pattern,omitempty"`
	// Forge declares the bnk-forge integration shape (slice 4+). Optional;
	// when omitted the new Go-SDK phased path skips the forge handoff
	// silently. Shape inspired by mwiget/kindbnkctl examples/two-node.yaml.
	Forge *ForgeSpec `yaml:"forge,omitempty"`
	// Bnk declares the BNK supply-chain credentials (slice 5+). Optional at
	// schema load time; required when running phase 12+. When present, FARArchive
	// and JWT paths are shape-validated at Load time (files exist + readable);
	// file-content validation (non-empty) is deferred to phase 12 entry.
	Bnk *BnkSpec `yaml:"bnk,omitempty"`
	// Tags are merged into every AWS resource created by awsbnkctl alongside
	// the required awsbnkctl:* keys.
	Tags map[string]string `yaml:"tags,omitempty"`
}

// ClusterSpec holds the EKS control plane and node group configuration.
// Corresponds to the `cluster:` block in cluster.yaml.
type ClusterSpec struct {
	// KubernetesVersion is the EKS Kubernetes version to deploy. Default "1.30".
	KubernetesVersion string `yaml:"kubernetesVersion,omitempty"`
	// NodeGroups defines one or more managed node groups. At least one is required
	// when the cluster block is present.
	NodeGroups []NodeGroupSpec `yaml:"nodeGroups,omitempty"`
}

// NodeGroupSpec configures one managed node group.
type NodeGroupSpec struct {
	// Name is required; used to form the node group name <cluster>-ng-<name>.
	// Must be lowercase alphanumeric + hyphens.
	Name string `yaml:"name"`
	// InstanceType for the Auto Scaling group. Default "t3.medium".
	InstanceType string `yaml:"instanceType,omitempty"`
	// DesiredSize is the initial node count. Default 1.
	DesiredSize int `yaml:"desiredSize,omitempty"`
	// MinSize for the Auto Scaling group. Default 1.
	MinSize int `yaml:"minSize,omitempty"`
	// MaxSize for the Auto Scaling group. Default 2.
	MaxSize int `yaml:"maxSize,omitempty"`
	// DiskSize in GiB for each node's root volume. Default 50.
	DiskSize int `yaml:"diskSize,omitempty"`
	// Labels are additional Kubernetes node labels.
	Labels map[string]string `yaml:"labels,omitempty"`
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

	applyDefaults(&c)
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

// EmbeddedCertManagerVersion is the cert-manager version baked into the binary.
// Phase 12 validates that bnk.certManagerVersion (if set) matches this exactly.
const EmbeddedCertManagerVersion = "1.16.1"

// applyDefaults fills in zero-value fields with their documented defaults.
// Called before validate so validation sees the post-default values.
func applyDefaults(c *Cluster) {
	if c.ClusterSpec != nil {
		if c.ClusterSpec.KubernetesVersion == "" {
			c.ClusterSpec.KubernetesVersion = "1.30"
		}
		for i := range c.ClusterSpec.NodeGroups {
			ng := &c.ClusterSpec.NodeGroups[i]
			if ng.InstanceType == "" {
				ng.InstanceType = "t3.medium"
			}
			if ng.DesiredSize == 0 {
				ng.DesiredSize = 1
			}
			if ng.MinSize == 0 {
				ng.MinSize = 1
			}
			if ng.MaxSize == 0 {
				ng.MaxSize = 2
			}
			if ng.DiskSize == 0 {
				ng.DiskSize = 50
			}
		}
	}
	if c.Bnk != nil && c.Bnk.CertManagerVersion == "" {
		c.Bnk.CertManagerVersion = EmbeddedCertManagerVersion
	}
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
	if c.ClusterSpec != nil {
		if len(c.ClusterSpec.NodeGroups) == 0 {
			return fmt.Errorf("cluster.nodeGroups must contain at least one node group when cluster block is present")
		}
		for _, ng := range c.ClusterSpec.NodeGroups {
			if !nodeGroupNameRE.MatchString(ng.Name) {
				return fmt.Errorf("cluster.nodeGroups[].name %q must be lowercase alphanumeric + hyphens", ng.Name)
			}
		}
	}
	if c.Bnk != nil {
		if err := validateBnk(c.Bnk); err != nil {
			return err
		}
	}
	return nil
}

// validateBnk shape-validates the bnk: block. File-content validation (non-empty
// check, dockerconfigjson parse) is deferred to phase 12 entry so operators can
// construct and validate cluster.yaml without having the supply-chain files
// present (e.g. during dry-run prep on a laptop before a lab session).
//
// Rules:
//   - farArchive must be a non-empty path string
//   - jwt must be a non-empty path string
//   - certManagerVersion must match the embedded YAML version (1.16.1)
func validateBnk(b *BnkSpec) error {
	if b.FARArchive == "" {
		return fmt.Errorf("bnk.farArchive is required when the bnk: block is present")
	}
	if b.JWT == "" {
		return fmt.Errorf("bnk.jwt is required when the bnk: block is present")
	}
	if b.CertManagerVersion != "" && b.CertManagerVersion != EmbeddedCertManagerVersion {
		return fmt.Errorf("bnk.certManagerVersion %q does not match the embedded cert-manager version %q; "+
			"slice 6+ may add multi-version support — for now omit the field to use the default",
			b.CertManagerVersion, EmbeddedCertManagerVersion)
	}
	return nil
}
