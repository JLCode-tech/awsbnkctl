package phases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// HydrateMissingStateFromAWS populates empty state keys via AWS tag discovery
// and synthesizes a temporary kubeconfig when the EKS cluster is still reachable
// so `awsbnkctl down` works cleanly even when `.awsbnkctl/<cluster>/state.env`
// is missing or incomplete (stateless teardown).
func HydrateMissingStateFromAWS(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) {
	if clients == nil || clients.EC2 == nil {
		return
	}
	name := cl.Metadata.Name
	clusterFilter := tags.ClusterFilter(name)

	// 1. VPC
	if st.Get("VPC_ID") == "" {
		if out, err := clients.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []ec2types.Filter{clusterFilter},
		}); err == nil && len(out.Vpcs) > 0 && out.Vpcs[0].VpcId != nil {
			st.Set("VPC_ID", *out.Vpcs[0].VpcId)
			fmt.Fprintf(os.Stderr, "[down hydrate] discovered VPC_ID=%s via tags\n", *out.Vpcs[0].VpcId)
		}
	}
	vpcID := st.Get("VPC_ID")

	// 2. Subnets (hydrate any missing subnet keys — including BNK_EXT_SUBNET and
	// BNK_INT_SUBNET when PUBLIC_SUBNETS/PRIVATE_SUBNETS were already populated).
	if st.Get("PUBLIC_SUBNETS") == "" || st.Get("PRIVATE_SUBNETS") == "" ||
		st.Get("BNK_EXT_SUBNET") == "" || st.Get("BNK_INT_SUBNET") == "" {
		filters := []ec2types.Filter{clusterFilter}
		if vpcID != "" {
			filters = append(filters, ec2types.Filter{Name: ptr("vpc-id"), Values: []string{vpcID}})
		}
		if out, err := clients.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{Filters: filters}); err == nil {
			var pub, priv []string
			for _, sn := range out.Subnets {
				if sn.SubnetId == nil {
					continue
				}
				sid := *sn.SubnetId
				var role, comp, sname string
				for _, t := range sn.Tags {
					if t.Key == nil || t.Value == nil {
						continue
					}
					switch *t.Key {
					case "bnk:subnet-role", "f5-bnk:subnet-role":
						role = *t.Value
					case tags.KeyComponent:
						comp = *t.Value
					case "Name":
						sname = *t.Value
					}
				}
				switch {
				case comp == tags.CompSubnetBNKExt || role == "bnk-external" || strings.HasSuffix(sname, "-bnk-ext"):
					if st.Get("BNK_EXT_SUBNET") == "" {
						st.Set("BNK_EXT_SUBNET", sid)
					}
				case comp == tags.CompSubnetBNKInt || role == "bnk-internal" || strings.HasSuffix(sname, "-bnk-int"):
					if st.Get("BNK_INT_SUBNET") == "" {
						st.Set("BNK_INT_SUBNET", sid)
					}
				case comp == tags.CompSubnetPublic || strings.Contains(sname, "public"):
					pub = append(pub, sid)
				case comp == tags.CompSubnetPrivate || strings.Contains(sname, "private"):
					priv = append(priv, sid)
				}
			}
			if st.Get("PUBLIC_SUBNETS") == "" && len(pub) > 0 {
				st.Set("PUBLIC_SUBNETS", strings.Join(pub, ","))
			}
			if st.Get("PRIVATE_SUBNETS") == "" && len(priv) > 0 {
				st.Set("PRIVATE_SUBNETS", strings.Join(priv, ","))
			}
		}
	}

	// 3. IGW
	if st.Get("IGW_ID") == "" {
		if out, err := clients.EC2.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
			Filters: []ec2types.Filter{clusterFilter},
		}); err == nil && len(out.InternetGateways) > 0 && out.InternetGateways[0].InternetGatewayId != nil {
			st.Set("IGW_ID", *out.InternetGateways[0].InternetGatewayId)
		}
	}

	// 4. NAT GW + EIP
	if st.Get("NAT_GW_ID") == "" {
		if out, err := clients.EC2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			Filter: []ec2types.Filter{clusterFilter},
		}); err == nil {
			for _, ng := range out.NatGateways {
				if ng.State != ec2types.NatGatewayStateDeleted && ng.NatGatewayId != nil {
					st.Set("NAT_GW_ID", *ng.NatGatewayId)
					if len(ng.NatGatewayAddresses) > 0 && ng.NatGatewayAddresses[0].AllocationId != nil && st.Get("NAT_EIP_ALLOC") == "" {
						st.Set("NAT_EIP_ALLOC", *ng.NatGatewayAddresses[0].AllocationId)
					}
					break
				}
			}
		}
	}

	// 5. Synthesize kubeconfig if missing on disk and EKS cluster is still active.
	kubePath := st.Get("KUBECONFIG_PATH")
	if (kubePath == "" || fileMissing(kubePath)) && clients.EKS != nil {
		if out, err := clients.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: ptr(name)}); err == nil &&
			out.Cluster != nil && out.Cluster.Arn != nil && out.Cluster.Endpoint != nil &&
			out.Cluster.CertificateAuthority != nil && out.Cluster.CertificateAuthority.Data != nil {
			target := filepath.Join(cl.StateDir(), "kubeconfig")
			_ = os.MkdirAll(cl.StateDir(), 0o750)
			if wErr := writeKubeconfig(target, kubeconfigData{
				ClusterARN:  *out.Cluster.Arn,
				Endpoint:    *out.Cluster.Endpoint,
				CA:          *out.Cluster.CertificateAuthority.Data,
				Region:      cl.Metadata.Region,
				ClusterName: name,
			}); wErr == nil {
				st.Set("KUBECONFIG_PATH", target)
				st.Set("EKS_CLUSTER_NAME", name)
				fmt.Fprintf(os.Stderr, "[down hydrate] synthesized kubeconfig from live EKS cluster %s\n", name)
			}
		}
	}

	_ = st.Save()
}

func fileMissing(p string) bool {
	if p == "" {
		return true
	}
	_, err := os.Stat(p)
	return err != nil
}

// CleanupBNKOverlayCRs deletes BNK custom resources (F5SPKVlan, L4Route,
// Gateway, GatewayClass, CNEInstance, F5BigCneLicense) and strips lingering
// finalizers while the FLO/CNE operator webhooks and controllers are still
// running, preventing namespace termination hangs during teardown.
func CleanupBNKOverlayCRs(ctx context.Context, clients *Clients) {
	if clients == nil || clients.Dynamic == nil {
		return
	}
	gvrs := []schema.GroupVersionResource{
		{Group: "k8s.f5net.com", Version: "v1", Resource: "f5spkvlan"},
		{Group: "k8s.f5net.com", Version: "v1", Resource: "f5spkvlans"},
		{Group: "gateway.networking.k8s.io", Version: "v1alpha2", Resource: "l4routes"},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"},
		{Group: "k8s.f5.com", Version: "v1", Resource: "cneinstances"},
		{Group: "k8s.f5.com", Version: "v1", Resource: "f5bigcnelicenses"},
	}

	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	for _, gvr := range gvrs {
		list, err := clients.Dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).List(cctx, metav1.ListOptions{})
		if err != nil || list == nil {
			continue
		}
		for _, item := range list.Items {
			ns := item.GetNamespace()
			name := item.GetName()
			var res interface {
				Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
				Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (meant interface{}, err error)
			}
			_ = res
			if ns != "" {
				_ = clients.Dynamic.Resource(gvr).Namespace(ns).Delete(cctx, name, metav1.DeleteOptions{})
				if len(item.GetFinalizers()) > 0 {
					_, _ = clients.Dynamic.Resource(gvr).Namespace(ns).Patch(
						cctx, name, types.MergePatchType,
						[]byte(`{"metadata":{"finalizers":null}}`),
						metav1.PatchOptions{},
					)
				}
			} else {
				_ = clients.Dynamic.Resource(gvr).Delete(cctx, name, metav1.DeleteOptions{})
				if len(item.GetFinalizers()) > 0 {
					_, _ = clients.Dynamic.Resource(gvr).Patch(
						cctx, name, types.MergePatchType,
						[]byte(`{"metadata":{"finalizers":null}}`),
						metav1.PatchOptions{},
					)
				}
			}
		}
	}
}

// CleanupBNKPVCsAndServices deletes LoadBalancer Services and PVCs in BNK and
// workload namespaces before the EBS CSI driver or AWS Load Balancer Controller
// are removed, preventing orphaned EBS volumes and NLBs/ENIs that would block
// subnet and VPC deletion.
func CleanupBNKPVCsAndServices(ctx context.Context, clients *Clients) {
	if clients == nil || clients.K8s == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// 1. Delete all LoadBalancer Services across non-system namespaces so cloud
	// controllers release NLBs/Classic ELBs before nodegroup teardown.
	if svcs, err := clients.K8s.CoreV1().Services(metav1.NamespaceAll).List(cctx, metav1.ListOptions{}); err == nil {
		for _, svc := range svcs.Items {
			if svc.Namespace == "kube-system" || svc.Namespace == "default" {
				continue
			}
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				fmt.Fprintf(os.Stderr, "[down cleanup] deleting LoadBalancer Service %s/%s\n", svc.Namespace, svc.Name)
				_ = clients.K8s.CoreV1().Services(svc.Namespace).Delete(cctx, svc.Name, metav1.DeleteOptions{})
			}
		}
	}

	// 2. Delete PVCs in f5-bnk / f5-utils / default workload namespaces while
	// aws-ebs-csi-driver is still active.
	for _, ns := range []string{"f5-bnk", "f5-utils", "default"} {
		pvcs, err := clients.K8s.CoreV1().PersistentVolumeClaims(ns).List(cctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, pvc := range pvcs.Items {
			fmt.Fprintf(os.Stderr, "[down cleanup] deleting PVC %s/%s\n", ns, pvc.Name)
			_ = clients.K8s.CoreV1().PersistentVolumeClaims(ns).Delete(cctx, pvc.Name, metav1.DeleteOptions{})
		}
	}
}
