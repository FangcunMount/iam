package authz

import (
	"context"

	authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// GrantAssignment 为主体授予角色。
func (c *Client) GrantAssignment(ctx context.Context, req *authzv1.GrantAssignmentRequest) (*authzv1.GrantAssignmentResponse, error) {
	resp, err := c.authorizationService.GrantAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeAssignment 撤销主体上的角色。
func (c *Client) RevokeAssignment(ctx context.Context, req *authzv1.RevokeAssignmentRequest) (*authzv1.RevokeAssignmentResponse, error) {
	resp, err := c.authorizationService.RevokeAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
