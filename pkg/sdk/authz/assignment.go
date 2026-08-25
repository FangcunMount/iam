package authz

import (
	"context"

	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	"github.com/FangcunMount/iam/v3/pkg/sdk/errors"
)

// GrantAssignment 为主体授予角色。
func (c *Client) GrantAssignment(ctx context.Context, req *authzv3.GrantAssignmentRequest) (*authzv3.GrantAssignmentResponse, error) {
	resp, err := c.authorizationService.GrantAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeAssignment 撤销主体上的角色。
func (c *Client) RevokeAssignment(ctx context.Context, req *authzv3.RevokeAssignmentRequest) (*authzv3.RevokeAssignmentResponse, error) {
	resp, err := c.authorizationService.RevokeAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
