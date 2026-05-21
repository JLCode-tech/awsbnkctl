package phases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// trustDoc is the typed Go struct for AssumeRole trust policy documents.
// Using a struct + json.Marshal avoids string templates and ensures valid JSON.
type trustDoc struct {
	Version   string `json:"Version"`
	Statement []struct {
		Effect    string            `json:"Effect"`
		Principal map[string]string `json:"Principal"`
		Action    string            `json:"Action"`
	} `json:"Statement"`
}

// tmmVpcRoutePolicy is the inline policy attached to the node role.
// Allows VPC route table management for self-managed node groups.
const tmmVpcRoutePolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:CreateRoute","ec2:DeleteRoute","ec2:ReplaceRoute","ec2:DescribeRouteTables","ec2:DescribeVpcs","ec2:DescribeSubnets","ec2:DescribeNetworkInterfaces","ec2:ModifyNetworkInterfaceAttribute"],"Resource":"*"}]}`

// Phase07IAM creates the EKS cluster service role, node instance role, and
// node instance profile. All resources are tagged and state ARNs are persisted.
//
// Idempotent: GetRole / GetInstanceProfile by name before creating.
// Dry-run: writes placeholder state values, makes zero IAM API mutations.
func Phase07IAM(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 07] iam: cluster=%s\n", name)

	clusterRoleName := name + "-eks-cluster-role"
	nodeRoleName := name + "-eks-node-role"
	profileName := name + "-node-instance-profile"

	if dryRun {
		fmt.Fprintf(os.Stderr, "[phase 07] dry-run: would create cluster role %s\n", clusterRoleName)
		fmt.Fprintf(os.Stderr, "[phase 07] dry-run: would create node role %s\n", nodeRoleName)
		fmt.Fprintf(os.Stderr, "[phase 07] dry-run: would create instance profile %s\n", profileName)
		st.Set("EKS_CLUSTER_ROLE_ARN", "arn:aws:iam::dry-run:role/"+clusterRoleName)
		st.Set("EKS_NODE_ROLE_ARN", "arn:aws:iam::dry-run:role/"+nodeRoleName)
		st.Set("NODE_INSTANCE_PROFILE_NAME", profileName)
		st.Set("NODE_INSTANCE_PROFILE_ARN", "arn:aws:iam::dry-run:instance-profile/"+profileName)
		return nil
	}

	// --- EKS cluster service role ---
	clusterRoleARN, err := ensureRole(ctx, clients.IAM, clusterRoleName, "eks.amazonaws.com",
		[]string{
			"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
			"arn:aws:iam::aws:policy/AmazonEKSVPCResourceController",
		},
		"",
		tags.IAMTags(
			tags.Required(name, tags.CompIAMClusterRole),
			cl.Tags,
			cl.Metadata.Labels,
		),
	)
	if err != nil {
		return fmt.Errorf("phase07: cluster role: %w", err)
	}
	st.Set("EKS_CLUSTER_ROLE_ARN", clusterRoleARN)

	// --- EKS node instance role ---
	nodeRoleARN, err := ensureRole(ctx, clients.IAM, nodeRoleName, "ec2.amazonaws.com",
		[]string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			// NOTE: service-role/ path prefix is required — the policy lives under
			// arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy, NOT
			// arn:aws:iam::aws:policy/AmazonEBSCSIDriverPolicy. Wrong path = not found.
			"arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy",
		},
		tmmVpcRoutePolicy,
		tags.IAMTags(
			tags.Required(name, tags.CompIAMNodeRole),
			cl.Tags,
			cl.Metadata.Labels,
		),
	)
	if err != nil {
		return fmt.Errorf("phase07: node role: %w", err)
	}
	st.Set("EKS_NODE_ROLE_ARN", nodeRoleARN)

	// --- Node instance profile ---
	profileARN, err := ensureInstanceProfile(ctx, clients.IAM, profileName, nodeRoleName,
		tags.IAMTags(
			tags.Required(name, tags.CompIAMNodeProfile),
			cl.Tags,
			cl.Metadata.Labels,
		),
	)
	if err != nil {
		return fmt.Errorf("phase07: instance profile: %w", err)
	}
	st.Set("NODE_INSTANCE_PROFILE_NAME", profileName)
	st.Set("NODE_INSTANCE_PROFILE_ARN", profileARN)

	return st.Save()
}

// Phase07IAMDown destroys IAM resources in reverse-create order.
// Each step tolerates NoSuchEntity (already gone).
// Destroy order: remove role from profile → delete profile → detach + delete
// inline policies on node role → delete node role → same for cluster role.
func Phase07IAMDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 07 down] iam: cluster=%s\n", name)

	// Name-based fallback — roles have well-known names.
	// TODO: tag-listing fallback (ListRoles + per-role ListRoleTags) if names
	// ever diverge from convention.
	clusterRoleName := name + "-eks-cluster-role"
	nodeRoleName := name + "-eks-node-role"
	profileName := st.Get("NODE_INSTANCE_PROFILE_NAME")
	if profileName == "" {
		profileName = name + "-node-instance-profile"
	}

	iamClient := clients.IAM

	// 1. Remove node role from instance profile.
	fmt.Fprintf(os.Stderr, "[phase 07 down] removing role from instance profile %s\n", profileName)
	_, err := iamClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: ptr(profileName),
		RoleName:            ptr(nodeRoleName),
	})
	if err != nil && !isNoSuchEntity(err) {
		return fmt.Errorf("phase07 down: RemoveRoleFromInstanceProfile: %w", err)
	}

	// 2. Delete instance profile.
	fmt.Fprintf(os.Stderr, "[phase 07 down] deleting instance profile %s\n", profileName)
	_, err = iamClient.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: ptr(profileName),
	})
	if err != nil && !isNoSuchEntity(err) {
		return fmt.Errorf("phase07 down: DeleteInstanceProfile: %w", err)
	}
	st.Set("NODE_INSTANCE_PROFILE_NAME", "")
	st.Set("NODE_INSTANCE_PROFILE_ARN", "")

	// 3–5. Tear down node role.
	if err := deleteRole(ctx, iamClient, nodeRoleName); err != nil {
		return fmt.Errorf("phase07 down: node role: %w", err)
	}
	st.Set("EKS_NODE_ROLE_ARN", "")

	// 6–8. Tear down cluster role.
	if err := deleteRole(ctx, iamClient, clusterRoleName); err != nil {
		return fmt.Errorf("phase07 down: cluster role: %w", err)
	}
	st.Set("EKS_CLUSTER_ROLE_ARN", "")

	return st.Save()
}

// --- helpers ---

// buildTrustPolicy serialises an AssumeRole trust policy for the given service
// principal (e.g. "eks.amazonaws.com").
func buildTrustPolicy(servicePrincipal string) (string, error) {
	doc := trustDoc{
		Version: "2012-10-17",
		Statement: []struct {
			Effect    string            `json:"Effect"`
			Principal map[string]string `json:"Principal"`
			Action    string            `json:"Action"`
		}{
			{
				Effect:    "Allow",
				Principal: map[string]string{"Service": servicePrincipal},
				Action:    "sts:AssumeRole",
			},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ensureRole creates an IAM role if it does not exist; returns the ARN.
// managedPolicies are attached (idempotent). inlinePolicy (if non-empty) is
// written as "TmmVpcRoute" via PutRolePolicy (always idempotent).
func ensureRole(
	ctx context.Context,
	iamClient IAMAPI,
	roleName, servicePrincipal string,
	managedPolicies []string,
	inlinePolicy string,
	iamTags []iamtypes.Tag,
) (string, error) {
	// Check existence by name.
	getOut, err := iamClient.GetRole(ctx, &iam.GetRoleInput{RoleName: ptr(roleName)})
	if err != nil && !isNoSuchEntity(err) {
		return "", fmt.Errorf("GetRole %s: %w", roleName, err)
	}

	var roleARN string
	if err == nil {
		// Role already exists.
		roleARN = *getOut.Role.Arn
		fmt.Fprintf(os.Stderr, "[phase 07] role %s already exists, skipping create\n", roleName)
	} else {
		// Create the role.
		trustPolicy, err := buildTrustPolicy(servicePrincipal)
		if err != nil {
			return "", fmt.Errorf("buildTrustPolicy: %w", err)
		}
		createOut, err := iamClient.CreateRole(ctx, &iam.CreateRoleInput{
			RoleName:                 ptr(roleName),
			AssumeRolePolicyDocument: ptr(trustPolicy),
			Tags:                     iamTags,
		})
		if err != nil {
			return "", fmt.Errorf("CreateRole %s: %w", roleName, err)
		}
		roleARN = *createOut.Role.Arn
		fmt.Fprintf(os.Stderr, "[phase 07] created role %s (%s)\n", roleName, roleARN)
	}

	// Attach managed policies (skip already-attached).
	attached, err := listAttachedPolicies(ctx, iamClient, roleName)
	if err != nil {
		return "", fmt.Errorf("ListAttachedRolePolicies %s: %w", roleName, err)
	}
	for _, policyARN := range managedPolicies {
		if attached[policyARN] {
			fmt.Fprintf(os.Stderr, "[phase 07] policy %s already attached to %s, skipping\n", policyARN, roleName)
			continue
		}
		if _, err := iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			RoleName:  ptr(roleName),
			PolicyArn: ptr(policyARN),
		}); err != nil {
			return "", fmt.Errorf("AttachRolePolicy %s → %s: %w", policyARN, roleName, err)
		}
		fmt.Fprintf(os.Stderr, "[phase 07] attached policy %s to %s\n", policyARN, roleName)
	}

	// Inline policy (PutRolePolicy is always idempotent — overwrites).
	if inlinePolicy != "" {
		if _, err := iamClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
			RoleName:       ptr(roleName),
			PolicyName:     ptr("TmmVpcRoute"),
			PolicyDocument: ptr(inlinePolicy),
		}); err != nil {
			return "", fmt.Errorf("PutRolePolicy TmmVpcRoute → %s: %w", roleName, err)
		}
		fmt.Fprintf(os.Stderr, "[phase 07] put inline policy TmmVpcRoute on %s\n", roleName)
	}

	return roleARN, nil
}

// ensureInstanceProfile creates a node instance profile if absent, adds the
// node role to it (idempotent via ListInstanceProfilesForRole check).
func ensureInstanceProfile(
	ctx context.Context,
	iamClient IAMAPI,
	profileName, nodeRoleName string,
	iamTags []iamtypes.Tag,
) (string, error) {
	getOut, err := iamClient.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: ptr(profileName),
	})
	if err != nil && !isNoSuchEntity(err) {
		return "", fmt.Errorf("GetInstanceProfile %s: %w", profileName, err)
	}

	var profileARN string
	if err == nil {
		profileARN = *getOut.InstanceProfile.Arn
		fmt.Fprintf(os.Stderr, "[phase 07] instance profile %s already exists, skipping create\n", profileName)
	} else {
		createOut, err := iamClient.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
			InstanceProfileName: ptr(profileName),
			Tags:                iamTags,
		})
		if err != nil {
			return "", fmt.Errorf("CreateInstanceProfile %s: %w", profileName, err)
		}
		profileARN = *createOut.InstanceProfile.Arn
		fmt.Fprintf(os.Stderr, "[phase 07] created instance profile %s (%s)\n", profileName, profileARN)
	}

	// Check whether the node role is already in this profile before adding.
	// AddRoleToInstanceProfile errors with LimitExceededException when a role is
	// already present (max 1 role per profile). Only add if absent.
	alreadyAttached, err := roleInProfile(ctx, iamClient, nodeRoleName)
	if err != nil {
		return "", fmt.Errorf("ListInstanceProfilesForRole %s: %w", nodeRoleName, err)
	}
	if !alreadyAttached {
		if _, err := iamClient.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
			InstanceProfileName: ptr(profileName),
			RoleName:            ptr(nodeRoleName),
		}); err != nil {
			return "", fmt.Errorf("AddRoleToInstanceProfile %s → %s: %w", nodeRoleName, profileName, err)
		}
		fmt.Fprintf(os.Stderr, "[phase 07] added role %s to instance profile %s\n", nodeRoleName, profileName)
	} else {
		fmt.Fprintf(os.Stderr, "[phase 07] role %s already in instance profile %s, skipping\n", nodeRoleName, profileName)
	}

	return profileARN, nil
}

// deleteRole detaches all managed policies, deletes all inline policies, then
// deletes the role. Tolerates NoSuchEntity at every step.
func deleteRole(ctx context.Context, iamClient IAMAPI, roleName string) error {
	// Detach managed policies.
	attached, err := listAttachedPolicies(ctx, iamClient, roleName)
	if err != nil && !isNoSuchEntity(err) {
		return fmt.Errorf("ListAttachedRolePolicies %s: %w", roleName, err)
	}
	for policyARN := range attached {
		policyARN := policyARN
		fmt.Fprintf(os.Stderr, "[phase 07 down] detaching policy %s from %s\n", policyARN, roleName)
		if _, err := iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
			RoleName:  ptr(roleName),
			PolicyArn: ptr(policyARN),
		}); err != nil && !isNoSuchEntity(err) {
			return fmt.Errorf("DetachRolePolicy %s: %w", policyARN, err)
		}
	}

	// Delete inline policies.
	inlineOut, err := iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: ptr(roleName)})
	if err != nil && !isNoSuchEntity(err) {
		return fmt.Errorf("ListRolePolicies %s: %w", roleName, err)
	}
	if err == nil {
		for _, policyName := range inlineOut.PolicyNames {
			policyName := policyName
			fmt.Fprintf(os.Stderr, "[phase 07 down] deleting inline policy %s from %s\n", policyName, roleName)
			if _, err := iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
				RoleName:   ptr(roleName),
				PolicyName: ptr(policyName),
			}); err != nil && !isNoSuchEntity(err) {
				return fmt.Errorf("DeleteRolePolicy %s: %w", policyName, err)
			}
		}
	}

	// Delete the role itself.
	fmt.Fprintf(os.Stderr, "[phase 07 down] deleting role %s\n", roleName)
	if _, err := iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: ptr(roleName)}); err != nil {
		if isNoSuchEntity(err) {
			fmt.Fprintf(os.Stderr, "[phase 07 down] role %s already gone\n", roleName)
			return nil
		}
		return fmt.Errorf("DeleteRole %s: %w", roleName, err)
	}
	return nil
}

// listAttachedPolicies returns the set of policy ARNs currently attached to a
// role, keyed by ARN for O(1) lookup.
func listAttachedPolicies(ctx context.Context, iamClient IAMAPI, roleName string) (map[string]bool, error) {
	out, err := iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: ptr(roleName),
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(out.AttachedPolicies))
	for _, p := range out.AttachedPolicies {
		result[*p.PolicyArn] = true
	}
	return result, nil
}

// roleInProfile returns true if the given role name appears in any instance
// profile listed by ListInstanceProfilesForRole.
func roleInProfile(ctx context.Context, iamClient IAMAPI, roleName string) (bool, error) {
	out, err := iamClient.ListInstanceProfilesForRole(ctx, &iam.ListInstanceProfilesForRoleInput{
		RoleName: ptr(roleName),
	})
	if err != nil {
		if isNoSuchEntity(err) {
			return false, nil
		}
		return false, err
	}
	return len(out.InstanceProfiles) > 0, nil
}

// isNoSuchEntity returns true when err is an IAM NoSuchEntityException.
// Uses errors.As for robustness across SDK versions.
func isNoSuchEntity(err error) bool {
	if err == nil {
		return false
	}
	var nse *iamtypes.NoSuchEntityException
	return errors.As(err, &nse)
}
