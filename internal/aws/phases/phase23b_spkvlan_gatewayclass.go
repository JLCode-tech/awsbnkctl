package phases

import (
	"context"
	"fmt"
	"os"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	k8swait "github.com/JLCode-tech/awsbnkctl/internal/k8s"
	k8smanifests "github.com/JLCode-tech/awsbnkctl/internal/k8s/manifests"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s/render"
)

const (
	// infraCRDName is the BNK 2.4 Infra CRD (VLANs, IPAM pools, listener
	// context). Installed by FLO's crd-installer after the CNEInstance reconciles.
	infraCRDName = "infras.gateway.k8s.f5.com"
	// Why: CRD applies are typically sub-second once available; 3 minutes is a
	// generous readiness budget on cold clusters.
	infraCRDWait  = 3 * time.Minute
	infraYAMLPath = "host-device/infra.yaml.tmpl"
	// infraProgrammedWait bounds the wait for Infra status Programmed=True, the
	// signal that TMM has the VLANs and the controller has the listener context.
	infraProgrammedWait = 5 * time.Minute
	// gatewayClassAcceptedWait bounds the wait for GatewayClass Accepted=True; the
	// Infra CR is only reconciled once the class exists.
	gatewayClassAcceptedWait = 3 * time.Minute
	gatewayClassYAMLPath     = "host-device/gatewayclass.yaml.tmpl"

	gatewayClassCRDName = "gatewayclasses.gateway.networking.k8s.io"

	// cneControllerRBACYAMLPath is the EndpointSlice ClusterRole + binding awsbnkctl
	// adds for the controller ServiceAccount (FLO 2.30 grants no get).
	cneControllerRBACYAMLPath = "shared/cne-controller-rbac.yaml.tmpl"

	// cneControllerAvailableWait bounds the wait for the cne-controller
	// Deployment to have an available replica before the Infra apply. The
	// F5 validating webhook (f5-validation-svc) is served by that pod; applying
	// before it is ready fails with "no endpoints available for service".
	// The controller is in its first rollout (and phase 21 may have restarted
	// it) when this phase runs, so a cold start of several minutes is normal.
	cneControllerAvailableWait = 10 * time.Minute
	// webhookRetryWait bounds the apply retries while the webhook endpoint is
	// still registering after the pod reports Ready.
	webhookRetryWait = 3 * time.Minute
	// Why: installed by the same FLO crd-installer Job as the Infra CRD; 3 min is generous.
	gatewayClassCRDWait = 3 * time.Minute
)

// f5spkvlanGVR is the 2.3.x F5SPKVlan CR; Phase23bDown still deletes it so a
// cluster built by an older awsbnkctl tears down cleanly.
var f5spkvlanGVR = schema.GroupVersionResource{
	Group:    "k8s.f5net.com",
	Version:  "v1",
	Resource: "f5-spk-vlans",
}

// gatewayClassGVR is the GVR for the cluster-scoped GatewayClass CR.
var gatewayClassGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "gatewayclasses",
}

// ipamGVR is the F5 IPAM controller's IPAM CR. The cne-controller creates one
// per Infra VLAN network and FIC writes the allocated self IP to its status.
var ipamGVR = schema.GroupVersionResource{
	Group:    "fic.f5.com",
	Version:  "v1",
	Resource: "ipams",
}

// clusterRoleGVR / clusterRoleBindingGVR name the RBAC supplement awsbnkctl
// manages for the cne-controller.
var (
	clusterRoleGVR        = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}
	clusterRoleBindingGVR = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}
)

// infraGVR is the BNK 2.4 Infra CR (singleton in the CNE namespace).
var infraGVR = schema.GroupVersionResource{
	Group:    "gateway.k8s.f5.com",
	Version:  "v1alpha1",
	Resource: "infras",
}

// Phase23bSPKVlanGatewayClass applies the Infra + GatewayClass CRs that
// complete TMM data-plane plumbing (host-device pattern).
//
//   - Infra (BNK 2.4) binds TMM trunks 1.1 (external) / 1.2 (internal) to named
//     virtual interfaces inside the TMM pod netns, announcing the SelfIPs
//     that Phase 17 assigned as secondary IPs on the AWS ENIs.
//   - GatewayClass registers the BNK cne-controller as a Gateway API
//     implementation so operator-facing Gateway CRs can target it via
//     spec.gatewayClassName.
//
// Runs AFTER Phase 23 (License) and BEFORE Phase 24 (CWC heal). Both the
// Infra and GatewayClass CRDs are installed by FLO once the CNEInstance
// reaches Reconciled, so we wait for BOTH CRDs before applying either CR
// (FLO can take 10+ min on a cold cluster).
//
// The int-vlan is applied only for dual-interface; single-interface patterns
// (external-only) get just ext-vlan + GatewayClass.
//
// Skipped when the cluster is not a BNK pattern.
// SSO sentinel: CheckAuthOrDie at entry.
func Phase23bSPKVlanGatewayClass(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name
	if !cl.IsBNKPattern() {
		fmt.Fprintf(os.Stderr, "[phase 23b] skipped: pattern=%q (BNK patterns only)\n", cl.Pattern)
		return nil
	}
	hasInternal := cl.HasInternalInterface()
	fmt.Fprintf(os.Stderr, "[phase 23b] Infra + GatewayClass: cluster=%s\n", name)

	if dryRun {
		if hasInternal {
			fmt.Fprintln(os.Stderr, "[phase 23b] dry-run: would wait for the Infra CRD then apply Infra (ext-vlan + int-vlan, listener pool) + GatewayClass")
		} else {
			fmt.Fprintln(os.Stderr, "[phase 23b] dry-run: would wait for the Infra CRD then apply Infra (ext-vlan, listener pool) + GatewayClass (single-interface)")
		}
		st.Set("INFRA_APPLIED_AT", "dry-run")
		st.Set("GATEWAYCLASS_NAME", name+"-gatewayclass")
		return nil
	}

	if clients.Dynamic == nil {
		return fmt.Errorf("phase23b: Clients.Dynamic is nil — call clients.AttachK8s(kubeconfigPath) first")
	}
	if cl.Network.DataPath == nil || cl.Network.DataPath.SelfIPs == nil {
		return fmt.Errorf("phase23b: cl.Network.DataPath.SelfIPs not set (intent.applyDefaults should derive from CIDRs)")
	}
	selfIPs := cl.Network.DataPath.SelfIPs
	if selfIPs.External == "" {
		return fmt.Errorf("phase23b: external SelfIP not derivable — DataPath external subnet must be /24 (see DeriveSelfIP)")
	}
	if hasInternal && selfIPs.Internal == "" {
		return fmt.Errorf("phase23b: internal SelfIP not derivable — DataPath internal subnet must be /24 (see DeriveSelfIP)")
	}

	// Wait for the Infra CRD (installed by FLO after CNEInstance reconciles).
	fmt.Fprintf(os.Stderr, "[phase 23b] waiting for CRD %s (up to %s)\n", infraCRDName, infraCRDWait)
	if err := k8swait.WaitForCRDExists(ctx, clients.Dynamic, infraCRDName, infraCRDWait); err != nil {
		return fmt.Errorf("phase23b: waiting for Infra CRD: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] CRD %s ready\n", infraCRDName)

	// Wait for the GatewayClass CRD too (installed by the same FLO crd-installer
	// Job). We wait for BOTH CRDs before applying EITHER CR: the GatewayClass
	// apply otherwise races the RESTMapper cache when the CRD landed <2s earlier
	// on the previous discovery snapshot. The apply path's reset-and-retry in
	// applyUnstructured re-queries discovery on the next call once the CRD is
	// present.
	fmt.Fprintf(os.Stderr, "[phase 23b] waiting for CRD %s (up to %s)\n", gatewayClassCRDName, gatewayClassCRDWait)
	if err := k8swait.WaitForCRDExists(ctx, clients.Dynamic, gatewayClassCRDName, gatewayClassCRDWait); err != nil {
		return fmt.Errorf("phase23b: waiting for GatewayClass CRD: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] CRD %s ready\n", gatewayClassCRDName)

	// Wait for the cne-controller to be available: its pod serves the F5
	// validating webhook that admits Infra. Seen live on BNK 2.4.0
	// (2026-09-11): the CRDs were established while the controller pod was
	// still starting and the apply failed with "no endpoints available for
	// service f5-validation-svc".
	if clients.K8s != nil {
		fmt.Fprintf(os.Stderr, "[phase 23b] waiting for deploy %s/%s to be available (up to %s)\n", InstanceNamespace, h4DeploymentName, cneControllerAvailableWait)
		if err := waitForDeploymentAvailable(ctx, clients, InstanceNamespace, h4DeploymentName, cneControllerAvailableWait); err != nil {
			return fmt.Errorf("phase23b: waiting for cne-controller: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[phase 23b] deploy %s/%s available\n", InstanceNamespace, h4DeploymentName)
	}

	// RBAC supplement: FLO 2.30's ClusterRole lets the controller list/watch
	// EndpointSlices but not get them, and the 2.4.0 controller GETs the slice of
	// every Gateway backend ("Error getting endpointSlice ... is forbidden", live
	// 2026-09-11), so pools stayed empty. The 2.4 f5ingress chart grants get.
	rbacTmpl, err := k8smanifests.FS.ReadFile(cneControllerRBACYAMLPath)
	if err != nil {
		return fmt.Errorf("phase23b: reading cne-controller rbac template: %w", err)
	}
	rbacRendered, err := render.RenderCNEControllerRBAC(rbacTmpl, cl)
	if err != nil {
		return fmt.Errorf("phase23b: rendering cne-controller rbac: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] applying ClusterRole/Binding %s (EndpointSlice get for %s/%s; FLO 2.30 omits it)\n", render.CNEControllerRBACName(cl), InstanceNamespace, render.CNEControllerServiceAccount)
	if err := applyRawYAML(ctx, clients, rbacRendered); err != nil {
		return fmt.Errorf("phase23b: applying cne-controller rbac: %w", err)
	}

	// Render + apply GatewayClass.
	gwcTmpl, err := k8smanifests.FS.ReadFile(gatewayClassYAMLPath)
	if err != nil {
		return fmt.Errorf("phase23b: reading gatewayclass template: %w", err)
	}
	gwcRendered, err := render.RenderGatewayClass(gwcTmpl, cl)
	if err != nil {
		return fmt.Errorf("phase23b: rendering gatewayclass: %w", err)
	}
	gwcName := name + "-gatewayclass"
	fmt.Fprintf(os.Stderr, "[phase 23b] applying GatewayClass %s\n", gwcName)
	if err := retryWhileWebhookUnavailable(ctx, webhookRetryWait, func() error { return applyRawYAML(ctx, clients, gwcRendered) }); err != nil {
		return fmt.Errorf("phase23b: applying GatewayClass: %w", err)
	}
	st.Set("GATEWAYCLASS_NAME", gwcName)
	fmt.Fprintf(os.Stderr, "[phase 23b] waiting for GatewayClass %s Accepted=True (up to %s)\n", gwcName, gatewayClassAcceptedWait)
	if err := waitForConditionTrue(ctx, clients.Dynamic, gatewayClassGVR, "", gwcName, "Accepted", gatewayClassAcceptedWait); err != nil {
		return fmt.Errorf("phase23b: GatewayClass %s did not reach Accepted=True: %w", gwcName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] GatewayClass %s Accepted=True\n", gwcName)

	// Render + apply Infra. The controller only reconciles the Infra CR once a
	// GatewayClass names it (F5: "Install GatewayClass" before "Deploy the Infra
	// CR"); applied first, Infra sits at "Accepted=Unknown Waiting for controller"
	// (seen live 2026-09-11).
	infraTmpl, err := k8smanifests.FS.ReadFile(infraYAMLPath)
	if err != nil {
		return fmt.Errorf("phase23b: reading infra template: %w", err)
	}
	infraRendered, err := render.RenderInfra(infraTmpl, cl, hasInternal)
	if err != nil {
		return fmt.Errorf("phase23b: rendering infra: %w", err)
	}
	if cl.IsBGPEnabled() {
		fmt.Fprintln(os.Stderr, "[phase 23b] warning: bnk.bgp is set, but the BNK 2.4 Infra CR has no per-VLAN allowed-services; verify BGP reachability to the external self IP live")
	}
	if hasInternal {
		fmt.Fprintf(os.Stderr, "[phase 23b] applying Infra %s: %s (selfip=%s) + %s (selfip=%s), listener pool %s\n",
			render.InfraName, render.InfraExtNetwork, selfIPs.External, render.InfraIntNetwork, selfIPs.Internal, render.InfraListenerPool)
	} else {
		fmt.Fprintf(os.Stderr, "[phase 23b] applying Infra %s: %s (selfip=%s), listener pool %s\n",
			render.InfraName, render.InfraExtNetwork, selfIPs.External, render.InfraListenerPool)
	}
	if err := retryWhileWebhookUnavailable(ctx, webhookRetryWait, func() error { return applyRawYAML(ctx, clients, infraRendered) }); err != nil {
		return fmt.Errorf("phase23b: applying Infra: %w", err)
	}
	st.Set("INFRA_APPLIED_AT", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "[phase 23b] waiting for Infra %s Programmed=True (up to %s)\n", render.InfraName, infraProgrammedWait)
	if err := waitForConditionTrue(ctx, clients.Dynamic, infraGVR, InstanceNamespace, render.InfraName, "Programmed", infraProgrammedWait); err != nil {
		return fmt.Errorf("phase23b: Infra %s did not reach Programmed=True: %w", render.InfraName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] Infra %s Programmed=True\n", render.InfraName)

	// Put the self IPs FIC allocated on the ENIs (AWS only delivers traffic
	// for addresses the ENI owns, F5 Multi-AZ PDF p.9) and record them: the
	// egress-snat SNAT check, the BGP how-to and the topology diagram need the
	// address TMM really got, not the nominal value phase 17 stored.
	if err := recordAllocatedSelfIP(ctx, clients, st, "TMM_EXT_SELFIP", "EXTERNAL_ENI", render.InfraExtNetwork); err != nil {
		return fmt.Errorf("phase23b: %w", err)
	}
	if hasInternal {
		if err := recordAllocatedSelfIP(ctx, clients, st, "TMM_INT_SELFIP", "INTERNAL_ENI", render.InfraIntNetwork); err != nil {
			return fmt.Errorf("phase23b: %w", err)
		}
	}

	return st.Save()
}

// Phase23bSPKVlanGatewayClassDown deletes the Infra CR (and any 2.3.x F5SPKVlan CRs)
// and the cluster-scoped GatewayClass. Tolerates NotFound everywhere.
// Skipped silently when the cluster is not a BNK pattern. Deleting a
// non-existent int-vlan (single-interface clusters) is tolerated via NotFound.
func Phase23bSPKVlanGatewayClassDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	checkAuthOrDie(clients)
	if !cl.IsBNKPattern() {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[phase 23b down] Infra + GatewayClass: cluster=%s\n", cl.Metadata.Name)

	if clients.Dynamic == nil {
		fmt.Fprintln(os.Stderr, "[phase 23b down] warning: dynamic client unavailable, skipping CR deletes")
		clearPhase23bState(st)
		return st.Save()
	}

	// Delete the Infra CR (BNK 2.4).
	if err := clients.Dynamic.Resource(infraGVR).Namespace(InstanceNamespace).Delete(ctx, render.InfraName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "[phase 23b down] warning: delete Infra %s: %v\n", render.InfraName, err)
	} else if err == nil {
		fmt.Fprintf(os.Stderr, "[phase 23b down] deleted Infra %s\n", render.InfraName)
	}

	// Delete 2.3.x F5SPKVlan CRs if an older awsbnkctl created them.
	for _, vlan := range []string{"ext-vlan", "int-vlan"} {
		err := clients.Dynamic.Resource(f5spkvlanGVR).Namespace(InstanceNamespace).Delete(ctx, vlan, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "[phase 23b down] warning: delete F5SPKVlan %s: %v\n", vlan, err)
		} else if err == nil {
			fmt.Fprintf(os.Stderr, "[phase 23b down] deleted F5SPKVlan %s\n", vlan)
		}
	}

	// Delete the cne-controller RBAC supplement (cluster-scoped).
	rbacName := render.CNEControllerRBACName(cl)
	for _, gvr := range []schema.GroupVersionResource{clusterRoleBindingGVR, clusterRoleGVR} {
		if err := clients.Dynamic.Resource(gvr).Delete(ctx, rbacName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "[phase 23b down] warning: delete %s %s: %v\n", gvr.Resource, rbacName, err)
		} else if err == nil {
			fmt.Fprintf(os.Stderr, "[phase 23b down] deleted %s %s\n", gvr.Resource, rbacName)
		}
	}

	// Delete GatewayClass (cluster-scoped).
	gwcName := cl.Metadata.Name + "-gatewayclass"
	err := clients.Dynamic.Resource(gatewayClassGVR).Delete(ctx, gwcName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "[phase 23b down] warning: delete GatewayClass %s: %v\n", gwcName, err)
	} else if err == nil {
		fmt.Fprintf(os.Stderr, "[phase 23b down] deleted GatewayClass %s\n", gwcName)
	}

	clearPhase23bState(st)
	return st.Save()
}

func clearPhase23bState(st *state.State) {
	for _, k := range []string{"INFRA_APPLIED_AT", "F5SPKVLAN_APPLIED_AT", "GATEWAYCLASS_NAME"} {
		st.Set(k, "")
	}
}

// infraVlanIPAMName is the IPAM CR the cne-controller creates for an Infra
// VLAN network: vlan-<namespace>-<network>.<infra> (seen live on BNK 2.4.0).
func infraVlanIPAMName(ns, network string) string {
	return fmt.Sprintf("vlan-%s-%s.%s", ns, network, render.InfraName)
}

// allocatedSelfIP returns the address FIC allocated for an Infra VLAN network,
// or "" when the IPAM CR has no allocation yet.
func allocatedSelfIP(ctx context.Context, dyn dynamic.Interface, ns, network string) (string, error) {
	obj, err := dyn.Resource(ipamGVR).Namespace(ns).Get(ctx, infraVlanIPAMName(ns, network), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	entries, _, _ := unstructured.NestedSlice(obj.Object, "status", "IPStatus")
	for _, e := range entries {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if ip, _ := m["ip"].(string); ip != "" {
			return ip, nil
		}
	}
	return "", nil
}

// recordAllocatedSelfIP puts the FIC-allocated self IP of an Infra VLAN
// network on the ENI named by eniKey in state and writes it to state under
// key. A missing allocation is a warning, not an error (the nominal value phase
// 17 stored stays); a failed ENI assignment is an error because TMM would own
// an address AWS never delivers to.
func recordAllocatedSelfIP(ctx context.Context, clients *Clients, st *state.State, key, eniKey, network string) error {
	ipamName := infraVlanIPAMName(InstanceNamespace, network)
	ip, err := allocatedSelfIP(ctx, clients.Dynamic, InstanceNamespace, network)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[phase 23b] warning: reading IPAM %s: %v (keeping %s=%s)\n", ipamName, err, key, st.Get(key))
		return nil
	}
	if ip == "" {
		fmt.Fprintf(os.Stderr, "[phase 23b] warning: IPAM %s has no allocated address yet (keeping %s=%s)\n", ipamName, key, st.Get(key))
		return nil
	}
	fmt.Fprintf(os.Stderr, "[phase 23b] %s self IP allocated by IPAM: %s=%s\n", network, key, ip)
	if eniID := st.Get(eniKey); eniID != "" && clients.EC2 != nil {
		if err := assignSelfIPIfNeeded(ctx, clients.EC2, eniID, ip, "[phase 23b]"); err != nil {
			return fmt.Errorf("assigning allocated self IP %s to %s (%s): %w", ip, eniID, eniKey, err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[phase 23b] warning: %s not in state or no EC2 client; %s not assigned to an ENI\n", eniKey, ip)
	}
	st.Set(key, ip)
	return nil
}
