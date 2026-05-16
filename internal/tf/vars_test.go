package tf

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// AWS retarget (PRD 04 fold): the renderer reads only Workspace.AWS.
// Tests below pin the AWS-shaped output.

func TestRenderTFVars_AWS(t *testing.T) {
	ws := &config.Workspace{
		AWS: config.AWSCfg{
			Region:    "us-east-1",
			Profile:   "bnk-dev",
			VPCID:     "vpc-0123abcd",
			SubnetIDs: []string{"subnet-a", "subnet-b", "subnet-c"},
			SupplyChain: config.SupplyChainCfg{
				FARArchivePath: "/tmp/far-auth.tar.gz",
				JWTPath:        "/tmp/subscription.jwt",
				KMSKeyARN:      "arn:aws:kms:us-east-1:111122223333:key/abc",
				FLONamespace:   "flo-system",
			},
		},
		Cluster: config.ClusterCfg{Create: true, Name: "bnk-demo"},
	}
	var buf bytes.Buffer
	if err := RenderTFVars(&buf, ws, "", ""); err != nil {
		t.Fatalf("RenderTFVars: %v", err)
	}
	out := buf.String()

	want := []string{
		`region = "us-east-1"`,
		`vpc_id = "vpc-0123abcd"`,
		`subnet_ids = ["subnet-a", "subnet-b", "subnet-c"]`,
		`cluster_name = "bnk-demo"`,
		`far_auth_file_local_path = "/tmp/far-auth.tar.gz"`,
		`jwt_file_local_path = "/tmp/subscription.jwt"`,
		`kms_key_arn = "arn:aws:kms:us-east-1:111122223333:key/abc"`,
		`flo_namespace = "flo-system"`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing line %q\noutput:\n%s", w, out)
		}
	}

	// No credentials, ever.
	for _, banned := range []string{"api_key", "access_key", "secret_access_key", "AKIA"} {
		if strings.Contains(out, banned) {
			t.Errorf("forbidden token %q present in tfvars output:\n%s", banned, out)
		}
	}
}

func TestRenderTFVars_EmptyAWSBlockEmitsNoRegion(t *testing.T) {
	// A workspace whose AWS block is empty renders no `region = ...`
	// line; the renderer consults only the AWS-shaped fields.
	ws := &config.Workspace{
		AWS:     config.AWSCfg{}, // explicitly empty
		Cluster: config.ClusterCfg{Create: false, Name: "legacy"},
	}
	var buf bytes.Buffer
	if err := RenderTFVars(&buf, ws, "", ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "region =") {
		t.Errorf("expected no region line when AWS.Region is empty; got:\n%s", out)
	}
}

func TestRenderTFVars_OmitsEmptyFields(t *testing.T) {
	ws := &config.Workspace{
		Cluster: config.ClusterCfg{Create: true, Name: "demo"},
	}
	var buf bytes.Buffer
	if err := RenderTFVars(&buf, ws, "", ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Region/VPC/subnets were unset — should not appear.
	for _, k := range []string{"region", "vpc_id", "subnet_ids"} {
		// Match as token prefix; cluster_name="demo" must still render.
		if strings.Contains(out, k+" =") {
			t.Errorf("%s should be omitted when empty\noutput:\n%s", k, out)
		}
	}
}

func TestRenderTFVars_KubeconfigDir(t *testing.T) {
	ws := &config.Workspace{
		Cluster: config.ClusterCfg{Create: true, Name: "demo"},
	}
	var buf bytes.Buffer
	if err := RenderTFVars(&buf, ws, "/home/user/.awsbnkctl/default/state/kubeconfig", "/home/user/.awsbnkctl/default/state/scratch"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`kubeconfig_dir = "/home/user/.awsbnkctl/default/state/kubeconfig"`,
		`scratch_dir = "/home/user/.awsbnkctl/default/state/scratch"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s\noutput:\n%s", want, out)
		}
	}

	buf.Reset()
	if err := RenderTFVars(&buf, ws, "", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"kubeconfig_dir", "scratch_dir"} {
		if strings.Contains(buf.String(), k) {
			t.Errorf("empty %s should not emit a line\noutput:\n%s", k, buf.String())
		}
	}
}

func TestRenderTFVars_EnableECRMirror(t *testing.T) {
	ws := &config.Workspace{
		AWS: config.AWSCfg{
			Region:      "us-west-2",
			SupplyChain: config.SupplyChainCfg{EnableECRMirror: true},
		},
		Cluster: config.ClusterCfg{Create: true, Name: "ecr"},
	}
	var buf bytes.Buffer
	if err := RenderTFVars(&buf, ws, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "enable_ecr_mirror = true") {
		t.Errorf("enable_ecr_mirror not rendered\noutput:\n%s", buf.String())
	}
}
