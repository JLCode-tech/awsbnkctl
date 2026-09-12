// Package render provides Go text/template rendering helpers for BNK manifest
// templates. Templates use {{ .Field }} syntax (not shell $VAR envsubst).
package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/JLCode-tech/awsbnkctl/internal/bnkconst"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// CertChainVars holds the substitution variables for shared/bnk-cert-chain.yaml.
// All fields are derived from cluster.metadata.name at Phase 12 entry time.
type CertChainVars struct {
	SelfSignedIssuer string // <cluster>-selfsigned-cluster-issuer
	CACertName       string // <cluster>-ca
	CASecretName     string // <cluster>-ca-secret
	CAIssuer         string // <cluster>-ca-cluster-issuer
}

// CertChainVarsFromCluster derives the BNK cert chain template variables from
// the cluster intent. All variable names match aws-gpu-setup's convention so
// existing cert naming is consistent between bash and Go paths.
func CertChainVarsFromCluster(cl *intent.Cluster) CertChainVars {
	name := cl.Metadata.Name
	return CertChainVars{
		SelfSignedIssuer: name + "-selfsigned-cluster-issuer",
		CACertName:       name + "-ca",
		CASecretName:     name + "-ca-secret",
		CAIssuer:         name + "-ca-cluster-issuer",
	}
}

// Render executes a Go text/template given in tmpl with data as the dot-value
// and returns the rendered bytes. Returns a descriptive error on any parse or
// execution failure.
func Render(tmpl []byte, data interface{}) ([]byte, error) {
	t, err := template.New("manifest").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("render: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render: execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderCertChain renders the BNK cert chain template with vars derived from
// the cluster intent. Convenience wrapper over Render + CertChainVarsFromCluster.
func RenderCertChain(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	vars := CertChainVarsFromCluster(cl)
	return Render(tmpl, vars)
}

// FLOValuesVars holds the substitution variables for shared/flo-values.yaml.tmpl.
// All fields are derived from the cluster intent plus state set by earlier phases
// at Phase 14 entry.
type FLOValuesVars struct {
	CAIssuer      string // <cluster>-ca-cluster-issuer
	FARSecretName string // far-secret
	JWT           string // raw JWT token contents (NOT base64)
	ClusterName   string // cl.Metadata.Name
}

// FLOValuesVarsFromCluster derives the FLO values template variables from the
// cluster intent. JWT is passed explicitly because it is file-read data, not
// derivable from the intent struct alone.
func FLOValuesVarsFromCluster(cl *intent.Cluster, jwt string) FLOValuesVars {
	cvars := CertChainVarsFromCluster(cl)
	return FLOValuesVars{
		CAIssuer:      cvars.CAIssuer,
		FARSecretName: "far-secret",
		JWT:           jwt,
		ClusterName:   cl.Metadata.Name,
	}
}

// RenderFLOValues renders the FLO values template with vars derived from
// the cluster intent and the raw JWT string.
func RenderFLOValues(tmpl []byte, cl *intent.Cluster, jwt string) ([]byte, error) {
	vars := FLOValuesVarsFromCluster(cl, jwt)
	return Render(tmpl, vars)
}

// OTELCertsVars holds the substitution variables for shared/otel-certs.yaml.
// All fields are derived from the cluster intent at Phase 15 entry.
type OTELCertsVars struct {
	OTELSvrCert     string // external-otelsvr
	OTELSvrSecret   string // external-otelsvr-secret
	OTELF5IngCert   string // external-f5ingotelsvr
	OTELF5IngSecret string // external-f5ingotelsvr-secret
	OperatorNS      string // f5-cne-core
	CAIssuer        string // <cluster>-ca-cluster-issuer
}

// OTELCertsVarsFromCluster derives the OTEL certs template variables from the
// cluster intent. Names match aws-gpu-setup's vars.env OTEL_* constants.
func OTELCertsVarsFromCluster(cl *intent.Cluster) OTELCertsVars {
	cvars := CertChainVarsFromCluster(cl)
	// #nosec G101 -- these are k8s resource names, not credential values
	return OTELCertsVars{
		OTELSvrCert:     "external-otelsvr",
		OTELSvrSecret:   "external-otelsvr-secret",
		OTELF5IngCert:   "external-f5ingotelsvr",
		OTELF5IngSecret: "external-f5ingotelsvr-secret",
		OperatorNS:      "f5-cne-core",
		CAIssuer:        cvars.CAIssuer,
	}
}

// RenderOTELCerts renders the OTEL certs template with vars derived from
// the cluster intent.
func RenderOTELCerts(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	vars := OTELCertsVarsFromCluster(cl)
	return Render(tmpl, vars)
}

// ─── cloud-network-mapping ConfigMap ─────────────────────────────────────────

type CloudNetworkMappingAZ struct {
	Name    string
	Subnets []CloudNetworkMappingSubnet
}

type CloudNetworkMappingSubnet struct {
	CIDR     string
	SubnetID string
}

// CloudNetworkMappingVars holds the substitution variables for
// shared/cloud-network-mapping.yaml.tmpl. All fields are derived from the
// cluster intent plus state set by earlier phases at Phase 19 entry.
type CloudNetworkMappingVars struct {
	AZs []CloudNetworkMappingAZ
}

// RenderCloudNetworkMapping renders the cloud-network-mapping ConfigMap
// template. It reads subnet IDs from state.env (written by Phase 03) and CIDRs
// from the cluster intent. Returns an error if required state keys or intent
// fields are missing.
func RenderCloudNetworkMapping(tmpl []byte, cl *intent.Cluster, getter func(string) string) ([]byte, error) {
	if len(cl.Network.AZs) == 0 {
		return nil, fmt.Errorf("render: network.azs is empty")
	}
	if len(cl.Network.Subnets.Public) == 0 {
		return nil, fmt.Errorf("render: network.subnets.public is empty")
	}
	if cl.Network.DataPath == nil {
		return nil, fmt.Errorf("render: network.dataPath is nil (required for BNK patterns)")
	}
	mgmtSubnet := getter("MGMT_SUBNET")
	if mgmtSubnet == "" {
		return nil, fmt.Errorf("render: MGMT_SUBNET not in state (Phase 03 must run first)")
	}
	bnkExtSubnet := getter("BNK_EXT_SUBNET")
	if bnkExtSubnet == "" {
		return nil, fmt.Errorf("render: BNK_EXT_SUBNET not in state (Phase 03 must run first)")
	}

	azSubnets := make(map[string][]CloudNetworkMappingSubnet)

	mgmtAZ := cl.Network.Subnets.Public[0].AZ
	azSubnets[mgmtAZ] = append(azSubnets[mgmtAZ], CloudNetworkMappingSubnet{
		CIDR:     cl.Network.Subnets.Public[0].CIDR,
		SubnetID: mgmtSubnet,
	})

	extAZ := cl.Network.DataPath.External.AZ
	azSubnets[extAZ] = append(azSubnets[extAZ], CloudNetworkMappingSubnet{
		CIDR:     cl.Network.DataPath.External.CIDR,
		SubnetID: bnkExtSubnet,
	})

	if cl.HasInternalInterface() {
		bnkIntSubnet := getter("BNK_INT_SUBNET")
		if bnkIntSubnet == "" {
			return nil, fmt.Errorf("render: BNK_INT_SUBNET not in state (Phase 03 must run first)")
		}
		intAZ := cl.Network.DataPath.Internal.AZ
		azSubnets[intAZ] = append(azSubnets[intAZ], CloudNetworkMappingSubnet{
			CIDR:     cl.Network.DataPath.Internal.CIDR,
			SubnetID: bnkIntSubnet,
		})
	}

	var vars CloudNetworkMappingVars
	// Preserve deterministic order by iterating over all defined AZs
	for _, az := range cl.Network.AZs {
		if subnets, ok := azSubnets[az]; ok {
			vars.AZs = append(vars.AZs, CloudNetworkMappingAZ{
				Name:    az,
				Subnets: subnets,
			})
		}
	}

	return Render(tmpl, vars)
}

// ─── NetworkAttachmentDefinitions (host-device) ───────────────────────────────

// NADVars holds the substitution variables for
// host-device/network-attachment-defs.yaml.tmpl.
// For the host-device pattern the interface names and NAD names are
// architecture constants (Architect: not operator knobs).
type NADVars struct {
	Namespace       string // target namespace for the NADs
	ExternalNADName string // external-netdevice
	InternalNADName string // internal-netdevice
	ExternalPCI     string // 0000:00:08.0
	InternalPCI     string // 0000:00:07.0
	HasInternal     bool   // render the internal NAD (dual-interface only)
}

// orDefault returns getter(key) if non-empty, otherwise def.
// Used to source discovered values from state with a constant fallback.
func orDefault(getter func(string) string, key, def string) string {
	if v := getter(key); v != "" {
		return v
	}
	return def
}

// RenderNADs renders the host-device NADs template for the given namespace.
// PCI bus addresses are sourced from the state getter (set by phase 17c iface-
// discovery). Falls back to architecture constants when the getter returns "".
// hasInternal controls whether the internal NAD is emitted (dual-interface only).
func RenderNADs(tmpl []byte, namespace string, hasInternal bool, getter func(string) string) ([]byte, error) {
	vars := NADVars{
		Namespace:       namespace,
		ExternalNADName: "external-netdevice",
		InternalNADName: "internal-netdevice",
		ExternalPCI:     orDefault(getter, "EXTERNAL_PCI", "0000:00:08.0"), // ens8, device-index 3
		InternalPCI:     orDefault(getter, "INTERNAL_PCI", "0000:00:07.0"), // ens7, device-index 2
		HasInternal:     hasInternal,
	}
	return Render(tmpl, vars)
}

// ─── sriov-external dataplane (vfio/DPDK) ─────────────────────────────────────

// SriovVars holds substitution variables for the sriov-external manifests
// (vfio-node-prep, sriovdp, network-attachment-defs). Each template uses a
// subset; passing the full struct is harmless.
type SriovVars struct {
	ExternalPCI     string // the external ENA PCI BDF (e.g. 0000:00:08.0)
	Namespace       string // target namespace (f5-cne-system / default)
	ExternalNADName string // external-sriov
}

// RenderSriov renders any sriov-external template with the given values.
func RenderSriov(tmpl []byte, externalPCI, namespace, nadName string) ([]byte, error) {
	return Render(tmpl, SriovVars{
		ExternalPCI:     externalPCI,
		Namespace:       namespace,
		ExternalNADName: nadName,
	})
}

// ─── NVIDIA device-plugin (GPU node groups) ────────────────────────────────────

// NvidiaDevicePluginVars holds substitution variables for the NVIDIA device-plugin
// DaemonSet template. Version is templated so the phase's const is the single
// source of truth for the pinned image tag.
type NvidiaDevicePluginVars struct {
	// Version is the NVIDIA k8s-device-plugin image tag (e.g. "v0.17.1").
	// Rendered as: nvcr.io/nvidia/k8s-device-plugin:{{ .Version }}.
	Version string
}

// RenderNvidiaDevicePlugin renders the NVIDIA device-plugin DaemonSet template
// with the given version tag. The template is upstream-verbatim with a
// nodeSelector targeting awsbnkctl.io/gpu=true nodes only.
func RenderNvidiaDevicePlugin(tmpl []byte, version string) ([]byte, error) {
	return Render(tmpl, NvidiaDevicePluginVars{Version: version})
}

// ─── CNEInstance CR ────────────────────────────────────────────────────────

// cneInstanceNamespace is the k8s namespace for CNEInstance and related resources.
// Sourced from bnkconst to avoid an import cycle (phases imports render; render
// cannot import phases). bnkconst is the single source of truth.
const cneInstanceNamespace = bnkconst.InstanceNamespace

// CNEInstanceVars holds the substitution variables for
// shared/cneinstance.yaml.tmpl. Fields are split into three categories:
//   - Operator-knobs: sourced from cl.Bnk.* (set by cluster.yaml, defaults applied).
//   - State-derived: sourced from st.Get() (written by earlier phases).
//   - Hardcoded constants: baked into the template, NOT templated.
type CNEInstanceVars struct {
	// Operator-knobs (cluster.yaml bnk:)
	DeploymentSize   string // default "Small"
	StorageClassName string // default "gp3"
	ManifestVersion  string // default manifest.DefaultManifestVersion
	TmmMtu           int    // default 9000
	TmmCpu           string // default "4"
	TmmMemory        string // default "16Gi"
	TmmHugepages     string // default "8Gi"
	PalCpuSet        string // default "0-3"

	// State-derived
	InstanceNameCR      string // <cluster>-bnk
	InstanceNS          string // f5-cne-system
	LabName             string // cluster.metadata.name
	CAIssuer            string // <cluster>-ca-cluster-issuer
	FARSecretName       string // far-secret
	VPCID               string // VPC_ID from state
	AWSRegion           string // cl.Metadata.Region
	ExternalNAD         string // external-netdevice
	InternalNAD         string // internal-netdevice
	ExternalIFName      string // ens8 (or discovered value)
	InternalIFName      string // ens7 (or discovered value)
	ExternalIFUpper     string // ENS8 (uppercase of ExternalIFName, for PCIDEVICE_INTEL_COM_<NAME>)
	InternalIFUpper     string // ENS7 (uppercase of InternalIFName, for PCIDEVICE_INTEL_COM_<NAME>)
	ExternalPCI         string // 0000:00:08.0 (or discovered value)
	InternalPCI         string // 0000:00:07.0 (or discovered value)
	CloudHostDeviceName string // ens8 (or discovered value)
	CloudHostDeviceTag  string // f5-cne-device
	HasInternal         bool   // list the internal NAD + internal ROBIN/PCIDEVICE env (dual-interface only)
	Sriov               bool   // sriov-external: drop TMM_GENERIC_SOCKET_DRIVER + let the device plugin inject PCIDEVICE_INTEL_COM
}

// RenderCNEInstance renders the CNEInstance CR template with vars derived
// from the cluster intent and state.
// Requires VPC_ID in state (Phase 02). All BnkSpec fields must have defaults
// applied (intent.Load does this).
func RenderCNEInstance(tmpl []byte, cl *intent.Cluster, getter func(string) string) ([]byte, error) {
	if cl.Bnk == nil {
		return nil, fmt.Errorf("render: cl.Bnk is nil — bnk: block required in cluster.yaml")
	}
	vpcID := getter("VPC_ID")
	if vpcID == "" {
		return nil, fmt.Errorf("render: VPC_ID not in state (Phase 02 must run first)")
	}
	cvars := CertChainVarsFromCluster(cl)
	extIFName := orDefault(getter, "EXTERNAL_IFNAME", "ens8")
	intIFName := orDefault(getter, "INTERNAL_IFNAME", "ens7")
	sriov := cl.DataplaneBinding() == "sriov"
	// sriov-external attaches the sriov NAD (external-sriov, backed by the
	// device-plugin vfio resource) instead of the host-device NAD.
	externalNAD := "external-netdevice"
	if sriov {
		externalNAD = "external-sriov"
	}
	vars := CNEInstanceVars{
		// Operator-knobs
		DeploymentSize:   cl.Bnk.DeploymentSize,
		StorageClassName: cl.Bnk.StorageClassName,
		ManifestVersion:  cl.Bnk.ManifestVersion,
		TmmMtu:           cl.Bnk.TmmMtu,
		TmmCpu:           cl.Bnk.TmmCpu,
		TmmMemory:        cl.Bnk.TmmMemory,
		TmmHugepages:     cl.Bnk.TmmHugepages,
		PalCpuSet:        cl.Bnk.PalCpuSet,
		// State-derived
		InstanceNameCR:      cl.Metadata.Name + "-bnk",
		InstanceNS:          cneInstanceNamespace,
		LabName:             cl.Metadata.Name,
		CAIssuer:            cvars.CAIssuer,
		FARSecretName:       "far-secret",
		VPCID:               vpcID,
		AWSRegion:           cl.Metadata.Region,
		ExternalNAD:         externalNAD,
		InternalNAD:         "internal-netdevice",
		ExternalIFName:      extIFName,
		InternalIFName:      intIFName,
		ExternalIFUpper:     strings.ToUpper(extIFName),
		InternalIFUpper:     strings.ToUpper(intIFName),
		ExternalPCI:         orDefault(getter, "EXTERNAL_PCI", "0000:00:08.0"),
		InternalPCI:         orDefault(getter, "INTERNAL_PCI", "0000:00:07.0"),
		CloudHostDeviceName: orDefault(getter, "CLOUD_HOST_DEVICE_NAME", extIFName),
		CloudHostDeviceTag:  "f5-cne-device",
		HasInternal:         cl.HasInternalInterface(),
		Sriov:               sriov,
	}
	return Render(tmpl, vars)
}

// ─── License CR ───────────────────────────────────────────────────────────────

// LicenseCRVars holds the substitution variables for shared/license-cr.yaml.tmpl.
type LicenseCRVars struct {
	LabName string // cluster.metadata.name
	JWT     string // raw JWT token string (file contents, whitespace-trimmed)
}

// RenderLicenseCR renders the License CR template. jwt must be the raw token
// string (already read from disk and whitespace-trimmed by the caller).
// The caller (Phase 23) is responsible for file I/O so that dry-run mode can
// pass a placeholder string without a real file on disk.
func RenderLicenseCR(tmpl []byte, cl *intent.Cluster, jwt string) ([]byte, error) {
	vars := LicenseCRVars{
		LabName: cl.Metadata.Name,
		JWT:     jwt,
	}
	return Render(tmpl, vars)
}

// ─── Iface-discovery pod (phase 17c) ─────────────────────────────────────────

// IfaceDiscoveryPodVars holds the substitution variables for
// host-device/iface-discovery-pod.yaml.tmpl.
type IfaceDiscoveryPodVars struct {
	Namespace string // target namespace (kube-system)
	NodeName  string // TMM_NODE_NAME (schedules pod on the exact node)
}

// RenderIfaceDiscoveryPod renders the iface-discovery pod manifest.
func RenderIfaceDiscoveryPod(tmpl []byte, namespace, nodeName string) ([]byte, error) {
	vars := IfaceDiscoveryPodVars{
		Namespace: namespace,
		NodeName:  nodeName,
	}
	return Render(tmpl, vars)
}

// GatewayClassVars holds the substitution variables for host-device/gatewayclass.yaml.tmpl.
type GatewayClassVars struct {
	GwcName    string // <cluster>-gatewayclass
	LabName    string // cl.Metadata.Name
	InstanceNS string // f5-cne-system
}

// RenderGatewayClass renders the GatewayClass template for the host-device pattern.
func RenderGatewayClass(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	vars := GatewayClassVars{
		GwcName:    cl.Metadata.Name + "-gatewayclass",
		LabName:    cl.Metadata.Name,
		InstanceNS: cneInstanceNamespace,
	}
	return Render(tmpl, vars)
}

// ─── Infra (BNK 2.4 network model) ──────────────────────────────────────────

// Infra network names; GatewaySettings networkRefs (ingress listener network,
// egress tunnel network) point at these.
const (
	InfraName          = "infra"
	InfraExtNetwork    = "ext-vlan-infra"
	InfraIntNetwork    = "int-vlan-infra"
	InfraListenerPool  = "listener-pool"
	infraListenerFirst = 100
	infraListenerLast  = 199

	// InfraEgressSubnet / InfraEgressPort are the egress tunnel defaults
	// (Infra spec.egressDefaults): the VXLAN overlay subnet between the worker
	// nodes and TMM and its UDP port. 192.168.0.0/16 stays clear of every
	// example VPC (10.x); the product default would be 10.0.0.0/16.
	InfraEgressSubnet = "192.168.0.0/16"
	InfraEgressPort   = 4789
	// Static route names in the Infra CR (spec.staticRoutes[].name).
	InfraRouteVPC     = "vpc"
	InfraRouteDefault = "default"
)

// ZebOSConfigMapName is the ConfigMap the ZebOS routing container reads its
// BGP configuration from (key ZebOS.conf; F5 "Set up dynamic routing with
// BGP"). FLO creates it empty; the operator fills it in for BGP peering
// (examples/*/bgp-route-server.yaml, docs/BGP-ROUTE-SERVER.md). BNK 2.4.0 as
// FLO 2.30 installs it runs ZebOS (ZEBOS_STATE=legacy), so this — not the
// OcNOS-only GlobalRoutingConfig/RoutingTemplate CRs — is the 2.4 BGP config.
const ZebOSConfigMapName = "f5-tmm-dynamic-routing-template"

// InfraTunnelNetwork is the Infra network the egress tunnel terminates on
// (Infra egressDefaults.networkRef and the GatewaySettings egressConfigs
// networkRef): the internal VLAN on dual-interface clusters, the external VLAN
// when it is the only one.
func InfraTunnelNetwork(hasInternal bool) string {
	if hasInternal {
		return InfraIntNetwork
	}
	return InfraExtNetwork
}

// InfraVars holds the substitution variables for host-device/infra.yaml.tmpl.
type InfraVars struct {
	InfraName       string // infra (singleton per BNK namespace)
	InstanceNS      string // f5-cne-system (matches CNEInstance namespace)
	LabName         string // cl.Metadata.Name
	TmmExtSelfIP    string // pool start, e.g. 10.0.10.224 (the /27 holding the configured .240)
	TmmExtSelfIPEnd string // pool end, e.g. 10.0.10.254
	TmmIntSelfIP    string // pool start, e.g. 10.0.20.224
	TmmIntSelfIPEnd string // pool end, e.g. 10.0.20.254
	ListenerPool    string // IPAM entry the GatewaySettings reference for VIPs
	ListenerStart   string // e.g. 10.0.10.100
	ListenerEnd     string // e.g. 10.0.10.199
	ExternalNAD     string // external-netdevice or external-sriov
	InternalNAD     string // internal-netdevice
	ExtNetwork      string // ext-vlan
	IntNetwork      string // int-vlan
	Mtu             int    // cl.Bnk.TmmMtu
	HasInternal     bool   // render the internal pool, attachment and network
	ExtAZ           string // availability zone of the external data-path subnet (pool zone)
	IntAZ           string // availability zone of the internal data-path subnet
	// Egress: the tunnel defaults and the two static routes the egress path
	// needs (see infra.yaml.tmpl). The routes are rendered only when VpcCidr
	// and both gateways are known.
	TunnelNetwork string // int-vlan-infra (dual-interface) or ext-vlan-infra
	EgressSubnet  string // InfraEgressSubnet
	EgressPort    int    // InfraEgressPort
	VpcCidr       string // network.vpcCidr, e.g. 10.0.0.0/16
	TunnelGateway string // AWS router of the tunnel VLAN's subnet, e.g. 10.0.20.1
	ExtGateway    string // AWS router of the external subnet, e.g. 10.0.10.1
	RouteVPC      string // InfraRouteVPC
	RouteDefault  string // InfraRouteDefault
}

// RenderInfra renders the Infra CR for a BNK pattern. Each self-IP pool is the
// /27 that contains cl.Network.DataPath.SelfIPs (auto-derived by
// intent.applyDefaults; see intent.SelfIPPoolRange for why); the
// listener pool is hosts .100-.199 of the external data-path subnet, which
// covers the VIP plan and stays clear of the self IP (.240) and the jumphost
// (.200).
func RenderInfra(tmpl []byte, cl *intent.Cluster, hasInternal bool) ([]byte, error) {
	if cl.Network.DataPath == nil || cl.Network.DataPath.SelfIPs == nil || cl.Network.DataPath.External.CIDR == "" {
		return nil, fmt.Errorf("render infra: network.dataPath external subnet and self IPs are required")
	}
	sel := cl.Network.DataPath.SelfIPs
	if sel.External == "" {
		return nil, fmt.Errorf("render infra: external self IP not derivable (external subnet must be /24)")
	}
	if hasInternal && sel.Internal == "" {
		return nil, fmt.Errorf("render infra: internal self IP not derivable (internal subnet must be /24)")
	}
	extStart, extEnd, err := intent.SelfIPPoolRange(sel.External)
	if err != nil {
		return nil, fmt.Errorf("render infra: external self IP pool: %w", err)
	}
	var intStart, intEnd string
	if hasInternal {
		if intStart, intEnd, err = intent.SelfIPPoolRange(sel.Internal); err != nil {
			return nil, fmt.Errorf("render infra: internal self IP pool: %w", err)
		}
	}
	start, _ := intent.DeriveSelfIP(cl.Network.DataPath.External.CIDR, infraListenerFirst)
	end, _ := intent.DeriveSelfIP(cl.Network.DataPath.External.CIDR, infraListenerLast)
	if start == "" || end == "" {
		return nil, fmt.Errorf("render infra: listener pool not derivable from external CIDR %q (must be /24)", cl.Network.DataPath.External.CIDR)
	}
	externalNAD := "external-netdevice"
	if cl.DataplaneBinding() == "sriov" {
		externalNAD = "external-sriov"
	}
	mtu := 1500
	if cl.Bnk != nil && cl.Bnk.TmmMtu > 0 {
		mtu = cl.Bnk.TmmMtu
	}
	// AWS puts the subnet router on the first host address of every subnet.
	extGW, _ := intent.DeriveSelfIP(cl.Network.DataPath.External.CIDR, 1)
	tunnelGW := extGW
	if hasInternal {
		tunnelGW, _ = intent.DeriveSelfIP(cl.Network.DataPath.Internal.CIDR, 1)
	}
	vars := InfraVars{
		InfraName:       InfraName,
		InstanceNS:      cneInstanceNamespace,
		LabName:         cl.Metadata.Name,
		TmmExtSelfIP:    extStart,
		TmmExtSelfIPEnd: extEnd,
		TmmIntSelfIP:    intStart,
		TmmIntSelfIPEnd: intEnd,
		ListenerPool:    InfraListenerPool,
		ListenerStart:   start,
		ListenerEnd:     end,
		ExternalNAD:     externalNAD,
		InternalNAD:     "internal-netdevice",
		ExtNetwork:      InfraExtNetwork,
		IntNetwork:      InfraIntNetwork,
		Mtu:             mtu,
		HasInternal:     hasInternal,
		ExtAZ:           cl.Network.DataPath.External.AZ,
		IntAZ:           cl.Network.DataPath.Internal.AZ,
		TunnelNetwork:   InfraTunnelNetwork(hasInternal),
		EgressSubnet:    InfraEgressSubnet,
		EgressPort:      InfraEgressPort,
		VpcCidr:         cl.Network.VPCCidr,
		TunnelGateway:   tunnelGW,
		ExtGateway:      extGW,
		RouteVPC:        InfraRouteVPC,
		RouteDefault:    InfraRouteDefault,
	}
	return Render(tmpl, vars)
}

// ─── cne-controller RBAC supplement (BNK 2.4 / FLO 2.30) ─────────────────────

// CNEControllerServiceAccount is the ServiceAccount FLO creates for the
// cne-controller Deployment.
const CNEControllerServiceAccount = "f5-cne-controller"

// CNEControllerRBACName is the ClusterRole/ClusterRoleBinding awsbnkctl adds
// so the controller can read EndpointSlices (see shared/cne-controller-rbac.yaml.tmpl).
func CNEControllerRBACName(cl *intent.Cluster) string {
	return cl.Metadata.Name + "-cne-controller-endpointslices"
}

// CNEControllerRBACVars holds the substitution variables for
// shared/cne-controller-rbac.yaml.tmpl.
type CNEControllerRBACVars struct {
	Name           string // <cluster>-cne-controller-endpointslices
	LabName        string // cl.Metadata.Name
	InstanceNS     string // f5-cne-system
	ServiceAccount string // f5-cne-controller
}

// RenderCNEControllerRBAC renders the EndpointSlice ClusterRole + binding for
// the cne-controller ServiceAccount.
func RenderCNEControllerRBAC(tmpl []byte, cl *intent.Cluster) ([]byte, error) {
	if cl == nil || cl.Metadata.Name == "" {
		return nil, fmt.Errorf("render cne-controller rbac: cluster name is required")
	}
	return Render(tmpl, CNEControllerRBACVars{
		Name:           CNEControllerRBACName(cl),
		LabName:        cl.Metadata.Name,
		InstanceNS:     cneInstanceNamespace,
		ServiceAccount: CNEControllerServiceAccount,
	})
}
