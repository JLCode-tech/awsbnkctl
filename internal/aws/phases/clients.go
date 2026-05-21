// Package phases implements the imperative phased provisioning model for
// awsbnkctl's post-Terraform direction.
//
// Each phase is a top-level function with a consistent signature. Phases are
// called in order by the up/down orchestrators in internal/cli. Phase 01 is
// reserved for IAM (slice 2). Network phases are numbered 02–06.
//
// See docs/POST_TERRAFORM_DIRECTION.md §6–§7 for the full ordering spec.
package phases

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithymw "github.com/aws/smithy-go/middleware"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
)

// EC2API is the subset of ec2.Client surface used by the phase functions.
// Tests inject a fake implementation.
type EC2API interface {
	// VPC
	DescribeVpcs(ctx context.Context, in *ec2.DescribeVpcsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	CreateVpc(ctx context.Context, in *ec2.CreateVpcInput, opts ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error)
	ModifyVpcAttribute(ctx context.Context, in *ec2.ModifyVpcAttributeInput, opts ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error)
	DeleteVpc(ctx context.Context, in *ec2.DeleteVpcInput, opts ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error)

	// Subnets
	DescribeSubnets(ctx context.Context, in *ec2.DescribeSubnetsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	CreateSubnet(ctx context.Context, in *ec2.CreateSubnetInput, opts ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error)
	ModifySubnetAttribute(ctx context.Context, in *ec2.ModifySubnetAttributeInput, opts ...func(*ec2.Options)) (*ec2.ModifySubnetAttributeOutput, error)
	DeleteSubnet(ctx context.Context, in *ec2.DeleteSubnetInput, opts ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error)

	// IGW
	DescribeInternetGateways(ctx context.Context, in *ec2.DescribeInternetGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
	CreateInternetGateway(ctx context.Context, in *ec2.CreateInternetGatewayInput, opts ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error)
	AttachInternetGateway(ctx context.Context, in *ec2.AttachInternetGatewayInput, opts ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error)
	DetachInternetGateway(ctx context.Context, in *ec2.DetachInternetGatewayInput, opts ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error)
	DeleteInternetGateway(ctx context.Context, in *ec2.DeleteInternetGatewayInput, opts ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error)

	// NAT / EIP
	DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	CreateNatGateway(ctx context.Context, in *ec2.CreateNatGatewayInput, opts ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error)
	DeleteNatGateway(ctx context.Context, in *ec2.DeleteNatGatewayInput, opts ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error)
	DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	AllocateAddress(ctx context.Context, in *ec2.AllocateAddressInput, opts ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error)
	ReleaseAddress(ctx context.Context, in *ec2.ReleaseAddressInput, opts ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error)
	CreateTags(ctx context.Context, in *ec2.CreateTagsInput, opts ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)

	// Route tables
	DescribeRouteTables(ctx context.Context, in *ec2.DescribeRouteTablesInput, opts ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	CreateRouteTable(ctx context.Context, in *ec2.CreateRouteTableInput, opts ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error)
	CreateRoute(ctx context.Context, in *ec2.CreateRouteInput, opts ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	AssociateRouteTable(ctx context.Context, in *ec2.AssociateRouteTableInput, opts ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error)
	DeleteRouteTable(ctx context.Context, in *ec2.DeleteRouteTableInput, opts ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error)
	DisassociateRouteTable(ctx context.Context, in *ec2.DisassociateRouteTableInput, opts ...func(*ec2.Options)) (*ec2.DisassociateRouteTableOutput, error)

	// STS-like: needed by preflight (caller identity check in phases package)
	DescribeAvailabilityZones(ctx context.Context, in *ec2.DescribeAvailabilityZonesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
}

// STSAPI is the subset of sts.Client used by the preflight phase.
type STSAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// Clients bundles the AWS service clients needed by phases in this slice.
// Later slices add EKS/IAM/S3 fields here without changing existing phases.
type Clients struct {
	EC2     EC2API
	STS     STSAPI
	Profile string // the AWS profile used — passed to CheckAuthOrDie hints
}

// NewClients constructs real AWS SDK clients wrapped with the SSO sentinel
// middleware. Region and Profile are read from the cluster intent by the
// caller — this constructor is the single place the middleware is applied.
func NewClients(ctx context.Context, region, profile string) (*Clients, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithAPIOptions([]func(*smithymw.Stack) error{
			awsmw.WithSSOWatch,
		}),
	}
	if region != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}
	if profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS region is empty; set metadata.region in cluster.yaml or AWS_REGION")
	}

	return &Clients{
		EC2:     ec2.NewFromConfig(cfg),
		STS:     sts.NewFromConfig(cfg),
		Profile: profile,
	}, nil
}

// ptr returns a pointer to a string — avoids aws.String import at every call
// site within the phases package.
func ptr(s string) *string { return &s }

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool { return &b }

// isNotFoundCode reports whether the smithy error code is one of the EC2
// "already gone" codes that down phases should swallow. See spec §7.
func isNotFoundCode(code string) bool {
	switch code {
	case "InvalidVpcID.NotFound",
		"InvalidSubnetID.NotFound",
		"InvalidRouteTableID.NotFound",
		"InvalidInternetGatewayID.NotFound",
		"InvalidNatGatewayID.NotFound",
		"InvalidAllocationID.NotFound",
		"InvalidNetworkInterfaceID.NotFound":
		return true
	}
	return false
}

// ignoreNotFound swallows EC2 "already gone" errors on destroy. Returns nil
// when the error is a known not-found code; otherwise returns err unchanged.
func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	// Extract the smithy APIError code.
	type coder interface{ ErrorCode() string }
	var c coder
	// Walk the error chain.
	e := err
	for e != nil {
		if ce, ok := e.(coder); ok {
			c = ce
			break
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	if c != nil && isNotFoundCode(c.ErrorCode()) {
		return nil
	}
	return err
}

// tagSpecification builds the EC2 TagSpecification for resource creation.
func tagSpecification(resourceType ec2types.ResourceType, tags []ec2types.Tag) ec2types.TagSpecification {
	return ec2types.TagSpecification{
		ResourceType: resourceType,
		Tags:         tags,
	}
}
