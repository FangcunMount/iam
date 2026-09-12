package authz

import (
	"context"

	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	"github.com/FangcunMount/iam/v5/pkg/sdk/errors"
)

// GrantAssignment 为主体授予角色。
func (c *Client) GrantAssignment(ctx context.Context, req *authzv4.GrantAssignmentRequest) (*authzv4.GrantAssignmentResponse, error) {
	resp, err := c.authorizationService.GrantAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeAssignment 撤销主体上的角色。
func (c *Client) RevokeAssignment(ctx context.Context, req *authzv4.RevokeAssignmentRequest) (*authzv4.RevokeAssignmentResponse, error) {
	resp, err := c.authorizationService.RevokeAssignment(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ReplaceManagedAssignments atomically replaces only the role set delegated to
// the authenticated calling service. Assignments owned by other services are preserved.
func (c *Client) ReplaceManagedAssignments(ctx context.Context, req *authzv4.ReplaceManagedAssignmentsRequest) (*authzv4.ReplaceManagedAssignmentsResponse, error) {
	resp, err := c.authorizationService.ReplaceManagedAssignments(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ReplaceScopedAssignments changes only the caller-managed roles in one company.
// It deliberately uses a distinct RPC: an older server must reject the request.
func (c *Client) ReplaceScopedAssignments(ctx context.Context, req *authzv4.ReplaceScopedAssignmentsRequest) (*authzv4.ReplaceManagedAssignmentsResponse, error) {
	response, err := c.authorizationService.ReplaceScopedAssignments(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return response, nil
}
