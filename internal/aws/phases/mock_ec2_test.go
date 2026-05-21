package phases

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithy "github.com/aws/smithy-go"
)

// mockEC2 is the shared test double for EC2API.
// Each method has a configurable result; call counts support idempotency assertions.
type mockEC2 struct {
	// VPC
	describeVpcsOut *ec2.DescribeVpcsOutput
	describeVpcsErr error
	createVpcOut    *ec2.CreateVpcOutput
	createVpcErr    error
	createVpcCalls  int

	// Subnet
	describeSubnetsOut *ec2.DescribeSubnetsOutput
	describeSubnetsErr error
	createSubnetOut    *ec2.CreateSubnetOutput
	createSubnetErr    error
	createSubnetCalls  int

	// IGW
	describeIGWsOut *ec2.DescribeInternetGatewaysOutput
	describeIGWsErr error
	createIGWOut    *ec2.CreateInternetGatewayOutput
	createIGWErr    error
	createIGWCalls  int
	attachIGWCalls  int

	// NAT
	describeNATsOut *ec2.DescribeNatGatewaysOutput
	describeNATsErr error
	createNATOut    *ec2.CreateNatGatewayOutput
	createNATErr    error
	createNATCalls  int

	// EIP
	describeAddrsOut *ec2.DescribeAddressesOutput
	describeAddrsErr error
	allocAddrOut     *ec2.AllocateAddressOutput
	allocAddrErr     error
	allocAddrCalls   int
	releaseAddrCalls int

	// Route table
	describeRTBsOut  *ec2.DescribeRouteTablesOutput
	describeRTBsErr  error
	createRTBOut     *ec2.CreateRouteTableOutput
	createRTBErr     error
	createRTBCalls   int
	createRouteCalls int
	assocRTBCalls    int
	disassocRTBCalls int
}

func (m *mockEC2) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if m.describeVpcsOut == nil {
		return &ec2.DescribeVpcsOutput{}, m.describeVpcsErr
	}
	return m.describeVpcsOut, m.describeVpcsErr
}
func (m *mockEC2) CreateVpc(_ context.Context, _ *ec2.CreateVpcInput, _ ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	m.createVpcCalls++
	return m.createVpcOut, m.createVpcErr
}
func (m *mockEC2) ModifyVpcAttribute(_ context.Context, _ *ec2.ModifyVpcAttributeInput, _ ...func(*ec2.Options)) (*ec2.ModifyVpcAttributeOutput, error) {
	return &ec2.ModifyVpcAttributeOutput{}, nil
}
func (m *mockEC2) DeleteVpc(_ context.Context, _ *ec2.DeleteVpcInput, _ ...func(*ec2.Options)) (*ec2.DeleteVpcOutput, error) {
	return &ec2.DeleteVpcOutput{}, nil
}

func (m *mockEC2) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if m.describeSubnetsOut == nil {
		return &ec2.DescribeSubnetsOutput{}, m.describeSubnetsErr
	}
	return m.describeSubnetsOut, m.describeSubnetsErr
}
func (m *mockEC2) CreateSubnet(_ context.Context, _ *ec2.CreateSubnetInput, _ ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	m.createSubnetCalls++
	return m.createSubnetOut, m.createSubnetErr
}
func (m *mockEC2) ModifySubnetAttribute(_ context.Context, _ *ec2.ModifySubnetAttributeInput, _ ...func(*ec2.Options)) (*ec2.ModifySubnetAttributeOutput, error) {
	return &ec2.ModifySubnetAttributeOutput{}, nil
}
func (m *mockEC2) DeleteSubnet(_ context.Context, _ *ec2.DeleteSubnetInput, _ ...func(*ec2.Options)) (*ec2.DeleteSubnetOutput, error) {
	return &ec2.DeleteSubnetOutput{}, nil
}

func (m *mockEC2) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if m.describeIGWsOut == nil {
		return &ec2.DescribeInternetGatewaysOutput{}, m.describeIGWsErr
	}
	return m.describeIGWsOut, m.describeIGWsErr
}
func (m *mockEC2) CreateInternetGateway(_ context.Context, _ *ec2.CreateInternetGatewayInput, _ ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	m.createIGWCalls++
	return m.createIGWOut, m.createIGWErr
}
func (m *mockEC2) AttachInternetGateway(_ context.Context, _ *ec2.AttachInternetGatewayInput, _ ...func(*ec2.Options)) (*ec2.AttachInternetGatewayOutput, error) {
	m.attachIGWCalls++
	return &ec2.AttachInternetGatewayOutput{}, nil
}
func (m *mockEC2) DetachInternetGateway(_ context.Context, _ *ec2.DetachInternetGatewayInput, _ ...func(*ec2.Options)) (*ec2.DetachInternetGatewayOutput, error) {
	return &ec2.DetachInternetGatewayOutput{}, nil
}
func (m *mockEC2) DeleteInternetGateway(_ context.Context, _ *ec2.DeleteInternetGatewayInput, _ ...func(*ec2.Options)) (*ec2.DeleteInternetGatewayOutput, error) {
	return &ec2.DeleteInternetGatewayOutput{}, nil
}

func (m *mockEC2) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if m.describeNATsOut == nil {
		return &ec2.DescribeNatGatewaysOutput{}, m.describeNATsErr
	}
	return m.describeNATsOut, m.describeNATsErr
}
func (m *mockEC2) CreateNatGateway(_ context.Context, _ *ec2.CreateNatGatewayInput, _ ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
	m.createNATCalls++
	return m.createNATOut, m.createNATErr
}
func (m *mockEC2) DeleteNatGateway(_ context.Context, _ *ec2.DeleteNatGatewayInput, _ ...func(*ec2.Options)) (*ec2.DeleteNatGatewayOutput, error) {
	return &ec2.DeleteNatGatewayOutput{}, nil
}

func (m *mockEC2) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	if m.describeAddrsOut == nil {
		return &ec2.DescribeAddressesOutput{}, m.describeAddrsErr
	}
	return m.describeAddrsOut, m.describeAddrsErr
}
func (m *mockEC2) AllocateAddress(_ context.Context, _ *ec2.AllocateAddressInput, _ ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error) {
	m.allocAddrCalls++
	return m.allocAddrOut, m.allocAddrErr
}
func (m *mockEC2) ReleaseAddress(_ context.Context, _ *ec2.ReleaseAddressInput, _ ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
	m.releaseAddrCalls++
	return &ec2.ReleaseAddressOutput{}, nil
}
func (m *mockEC2) CreateTags(_ context.Context, _ *ec2.CreateTagsInput, _ ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return &ec2.CreateTagsOutput{}, nil
}

func (m *mockEC2) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	if m.describeRTBsOut == nil {
		return &ec2.DescribeRouteTablesOutput{}, m.describeRTBsErr
	}
	return m.describeRTBsOut, m.describeRTBsErr
}
func (m *mockEC2) CreateRouteTable(_ context.Context, _ *ec2.CreateRouteTableInput, _ ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	m.createRTBCalls++
	return m.createRTBOut, m.createRTBErr
}
func (m *mockEC2) CreateRoute(_ context.Context, _ *ec2.CreateRouteInput, _ ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	m.createRouteCalls++
	return &ec2.CreateRouteOutput{}, nil
}
func (m *mockEC2) AssociateRouteTable(_ context.Context, _ *ec2.AssociateRouteTableInput, _ ...func(*ec2.Options)) (*ec2.AssociateRouteTableOutput, error) {
	m.assocRTBCalls++
	assocID := "rtbassoc-mock"
	return &ec2.AssociateRouteTableOutput{AssociationId: &assocID}, nil
}
func (m *mockEC2) DeleteRouteTable(_ context.Context, _ *ec2.DeleteRouteTableInput, _ ...func(*ec2.Options)) (*ec2.DeleteRouteTableOutput, error) {
	return &ec2.DeleteRouteTableOutput{}, nil
}
func (m *mockEC2) DisassociateRouteTable(_ context.Context, _ *ec2.DisassociateRouteTableInput, _ ...func(*ec2.Options)) (*ec2.DisassociateRouteTableOutput, error) {
	m.disassocRTBCalls++
	return &ec2.DisassociateRouteTableOutput{}, nil
}
func (m *mockEC2) DescribeAvailabilityZones(_ context.Context, _ *ec2.DescribeAvailabilityZonesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return &ec2.DescribeAvailabilityZonesOutput{}, nil
}

// mockSTSImpl implements STSAPI for tests.
type mockSTSImpl struct {
	accountID string
	err       error
}

func (m *mockSTSImpl) GetCallerIdentity(_ context.Context, _ *sts.GetCallerIdentityInput, _ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &sts.GetCallerIdentityOutput{Account: &m.accountID}, m.err
}

// notFoundAPIError implements smithy.APIError with a configurable error code.
type notFoundAPIError struct{ code string }

func (e *notFoundAPIError) Error() string                 { return fmt.Sprintf("api error %s", e.code) }
func (e *notFoundAPIError) ErrorCode() string             { return e.code }
func (e *notFoundAPIError) ErrorMessage() string          { return e.code }
func (e *notFoundAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
