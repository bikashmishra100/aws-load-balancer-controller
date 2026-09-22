package elbv2

import (
	"context"
	awsarn "github.com/aws/aws-sdk-go-v2/aws/arn"
	elbv2sdk "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/pkg/errors"
	elbv2api "sigs.k8s.io/aws-load-balancer-controller/v3/apis/elbv2/v1beta1"
	"sigs.k8s.io/aws-load-balancer-controller/v3/pkg/aws/services"
)

// getTargetGroupFromAWS returns the AWS target group corresponding to the arn
func getTargetGroupFromAWS(ctx context.Context, elbv2Client services.ELBV2, tgb *elbv2api.TargetGroupBinding) (*elbv2types.TargetGroup, error) {
	tgARN := tgb.Spec.TargetGroupARN
	req := &elbv2sdk.DescribeTargetGroupsInput{
		TargetGroupArns: []string{tgARN},
	}
	return getTargetGroupHelper(ctx, elbv2Client, tgb, tgARN, req)
}

// getTargetGroupsByNameFromAWS returns the AWS target group corresponding to the name
func getTargetGroupsByNameFromAWS(ctx context.Context, elbv2Client services.ELBV2, tgb *elbv2api.TargetGroupBinding) (*elbv2types.TargetGroup, error) {
	req := &elbv2sdk.DescribeTargetGroupsInput{
		Names: []string{tgb.Spec.TargetGroupName},
	}

	return getTargetGroupHelper(ctx, elbv2Client, tgb, tgb.Spec.TargetGroupName, req)
}

func getTargetGroupHelper(ctx context.Context, elbv2Client services.ELBV2, tgb *elbv2api.TargetGroupBinding, tgIdentifier string, req *elbv2sdk.DescribeTargetGroupsInput) (*elbv2types.TargetGroup, error) {
	ctx = services.WithRegion(ctx, resolveTGBRegion(tgb))
	clientToUse, err := elbv2Client.AssumeRole(ctx, tgb.Spec.IamRoleArnToAssume, tgb.Spec.AssumeRoleExternalId)

	if err != nil {
		return nil, err
	}

	// Use non-paginated call: name/ARN queries return at most one result.
	resp, err := clientToUse.DescribeTargetGroupsWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.TargetGroups) != 1 {
		return nil, errors.Errorf("expecting a single targetGroup with query [%s] but got %v", tgIdentifier, len(resp.TargetGroups))
	}
	return &resp.TargetGroups[0], nil
}

// validateRegionARNConsistency returns an error if spec.region and the region
// embedded in the ARN are both set but disagree with each other.
func validateRegionARNConsistency(tgb *elbv2api.TargetGroupBinding) error {
	if tgb.Spec.Region == "" || tgb.Spec.TargetGroupARN == "" {
		return nil
	}
	if parsed, err := awsarn.Parse(tgb.Spec.TargetGroupARN); err == nil && parsed.Region != "" {
		if parsed.Region != tgb.Spec.Region {
			return errors.Errorf("spec.region %q does not match the region in targetGroupARN %q", tgb.Spec.Region, tgb.Spec.TargetGroupARN)
		}
	}
	return nil
}

// resolveTGBRegion returns the effective AWS region for a TGB's target group.
// Priority: spec.region > region parsed from ARN > controller region.
func resolveTGBRegion(tgb *elbv2api.TargetGroupBinding) string {
	if tgb.Spec.Region != "" {
		return tgb.Spec.Region
	}
	if tgb.Spec.TargetGroupARN != "" {
		if parsed, err := awsarn.Parse(tgb.Spec.TargetGroupARN); err == nil && parsed.Region != "" {
			return parsed.Region
		}
	}
	return ""
}
