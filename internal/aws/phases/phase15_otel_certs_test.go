//lint:file-ignore SA1019 k8sfake.NewSimpleClientset is still functional — NewClientset requires --with-applyconfig which adds significant test-codegen complexity

package phases

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	k8swait "github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

// buildP15Clients builds a Clients struct seeded with both OTEL certs in Ready state.
func buildP15ReadyClients(t *testing.T) *Clients {
	t.Helper()
	cs := k8sfake.NewSimpleClientset()
	scheme := buildScheme()

	otelSvr := buildReadyCertificate(otelSvrCertName, operatorNS)
	otelF5Ing := buildReadyCertificate(otelF5IngCertName, operatorNS)

	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		k8swait.CertificateGVR: "CertificateList",
	}, otelSvr, otelF5Ing)

	return &Clients{K8s: cs, Dynamic: dyn, Profile: "test"}
}

// ─── Test 1: Dry-run ─────────────────────────────────────────────────────────

func TestPhase15_DryRun_ReturnsNil(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	clients := &Clients{Profile: "test"}

	if err := Phase15OTELCerts(context.Background(), cl, st, clients, true); err != nil {
		t.Fatalf("Phase15OTELCerts dry-run: %v", err)
	}
	if st.Get("OTEL_SVR_CERT_NAME") != "dry-run" {
		t.Errorf("OTEL_SVR_CERT_NAME = %q, want dry-run", st.Get("OTEL_SVR_CERT_NAME"))
	}
	if st.Get("OTEL_F5ING_CERT_NAME") != "dry-run" {
		t.Errorf("OTEL_F5ING_CERT_NAME = %q, want dry-run", st.Get("OTEL_F5ING_CERT_NAME"))
	}
}

// ─── Test 2: Wait for both certs succeeds (both certs Ready) ────────────────
// Note: Phase15OTELCerts calls applyRawYAML via SSA which the dynamic fake
// does not support. We test the waiter sub-step directly (same approach as
// phase12_k8s_foundation_test.go tests that avoid calling the full phase).

func TestPhase15_WaitBothCertsReady_Succeeds(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	clients := buildP15ReadyClients(t)

	// Test the wait step directly — both certs are pre-seeded as Ready.
	if err := waitForOTELCerts(context.Background(), cl, clients, st); err != nil {
		t.Fatalf("waitForOTELCerts: %v", err)
	}
	if st.Get("OTEL_SVR_CERT_NAME") != otelSvrCertName {
		t.Errorf("OTEL_SVR_CERT_NAME = %q, want %q", st.Get("OTEL_SVR_CERT_NAME"), otelSvrCertName)
	}
	if st.Get("OTEL_F5ING_CERT_NAME") != otelF5IngCertName {
		t.Errorf("OTEL_F5ING_CERT_NAME = %q, want %q", st.Get("OTEL_F5ING_CERT_NAME"), otelF5IngCertName)
	}
}

// ─── Test 3: Idempotent re-apply (state already set) ────────────────────────

func TestPhase15_IdempotentReApply(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	clients := buildP15ReadyClients(t)

	// Run waitForOTELCerts twice — should succeed both times.
	for i := 0; i < 2; i++ {
		if err := waitForOTELCerts(context.Background(), cl, clients, st); err != nil {
			t.Fatalf("waitForOTELCerts run %d: %v", i+1, err)
		}
	}
}

// ─── Test 4: Both certs must be Ready before returning nil ──────────────────

func TestPhase15_NotReadyCert_ReturnsError(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)

	// otelSvr is not ready → waitForOTELCerts should fail.
	cs := k8sfake.NewSimpleClientset()
	scheme := buildScheme()
	otelSvr := buildNotReadyCertificate(otelSvrCertName, operatorNS)
	otelF5Ing := buildReadyCertificate(otelF5IngCertName, operatorNS)
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		k8swait.CertificateGVR: "CertificateList",
	}, otelSvr, otelF5Ing)
	clients := &Clients{K8s: cs, Dynamic: dyn, Profile: "test"}

	// Use a cancelled context so the wait times out immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForOTELCerts(ctx, cl, clients, st)
	if err == nil {
		t.Fatal("expected error when cert not ready, got nil")
	}
	if !strings.Contains(err.Error(), otelSvrCertName) {
		t.Errorf("error should name the cert %q: %v", otelSvrCertName, err)
	}
}

// ─── Test 5: Down deletes certs ─────────────────────────────────────────────

func TestPhase15_Down_DeletesCerts(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	clients := buildP15ReadyClients(t)

	if err := Phase15OTELCertsDown(context.Background(), cl, st, clients); err != nil {
		t.Fatalf("Phase15OTELCertsDown: %v", err)
	}

	// Certs should be gone from the fake dynamic client.
	certGVR := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
	for _, certName := range []string{otelSvrCertName, otelF5IngCertName} {
		_, err := clients.Dynamic.Resource(certGVR).Namespace(operatorNS).Get(context.Background(), certName, metav1.GetOptions{})
		if err == nil {
			t.Errorf("Certificate %s/%s should be deleted but still exists", operatorNS, certName)
		}
	}
	// State keys cleared.
	if st.Get("OTEL_SVR_CERT_NAME") != "" {
		t.Errorf("OTEL_SVR_CERT_NAME should be cleared after down")
	}
}

// ─── Test 6: Down tolerates NotFound ────────────────────────────────────────

func TestPhase15_Down_ToleratesNotFound(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	dir := t.TempDir()
	st, _ := state.Load(dir)

	// Empty dynamic client — certs don't exist.
	cs := k8sfake.NewSimpleClientset()
	scheme := buildScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		k8swait.CertificateGVR: "CertificateList",
	})
	clients := &Clients{K8s: cs, Dynamic: dyn, Profile: "test"}

	// Should not return error.
	if err := Phase15OTELCertsDown(context.Background(), cl, st, clients); err != nil {
		t.Fatalf("Phase15OTELCertsDown NotFound: %v", err)
	}
}

// ─── Test 7: FLO disabled → skip ────────────────────────────────────────────

func TestPhase15_FLODisabled_Skips(t *testing.T) {
	awsmw.ResetForTest()
	cl := sydTracerCluster()
	disabled := false
	cl.Addons = &intent.AddonsSpec{
		Flo: &intent.FloSpec{Enabled: &disabled},
	}
	dir := t.TempDir()
	st, _ := state.Load(dir)
	clients := &Clients{Profile: "test"}

	if err := Phase15OTELCerts(context.Background(), cl, st, clients, false); err != nil {
		t.Fatalf("Phase15OTELCerts disabled: %v", err)
	}
	if st.Get("OTEL_SVR_CERT_NAME") != "" {
		t.Errorf("OTEL_SVR_CERT_NAME should be empty when FLO disabled, got %q", st.Get("OTEL_SVR_CERT_NAME"))
	}
}

// ensure unused unstructured import is consumed
var _ = &unstructured.Unstructured{}
