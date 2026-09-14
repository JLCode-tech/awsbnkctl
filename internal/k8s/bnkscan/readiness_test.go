package bnkscan

import (
	"context"
	"strings"
	"testing"
	"time"

	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const fixtureLegacy = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: {name: bnknetpolicies.gateway.k8s.f5net.com}
spec: {group: gateway.k8s.f5net.com}
---
apiVersion: k8s.f5net.com/v1
kind: F5BnkGateway
metadata: {name: web, namespace: apps}
spec: {}
`

// fixtureMixed is a 2.4 cluster whose 2.3 policy CRDs and one F5BnkGateway
// were never removed.
var fixtureMixed = fixture24 + "\n---\n" + fixtureLegacy

func TestReadiness_Programmed24(t *testing.T) {
	dyn := newFakeDynamic(t, fixture24)
	rd, idx, err := CheckReadiness(context.Background(), dyn, k8sfake.NewClientset(controllerDeployment(1)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !rd.Ready || rd.Generation != Gen24 || len(rd.Problems) != 0 {
		t.Fatalf("readiness = %+v", rd)
	}
	if rd.Infras.String() != "1/1" || rd.Gateways.String() != "1/1" {
		t.Errorf("infras %s gateways %s", rd.Infras, rd.Gateways)
	}
	if rd.LegacyCRDs || rd.LegacyCRs != 0 || rd.MigrationAdvice() != "" {
		t.Errorf("legacy: crds=%v crs=%d advice=%q", rd.LegacyCRDs, rd.LegacyCRs, rd.MigrationAdvice())
	}
	if s := rd.Summary(); !strings.Contains(s, "BNK 2.4: Infra 1/1 Programmed, Gateway 1/1 Accepted+Programmed, controller 1/1 available; ready") {
		t.Errorf("summary = %q", s)
	}
	if idx == nil || len(idx.MCP) != 1 {
		t.Errorf("index MCP = %+v", idx)
	}
}

func TestReadiness_NotReadyControllerAndSummary(t *testing.T) {
	dyn := newFakeDynamic(t, fixture24)
	rd, _, err := CheckReadiness(context.Background(), dyn, k8sfake.NewClientset(controllerDeployment(0)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rd.Ready || len(rd.Problems) != 1 || !strings.Contains(rd.Problems[0], "0/1 replicas available") {
		t.Fatalf("readiness = %+v", rd)
	}
	if !strings.Contains(rd.Summary(), "not ready (1 problem(s))") {
		t.Errorf("summary = %q", rd.Summary())
	}
}

func TestReadiness_LegacyStrictVsAccepted(t *testing.T) {
	dyn := newFakeDynamic(t, fixtureLegacy)
	cs := k8sfake.NewClientset(controllerDeployment(1))

	strict, _, err := CheckReadiness(context.Background(), dyn, cs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strict.Ready || strict.Generation != Gen23 || !strings.Contains(strings.Join(strict.Problems, "\n"), "2.3 API") {
		t.Errorf("strict = %+v", strict)
	}
	if !strings.Contains(strict.Summary(), "BNK 2.3: Gateway 0/0") || strings.Contains(strict.Summary(), "Infra") {
		t.Errorf("2.3 summary must not mention Infra: %q", strict.Summary())
	}
	if strict.MigrationAdvice() != "" {
		t.Errorf("a pure 2.3 cluster gets no migrate advice (bnk upgrade first): %q", strict.MigrationAdvice())
	}

	lenient, _, err := CheckReadiness(context.Background(), dyn, cs, Options{AcceptLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	if !lenient.Ready || lenient.Generation != Gen23 {
		t.Errorf("AcceptLegacy = %+v", lenient)
	}
}

func TestReadiness_MixedAdvisesMigrate(t *testing.T) {
	dyn := newFakeDynamic(t, fixtureMixed)
	rd, _, err := CheckReadiness(context.Background(), dyn, k8sfake.NewClientset(controllerDeployment(1)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rd.Generation != GenMixed || !rd.Ready {
		t.Fatalf("readiness = %+v", rd)
	}
	if !rd.LegacyCRDs || rd.LegacyCRs != 1 {
		t.Errorf("legacy crds=%v crs=%d", rd.LegacyCRDs, rd.LegacyCRs)
	}
	advice := rd.MigrationAdvice()
	if !strings.Contains(advice, LegacyPolicyGroup) || !strings.Contains(advice, "1 legacy CR(s)") || !strings.Contains(advice, MigrateHint) {
		t.Errorf("advice = %q", advice)
	}
}

func TestReadiness_NoBNK(t *testing.T) {
	dyn := newFakeDynamic(t, "")
	rd, _, err := CheckReadiness(context.Background(), dyn, k8sfake.NewClientset(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rd.Ready || rd.Generation != GenNone || !strings.Contains(rd.Summary(), "controller f5-cne-system/f5-cne-controller not found") {
		t.Errorf("readiness = %+v summary=%q", rd, rd.Summary())
	}
}

func TestWaitReady(t *testing.T) {
	var log strings.Builder
	dyn := newFakeDynamic(t, fixture24)
	rd, idx, err := WaitReady(context.Background(), dyn, k8sfake.NewClientset(controllerDeployment(1)), Options{}, time.Second, time.Millisecond, &log)
	if err != nil || !rd.Ready || idx == nil {
		t.Fatalf("err=%v rd=%+v", err, rd)
	}
	if !strings.Contains(log.String(), "; ready") {
		t.Errorf("log = %q", log.String())
	}

	log.Reset()
	_, _, err = WaitReady(context.Background(), dyn, k8sfake.NewClientset(controllerDeployment(0)), Options{}, 5*time.Millisecond, time.Millisecond, &log)
	if err == nil || !strings.Contains(err.Error(), "cluster not ready after") || !strings.Contains(err.Error(), "0/1 replicas available") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(log.String(), "not ready (1 problem(s)): deployment") {
		t.Errorf("log = %q", log.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := WaitReady(ctx, dyn, k8sfake.NewClientset(controllerDeployment(0)), Options{}, time.Minute, time.Minute, nil); err != context.Canceled {
		t.Errorf("cancelled wait err = %v", err)
	}
}

func TestScanMCP(t *testing.T) {
	dyn := newFakeDynamic(t, fixture24)
	eps, idx, err := ScanMCP(context.Background(), dyn, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || idx == nil || eps[0].Name() != idx.MCP[0].Name() {
		t.Errorf("eps = %+v", eps)
	}
}
