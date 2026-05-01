package authz

import (
	"context"

	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// GrantAssignment 为主体授予角色。
func (c *Client) GrantAssignment(ctx context.Context, req *authzv2.GrantAssignmentRequest) (*authzv2.GrantAssignmentResponse, error) {
	resp, err := c.authorizationService.GrantAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeAssignment 撤销主体上的角色。
func (c *Client) RevokeAssignment(ctx context.Context, req *authzv2.RevokeAssignmentRequest) (*authzv2.RevokeAssignmentResponse, error) {
	resp, err := c.authorizationService.RevokeAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
