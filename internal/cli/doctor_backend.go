package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/dynamic"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/phases"
	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/doctor"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s/bnkscan"
	"github.com/JLCode-tech/awsbnkctl/internal/remote"
	"github.com/JLCode-tech/awsbnkctl/internal/test"
)

// runBackendChecks dispatches to the per-backend doctor probes. `spec` is one of:
//
//	k8s              → cluster reachable, BNK readiness, the bnk heal detections
//	ssh:<target>     → target resolves, ssh connects, sudo / PATH readiness
//
// Each probe returns one or more doctor.Check entries with BackendName
// set (so PrintResults could later split them out per backend); the
// rendering is unchanged today.
func runBackendChecks(ctx context.Context, cctx *config.Context, spec string) []doctor.Check {
	switch {
	case spec == "k8s":
		return runK8sBackendChecks(ctx)
	case strings.HasPrefix(spec, "ssh:"):
		target := strings.TrimPrefix(spec, "ssh:")
		return runSSHBackendChecks(ctx, cctx, target)
	default:
		return []doctor.Check{{
			Name:     "doctor backend " + spec,
			Status:   doctor.StatusError,
			Detail:   fmt.Sprintf("unsupported --backend value %q (want k8s | ssh:<target>)", spec),
			Optional: false,
		}}
	}
}

// runK8sBackendChecks probes the k8s execution backend's prerequisites.
//
//   - apiserver reachable (clientset construction succeeds)
//   - BNK API generation, Infra Programmed, Gateway Accepted+Programmed,
//     f5-cne-controller rollout (bnkscan), with a warning and
//     `awsbnkctl bnk migrate-2.4` hint when 2.3 CRDs or CRs remain
//   - every bnk heal repair Detect as a row (phases.DetectAll), including
//     the awsbnkctl-test namespace that test --backend k8s Jobs run in
func runK8sBackendChecks(ctx context.Context) []doctor.Check {
	out := []doctor.Check{}
	add := func(name string, status doctor.CheckStatus, detail string) {
		out = append(out, doctor.Check{
			Name:        name,
			Status:      status,
			Detail:      detail,
			BackendName: "k8s",
		})
	}

	cs, err := k8s.BuildClientset("")
	if err != nil {
		add("k8s cluster reachable", doctor.StatusError, err.Error())
		return out
	}
	restCfg, restErr := k8s.BuildRESTConfig("")
	if restErr != nil {
		// Non-fatal; the env-runtime probe degrades to skip.
		restCfg = nil
	}
	add("k8s cluster reachable", doctor.StatusOK, "kubeconfig loaded")

	// BNK readiness from the shared bnkscan index: API generation, Infra,
	// Gateways, controller, and a migrate-2.4 hint when 2.3 objects remain.
	idx, scanErr := scanBNK(ctx, "", bnkscan.DefaultControllerNamespace, statusScanTimeout)
	out = append(out, bnkDoctorChecks(idx, scanErr)...)
	healClients := &phases.Clients{K8s: cs}
	if restCfg != nil {
		if dyn, derr := dynamic.NewForConfig(restCfg); derr == nil {
			healClients.Dynamic = dyn
		}
	}
	out = append(out, healDoctorChecks(ctx, healClients)...)

	return out
}

// runSSHBackendChecks probes the SSH backend's prerequisites for the
// named target.
//
//   - target resolves in the workspace config
//   - ssh connect succeeds
//   - sudo -n true succeeds (for the apt bootstrap path)
//   - if a tool name is implied, command -v finds it on PATH
func runSSHBackendChecks(ctx context.Context, cctx *config.Context, name string) []doctor.Check {
	out := []doctor.Check{}
	add := func(rowName string, status doctor.CheckStatus, detail string) {
		out = append(out, doctor.Check{
			Name:        rowName,
			Status:      status,
			Detail:      detail,
			BackendName: "ssh",
		})
	}

	if cctx == nil || cctx.Workspace == nil {
		add("ssh:"+name+" target", doctor.StatusError, "no workspace context")
		return out
	}

	t, err := remote.LoadTarget(cctx.WorkspaceName, name)
	if err != nil {
		add("ssh:"+name+" target", doctor.StatusError, err.Error())
		return out
	}
	signer, err := remote.ResolveSigner(t, nil)
	if err != nil {
		add("ssh:"+name+" target", doctor.StatusError, "key: "+err.Error())
		return out
	}
	t.Signer = signer
	t.HostKeyCallback = remote.HostKeyCallback(remote.HostKeyOptions{Insecure: flagInsecureHostKey})
	add("ssh:"+name+" target", doctor.StatusOK, fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port))

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := remote.Connect(probeCtx, t)
	if err != nil {
		add("ssh:"+name+" connect", doctor.StatusError, err.Error())
		return out
	}
	defer client.Close()
	add("ssh:"+name+" connect", doctor.StatusOK, "tcp + handshake OK")

	// sudo -n true → exit 0 ⇒ passwordless sudo configured.
	rc, _ := client.Run(probeCtx, []string{"sudo", "-n", "true"}, remote.RunOpts{})
	if rc == 0 {
		add("ssh:"+name+" sudo", doctor.StatusOK, "passwordless (apt bootstrap feasible)")
	} else {
		add("ssh:"+name+" sudo", doctor.StatusWarning, fmt.Sprintf("sudo -n true rc=%d — bootstrap will fail; pre-install tools or configure NOPASSWD", rc))
	}

	return out
}

// runDNSProbeCheck runs the embedded miekg/dns probe against the
// workspace's configured default DNS target. Returns (Check, true)
// when a probe was attempted; (zero, false) when there's no
// default_target configured so the doctor output stays compact.
//
// The probe library is built into the binary (no external `dig` install
// required), so this is mostly an informational latency measurement;
// an actual failure would surface a real DNS infrastructure problem worth flagging.
func runDNSProbeCheck(ctx context.Context, cctx *config.Context) (doctor.Check, bool) {
	if cctx == nil || cctx.Workspace == nil {
		return doctor.Check{}, false
	}
	target := cctx.Workspace.Test.DNS.DefaultTarget
	if target == "" {
		return doctor.Check{}, false
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p := &test.Probe{
		Target:     target,
		Type:       1, // dns.TypeA — inlined to avoid pulling miekg/dns into the cli package directly
		Server:     "system",
		Iterations: 1,
		Timeout:    2 * time.Second,
		Backend:    "local",
	}
	res, err := p.Run(probeCtx)
	c := doctor.Check{Name: "dns probe (" + target + ")"}
	if err != nil {
		c.Status = doctor.StatusError
		c.Detail = err.Error()
		return c, true
	}
	if res.Err != "" {
		c.Status = doctor.StatusError
		c.Detail = fmt.Sprintf("%s: %s", res.Rcode, res.Err)
		return c, true
	}
	if len(res.Answers) == 0 {
		c.Status = doctor.StatusWarning
		c.Detail = fmt.Sprintf("no answers (rcode=%s, server=%s)", res.Rcode, res.Server)
		return c, true
	}
	c.Status = doctor.StatusOK
	c.Detail = fmt.Sprintf("%d answer(s) in %.1fms (server=%s)", len(res.Answers), res.RTTMs.P50, res.Server)
	return c, true
}
