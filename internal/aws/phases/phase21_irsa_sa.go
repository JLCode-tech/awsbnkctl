package phases

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

const (
	phase21FieldMgr = "awsbnkctl-phase21"

	// irsaRoleARNAnnotation is the annotation the EKS pod-identity webhook reads.
	irsaRoleARNAnnotation = "eks.amazonaws.com/role-arn"
	// irsaInjectedEnv is set on every container of a pod the webhook mutated.
	irsaInjectedEnv = "AWS_ROLE_ARN"

	cneDeployWaitTimeout = 3 * time.Minute
)

// Phase21IRSASA binds the IRSA role to the ServiceAccount the CNE controller
// Deployment actually runs as. It runs after Phase 22 (CNEInstance) because
// FLO creates that SA and Deployment together and the name is release-specific
// (f5-cne-controller-<instance>-serviceaccount on older 2.3.x, plain
// f5-cne-controller on 2.3.3), so it is discovered, never derived:
//
//  1. Wait for the controller Deployment; read spec.template.spec.serviceAccountName.
//  2. Scope the role trust policy from Phase 18's namespace wildcard to that SA.
//  3. Annotate the SA with eks.amazonaws.com/role-arn.
//  4. The pods predate the annotation, so if none carries AWS_ROLE_ARN,
//     rollout-restart the Deployment; the webhook injects on the new pod.
//
// Without this the controller logs "IRSA env not found" / "Cloud prerequisites
// not met" and never assigns Gateway VIPs to the TMM ENI (bnk-staging-test,
// 2026-09-11). Idempotent. D-005: CheckAuthOrDie at entry.
func Phase21IRSASA(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	fmt.Fprintf(os.Stderr, "[phase 21] IRSA SA: cluster=%s\n", cl.Metadata.Name)

	roleARN := st.Get("CNE_IRSA_ROLE_ARN")
	if roleARN == "" {
		return fmt.Errorf("phase21: CNE_IRSA_ROLE_ARN not in state — Phase 18 (IRSA/OIDC) must run first")
	}

	if dryRun {
		fmt.Fprintf(os.Stderr,
			"[phase 21] dry-run: would read the SA of deploy %s/%s, scope the IRSA trust policy to it, annotate it, and restart the controller if its pods lack IRSA credentials\n",
			InstanceNamespace, h4DeploymentName)
		st.Set("IRSA_SA_APPLIED_AT", "dry-run")
		return nil
	}
	if clients.K8s == nil {
		return fmt.Errorf("phase21: Clients.K8s is nil — call clients.AttachK8s(kubeconfigPath) first")
	}

	// 1. Discover the SA from the live Deployment.
	deploy, err := waitForDeployment(ctx, clients, InstanceNamespace, h4DeploymentName, cneDeployWaitTimeout)
	if err != nil {
		return fmt.Errorf("phase21: %w", err)
	}
	saName := deploy.Spec.Template.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	fmt.Fprintf(os.Stderr, "[phase 21] deploy %s/%s runs as ServiceAccount %s\n", InstanceNamespace, h4DeploymentName, saName)

	// 2. Trust exactly that SA.
	oidcHost := strings.TrimPrefix(st.Get("EKS_OIDC_URL"), "https://")
	roleName := st.Get("CNE_IRSA_ROLE_NAME")
	if oidcHost == "" || roleName == "" {
		return fmt.Errorf("phase21: EKS_OIDC_URL / CNE_IRSA_ROLE_NAME not in state — Phase 18 (IRSA/OIDC) must run first")
	}
	trust, err := oidcFederatedTrustPolicy(oidcHost, extractAccountID(roleARN), irsaSubject(InstanceNamespace, saName))
	if err != nil {
		return fmt.Errorf("phase21: building trust policy: %w", err)
	}
	if _, err := clients.IAM.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName: ptr(roleName), PolicyDocument: ptr(trust),
	}); err != nil {
		return fmt.Errorf("phase21: iam:UpdateAssumeRolePolicy %s: %w", roleName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 21] trust policy of %s scoped to %s\n", roleName, irsaSubject(InstanceNamespace, saName))

	// 3. Annotate the SA (JSON merge patch: idempotent, keeps FLO's fields).
	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, irsaRoleARNAnnotation, roleARN)
	if _, err := clients.K8s.CoreV1().ServiceAccounts(InstanceNamespace).Patch(ctx, saName,
		types.MergePatchType, []byte(patch), metav1.PatchOptions{FieldManager: phase21FieldMgr}); err != nil {
		return fmt.Errorf("phase21: annotating SA %s/%s: %w", InstanceNamespace, saName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 21] annotated SA %s/%s with %s\n", InstanceNamespace, saName, irsaRoleARNAnnotation)

	// 4. Restart only if the running pods missed the injection.
	injected, err := deploymentPodsHaveEnv(ctx, clients, deploy, irsaInjectedEnv)
	if err != nil {
		return fmt.Errorf("phase21: %w", err)
	}
	if injected {
		fmt.Fprintln(os.Stderr, "[phase 21] controller pods already carry IRSA credentials — no restart")
	} else {
		if err := restartDeployment(ctx, clients, InstanceNamespace, h4DeploymentName); err != nil {
			return fmt.Errorf("phase21: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[phase 21] rollout-restarted deploy %s/%s so the pod-identity webhook injects IRSA\n", InstanceNamespace, h4DeploymentName)
	}

	st.Set("IRSA_SA_APPLIED_AT", time.Now().UTC().Format(time.RFC3339))
	st.Set("CNE_SA_NAME", saName)
	return st.Save()
}

// Phase21IRSASADown clears phase 21 state. The ServiceAccount belongs to FLO
// and leaves with its namespace in Phase 12 down; the IRSA role goes in Phase 18 down.
func Phase21IRSASADown(_ context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	checkAuthOrDie(clients)
	fmt.Fprintf(os.Stderr, "[phase 21 down] IRSA SA: cluster=%s (SA is FLO-owned; clearing state only)\n", cl.Metadata.Name)
	for _, k := range []string{"IRSA_SA_APPLIED_AT", "CNE_SA_NAME"} {
		st.Set(k, "")
	}
	return st.Save()
}
