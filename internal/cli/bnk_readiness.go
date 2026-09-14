package cli

// bnk_readiness.go — the one place the CLI reads a cluster's BNK state.
// status, doctor, logs --governance, targets scan, benchmark setup
// --auto-discover and forge scan all go through these helpers so they share
// bnkscan's index, readiness verdict and MCP discovery.

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/JLCode-tech/awsbnkctl/internal/doctor"
	"github.com/JLCode-tech/awsbnkctl/internal/forge"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s/bnkscan"
)

// scanClients builds the dynamic + typed clients bnkscan needs. Tests replace it.
var scanClients = func(kubeconfigPath string) (dynamic.Interface, kubernetes.Interface, error) {
	cfg, err := k8s.BuildRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("dynamic client: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("clientset: %w", err)
	}
	return dyn, cs, nil
}

// statusScanTimeout bounds the best-effort BNK scan in status and doctor.
const statusScanTimeout = 20 * time.Second

// scanBNK indexes the cluster behind kubeconfigPath within timeout.
func scanBNK(ctx context.Context, kubeconfigPath, controllerNS string, timeout time.Duration) (*bnkscan.Index, error) {
	dyn, cs, err := scanClients(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	sctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return bnkscan.Scan(sctx, dyn, cs, bnkscan.Options{ControllerNamespace: controllerNS})
}

// writeStatusBNK prints the BNK block of `awsbnkctl status`. Best-effort like
// every other status section: a scan error is one line, not a failure.
func writeStatusBNK(w io.Writer, idx *bnkscan.Index, scanErr error) {
	if scanErr != nil {
		fmt.Fprintf(w, "BNK API:\t(scan failed: %v)\n", scanErr)
		return
	}
	rd := idx.Readiness()
	gen := string(rd.Generation)
	if len(idx.Groups) > 0 {
		gen += " (CRD groups: " + strings.Join(idx.Groups, ", ") + ")"
	}
	fmt.Fprintf(w, "BNK API:\t%s\n", gen)
	if rd.Generation == bnkscan.GenNone {
		fmt.Fprintln(w, "BNK readiness:\tnot installed")
		return
	}
	if rd.Generation == bnkscan.Gen24 || rd.Generation == bnkscan.GenMixed {
		fmt.Fprintf(w, "Infra:\t%s Programmed\n", rd.Infras)
	}
	fmt.Fprintf(w, "Gateways:\t%s Accepted+Programmed\n", rd.Gateways)
	if rd.Controller.Found {
		fmt.Fprintf(w, "CNE controller:\t%s/%s %d/%d available\n", rd.Controller.Namespace, rd.Controller.Name, rd.Controller.Available, rd.Controller.Desired)
	} else {
		fmt.Fprintf(w, "CNE controller:\t%s/%s not found\n", rd.Controller.Namespace, rd.Controller.Name)
	}
	if len(idx.MCP) > 0 {
		fmt.Fprintf(w, "MCP endpoints:\t%d\n", len(idx.MCP))
	}
	if rd.Ready {
		fmt.Fprintln(w, "BNK readiness:\tready")
	} else {
		fmt.Fprintf(w, "BNK readiness:\tnot ready — %s\n", strings.Join(rd.Problems, "; "))
	}
	if advice := rd.MigrationAdvice(); advice != "" {
		fmt.Fprintf(w, "BNK migration:\t%s\n", advice)
	}
}

// bnkDoctorChecks renders the readiness verdict as doctor rows. Readiness
// failures are errors; a 2.3 or mixed API generation and leftover 2.3 objects
// are warnings that point at bnk upgrade / bnk migrate-2.4.
func bnkDoctorChecks(idx *bnkscan.Index, scanErr error) []doctor.Check {
	row := func(name string, status doctor.CheckStatus, detail string) doctor.Check {
		return doctor.Check{Name: name, Status: status, Detail: detail, BackendName: "k8s"}
	}
	if scanErr != nil {
		return []doctor.Check{row("bnk api generation", doctor.StatusWarning, "scan failed: "+scanErr.Error())}
	}
	rd := idx.Readiness()
	var out []doctor.Check
	groups := strings.Join(idx.Groups, ", ")
	switch rd.Generation {
	case bnkscan.Gen24:
		out = append(out, row("bnk api generation", doctor.StatusOK, "2.4 ("+groups+")"))
	case bnkscan.GenMixed:
		out = append(out, row("bnk api generation", doctor.StatusWarning, "mixed: "+rd.MigrationAdvice()))
	case bnkscan.Gen23:
		out = append(out, row("bnk api generation", doctor.StatusWarning, "2.3 ("+bnkscan.LegacyPolicyGroup+"); run `awsbnkctl bnk upgrade`"))
	default:
		out = append(out, row("bnk api generation", doctor.StatusWarning, "none — BNK CRDs not served (not installed, or `awsbnkctl up` has not reached phase 22)"))
		return out
	}
	if rd.Generation == bnkscan.Gen24 || rd.Generation == bnkscan.GenMixed {
		st, detail := doctor.StatusOK, rd.Infras.String()+" Programmed=True"
		if rd.Infras.Total == 0 || rd.Infras.Ready < rd.Infras.Total {
			st = doctor.StatusError
			detail += " — " + strings.Join(filterPrefix(rd.Problems, "Infra", "no Infra"), "; ")
		}
		out = append(out, row("bnk infra", st, detail))
		if rd.Generation == bnkscan.Gen24 && (rd.LegacyCRDs || rd.LegacyCRs > 0) {
			out = append(out, row("bnk legacy 2.3 objects", doctor.StatusWarning, rd.MigrationAdvice()))
		}
	}
	gwStatus, gwDetail := doctor.StatusOK, rd.Gateways.String()+" Accepted=True,Programmed=True"
	if rd.Gateways.Ready < rd.Gateways.Total {
		gwStatus = doctor.StatusError
		gwDetail += " — " + strings.Join(filterPrefix(rd.Problems, "Gateway"), "; ")
	}
	out = append(out, row("bnk gateways", gwStatus, gwDetail))
	ctrl := rd.Controller
	switch {
	case !ctrl.Found:
		out = append(out, row("bnk cne controller", doctor.StatusError, fmt.Sprintf("deployment %s/%s not found", ctrl.Namespace, ctrl.Name)))
	case !ctrl.Ready:
		out = append(out, row("bnk cne controller", doctor.StatusError, fmt.Sprintf("%s/%s %d/%d replicas available", ctrl.Namespace, ctrl.Name, ctrl.Available, ctrl.Desired)))
	default:
		out = append(out, row("bnk cne controller", doctor.StatusOK, fmt.Sprintf("%s/%s %d/%d available", ctrl.Namespace, ctrl.Name, ctrl.Available, ctrl.Desired)))
	}
	return out
}

// filterPrefix keeps the lines starting with one of the prefixes.
func filterPrefix(lines []string, prefixes ...string) []string {
	var out []string
	for _, l := range lines {
		for _, p := range prefixes {
			if strings.HasPrefix(l, p) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

// Shared flags of forge scan and targets scan. Both commands bind the same
// variables; only one runs per process.
var (
	flagForgeScanKubeconfig string
	flagForgeScanProbe      bool
	flagForgeScanBearerEnv  string
	flagForgeScanRestURL    string
	flagForgeScanUser       string
	flagForgeScanPass       string
	flagForgeScanCtrlNS     string
	flagForgeScanTimeout    time.Duration
)

// addScanFlags registers the flags every MCP discovery command takes.
func addScanFlags(f *pflag.FlagSet) {
	f.StringVar(&flagForgeScanKubeconfig, "kubeconfig", "", "explicit kubeconfig path (default: cluster.yaml state, then $KUBECONFIG / ~/.kube/config)")
	f.BoolVar(&flagForgeScanProbe, "probe", false, "call initialize + tools/list on every MCP endpoint and record its tools")
	f.StringVar(&flagForgeScanBearerEnv, "bearer-env", "", "environment variable holding the bearer token for --probe")
	f.StringVar(&flagForgeScanRestURL, "forge-rest-url", "", "Forge REST base URL for target registration (default: cluster.yaml forge.url, forge_link.json, "+intent.DefaultForgeRESTURL+")")
	f.StringVar(&flagForgeScanUser, "forge-user", "", "Forge username (default: $AWSBNKCTL_FORGE_USERNAME, cluster.yaml forge.username, admin)")
	f.StringVar(&flagForgeScanPass, "forge-pass", "", "Forge password (default: $AWSBNKCTL_FORGE_PASSWORD, cluster.yaml forge.password)")
	f.StringVar(&flagForgeScanCtrlNS, "controller-namespace", bnkscan.DefaultControllerNamespace, "namespace of the f5-cne-controller Deployment")
	f.DurationVar(&flagForgeScanTimeout, "timeout", 2*time.Minute, "bound for the whole scan")
}

// forgeScanTarget is one registration outcome, in text and -o json output.
type forgeScanTarget struct {
	Endpoint string `json:"endpoint"`
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Error    string `json:"error,omitempty"`
}

// discoverMCP loads the optional cluster.yaml, resolves the kubeconfig and
// indexes the cluster. With probe, every endpoint's tool catalogue is fetched;
// the per-endpoint probe errors come back in the map.
func discoverMCP(ctx context.Context, configPath string, probe bool) (*intent.Cluster, *bnkscan.Index, map[string]string, error) {
	var cl *intent.Cluster
	if configPath != "" {
		var err error
		if cl, err = intent.Load(configPath); err != nil {
			return nil, nil, nil, fmt.Errorf("loading --config: %w", err)
		}
	}
	kubeconfigPath, err := resolveKubeconfigFlags(flagForgeScanKubeconfig, configPath)
	if err != nil {
		return nil, nil, nil, err
	}
	dyn, cs, err := scanClients(kubeconfigPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("building kube client: %w", err)
	}
	idx, err := bnkscan.Scan(ctx, dyn, cs, bnkscan.Options{ControllerNamespace: flagForgeScanCtrlNS})
	if err != nil {
		return cl, nil, nil, err
	}
	var probes map[string]string
	if probe {
		probes = probeEndpoints(ctx, idx)
	}
	return cl, idx, probes, nil
}

// registerDiscoveredTargets writes every MCP endpoint into Forge's Target
// Catalog, idempotently (RegisterBenchmarkTarget reuses a target by name).
// restURL and creds empty fall back to the cluster.yaml / link / env defaults.
func registerDiscoveredTargets(ctx context.Context, cl *intent.Cluster, link *forge.Link, restURL string, creds forge.RestCreds, eps []bnkscan.MCPEndpoint) []forgeScanTarget {
	if restURL == "" {
		restURL = resolveForgeScanRestURL(cl, link)
	}
	if creds.Username == "" && creds.Password == "" {
		creds = resolveForgeScanCreds(cl)
	}
	opts := forge.MCPTargetOptions{
		RestURL:        restURL,
		Creds:          creds,
		ClusterID:      link.ClusterID,
		ClusterName:    link.ClusterName,
		ProxyNamespace: flagForgeScanCtrlNS,
	}
	var out []forgeScanTarget
	for _, r := range forge.RegisterMCPTargets(ctx, opts, eps) {
		t := forgeScanTarget{Endpoint: r.Endpoint.Name(), ID: r.Target.ID, Name: r.Target.Name}
		if r.Err != nil {
			t.Error = r.Err.Error()
		}
		out = append(out, t)
	}
	return out
}

// writeTargetResults prints the registration outcomes.
func writeTargetResults(w io.Writer, targets []forgeScanTarget) {
	if len(targets) == 0 {
		return
	}
	fmt.Fprintln(w, "forge targets")
	for _, t := range targets {
		if t.Error != "" {
			fmt.Fprintf(w, "  %-40s FAILED %s\n", t.Endpoint, t.Error)
		} else {
			fmt.Fprintf(w, "  %-40s id=%d\n", t.Name, t.ID)
		}
	}
}

// warnf prints an operator warning to stderr.
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "⚠ "+format+"\n", args...)
}

// multusDoctorCheck reports whether the Multus pods still hold a valid
// kubeconfig token (see k8s.HealMultusToken). Detect only: doctor never
// changes the cluster.
func multusDoctorCheck(ctx context.Context, cs kubernetes.Interface) doctor.Check {
	c := doctor.Check{Name: "multus kubeconfig token", BackendName: "k8s"}
	installed, stale, reason, err := k8s.MultusTokenStale(ctx, cs, 0, time.Now())
	switch {
	case err != nil:
		c.Status, c.Detail = doctor.StatusWarning, "could not inspect Multus: "+err.Error()
	case !installed:
		c.Status, c.Detail = doctor.StatusOK, "Multus not installed"
	case stale:
		c.Status, c.Detail = doctor.StatusWarning, reason+" — `awsbnkctl bnk upgrade` and `awsbnkctl up` restart it; or `kubectl -n kube-system rollout restart ds kube-multus-ds`"
	default:
		c.Status, c.Detail = doctor.StatusOK, "pods younger than "+k8s.MultusMaxPodAge.String()+", no Unauthorized sandbox events"
	}
	return c
}

// cneIRSADoctorCheck reports whether the ServiceAccount the f5-cne-controller
// Deployment runs as carries the IRSA role annotation. Without it the 2.4
// controller starts with no cloud provider and cannot allocate Gateway VIPs
// on AWS (seen live after a 2.3.0 -> 2.4 upgrade, where the SA name changed).
func cneIRSADoctorCheck(ctx context.Context, cs kubernetes.Interface, ns string) doctor.Check {
	c := doctor.Check{Name: "bnk controller IRSA", BackendName: "k8s"}
	dep, err := cs.AppsV1().Deployments(ns).Get(ctx, bnkscan.DefaultControllerName, metav1.GetOptions{})
	if err != nil {
		c.Status, c.Detail = doctor.StatusOK, fmt.Sprintf("deployment %s/%s not found; skipped", ns, bnkscan.DefaultControllerName)
		return c
	}
	saName := dep.Spec.Template.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	sa, err := cs.CoreV1().ServiceAccounts(ns).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		c.Status, c.Detail = doctor.StatusWarning, fmt.Sprintf("serviceaccount %s/%s: %v", ns, saName, err)
		return c
	}
	if arn := sa.Annotations["eks.amazonaws.com/role-arn"]; arn != "" {
		c.Status, c.Detail = doctor.StatusOK, saName+" → "+arn
		return c
	}
	c.Status, c.Detail = doctor.StatusWarning, fmt.Sprintf("serviceaccount %s/%s has no eks.amazonaws.com/role-arn; the controller runs without a cloud provider — `awsbnkctl bnk upgrade` re-binds it (phase 21)", ns, saName)
	return c
}
